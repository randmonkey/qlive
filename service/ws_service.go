// Copyright 2020 Qiniu Cloud (qiniu.com)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package service

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/qiniu/x/xlog"
	"github.com/someonegg/msgpump"
	"go.uber.org/atomic"

	"github.com/qrtc/qlive/config"
	"github.com/qrtc/qlive/controller"
	"github.com/qrtc/qlive/errors"
	"github.com/qrtc/qlive/protocol"
	"github.com/qrtc/qlive/service/websocket"
)

type PMessage interface {
	Marshal() ([]byte, error)
}

type WSClient struct {
	s               *WSServer
	p               *msgpump.Pump
	st              time.Time
	xl              *xlog.Logger
	playerID        string
	online          *atomic.Bool
	authorizeChan   chan struct{}
	disconnectChan  chan struct{}
	remoteAddr      string
	remotePort      string
	lastMessageTime time.Time
}

const (
	defaultPKRequestTimeoutSecond = 10
)

// Start start websocket client. Implement for github.com/qrtc/qlive/service/websocket Client
func (c *WSClient) Start(p *msgpump.Pump) {
	c.p = p
	c.p.Start(c.s.QuitCtx())
	go c.monitor()
}

// Process listen all requests. Implement for github.com/qrtc/qlive/service/websocket Client
func (c *WSClient) Process(ctx context.Context, t string, m msgpump.Message) {
	go func(ctx context.Context, t string, m msgpump.Message) {
		c.parallelProcess(ctx, t, m)
	}(ctx, t, m)
}

// IsOnline get client online status.
func (c *WSClient) IsOnline() bool {
	return c.online.Load()
}

// Close stop client msgpump.
func (c *WSClient) Close() error {
	c.online.Store(false)
	if c.p != nil {
		c.p.Stop()
	}
	return nil
}

// StartTime get client start time.
func (c *WSClient) StartTime() time.Time {
	return c.st
}

// Notify write a notify to client.
func (c *WSClient) Notify(t string, v PMessage) {
	m, err := v.Marshal()
	if err != nil {
		return
	}

	if t != protocol.MT_Ping && t != protocol.MT_Pong {
		c.xl.Infof("message to %v at %s:%s, %v=%v", c.playerID, c.remoteAddr, c.remotePort, t, string(m))
	}

	if ok := c.p.TryOutput(t, m); !ok {
		c.xl.Errorf("TryOutput failed %v", c.playerID)
	}

}

func (c *WSClient) monitor() {
	c.xl.Infof("%v:%v connected.", c.remoteAddr, c.remotePort)

	select {
	case <-c.p.StopD():
		c.xl.Infof("%v:%v disconnected.", c.remoteAddr, c.remotePort)
	case <-time.After(time.Millisecond * time.Duration(c.s.conf.WsConf.AuthorizeTimeoutMS)):
		c.xl.Infof("%v:%v authentication failure", c.remoteAddr, c.remotePort)
		c.Close()
		c.xl.Infof("%v:%v disconnected.", c.remoteAddr, c.remotePort)
	case <-c.authorizeChan:
		c.xl.Infof("%v:%v authorized successful as %v", c.remoteAddr, c.remotePort, c.playerID)
		c.s.AddPlayer(c.playerID, c)
		c.online.Store(true)
		ping := &protocol.Ping{}
		c.Notify(protocol.MT_Ping, ping)
		for {
			select {
			case <-time.After(time.Second * time.Duration(c.s.conf.WsConf.PingTickerSecond)):
				c.xl.Debugf("ping %s at %s:%s", c.playerID, c.remoteAddr, c.remotePort)
				c.Notify(protocol.MT_Ping, ping)
				if time.Now().Sub(c.lastMessageTime) > time.Second*time.Duration(c.s.conf.WsConf.PongTimeoutSecond) {
					c.xl.Infof("%v pingpong timeout", c.playerID)
					c.Close()
					c.s.RemovePlayer(c.playerID)
					c.xl.Infof("%v:%v %v disconnected.", c.remoteAddr, c.remotePort, c.playerID)
					return
				}
			case <-c.disconnectChan:
				c.Close()
				c.xl.Infof("%v:%v connection closed.", c.remoteAddr, c.remotePort)
				return
			}
		}
	}
}

