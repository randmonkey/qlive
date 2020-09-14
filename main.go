package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	qconfig "github.com/qiniu/x/config"
	"github.com/qiniu/x/log"

	"github.com/qrtc/qlive/config"
	"github.com/qrtc/qlive/service"
)

var (
	configFilePath = "qlive.conf"
)

func main() {
	flag.StringVar(&configFilePath, "f", configFilePath, "configuration file to run qlive server")
	flag.Parse()

	conf := &config.Config{}
	err := qconfig.LoadFile(conf, configFilePath)
	if err != nil {
		log.Fatalf("failed to load config file, error %v", err)
	}

	// 启动 gin HTTP server。
	r, err := service.NewRouter(conf)
	if err != nil {
		log.Fatalf("failed to create gin HTTP server, error %v", err)
	}
	go r.Run(conf.ListenAddr)

	server := service.NewWSServer(conf)
	serv := service.NewService(&service.Config{
		ListenAddr: conf.WsConf.ListenAddr,
		ServeURI:   conf.WsConf.ServeURI,
	}, server)
	serv.Start()

	qC := make(chan os.Signal, 1)
	signal.Notify(qC, syscall.SIGINT, syscall.SIGTERM)

	select {
	case s := <-qC:
		log.Info(s.String())
	}
}
