package service

import (
	"github.com/gin-gonic/gin"
	"github.com/qiniu/x/xlog"

	"github.com/qrtc/qlive/config"
	"github.com/qrtc/qlive/controller"
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
	v1 := router.Group("/v1")
	{
		v1.POST("login", addRequestID, accountHandler.Login)
		v1.POST("send_sms_code", addRequestID, accountHandler.SendSMSCode)
		v1.POST("profile", addRequestID, authHandler.Authenticate, accountHandler.UpdateProfile)
		v1.POST("logout", addRequestID, authHandler.Authenticate, accountHandler.Logout)
	}
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
