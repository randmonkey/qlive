package router

import (
	"github.com/gin-gonic/gin"

	"github.com/qrtc/qlive/handler"
)

// NewRouter 返回gin router，分流API。
func NewRouter() *gin.Engine {
	router := gin.New()
	accountHandler := &handler.AccountHandler{
		Account: &handler.MockAccount{},
		SMSCode: &handler.MockSMSCode{},
	}
	authHandler := &handler.AuthHandler{
		Auth: &handler.MockAuth{},
	}
	v1 := router.Group("/v1")
	{
		v1.GET("hello", func(c *gin.Context) { c.Writer.WriteString("Hello qiniu") })
		v1.POST("login", accountHandler.Login)
		v1.GET("smscode", accountHandler.GetSMSCode)
		v1.POST("profile", authHandler.Authenticate, accountHandler.UpdateProfile)
		v1.POST("logout", authHandler.Authenticate, accountHandler.Logout)
	}
	return router
}
