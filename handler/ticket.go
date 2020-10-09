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

// TicketInterface 工单接口。
type TicketInterface interface {
	SubmitTicket(xl *xlog.Logger, ticket *protocol.Ticket) (ticketID string, err error)
}

// TicketHandler 处理工单相关接口。
type TicketHandler struct {
	Ticket TicketInterface
}

// SubmitTicket 提交工单。
// @Tags qlive api ticket
// @ID submit-ticket
// @Summary submit a ticket
// @Description submit a ticket if user has any problem or advice
// @Accept  json
// @Produce  json
// @Success 200 {object} protocol.SubmitTicketResponse
// @Failure 400 {object} errors.HTTPError
// @Router /tickets [post]
func (h *TicketHandler) SubmitTicket(c *gin.Context) {
	xl := c.MustGet(protocol.XLogKey).(*xlog.Logger)
	requestID := xl.ReqId

	userID := c.GetString(protocol.UserIDContextKey)
	args := protocol.SubmitTicketArgs{}
	err := c.BindJSON(&args)
	if err != nil {
		xl.Infof("invalid args in body, error %v", err)
		httpErr := errors.NewHTTPErrorBadRequest().WithRequestID(requestID).WithMessage("invalid args in request body")
		c.JSON(http.StatusBadRequest, httpErr)
		return
	}
	ticket := &protocol.Ticket{
		Submitter:    userID,
		Content:      args.Content,
		SDKLogURL:    args.SDKLogURL,
		SnapshotURLs: args.SnapshotURLs,
		SubmitTime:   time.Now(),
	}
	id, err := h.Ticket.SubmitTicket(xl, ticket)
	if err != nil {
		xl.Errorf("failed to submit ticket, error %v", err)
		httpErr := errors.NewHTTPErrorInternal().WithRequestID(requestID).WithMessage("failed to submit ticket")
		c.JSON(http.StatusInternalServerError, httpErr)
		return
	}
	resp := &protocol.SubmitTicketResponse{TicketID: id}
	c.JSON(http.StatusOK, resp)
}
