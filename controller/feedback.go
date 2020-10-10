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
	"fmt"
	"strconv"
	"time"

	"github.com/qiniu/qmgo"
	qmgoOpts "github.com/qiniu/qmgo/options"
	"github.com/qiniu/x/xlog"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/qrtc/qlive/protocol"
)

// FeedbackController 反馈消息控制器，执行发送反馈消息等操作。
type FeedbackController struct {
	mongoClient  *qmgo.Client
	feedbackColl *qmgo.Collection
	counterColl  *qmgo.Collection
	processQueue chan *protocol.Feedback
	xl           *xlog.Logger
}

const (
	FeedbackObjectName         = "feedback"
	FeedbackProcessQueueLength = 16
)

// NewFeedbackController 创建反馈消息控制器。
func NewFeedbackController(mongoURI string, database string, xl *xlog.Logger) (*FeedbackController, error) {
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
			c.xl.Infof("got feedback message %s from %s", feedback.ID, feedback.Sender)
			// TODO：进行进一步的处理，如发送邮件等
		}
	}
}
