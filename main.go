package main

import (
	"flag"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	log.SetOutputLevel(conf.DebugLevel)
	rand.Seed(time.Now().UnixNano())

	// 启动 gin HTTP server。
	r, err := service.NewRouter(conf)
	if err != nil {
		log.Fatalf("failed to create gin HTTP server, error %v", err)
	}
	go r.Run(conf.ListenAddr)

	// 启动 WebSocket server。
	ws, err := service.NewWSServer(conf)
	if err != nil {
		log.Fatalf("failed to create Websocket server, error %v", err)
	}
	ws.Start()
	log.Infof("WebSocket listening and serving on %s%s", conf.WsConf.ListenAddr, conf.WsConf.ServeURI)

	qC := make(chan os.Signal, 1)
	signal.Notify(qC, syscall.SIGINT, syscall.SIGTERM)

	select {
	case s := <-qC:
		log.Info(s.String())
	case <-ws.StopD():
		log.Error("WebSocket service stoped: ", ws.Error())
	}

	ws.WaitClients()
}
