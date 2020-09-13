package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/qiniu/x/log"
	"github.com/qrtc/qlive/config"
	"github.com/qrtc/qlive/router"
	"github.com/qrtc/qlive/service"
)

func main() {
	r := router.NewRouter()
	cfg := config.NewSample()
	go r.Run(cfg.ListenAddr)

	server := service.NewWSServer(cfg)
	serv := service.NewService(&service.Config{
		ListenAddr: cfg.WsConf.ListenAddr,
		ServeURI:   cfg.WsConf.ServeURI,
	}, server)
	serv.Start()

	qC := make(chan os.Signal, 1)
	signal.Notify(qC, syscall.SIGINT, syscall.SIGTERM)

	select {
	case s := <-qC:
		log.Info(s.String())
	}
}
