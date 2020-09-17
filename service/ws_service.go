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
	s  *WSServer
	p  *msgpump.Pump
	st time.Time
	xl *xlog.Logger

	playerID      string
	online        *atomic.Bool
	authorizeDone chan struct{}

	remoteAddr string

	lastMessageTime time.Time
}

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

	if c.online.Load() {
		if t != protocol.MT_Ping && t != protocol.MT_Pong {
			c.xl.Infof("message to %v, %v=%v", c.playerID, t, string(m))
		}

		if ok := c.p.TryOutput(t, m); !ok {
			c.xl.Errorf("TryOutput failed %v", c.playerID)
			c.Close()
		}
	}
}

func (c *WSClient) monitor() {
	select {
	case <-c.p.StopD():
	case <-time.After(time.Millisecond * time.Duration(c.s.conf.WsConf.AuthorizeTimeoutMS)):
		c.Close()
	case <-c.authorizeDone:
		ping := &protocol.Ping{}
		c.s.AddPlayer(c.playerID, c)
		c.online.Store(true)
		for {
			select {
			case <-c.p.StopD():
				c.online.Store(false)
				c.s.RemovePlayer(c.playerID)
				break
			case <-time.After(time.Second * time.Duration(c.s.conf.WsConf.PingTickerSecond)):
				c.Notify(protocol.MT_Ping, ping)
				if time.Now().Sub(c.lastMessageTime) > time.Second*time.Duration(c.s.conf.WsConf.PongTimeoutSecond) {
					c.Close()
					c.s.RemovePlayer(c.playerID)
					break
				}
			}
		}
	}
}

func (c *WSClient) parallelProcess(ctx context.Context, t string, m msgpump.Message) {
	defer c.recover()

	if t != protocol.MT_Ping && t != protocol.MT_Pong {
		c.xl.Infof("message from %v, %v=%v", c.playerID, t, string(m))
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
	case protocol.MT_EndPKRequest:
		c.onEndPK(ctx, m)
	case protocol.MT_AnswerPKRequest:
		c.onAnswerPK(ctx, m)
	default:
		c.xl.Errorf("unknown message from %v, %v=%v", c.playerID, t, string(m))
	}
}

func (c *WSClient) recover() {
	if e := recover(); e != nil {
		const size = 16 << 10
		buf := make([]byte, size)
		buf = buf[:runtime.Stack(buf, false)]
		c.xl.Error("process panic: ", c.playerID, e, fmt.Sprintf("\n%s", buf))
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
	close(c.authorizeDone)

	res := &protocol.AuthorizeResponse{
		RPCID: req.RPCID,
		Code:  errors.WSErrorOK,
		Error: errors.WSErrorToString[errors.WSErrorOK],
	}
	c.Notify(protocol.MT_AuthorizeResponse, res)
	return
}

func (c *WSClient) onStartPK(ctx context.Context, m msgpump.Message) {
	var req protocol.StartPKRequest
	err := req.Unmarshal(m)
	if err != nil {
		res := &protocol.StartPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorUnknownMessage,
			Error: errors.WSErrorToString[errors.WSErrorUnknownMessage],
		}
		c.Notify(protocol.MT_StartResponse, res)
		return
	}
	// TODO
	// A 向 B 发起 PK，发送 offer-notify 给 B
}

func (c *WSClient) onEndPK(ctx context.Context, m msgpump.Message) {
	var req protocol.EndPKRequest
	err := req.Unmarshal(m)
	if err != nil {
		res := &protocol.EndPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorUnknownMessage,
			Error: errors.WSErrorToString[errors.WSErrorUnknownMessage],
		}
		c.Notify(protocol.MT_EndPKResponse, res)
		return
	}
	// TODO
	// A 结束 PK
}

func (c *WSClient) onAnswerPK(ctx context.Context, m msgpump.Message) {
	var req protocol.AnswerPKRequest
	err := req.Unmarshal(m)
	if err != nil {
		res := &protocol.AnswerPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorUnknownMessage,
			Error: errors.WSErrorToString[errors.WSErrorUnknownMessage],
		}
		c.Notify(protocol.MT_AnswerPKResponse, res)
		return
	}
	// TODO
	// B 应答 A 发起的 PK，发送 answer-notify 给 A
}

// WebSocket Server
type WSServer struct {
	conf config.Config
	xl   *xlog.Logger

	cl    sync.RWMutex
	conns map[string]*WSClient

	accountCtl *controller.AccountController
	authCtl    *controller.AuthController
	roomCtl    *controller.RoomController

	*websocket.Service
}

// CreateClient Implement for github.com/qrtc/qlive/service/websocket ClientCreator
func (s *WSServer) CreateClient(r *http.Request, rAddr, rPort string) (websocket.Client, error) {

	return &WSClient{
		s:             s,
		st:            time.Now(),
		xl:            xlog.New(NewReqID()),
		online:        atomic.NewBool(false),
		authorizeDone: make(chan struct{}),
		remoteAddr:    rAddr,
	}, nil
}

// AddPlayer add player on player list
func (s *WSServer) AddPlayer(id string, c *WSClient) error {
	s.cl.Lock()
	defer s.cl.Unlock()

	s.conns[id] = c
	return nil
}

// RemovePlayer remove player from player list
func (s *WSServer) RemovePlayer(id string) error {
	s.cl.Lock()
	defer s.cl.Unlock()

	if _, ok := s.conns[id]; !ok {
		return errors.NewWSError("player not online")
	}
	delete(s.conns, id)
	return nil
}

// NotifyPlayer send player notify message
func (s *WSServer) NotifyPlayer(id string, t string, v PMessage) error {
	s.cl.RLock()
	defer s.cl.RUnlock()

	player, ok := s.conns[id]
	if !ok || !player.IsOnline() {
		return errors.NewWSError("player not online")
	}
	player.Notify(t, v)

	return nil
}

// NewWSServer return a new websocket server
func NewWSServer(conf *config.Config) (s *WSServer, err error) {
	s = &WSServer{
		conf:  *conf,
		xl:    xlog.New(NewReqID()),
		conns: make(map[string]*WSClient),
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

	s.Service = websocket.NewService(&websocket.Config{
		ListenAddr: conf.WsConf.ListenAddr,
		ServeURI:   conf.WsConf.ServeURI,
	}, s)

	return s, err
}
