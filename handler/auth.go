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
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/qiniu/x/xlog"

	"github.com/qrtc/qlive/errors"
	"github.com/qrtc/qlive/protocol"
)

// AuthHandler 处理请求鉴权的需求。
type AuthHandler struct {
	Auth AuthInterface
}

// AuthInterface 对用户请求进行鉴权。
type AuthInterface interface {
	GetIDByToken(xl *xlog.Logger, token string) (id string, err error)
}

// Authenticate 校验请求者的身份。
func (h *AuthHandler) Authenticate(c *gin.Context) {
	xl := c.MustGet(protocol.XLogKey).(*xlog.Logger)
	requestID := xl.ReqId
	// 优先根据Authorization:Bearer <token>校验。
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		xl.Debug("authorization header is empty or in wrong format")
		xl.Debugf("auth header: %v", authHeader)
		xl.Debugf("%s %s: request unauthorized, wrong auth header format", c.Request.Method, c.Request.URL.Path)
		httpError := errors.NewHTTPErrorNotLoggedIn().WithRequestID(requestID).WithMessage("user not logged in")
		c.JSON(http.StatusUnauthorized, httpError)
		c.Abort()
		return
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	id, err := h.Auth.GetIDByToken(xl, token)

	if err != nil {
		xl.Debugf("%s %s: request unauthorized, error %v", c.Request.Method, c.Request.URL.Path, err)
		httpError := errors.NewHTTPErrorBadToken().WithRequestID(requestID).WithMessage("failed to authenticate with token")
		c.JSON(http.StatusUnauthorized, httpError)
		c.Abort()
		return
	}
	c.Set(protocol.UserIDContextKey, id)
}
