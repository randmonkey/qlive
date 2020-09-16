package service

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/qiniu/x/xlog"
	"github.com/someonegg/gox/syncx"
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

	playerID  string
	closeNow  *atomic.Bool
	online    *atomic.Bool
	cleared   *atomic.Bool
	cancel    chan struct{}
	closeChan chan struct{}

	authorized    *atomic.Bool
	authorizeDone syncx.DoneChan

	remoteAddr string

	lastMessageTime time.Time

	ppCancel chan struct{}
}

func (c *WSClient) Start(p *msgpump.Pump) {
	c.p = p
	c.p.Start(c.s.QuitCtx())
	go c.monitor()
}

func (c *WSClient) monitor() {
	c.xl.Infof("monitor start")
	defer c.ending()
	to := time.Millisecond * time.Duration(c.s.conf.WsConf.AuthorizeTimeoutMS)
	select {
	case <-c.p.StopD():
	case <-c.authorizeDone:
	case <-time.After(to):
		c.p.Stop()
	}

	select {
	case <-c.p.StopD():
	}

	c.xl.Infof("monitor end: %v, closeNow: %v, cleared: %v, authorized: %v", c.playerID, c.closeNow.Load(), c.cleared.Load(), c.authorized.Load())
}

func (c *WSClient) ending() {
	c.recover()

	c.online.Store(false)

	if c.p != nil {
		c.p.Stop()
	}

	select {
	case c.ppCancel <- struct{}{}:
	default:
	}

	if c.cleared.Load() {
		return
	}

	if !c.authorized.Load() {
		return
	}

	if !c.closeNow.Load() {
		c.xl.Errorf("waiting reconnect: %v", c.playerID)

		timeout := time.Second * time.Duration(c.s.conf.WsConf.ReconnectTimeoutSecond)
		select {
		case <-c.cancel:
			c.xl.Infof("reconnect succeed: %v", c.playerID)
			return
		case <-time.After(timeout):
			c.xl.Infof("reconnect timeout: %v", c.playerID)
			break
		case <-c.closeChan:
			c.xl.Infof("reconnect break: %v, closeNow: %v", c.playerID, c.closeNow.Load())
			break
		}
	}

	//Do some disconnect logic
}

func (c *WSClient) pingPong() {
	ping := &protocol.Ping{}

	for {
		select {
		case <-time.After(time.Second * time.Duration(c.s.conf.WsConf.PingTickerSecond)):
			c.Notify(protocol.MT_Ping, ping)

			now := time.Now()
			if now.After(c.lastMessageTime.Add(time.Second * time.Duration(c.s.conf.WsConf.PongTimeoutSecond))) {
				c.xl.Errorf("pingpong timeout: %v", c.playerID)
				c.p.Stop()
				return
			}
		case <-c.ppCancel:
			return
		}
	}
}

// Online get client online status.
func (c *WSClient) Online() bool {
	return c.online.Load()
}

// Cancel cancel ending function.
func (c *WSClient) Cancel() {
	select {
	case c.cancel <- struct{}{}:
	default:
	}
}

// close stop client msgpump.
func (c *WSClient) close() error {
	c.closeNow.Store(true)
	if c.p != nil {
		c.p.Stop()
	}
	c.closeChan <- struct{}{}
	return nil
}

// StartTime get client start time.
func (c *WSClient) StartTime() time.Time {
	return c.st
}

// notify write a notify to client.
func (c *WSClient) Notify(t string, v PMessage) {
	if t == protocol.MT_Disconnect {
		c.cleared.Store(true)
	}

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
			c.close()
		}
	}
}

// Process listen all requests.
func (c *WSClient) Process(ctx context.Context, t string, m msgpump.Message) {
	go func(ctx context.Context, t string, m msgpump.Message) {
		c.parallelProcess(ctx, t, m)
	}(ctx, t, m)
}

func (c *WSClient) parallelProcess(ctx context.Context, t string, m msgpump.Message) {
	defer c.recover()

	if t != protocol.MT_Ping && t != protocol.MT_Pong {
		c.xl.Infof("message from %v, %v=%v", c.playerID, t, string(m))
	}

	if !c.authorized.Load() && t != protocol.MT_Authorize {
		return
	}

	c.lastMessageTime = time.Now()

	switch t {
	case protocol.MT_Ping:
		c.Notify(protocol.MT_Pong, &protocol.Pong{})
	case protocol.MT_Pong:
	case protocol.MT_Authorize:
		c.onAuthorize(ctx, m)
	case protocol.MT_Disconnect:
		c.close()
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
	var req protocol.Authorize
	err := req.Unmarshal(m)
	if err != nil {
		return
	}

	_, err = c.s.authCtl.GetIDByToken(c.xl, req.Token)
	if err != nil {
		res := &protocol.AuthorizeResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorTokenInvalid,
			Error: errors.WSErrorToString[errors.WSErrorTokenInvalid],
		}
		c.Notify(protocol.MT_AuthorizeResponse, res)
		return
	}

	c.authorized.Store(true)
	c.authorizeDone.SetDone()
	res := &protocol.AuthorizeResponse{
		RPCID: req.RPCID,
		Code:  errors.WSErrorOK,
		Error: errors.WSErrorToString[errors.WSErrorOK],
	}
	c.Notify(protocol.MT_AuthorizeResponse, res)
	go c.pingPong()
	return
}

// WebSocket Server
type WSServer struct {
	conf config.Config
	xl   *xlog.Logger

	cl    sync.RWMutex
	Conns map[string]*WSClient

	accountCtl *controller.AccountController
	authCtl    *controller.AuthController
	roomCtl    *controller.RoomController

	*websocket.Service
}

func (s *WSServer) CreateClient(r *http.Request, rAddr, rPort string) (websocket.Client, error) {

	return &WSClient{
		s:             s,
		st:            time.Now(),
		xl:            xlog.New(NewReqID()),
		closeNow:      atomic.NewBool(false),
		online:        atomic.NewBool(true),
		cleared:       atomic.NewBool(false),
		cancel:        make(chan struct{}, 1),
		closeChan:     make(chan struct{}, 1),
		authorized:    atomic.NewBool(false),
		authorizeDone: syncx.NewDoneChan(),
		remoteAddr:    rAddr,
		ppCancel:      make(chan struct{}, 1),
	}, nil
}

func NewWSServer(conf *config.Config) (s *WSServer, err error) {
	s = &WSServer{
		conf:  *conf,
		xl:    xlog.New(NewReqID()),
		Conns: make(map[string]*WSClient),
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
