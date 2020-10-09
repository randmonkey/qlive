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

package controller

import (
	"context"
	"time"

	"github.com/qiniu/qmgo"
	"github.com/qiniu/x/xlog"

	"github.com/qrtc/qlive/protocol"
)

// TicketController 工单控制器，执行提交工单等操作。
type TicketController struct {
	mongoClient *qmgo.Client
	ticketColl  *qmgo.Collection
	xl          *xlog.Logger
}

// NewTicketController 创建工单控制器。
func NewTicketController(mongoURI string, database string, xl *xlog.Logger) (*TicketController, error) {
	if xl == nil {
		xl = xlog.New("qlive-ticket-controller")
	}
	mongoClient, err := qmgo.NewClient(context.Background(), &qmgo.Config{
		Uri:      mongoURI,
		Database: database,
	})
	if err != nil {
		xl.Errorf("failed to create mongo client, error %v", err)
		return nil, err
	}
	ticketColl := mongoClient.Database(database).Collection(TicketCollection)
	return &TicketController{
		mongoClient: mongoClient,
		ticketColl:  ticketColl,
		xl:          xl,
	}, nil
}

// SubmitTicket 提交工单，并返回工单ID。
func (c *TicketController) SubmitTicket(xl *xlog.Logger, ticket *protocol.Ticket) (string, error) {
	if xl == nil {
		xl = c.xl
	}
	if ticket.SubmitTime.IsZero() {
		ticket.SubmitTime = time.Now()
	}
	res, err := c.ticketColl.InsertOne(context.Background(), ticket)
	if err != nil {
		xl.Errorf("failed to insert ticket into mongo, error %v", err)
		return "", err
	}
	return res.InsertedID.(string), nil
}
