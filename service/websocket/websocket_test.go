package websocket

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/someonegg/msgpump"
)

type mockService struct {
	*Service

	count int
}

func (s *mockService) CreateClient(r *http.Request, rAddr, rPort string) (Client, error) {
	return &mockClient{s: s}, nil
}

type mockClient struct {
	s *mockService
	p *msgpump.Pump
}

func (c *mockClient) Start(p *msgpump.Pump) {
	c.p = p
	c.p.Start(c.s.QuitCtx())
}

func (c *mockClient) Process(ctx context.Context, t string, m msgpump.Message) {
	c.s.count++
	c.p.Output(t, m)
}

func TestNormal(test *testing.T) {
	s := &mockService{}
	s.Service = NewService(
		&Config{
			ListenAddr:     "127.0.0.1:12345",
			ServeURI:       "/websocket",
			PumpWriteQueue: 10,
		}, s)
	s.Service.Start()

	time.Sleep(5 * time.Millisecond)

	dialer := &websocket.Dialer{}
	conn, resp, err := dialer.Dial("ws://127.0.0.1:12345/websocket", nil)
	if err != nil {
		test.Fatal("dial: ", err, resp)
	}

	conn.WriteMessage(websocket.TextMessage, []byte("t1=m1"))
	t, p, err := conn.ReadMessage()

	if s.count != 1 {
		test.Fatal("message count")
	}

	if t != websocket.TextMessage {
		test.Fatal("message type")
	}
	if string(p) != "t1=m1" {
		test.Fatal("message content")
	}

	s.Service.Stop()
	select {
	case <-s.Service.StopD():
	}
	s.Service.WaitClients()
}

func TestOriginCheck(test *testing.T) {
	s := &mockService{}
	s.Service = NewService(
		&Config{
			ListenAddr:     "127.0.0.1:12345",
			ServeURI:       "/websocket",
			PumpWriteQueue: 10,
			OriginHost:     "test.com",
		}, s)
	s.Service.Start()

	time.Sleep(5 * time.Millisecond)

	dialer := &websocket.Dialer{}
	_, _, err := dialer.Dial("ws://127.0.0.1:12345/websocket", nil)
	if err != websocket.ErrBadHandshake {
		test.Fatal("origin host uncheck")
	}

	header := http.Header{}
	header.Set("Origin", "http://test2.com")
	_, _, err = dialer.Dial("ws://127.0.0.1:12345/websocket", header)
	if err != websocket.ErrBadHandshake {
		test.Fatal("origin host check")
	}

	header.Set("Origin", "http://test.com")
	conn, _, err := dialer.Dial("ws://127.0.0.1:12345/websocket", header)
	if err != nil {
		test.Fatal("dial: ", err)
	}
	conn.Close()

	s.Service.Stop()
	select {
	case <-s.Service.StopD():
	}
	s.Service.WaitClients()
}
