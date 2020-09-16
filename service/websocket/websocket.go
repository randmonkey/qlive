package websocket

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/someonegg/gox/syncx"
	"github.com/someonegg/msgpump"
)

type Client interface {
	// Client should start the pump with Service.QuitCtx.
	Start(*msgpump.Pump)

	msgpump.Handler
}

type ClientCreator interface {
	CreateClient(r *http.Request, rAddr, rPort string) (Client, error)
}

type Config struct {
	ListenAddr      string `json:"listen_addr" validate:"nonzero"`
	ServeURI        string `json:"serve_uri" validate:"nonzero"`
	PumpWriteQueue  int    `json:"pump_write_queue" validate:"nonzero"`
	OriginHost      string `json:"origin_host"`
	ReadBufferSize  int    `json:"conn_read_size"`
	WriteBufferSize int    `json:"conn_write_size"`
}

type Service struct {
	creator        ClientCreator
	pumpWriteQueue int
	upgrader       websocket.Upgrader

	err     error
	quitCtx context.Context
	quitF   context.CancelFunc
	stopD   syncx.DoneChan

	hs    *http.Server
	cliWG sync.WaitGroup
}

func NewService(conf *Config, creator ClientCreator) *Service {
	s := &Service{}

	s.creator = creator
	s.pumpWriteQueue = conf.PumpWriteQueue

	s.upgrader.ReadBufferSize = conf.ReadBufferSize
	s.upgrader.WriteBufferSize = conf.WriteBufferSize
	if conf.OriginHost == "" {
		s.upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	} else {
		s.upgrader.CheckOrigin = func(r *http.Request) bool {
			org := r.Header.Get("Origin")
			return org == "http://"+conf.OriginHost || org == "https://"+conf.OriginHost
		}
	}

	s.quitCtx, s.quitF = context.WithCancel(context.Background())
	s.stopD = syncx.NewDoneChan()

	mux := http.NewServeMux()
	mux.HandleFunc(conf.ServeURI, s.websocketHandler)

	s.hs = &http.Server{
		Addr:           conf.ListenAddr,
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	return s
}

func (s *Service) Start() {
	go s.serve()
}

func (s *Service) serve() {
	defer s.ending()

	s.err = s.hs.ListenAndServe()
}

func (s *Service) ending() {
	if e := recover(); e != nil {
		switch v := e.(type) {
		case error:
			s.err = v
		default:
			s.err = errors.New("unknown panic")
		}
	}

	s.quitF()
	s.stopD.SetDone()
}

func (s *Service) Error() error {
	return s.err
}

func (s *Service) Stop() {
	s.quitF()
	s.hs.Close()
}

func (s *Service) StopD() syncx.DoneChanR {
	return s.stopD.R()
}

func (s *Service) Stopped() bool {
	return s.stopD.R().Done()
}

func (s *Service) WaitClients() {
	s.cliWG.Wait()
}

func (s *Service) QuitCtx() context.Context {
	return s.quitCtx
}

func (s *Service) websocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	var rAddr, rPort string
	rAddr = getRemoteAddr(r)
	if host, port, err := net.SplitHostPort(rAddr); err == nil {
		rAddr = host
		rPort = port
	}

	cli, err := s.creator.CreateClient(r, rAddr, rPort)
	if err != nil {
		conn.Close()
		return
	}

	s.cliWG.Add(1)
	cw := connwrapper{s: s, Conn: conn}

	rw := msgpump.WebsocketMRW(cw)
	pump := msgpump.NewPump(rw, cli, s.pumpWriteQueue)
	cli.Start(pump)
}

type connwrapper struct {
	s *Service
	*websocket.Conn
}

func (c connwrapper) Close() error {
	c.s.cliWG.Done()
	return c.Conn.Close()
}

func getRemoteAddr(r *http.Request) string {

	if addr := r.Header.Get("X-Forwarded-For"); addr != "" {
		if idx := strings.Index(addr, ","); idx != -1 {
			addr = addr[:idx]
		}
		return addr
	}

	if addr := r.Header.Get("X-Real-IP"); addr != "" {
		if port := r.Header.Get("X-Real-PORT"); port != "" {
			addr += ":" + port
		}
		return addr
	}
	return r.RemoteAddr
}
