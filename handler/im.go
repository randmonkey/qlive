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

package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/qiniu/x/xlog"

	"github.com/qrtc/qlive/controller"
	"github.com/qrtc/qlive/errors"
	"github.com/qrtc/qlive/protocol"
)

// IMHandler 处理IM相关API。
type IMHandler struct {
	IMService controller.IMInterface
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

// ProcessMessage 处理IM消息。
// @ID process-im-message
// @Summary process IM messages
// @Description callback API to receive and process messages from IM
// @Accept json
// @Produce json
// @Success 200 {object} protocol.IMMessageResponse
// @Failure 400 {object} errors.HTTPError
// @Router /im_messages [post]
func (h *IMHandler) ProcessMessage(c *gin.Context) {
	xl := c.MustGet(protocol.XLogKey).(*xlog.Logger)
	requestID := xl.ReqId

	provider := c.Param("provider")
	switch provider {
	case "rongcloud":
		msg := &protocol.RongCloudMessage{}

		err := c.ShouldBind(msg)
		if err != nil {
			xl.Infof("failed to parse rongcloud message, error %v", err)
			httpErr := errors.NewHTTPErrorBadRequest().WithRequestID(requestID).WithMessage("invalid message body")
			c.JSON(http.StatusBadRequest, httpErr)
			return
		}
		sign := &protocol.RongCloudSignature{}
		err = c.ShouldBindQuery(sign)
		if err != nil {
			xl.Infof("failed to get rongcloud signature, error %v", err)
			httpErr := errors.NewHTTPErrorBadRequest().WithRequestID(requestID).WithMessage("invalid message signature")
			c.JSON(http.StatusBadRequest, httpErr)
			return
		}
		msg.Signature = *sign
		xl.Debugf("%+v", msg)
		h.IMService.ProcessMessage(xl, msg)
		c.JSON(http.StatusOK, struct{}{})
		return
	default:
		xl.Infof("unsupported IM provider %s", provider)
		httpErr := errors.NewHTTPErrorBadRequest().WithRequestID(requestID).WithMessage("unsupported IM provider")
		c.JSON(http.StatusBadRequest, httpErr)
		return
	}
}

// OnUserStatusChange 处理用户在线状态改变的回调消息。
func (h *IMHandler) OnUserStatusChange(c *gin.Context) {
	xl := c.MustGet(protocol.XLogKey).(*xlog.Logger)
	requestID := xl.ReqId

	provider := c.Param("provider")
	switch provider {
	case "rongcloud":
		statusList := map[int]*protocol.RongCloudUserStatus{}

		for i := 0; ; i++ {
			m := c.PostFormMap(strconv.Itoa(i))
			if m == nil || len(m) == 0 {
				break
			}
			userID := m["userid"]
			status := m["status"]
			userOS := m["os"]
			clientIP := m["clientIp"]
			if userID == "" || status == "" || userOS == "" || clientIP == "" {
				continue
			}
			userStatus := &protocol.RongCloudUserStatus{
				UserID:   userID,
				Status:   status,
				OS:       userOS,
				ClientIP: clientIP,
			}
			statusList[i] = userStatus
		}

		sign := &protocol.RongCloudSignature{}
		err := c.ShouldBindQuery(sign)
		if err != nil {
			xl.Infof("failed to get rongcloud signature, error %v", err)
			httpErr := errors.NewHTTPErrorBadRequest().WithRequestID(requestID).WithMessage("invalid message signature")
			c.JSON(http.StatusBadRequest, httpErr)
			return
		}
		for _, status := range statusList {
			userID := status.UserID
			switch status.Status {
			case string(protocol.RongCloudUserOnline):
				xl.Debugf("user %s rongcloud IM online", userID)
				h.IMService.UserOnline(xl, userID)
			case string(protocol.RongClouduserOffline):
				xl.Debugf("user %s rongcloud IM offline", userID)
				h.IMService.UserOffline(xl, userID)
			case string(protocol.RongCloudUserLogout):
				xl.Debugf("user %s rongcloud IM logout", userID)
				h.IMService.UserOffline(xl, userID)
			default:
				xl.Debugf("user %s undefined status %s", userID, status.Status)
			}
		}
	default:
		xl.Infof("unsupported IM provider %s", provider)
		httpErr := errors.NewHTTPErrorBadRequest().WithRequestID(requestID).WithMessage("unsupported IM provider")
		c.JSON(http.StatusBadRequest, httpErr)
		return
	}
	c.JSON(http.StatusOK, struct{}{})
}
