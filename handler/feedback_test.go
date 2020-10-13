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
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qiniu/x/xlog"
	"github.com/qrtc/qlive/protocol"
	"github.com/stretchr/testify/assert"
)

func TestSendFeedback(t *testing.T) {
	testCases := []struct {
		userID              string
		content             string
		attachmentURL       string
		storedAttachmentURL string
	}{
		{
			userID:              "user-1",
			content:             "problem",
			attachmentURL:       "example.com/feedback1.zip",
			storedAttachmentURL: "example.com/feedback1.zip",
		},
		{
			userID:              "user-2",
			content:             "problem2",
			attachmentURL:       "aa/feedback2.zip",
			storedAttachmentURL: "example.com/aa/feedback2.zip",
		},
		{
			userID:              "user-3",
			content:             "problem3",
			attachmentURL:       "http://example.com:8080/feedback3.zip",
			storedAttachmentURL: "http://example.com:8080/feedback3.zip",
		},
		{
			userID:              "user-4",
			content:             "problem4",
			attachmentURL:       "127.0.0.1/feedback4.zip",
			storedAttachmentURL: "127.0.0.1/feedback4.zip",
		},
		{
			userID:              "user-5",
			content:             "problem5",
			attachmentURL:       "127.0.0.1:8080/feedback5.zip",
			storedAttachmentURL: "127.0.0.1:8080/feedback5.zip",
		},
		{
			userID:              "user-6",
			content:             "problem6",
			attachmentURL:       "",
			storedAttachmentURL: "",
		},
	}

	for i, testCase := range testCases {
		mockFeedback := &mockFeedback{}
		handler := &FeedbackHandler{
			Feedback:            mockFeedback,
			AttachmentURLPrefix: "example.com",
		}
		// intitialize test recorder and context
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(protocol.XLogKey, xlog.New(fmt.Sprintf("test-send-feedback-%d", i)))
		c.Set(protocol.UserIDContextKey, testCase.userID)
		// build request
		args := &protocol.SendFeedbackArgs{
			Content:       testCase.content,
			AttachmentURL: testCase.attachmentURL,
		}
		buf, err := json.Marshal(args)
		assert.Nilf(t, err, "failed to build request body for case %d, error %v", i, err)
		bodyReader := bytes.NewBuffer(buf)
		req, err := http.NewRequest("POST", "/v1/feedbacks", bodyReader)
		assert.Nilf(t, err, "failed to craete request for case %d, error %v", i, err)
		c.Request = req

		handler.SendFeedback(c)
		assert.Equalf(t, http.StatusOK, w.Code, "test case %d got non-200 status code: %d", i, w.Code)

		res := &protocol.SendFeedbackResponse{}
		err = json.Unmarshal(w.Body.Bytes(), res)
		assert.Nilf(t, err, "failed to read response for test case %d,error %v", i, err)
		// check feedbacks stored in mockFeedback
		id := res.FeedbackID
		assert.Lenf(t, mockFeedback.feedbacks, 1, "test case %d: should store 1 feedback message", i)
		storedFeedback := mockFeedback.feedbacks[0]
		assert.Equalf(t, id, storedFeedback.ID, "test case %d: feedback ID does not match", i)
		assert.Equalf(t, testCase.userID, storedFeedback.Sender, "test case %d: sender should be the same as expected", i)
		assert.Equalf(t, testCase.storedAttachmentURL, storedFeedback.AttachementURL, "test case %d: attachment URL should be the same as expected", i)
	}
}
