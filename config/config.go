package config

import "os"

// WebSocketConf websocket长连接配置。
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

// MongoConfig mongo 数据库配置。
type MongoConfig struct {
	URI      string `json:"uri"`
	Database string `json:"database"`
}

// QiniuKeyPair 七牛APIaccess key/secret key配置。
type QiniuKeyPair struct {
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
}

// QiniuSMSConfig 七牛云短信配置。
type QiniuSMSConfig struct {
	KeyPair     QiniuKeyPair `json:"key_pair"`
	SignatureID string       `json:"signature_id"`
	TemplateID  string       `json:"template_id"`
}

// SMSConfig 短信服务配置。
type SMSConfig struct {
	Provider string          `json:"provider"`
	QiniuSMS *QiniuSMSConfig `json:"qiniu_sms"`
}

// QiniuRTCConfig 七牛RTC服务配置。
type QiniuRTCConfig struct {
	KeyPair QiniuKeyPair `json:"key_pair"`
	AppID   string       `json:"app_id"`
	// 合流转推的域名。
	PublishHost string `json:"publish_host"`
	// 合流转推的Hub名称。
	PublishHub string `json:"publish_hub"`
}

// Config 后端配置。
type Config struct {
	// debug等级，为1时输出info/warn/error日志，为0除以上外还输出debug日志
	DebugLevel int    `json:"debug_level"`
	ListenAddr string `json:"listen_addr"`

	WsConf *WebSocketConf  `json:"websocket_conf"`
	Mongo  *MongoConfig    `json:"mongo"`
	SMS    *SMSConfig      `json:"sms"`
	RTC    *QiniuRTCConfig `json:"rtc"`
}

// NewSample 返回样例配置。
func NewSample() *Config {
	return &Config{
		DebugLevel: 0,
		ListenAddr: ":8080",
		WsConf: &WebSocketConf{
			ListenAddr: ":8443",
			ServeURI:   "/qlive",
		},
		Mongo: &MongoConfig{
			URI:      "mongodb://localhost:27017",
			Database: "qrtc_qlive_test",
		},
		SMS: &SMSConfig{
			Provider: "test",
			QiniuSMS: &QiniuSMSConfig{
				KeyPair: QiniuKeyPair{
					AccessKey: os.Getenv("QINIU_ACCESS_KEY"),
					SecretKey: os.Getenv("QINIU_SECRET_KEY"),
				},
				SignatureID: os.Getenv("QINIU_SMS_SIGN_ID"),
				TemplateID:  os.Getenv("QINIU_SMS_TEMP_ID"),
			},
		},
		RTC: &QiniuRTCConfig{
			KeyPair: QiniuKeyPair{
				AccessKey: os.Getenv("QINIU_ACCESS_KEY"),
				SecretKey: os.Getenv("QINIU_SECRET_KEY"),
			},
			AppID:       os.Getenv("QINIU_RTC_APP_ID"),
			PublishHost: "localhost:1935",
			PublishHub:  "test",
		},
	}
}
