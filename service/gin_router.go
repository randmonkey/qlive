package service

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/qiniu/x/xlog"

	"github.com/qrtc/qlive/config"
	"github.com/qrtc/qlive/controller"
	"github.com/qrtc/qlive/errors"
	"github.com/qrtc/qlive/handler"
	"github.com/qrtc/qlive/protocol"
)

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
	}

	v1 := router.Group("/v1")
	{
		// 账号相关API。
		v1.POST("login", addRequestID, accountHandler.Login)
		v1.POST("send_sms_code", addRequestID, accountHandler.SendSMSCode)
		v1.POST("profile", addRequestID, authHandler.Authenticate, accountHandler.UpdateProfile)
		v1.POST("logout", addRequestID, authHandler.Authenticate, accountHandler.Logout)

		// 主播端API：创建、关闭房间。
		v1.POST("rooms", addRequestID, authHandler.Authenticate, roomHandler.CreateRoom)
		v1.POST("close_room", addRequestID, authHandler.Authenticate, roomHandler.CloseRoom)

		// 观众端API：进入、退出房间。
		v1.POST("enter_room", addRequestID, authHandler.Authenticate, roomHandler.EnterRoom)
		v1.POST("leave_room", addRequestID, authHandler.Authenticate, roomHandler.LeaveRoom)

		// 观众端/主播端API：获取全部房间或者PK房间。
		v1.GET("rooms", addRequestID, authHandler.Authenticate, roomHandler.ListRooms)
	}
	router.NoRoute(addRequestID, returnNotFound)
	return router, nil
}

func addRequestID(c *gin.Context) {
	requestID := ""
	if requestID = c.Request.Header.Get(protocol.RequestIDHeader); requestID == "" {
		requestID = NewReqID()
		c.Request.Header.Set(protocol.RequestIDHeader, requestID)
	}
	xl := xlog.New(requestID)
	c.Set(protocol.XLogKey, xl)
}

func returnNotFound(c *gin.Context) {
	xl := c.MustGet(protocol.XLogKey).(*xlog.Logger)
	httpErr := errors.NewHTTPErrorNotFound().WithRequestID(xl.ReqId)
	xl.Debugf("%s %s: not found", c.Request.Method, c.Request.URL.Path)
	c.JSON(http.StatusNotFound, httpErr)
}
