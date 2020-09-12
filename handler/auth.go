package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/qrtc/qlive/errors"
	"github.com/qrtc/qlive/protocol"
)

// AuthHandler 处理请求鉴权的需求。
type AuthHandler struct {
	Auth AuthInterface
}

// AuthInterface 对用户请求进行鉴权。
type AuthInterface interface {
	GetIDByToken(token string) (id string, err error)
}

// Authenticate 校验请求者的身份。
func (h *AuthHandler) Authenticate(c *gin.Context) {

	token, err := c.Cookie(protocol.LoginCookieKey)
	if err != nil {
		httpError := errors.NewHTTPErrorUnauthorized().WithMessage("login cookie not found")
		c.JSON(http.StatusUnauthorized, httpError)
		c.Abort()
		return
	}
	id, err := h.Auth.GetIDByToken(token)

	if err != nil {
		httpError := errors.NewHTTPErrorUnauthorized().WithMessage("failed to authenticate with token")
		c.JSON(http.StatusUnauthorized, httpError)
		c.Abort()
		return
	}
	c.Set(protocol.UserIDContextKey, id)
}