func (c *WSClient) parallelProcess(ctx context.Context, t string, m msgpump.Message) {
	defer c.recover()

	if t != protocol.MT_Ping && t != protocol.MT_Pong {
		c.xl.Infof("message from %v at %s:%s, %v=%v", c.playerID, c.remoteAddr, c.remotePort, t, string(m))
	}

	if !c.IsOnline() && t != protocol.MT_AuthorizeRequest {
		return
	}

	c.lastMessageTime = time.Now()

	switch t {
	case protocol.MT_Ping:
		c.Notify(protocol.MT_Pong, &protocol.Pong{})
	case protocol.MT_Pong:
	case protocol.MT_AuthorizeRequest:
		c.onAuthorize(ctx, m)
	case protocol.MT_StartPKRequest:
		c.onStartPK(ctx, m)
	case protocol.MT_AnswerPKRequest:
		c.onAnswerPK(ctx, m)
	case protocol.MT_EndPKRequest:
		c.onEndPK(ctx, m)
	case protocol.MT_DisconnectNotify:
		c.onDisconnect(ctx, m)
	default:
		c.xl.Errorf("unknown message from %v, %v=%v", c.playerID, t, string(m))
	}
}

func (c *WSClient) recover() {
	if e := recover(); e != nil {
		const size = 16 << 10
		buf := make([]byte, size)
		buf = buf[:runtime.Stack(buf, false)]
		var xl *xlog.Logger
		if c == nil {
			xl = xlog.New("ws-client-recover-nil-client")
		} else if c.xl == nil {
			xl = xlog.New("ws-client-recover-nil-logger")
		} else {
			xl = c.xl
		}
		xl.Error("process panic: ", c.playerID, e, fmt.Sprintf("\n%s", buf))
	}
}

func (c *WSClient) onAuthorize(ctx context.Context, m msgpump.Message) {
	var req protocol.AuthorizeRequest
	err := req.Unmarshal(m)
	if err != nil {
		res := &protocol.AuthorizeResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorUnknownMessage,
			Error: errors.WSErrorToString[errors.WSErrorUnknownMessage],
		}
		c.Notify(protocol.MT_AuthorizeResponse, res)
		return
	}

	id, err := c.s.authCtl.GetIDByToken(c.xl, req.Token)
	if err != nil {
		res := &protocol.AuthorizeResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorTokenInvalid,
			Error: errors.WSErrorToString[errors.WSErrorTokenInvalid],
		}
		c.Notify(protocol.MT_AuthorizeResponse, res)
		return
	}

	c.playerID = id
	close(c.authorizeChan)

	res := &protocol.AuthorizeResponse{
		RPCID:       req.RPCID,
		Code:        errors.WSErrorOK,
		Error:       errors.WSErrorToString[errors.WSErrorOK],
		PongTimeout: c.s.conf.WsConf.PongTimeoutSecond,
	}
	c.Notify(protocol.MT_AuthorizeResponse, res)
	return
}

func (c *WSClient) onStartPK(ctx context.Context, m msgpump.Message) {
	c.s.signaling.OnStartPK(c.xl, c.playerID, m)
}

func (c *WSClient) onAnswerPK(ctx context.Context, m msgpump.Message) {
	c.s.signaling.OnAnswerPK(c.xl, c.playerID, m)
}

func (c *WSClient) onEndPK(ctx context.Context, m msgpump.Message) {
	c.s.signaling.OnEndPK(c.xl, c.playerID, m)
}

func (c *WSClient) onDisconnect(ctx context.Context, m msgpump.Message) {
	var notify protocol.DisconnectNotify
	err := notify.Unmarshal(m)
	if err != nil {
		c.xl.Errorf("unknown disconnect message: %v from %s", m, c.playerID)
		return
	}
	c.xl.Infof("%v at %s:%s requested to disconnect", c.playerID, c.remoteAddr, c.remotePort)
	err = c.s.RemovePlayer(c.playerID)
	if err != nil {
		c.xl.Errorf("failed to remove player, error %v", err)
	}
	c.xl.Infof("%v at %s:%s take the initiative to disconnect.", c.playerID, c.remoteAddr, c.remotePort)
	close(c.disconnectChan)
}

