package main

import (
	"github.com/qrtc/qlive/config"
	"github.com/qrtc/qlive/router"
)

func main() {
	r := router.NewRouter()
	cfg := config.NewSample()
	r.Run(cfg.ListenAddr)
}
