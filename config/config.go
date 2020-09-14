package config

type WebSocketConf struct {
	AuthorizeTimeoutMS     int `json:"authorize_timeout_ms"`
	PingTickerSecond       int `json:"ping_ticker_s"`
	PongTimeoutSecond      int `json:"pong_timeout_s"`
	ReconnectTimeoutSecond int `json:"reconnect_timeout_s"`

	ListenAddr      string `json:"listen_addr" validate:"nonzero"`
	ServeURI        string `json:"serve_uri" validate:"nonzero"`
	PumpWriteQueue  int    `json:"pump_write_queue" validate:"nonzero"`
	OriginHost      string `json:"origin_host"`
	ReadBufferSize  int    `json:"conn_read_size"`
	WriteBufferSize int    `json:"conn_write_size"`
}

// Config 后端配置。
type Config struct {
	ListenAddr string `json:"listen_addr"`

	WsConf *WebSocketConf `json:"websocket_conf"`
}

// NewSample 返回样例配置。
func NewSample() *Config {
	return &Config{
		ListenAddr: ":8080",
		WsConf: &WebSocketConf{
			ListenAddr: ":8443",
			ServeURI:   "/qlive",
		},
	}
}
