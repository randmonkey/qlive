// Copyright 2020 Qiniu Cloud (qiniu.com)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// @title 互动直播API
// @version 1.0
// @description 互动直播API
// @termsOfService https://www.qiniu.com

// @contact.name qlive developer
// @contact.url https://github.com/qrtc/qlive
// @contact.email

// @license.name Apache 2.0
// @license.url https://www.apache.org/licenses/LICENSE-2.0

// @host localhost:8080
// @BasePath /v1

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
	errch := make(chan error, 1)
	go func() {
		httpServerErr := r.Run(conf.ListenAddr)
		errch <- httpServerErr
	}()

	// 启动 WebSocket server。
	if conf.Signaling.Type == "ws" || conf.Signaling.Type == "websocket" {
		ws, err := service.NewWSServer(conf)
		if err != nil {
			log.Fatalf("failed to create Websocket server, error %v", err)
		}
		ws.Start()
		log.Infof("WebSocket listening and serving on %s%s", conf.WsConf.ListenAddr, conf.WsConf.ServeURI)

		select {

		case <-ws.StopD():
			log.Error("WebSocket service stoped: ", ws.Error())
			errch <- ws.Error()
		}

		ws.WaitClients()
	}
	qC := make(chan os.Signal, 1)
	signal.Notify(qC, syscall.SIGINT, syscall.SIGTERM)
	select {
	case s := <-qC:
		log.Info(s.String())
	case err = <-errch:
		log.Error("service stopped, error", err.Error())
	}
}
