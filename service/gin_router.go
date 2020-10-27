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

package service

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qiniu/x/xlog"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/qrtc/qlive/config"
	"github.com/qrtc/qlive/controller"
	_ "github.com/qrtc/qlive/docs"
	"github.com/qrtc/qlive/errors"
	"github.com/qrtc/qlive/handler"
	"github.com/qrtc/qlive/protocol"
)

// @title 互动直播API
// @version 0.0.1
// @description  http apis
// @BasePath /v1
// NewRouter 返回gin router，分流API。
func NewRouter(conf *config.Config) (*gin.Engine, error) {
	router := gin.New()

	accountController, err := controller.NewAccountController(conf.Mongo.URI, conf.Mongo.Database, nil)
	if err != nil {
		return nil, err
	}

	smsCodeController, err := controller.NewSMSCodeController(conf.Mongo.URI, conf.Mongo.Database, conf.SMS, nil)
	if err != nil {
		return nil, err
	}

	accountHandler := &handler.AccountHandler{
		Account: accountController,
		SMSCode: smsCodeController,
	}

	authController, err := controller.NewAuthController(conf.Mongo.URI, conf.Mongo.Database, nil)
	if err != nil {
		return nil, err
	}
	authHandler := &handler.AuthHandler{
		Auth: authController,
	}

	roomController, err := controller.NewRoomController(conf.Mongo.URI, conf.Mongo.Database, nil)
	if err != nil {
		return nil, err
	}

	roomHandler := &handler.RoomHandler{
		Account:   accountController,
		Room:      roomController,
		RTCConfig: conf.RTC,
		WSAddress: conf.WsConf.ExternalWSAddr,
	}
	if conf.WsConf.ExternalWSPort != 0 {
		roomHandler.WSPort = conf.WsConf.ExternalWSPort
	} else {
		_, wsPortStr, err := net.SplitHostPort(conf.WsConf.ListenAddr)
		if err != nil {
			return nil, fmt.Errorf("invalid websocket listen address, failed to parse: %v", err)
		}
		wsPort, err := strconv.Atoi(wsPortStr)
		if err != nil {
			return nil, fmt.Errorf("invalid websocket listen port, failed to parse: %v", err)
		}
		roomHandler.WSPort = wsPort
	}

	if conf.WsConf.WSOverTLS {
		roomHandler.WSProtocol = "wss"
	} else {
		roomHandler.WSProtocol = "ws"
	}

	roomHandler.WSPath = "/qlive"

	imController, err := controller.NewIMController(conf.IM, nil)
	if err != nil {
		return nil, err
	}
	imHandler := &handler.IMHandler{
		IMService: imController,
	}
	if conf.Signaling.Type == "im" {
		signalingService := controller.NewSignalingService(nil, conf, accountController, roomController)
		imHandler.IMService = imHandler.IMService.WithSignalingService(signalingService)
	}

	uploadController, err := controller.NewQiniuUploadController(conf.Storage, nil)
	if err != nil {
		return nil, err
	}
	uploadHandler := &handler.UploadHandler{
		Upload: uploadController,
	}

	feedbackController, err := controller.NewFeedbackController(conf.Mongo.URI, conf.Mongo.Database, conf.FeedbackMail, nil)
	if err != nil {
		return nil, err
	}
	feedbackHandler := &handler.FeedbackHandler{
		Feedback:            feedbackController,
		AttachmentURLPrefix: conf.Storage.URLPrefix,
	}

	promHandler := handler.NewPromHandler(conf.Prometheus, nil)

	v1 := router.Group("/v1")
	v1.Use(addRequestID)
	{
		// 账号相关API。
		v1.POST("login", accountHandler.Login, handler.SetMetrics)
		v1.POST("login/", accountHandler.Login, handler.SetMetrics)
		v1.POST("send_sms_code", accountHandler.SendSMSCode, handler.SetMetrics)
		v1.POST("send_sms_code/", accountHandler.SendSMSCode, handler.SetMetrics)
		v1.POST("profile", authHandler.Authenticate, accountHandler.UpdateProfile, handler.SetMetrics)
		v1.POST("profile/", authHandler.Authenticate, accountHandler.UpdateProfile, handler.SetMetrics)
		v1.POST("logout", authHandler.Authenticate, accountHandler.Logout, handler.SetMetrics)
		v1.POST("logout/", authHandler.Authenticate, accountHandler.Logout, handler.SetMetrics)

		// 主播端API：创建、关闭房间。
		v1.POST("rooms", authHandler.Authenticate, roomHandler.CreateRoom, handler.SetMetrics)
		v1.POST("rooms/", authHandler.Authenticate, roomHandler.CreateRoom, handler.SetMetrics)
		v1.POST("close_room", authHandler.Authenticate, roomHandler.CloseRoom, handler.SetMetrics)
		v1.POST("close_room/", authHandler.Authenticate, roomHandler.CloseRoom, handler.SetMetrics)
		// 主播端API：更新房间信息。
		v1.PUT("rooms/:roomID", authHandler.Authenticate, roomHandler.UpdateRoom, handler.SetMetrics)
		v1.PUT("rooms/:roomID/", authHandler.Authenticate, roomHandler.UpdateRoom, handler.SetMetrics)
		// 主播端API：重新进入房间，刷新RTC room token。
		v1.POST("refresh_room", authHandler.Authenticate, roomHandler.RefreshRoom, handler.SetMetrics)
		v1.POST("refresh_room/", authHandler.Authenticate, roomHandler.RefreshRoom, handler.SetMetrics)
		// 观众端API：进入、退出房间。
		v1.POST("enter_room", authHandler.Authenticate, roomHandler.EnterRoom, handler.SetMetrics)
		v1.POST("enter_room/", authHandler.Authenticate, roomHandler.EnterRoom, handler.SetMetrics)
		v1.POST("leave_room", authHandler.Authenticate, roomHandler.LeaveRoom, handler.SetMetrics)
		v1.POST("leave_room/", authHandler.Authenticate, roomHandler.LeaveRoom, handler.SetMetrics)

		// 观众端/主播端API：获取全部房间或者PK房间。
		v1.GET("rooms", authHandler.Authenticate, roomHandler.ListRooms, handler.SetMetrics)
		v1.GET("rooms/", authHandler.Authenticate, roomHandler.ListRooms, handler.SetMetrics)
		// 根据房间ID获取房间。
		v1.GET("rooms/:roomID", authHandler.Authenticate, roomHandler.GetRoom, handler.SetMetrics)
		v1.GET("rooms/:roomID/", authHandler.Authenticate, roomHandler.GetRoom, handler.SetMetrics)
		// IM API：生成IM token。
		v1.POST("im_user_token", authHandler.Authenticate, imHandler.GetUserToken, handler.SetMetrics)
		v1.POST("im_user_token/", authHandler.Authenticate, imHandler.GetUserToken, handler.SetMetrics)
		v1.POST("im_messages/:provider", imHandler.ProcessMessage)
		v1.POST("im_user_status/:provider", imHandler.OnUserStatusChange)
		// 上传API：生成上传文件token。
		v1.POST("upload/token", authHandler.Authenticate, uploadHandler.GetUploadToken, handler.SetMetrics)
		v1.POST("upload/token/", authHandler.Authenticate, uploadHandler.GetUploadToken, handler.SetMetrics)
		// 反馈问题API：发送反馈。
		v1.POST("feedbacks", authHandler.Authenticate, feedbackHandler.SendFeedback, handler.SetMetrics)
		v1.POST("feedbacks/", authHandler.Authenticate, feedbackHandler.SendFeedback, handler.SetMetrics)
	}

	metricsPath := conf.Prometheus.MetricsPath
	if metricsPath == "" {
		metricsPath = handler.DefaultMetricsPath
	}
	router.GET(conf.Prometheus.MetricsPath, promHandler.HandleMetrics)
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	router.NoRoute(addRequestID, returnNotFound)
	router.RedirectTrailingSlash = false

	return router, nil
}

func addRequestID(c *gin.Context) {
	requestID := ""
	if requestID = c.Request.Header.Get(protocol.RequestIDHeader); requestID == "" {
		requestID = NewReqID()
		c.Request.Header.Set(protocol.RequestIDHeader, requestID)
	}
	xl := xlog.New(requestID)
	xl.Debugf("request: %s %s", c.Request.Method, c.Request.URL.Path)
	c.Set(protocol.XLogKey, xl)
	c.Set(protocol.RequestStartKey, time.Now())
}

func returnNotFound(c *gin.Context) {
	xl := c.MustGet(protocol.XLogKey).(*xlog.Logger)
	httpErr := errors.NewHTTPErrorNotFound().WithRequestID(xl.ReqId)
	xl.Debugf("%s %s: not found", c.Request.Method, c.Request.URL.Path)
	c.JSON(http.StatusNotFound, httpErr)
}
