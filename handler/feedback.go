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
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qiniu/x/xlog"

	"github.com/qrtc/qlive/errors"
	"github.com/qrtc/qlive/protocol"
)

// FeedbackInterface 反馈消息接口。
type FeedbackInterface interface {
	SendFeedback(xl *xlog.Logger, feedback *protocol.Feedback) (feedbackID string, err error)
}

// FeedbackHandler 处理反馈消息相关API。
type FeedbackHandler struct {
	Feedback            FeedbackInterface
	AttachmentURLPrefix string
}

const (
	// DomainRegexStr 域名的正则表达式。
	DomainRegexStr = `^([a-zA-Z0-9_][a-zA-Z0-9_-]{0,62}){1}(\.[a-zA-Z0-9_]{1}[a-zA-Z0-9_-]{0,62})+$`
)

// SendFeedback 提交反馈消息。
// @Tags qlive api feedback
// @ID send-feedback
// @Summary send a feedback message
// @Description send a feedback message if user has any problem or advice
// @Accept  json
// @Produce  json
// @Success 200 {object} protocol.SendFeedbackResponse
// @Failure 400 {object} errors.HTTPError
// @Router /feedbacks [post]
func (h *FeedbackHandler) SendFeedback(c *gin.Context) {
	xl := c.MustGet(protocol.XLogKey).(*xlog.Logger)
	requestID := xl.ReqId

	userID := c.GetString(protocol.UserIDContextKey)
	args := protocol.SendFeedbackArgs{}
	err := c.BindJSON(&args)
	if err != nil {
		xl.Infof("invalid args in body, error %v", err)
		httpErr := errors.NewHTTPErrorBadRequest().WithRequestID(requestID).WithMessage("invalid args in request body")
		c.JSON(http.StatusBadRequest, httpErr)
		return
	}
	attachmentURL := args.AttachmentURL
	if attachmentURL != "" && h.shouldAddURLPrefix(attachmentURL) {
		attachmentURL = h.AttachmentURLPrefix + "/" + attachmentURL
	}

	feedback := &protocol.Feedback{
		Sender:         userID,
		Content:        args.Content,
		AttachementURL: attachmentURL,
		SendTime:       time.Now(),
	}
	id, err := h.Feedback.SendFeedback(xl, feedback)
	if err != nil {
		xl.Errorf("failed to send feedback, error %v", err)
		httpErr := errors.NewHTTPErrorInternal().WithRequestID(requestID).WithMessage("failed to send feedbacks")
		c.JSON(http.StatusInternalServerError, httpErr)
		return
	}
	resp := &protocol.SendFeedbackResponse{FeedbackID: id}
	c.JSON(http.StatusOK, resp)
}

func (h *FeedbackHandler) shouldAddURLPrefix(inputURL string) bool {
	if inputURL == "" {
		return false
	}
	if strings.HasPrefix(inputURL, "http://") {
		inputURL = strings.TrimPrefix(inputURL, "http://")
	} else if strings.HasPrefix(inputURL, "https://") {
		inputURL = strings.TrimPrefix(inputURL, "https://")
	}
	parts := strings.SplitN(inputURL, "/", 2)
	// does not have '/', return true
	if len(parts) < 2 {
		return true
	}
	// begin with a domain, e.g: example.com
	domainRegex := regexp.MustCompile(DomainRegexStr)
	if domainRegex.MatchString(parts[0]) {
		return false
	}
	// begin with an IP, e.g: 127.0.0.1
	if ip := net.ParseIP(parts[0]); ip != nil {
		return false
	}
	// begin with IP:port or domain:port, e.g: 127.0.0.1:8080
	if _, _, err := net.SplitHostPort(parts[0]); err == nil {
		return false
	}
	return true
}
