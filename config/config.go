package config

// Config 后端配置。
type Config struct {
	ListenAddr string `json:"listen_addr"`
}

// NewSample 返回样例配置。
func NewSample() *Config {
	return &Config{
		ListenAddr: ":8080",
	}
}
