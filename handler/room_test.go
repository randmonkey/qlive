package handler

import (
	"fmt"
	"testing"

	"github.com/qiniu/x/xlog"
	"github.com/stretchr/testify/assert"
)

func TestGeneratewsURL(t *testing.T) {
	xlog.SetOutputLevel(0)
	h := &RoomHandler{}
	testCases := []struct {
		wsAddress   string
		wsProtocol  string
		wsPort      int
		wsPath      string
		host        string
		expectedURL string
	}{

		{
			wsAddress:   "wss://localhost:1234/qlive",
			wsProtocol:  "wss",
			wsPort:      8081,
			wsPath:      "/qlive",
			host:        "127.0.0.1",
			expectedURL: "wss://localhost:1234/qlive",
		},
		{
			wsAddress:   "localhost:1234/qlive",
			wsProtocol:  "wss",
			wsPort:      8081,
			wsPath:      "/qlive",
			host:        "127.0.0.1",
			expectedURL: "wss://localhost:1234/qlive",
		},
		{
			wsAddress:   "",
			wsProtocol:  "ws",
			wsPort:      8081,
			wsPath:      "/qlive",
			host:        "127.0.0.1",
			expectedURL: "ws://127.0.0.1:8081/qlive",
		},
		{
			wsAddress:   "",
			wsProtocol:  "wss",
			wsPort:      8081,
			wsPath:      "/qlive",
			host:        "127.0.0.1:8080",
			expectedURL: "wss://127.0.0.1:8081/qlive",
		},
		{
			wsAddress:   "",
			wsProtocol:  "ws",
			wsPort:      80,
			wsPath:      "/qlive",
			host:        "127.0.0.1:8080",
			expectedURL: "ws://127.0.0.1/qlive",
		},
		{
			wsAddress:   "",
			wsProtocol:  "wss",
			wsPort:      443,
			wsPath:      "/qlive",
			host:        "127.0.0.1:80",
			expectedURL: "wss://127.0.0.1/qlive",
		},
	}
	for i, testCase := range testCases {
		h.WSAddress = testCase.wsAddress
		h.WSProtocol = testCase.wsProtocol
		h.WSPort = testCase.wsPort
		h.WSPath = testCase.wsPath
		xl := xlog.New(fmt.Sprintf("test-generatewsURL-%d", i))
		wsURL := h.generateWSURL(xl, testCase.host)
		assert.Equalf(t, testCase.expectedURL, wsURL, "test case %d: URL does not match", i)
	}
}
