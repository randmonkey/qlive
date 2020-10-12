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
	"crypto/tls"
	"fmt"
	"strconv"
	"time"

	"github.com/qiniu/qmgo"
	qmgoOpts "github.com/qiniu/qmgo/options"
	"github.com/qiniu/x/xlog"
	"go.mongodb.org/mongo-driver/bson"
	"gopkg.in/gomail.v2"

	"github.com/qrtc/qlive/config"
	"github.com/qrtc/qlive/protocol"
)

// FeedbackController 反馈消息控制器，执行发送反馈消息等操作。
type FeedbackController struct {
	mongoClient  *qmgo.Client
	feedbackColl *qmgo.Collection
	counterColl  *qmgo.Collection
	accountColl  *qmgo.Collection
	mailConfig   *config.MailConfig
	processQueue chan *protocol.Feedback
	xl           *xlog.Logger
}

const (
	FeedbackObjectName         = "feedback"
	FeedbackProcessQueueLength = 16
)

// NewFeedbackController 创建反馈消息控制器。
func NewFeedbackController(mongoURI string, database string, mailConf *config.MailConfig, xl *xlog.Logger) (*FeedbackController, error) {
	if xl == nil {
		xl = xlog.New("qlive-feedback-controller")
	}
	mongoClient, err := qmgo.NewClient(context.Background(), &qmgo.Config{
		Uri:      mongoURI,
		Database: database,
	})
	if err != nil {
		xl.Errorf("failed to create mongo client, error %v", err)
		return nil, err
	}
	feedbackColl := mongoClient.Database(database).Collection(FeedbackCollection)
	counterColl := mongoClient.Database(database).Collection(CounterCollection)
	accountColl := mongoClient.Database(database).Collection(AccountCollection)
	feedbackCounter := &protocol.ObjectCounter{
		ID: FeedbackObjectName,
	}
	err = counterColl.Find(context.Background(), bson.M{"_id": FeedbackObjectName}).One(feedbackCounter)
	if err != nil {
		if qmgo.IsErrNoDocuments(err) {
			xl.Infof("create feedback counter...")
			feedbackCounter.SequenceNumber = 1
			_, err = counterColl.InsertOne(context.Background(), feedbackCounter)
			if err != nil {
				xl.Errorf("failed to create feedback counter, error %v", err)
				return nil, err
			}
		} else {
			xl.Errorf("failed to get counter for feedback, error %v", err)
			return nil, err
		}
	}
	controller := &FeedbackController{
		mongoClient:  mongoClient,
		feedbackColl: feedbackColl,
		counterColl:  counterColl,
		accountColl:  accountColl,
		mailConfig:   mailConf,
		processQueue: make(chan *protocol.Feedback, FeedbackProcessQueueLength),
		xl:           xl,
	}
	go controller.ProcessFeedbacks()
	return controller, nil
}

type insertFeedbackHook struct {
	counterColl  *qmgo.Collection
	feedback     *protocol.Feedback
	processQueue chan *protocol.Feedback
	xl           *xlog.Logger
}

func (h *insertFeedbackHook) BeforeInsert() error {
	if h.feedback == nil {
		return fmt.Errorf("empty object")
	}
	if h.feedback.ID == "" {
		counter := &protocol.ObjectCounter{}
		err := h.counterColl.Find(context.Background(), bson.M{"_id": FeedbackObjectName}).One(counter)
		if err != nil {
			h.xl.Errorf("failed to find feedback counter, error %v", err)
			return err
		}
		h.feedback.ID = strconv.FormatInt(counter.SequenceNumber, 10)
		counter.SequenceNumber++
		err = h.counterColl.UpdateOne(context.Background(), bson.M{"_id": FeedbackObjectName}, bson.M{"$set": counter})
		if err != nil {
			h.xl.Errorf("failed to update feedback counter, error %v", err)
			return err
		}
	}
	return nil
}

func (h *insertFeedbackHook) AfterInsert() error {
	h.processQueue <- h.feedback
	return nil
}

// SendFeedback 提交反馈消息，并返回反馈消息ID。
func (c *FeedbackController) SendFeedback(xl *xlog.Logger, feedback *protocol.Feedback) (string, error) {
	if xl == nil {
		xl = c.xl
	}
	if feedback.SendTime.IsZero() {
		feedback.SendTime = time.Now()
	}

	res, err := c.feedbackColl.InsertOne(context.Background(), feedback, qmgoOpts.InsertOneOptions{
		InsertHook: &insertFeedbackHook{
			counterColl:  c.counterColl,
			feedback:     feedback,
			processQueue: c.processQueue,
			xl:           xl,
		},
	})
	if err != nil {
		xl.Errorf("failed to insert feedback message into mongo, error %v", err)
		return "", err
	}
	return res.InsertedID.(string), nil
}

// ProcessFeedbacks 处理反馈消息。
func (c *FeedbackController) ProcessFeedbacks() {
	for {
		select {
		case feedback := <-c.processQueue:
			c.xl.Debugf("got feedback message %s from %s", feedback.ID, feedback.Sender)
			if c.mailConfig != nil && c.mailConfig.Enabled {
				c.xl.Debugf("send feedback message by email")
				err := c.sendFeedbackByMail(feedback)
				if err != nil {
					c.xl.Warnf("failed to send feed back by email,error %v", err)
				}
			}
		}
	}
}

func (c *FeedbackController) sendFeedbackByMail(feedback *protocol.Feedback) error {
	senderAccount := &protocol.Account{}
	err := c.accountColl.Find(context.Background(), bson.M{"_id": feedback.Sender}).One(senderAccount)
	if err != nil {
		c.xl.Warnf("failed to get account info of sender %s", feedback.Sender)
	}
	// make subject and mail body.
	subject := fmt.Sprintf("互动直播反馈消息:消息ID %s", feedback.ID)
	msg := fmt.Sprintf("互动直播反馈消息:消息ID %s\n", feedback.ID)
	msg = msg + fmt.Sprintf("发送时间：%s\n", feedback.SendTime.Format("2006-01-02 15:04:05-0700"))
	msg = msg + fmt.Sprintf("发送者ID： %s\n", feedback.Sender)
	msg = msg + fmt.Sprintf("发送者手机号：%s\n", senderAccount.PhoneNumber)
	msg = msg + fmt.Sprintf("消息内容：%s\n", feedback.Content)
	msg = msg + fmt.Sprintf("相关附件URL：%s\n", feedback.AttachementURL)
	// generate message.
	d := gomail.NewDialer(c.mailConfig.SMTPHost, c.mailConfig.SMTPPort, c.mailConfig.Username, c.mailConfig.Password)
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	m := gomail.NewMessage()
	m.SetHeader("From", c.mailConfig.From)
	m.SetHeader("To", c.mailConfig.To...)
	m.SetHeader("Subject", subject)
	m.SetBody("text/plain", msg)
	// send email.
	if err := d.DialAndSend(m); err != nil {
		c.xl.Warnf("send feedback %s by email failed: error %+v", feedback.ID, err)
		return err
	}
	return nil
}
