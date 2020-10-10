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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qiniu/x/xlog"

	"github.com/qrtc/qlive/errors"
	"github.com/qrtc/qlive/protocol"
)

// UploadService 提供文件上传服务。
type UploadService interface {
	// 获取上传文件token。
	GetUploadToken(xl *xlog.Logger, userID string, filename string, expireSeconds int) (token string, err error)
}

// UploadHandler 处理上传文件的API。
type UploadHandler struct {
	Upload UploadService
}

// DefaultUploadTokenExpireSeconds 默认的上传文件token过期时间，3600秒(1小时)。
const DefaultUploadTokenExpireSeconds = 3600

// GetUploadToken 获取token。
// @Tags qlive api upload
// @ID get-upload-token
// @Summary get upload token
// @Description get a token for calling upload service
// @Accept  json
// @Produce  json
// @Success 200 {object} protocol.GetUploadTokenResponse
// @Failure 400 {object} errors.HTTPError
// @Router /feedbacks [post]
func (h *UploadHandler) GetUploadToken(c *gin.Context) {
	xl := c.MustGet(protocol.XLogKey).(*xlog.Logger)
	requestID := xl.ReqId

	userID := c.GetString(protocol.UserIDContextKey)
	args := protocol.GetUploadTokenArgs{}
	err := c.BindJSON(&args)
	if err != nil {
		xl.Infof("invalid args in body, error %v", err)
		httpErr := errors.NewHTTPErrorBadRequest().WithRequestID(requestID).WithMessage("invalid args in request body")
		c.JSON(http.StatusBadRequest, httpErr)
		return
	}
	tsSecond := time.Now().Unix()
	expireSeconds := args.ExpireSeconds
	if expireSeconds <= 0 {
		expireSeconds = DefaultUploadTokenExpireSeconds
	}
	token, err := h.Upload.GetUploadToken(xl, userID, args.Filename, args.ExpireSeconds)
	if err != nil {
		xl.Errorf("failed to get upload token, error %v", err)
		httpErr := errors.NewHTTPErrorInternal().WithRequestID(requestID).WithMessage("failed to send feedback")
		c.JSON(http.StatusInternalServerError, httpErr)
		return
	}
	resp := &protocol.GetUploadTokenResponse{
		Token:    token,
		ExpireAt: tsSecond + int64(expireSeconds),
	}
	c.JSON(http.StatusOK, resp)
}
