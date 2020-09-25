package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/qiniu/x/xlog"

	"github.com/qrtc/qlive/errors"
	"github.com/qrtc/qlive/protocol"
)

// IMInterface IM用户管理相关接口。
type IMInterface interface {
	GetUserToken(xl *xlog.Logger, userID string) (imUser *protocol.IMUser, err error)
}

// IMHandler 处理IM相关API。
type IMHandler struct {
	IMService IMInterface
}

// @Tags qlive api
// @ID get-user-token
// @Summary Get user token
// @Description User Gets user-token
// @Accept  json
// @Produce  json
// @Success 200 {object} protocol.IMTokenResponse
// @Failure 502 {object} errors.HTTPError
// @Router /im_user_token [post]
// GetUserToken 获取IM用户token。
func (h *IMHandler) GetUserToken(c *gin.Context) {
	xl := c.MustGet(protocol.XLogKey).(*xlog.Logger)
	requestID := xl.ReqId

	userID := c.GetString(protocol.UserIDContextKey)
	imUser, err := h.IMService.GetUserToken(xl, userID)
	if err != nil {
		xl.Errorf("failed to call IM service to get token")
		httpErr := errors.NewHTTPErrorExternalService().WithRequestID(requestID).WithMessagef("failed to call IM service to get user token, error %v", err)
		c.JSON(http.StatusBadGateway, httpErr)
		return
	}

	resp := &protocol.IMTokenResponse{
		UserID: imUser.UserID,
		Token:  imUser.Token,
	}
	c.JSON(http.StatusOK, resp)
	return
}