// WSServer WebSocket Server
type WSServer struct {
	conf config.Config
	xl   *xlog.Logger

	cl    sync.RWMutex
	conns map[string]*WSClient

	accountCtl *controller.AccountController
	authCtl    *controller.AuthController
	roomCtl    *controller.RoomController

	signaling *controller.SignalingService

	*websocket.Service
}

// CreateClient Implement for github.com/qrtc/qlive/service/websocket ClientCreator
func (s *WSServer) CreateClient(r *http.Request, rAddr, rPort string) (websocket.Client, error) {

	return &WSClient{
		s:              s,
		st:             time.Now(),
		xl:             xlog.New(NewReqID()),
		online:         atomic.NewBool(false),
		authorizeChan:  make(chan struct{}),
		disconnectChan: make(chan struct{}),
		remoteAddr:     rAddr,
		remotePort:     rPort,
	}, nil
}

// AddPlayer add player on player list
func (s *WSServer) AddPlayer(id string, c *WSClient) error {
	s.cl.Lock()
	defer s.cl.Unlock()

	if client, ok := s.conns[id]; ok && client != nil {
		s.xl.Infof("%v reconnect", id)
		close(client.disconnectChan)
	}

	s.conns[id] = c
	return nil
}

// RemovePlayer remove player from player list
func (s *WSServer) RemovePlayer(id string) error {

	c, ok := s.getPlayerClient(id)
	if !ok {
		s.xl.Errorf("user %s not online", id)
		return errors.NewWSError("player not online")
	}
	err := s.signaling.OnUserOffline(c.xl, id)
	if err != nil {
		c.xl.Errorf("failed to process user %s offline at %s:%s", id, c.remoteAddr, c.remotePort)
	}
	s.deletePlayerClient(id)
	c.xl.Debugf("player %s at %s:%s deleted", id, c.remoteAddr, c.remotePort)
	return nil
}

func (s *WSServer) getPlayerClient(id string) (c *WSClient, ok bool) {
	s.cl.RLock()
	defer s.cl.RUnlock()

	c, ok = s.conns[id]
	return c, ok
}

func (s *WSServer) deletePlayerClient(id string) {
	s.cl.Lock()
	defer s.cl.Unlock()
	delete(s.conns, id)
}

// NotifyPlayer send player notify message
func (s *WSServer) NotifyPlayer(id string, t string, v PMessage) error {

	return s.notifyPlayer(id, t, v)
}

func (s *WSServer) notifyPlayer(id string, t string, v PMessage) error {
	playerConn, ok := s.getPlayerClient(id)
	if !ok || !playerConn.IsOnline() {
		s.xl.Debugf("player %s not found or not online", id)
		return errors.NewWSError("player not online")
	}
	playerConn.Notify(t, v)

	return nil
}

// FindPlayer find player by ID
func (s *WSServer) FindPlayer(id string) (c *WSClient, err error) {

	player, ok := s.getPlayerClient(id)
	if !ok || !player.IsOnline() {
		return nil, errors.NewWSError("player not online")
	}
	return player, nil
}

// NewWSServer return a new websocket server
func NewWSServer(conf *config.Config) (s *WSServer, err error) {
	s = &WSServer{
		conf:  *conf,
		xl:    xlog.New(NewReqID()),
		conns: make(map[string]*WSClient),
	}
	if conf.Signaling.PKRequestTimeoutSecond == 0 {
		conf.Signaling.PKRequestTimeoutSecond = defaultPKRequestTimeoutSecond
	}

	s.accountCtl, err = controller.NewAccountController(conf.Mongo.URI, conf.Mongo.Database, nil)
	if err != nil {
		return nil, err
	}
	s.authCtl, err = controller.NewAuthController(conf.Mongo.URI, conf.Mongo.Database, nil)
	if err != nil {
		return nil, err
	}
	s.roomCtl, err = controller.NewRoomController(conf.Mongo.URI, conf.Mongo.Database, nil)
	if err != nil {
		return nil, err
	}

	signalingService, err := controller.NewSignalingService(nil, conf)
	signalingService.Notify = func(xl *xlog.Logger, userID string, msgType string, msg controller.MarshallableMessage) error {
		return s.NotifyPlayer(userID, msgType, msg)
	}
	s.signaling = signalingService

	s.Service = websocket.NewService(&websocket.Config{
		ListenAddr: conf.WsConf.ListenAddr,
		ServeURI:   conf.WsConf.ServeURI,
	}, s)

	return s, err
}
