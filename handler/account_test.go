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
	"github.com/stretchr/testify/assert"

	"github.com/qrtc/qlive/errors"
	"github.com/qrtc/qlive/protocol"
)

func TestSendSMSCode(t *testing.T) {
	handler := &AccountHandler{
		Account: &mockAccount{},
		SMSCode: &mockSMSCode{
			NumberToError: map[string]error{
				"19999990000": &errors.ServerError{Code: errors.ServerErrorSMSSendTooFrequent},
				"19999990001": &errors.ServerError{Code: errors.ServerErrorSMSSendFail},
			},
		},
	}
	// 构造测试用例列表。
	testCases := []struct {
		phoneNumber        string
		expectedStatusCode int
	}{
		{
			phoneNumber:        "110",
			expectedStatusCode: 400,
		},
		{
			phoneNumber:        "19999990000",
			expectedStatusCode: 429,
		},
		{
			phoneNumber:        "19999990001",
			expectedStatusCode: 500,
		},
		{
			phoneNumber:        "19999990002",
			expectedStatusCode: 200,
		},
	}
	for i, testCase := range testCases {
		// 创建 HTTP record，记录返回结果
		w := httptest.NewRecorder()
		// 创建测试用 gin context.
		c, _ := gin.CreateTestContext(w)
		// 创建xlog.Logger，记录请求处理过程中的日志（handler函数必需）。
		c.Set(protocol.XLogKey, xlog.New(fmt.Sprintf("test-send-sms-code-%d", i)))
		// 构造request.
		bodyBuf := &bytes.Buffer{}
		req, err := http.NewRequest("POST", "/v1/send_sms_code?phone_number="+testCase.phoneNumber, bodyBuf)
		assert.Nilf(t, err, "construct request failed for test case %d, error %v", i, err)
		c.Request = req
		// handler处理请求。处理完成后将会把response写入到之前定义的recorder中。
		handler.SendSMSCode(c)
		// 判断recorder中的HTTP状态码。
		assert.Equalf(t, testCase.expectedStatusCode, w.Code, "code is not the same as expectrd for test case %d", i)
	}
}

func TestLogin(t *testing.T) {
	handler := &AccountHandler{
		Account: &mockAccount{},
		SMSCode: &mockSMSCode{},
	}

	testCases := []struct {
		loginType          string
		phoneNumber        string
		smsCode            string
		expectedStatusCode int
	}{
		{
			loginType:          "invalid",
			expectedStatusCode: 400,
		},
		{
			loginType:          "smscode",
			phoneNumber:        "19999990002",
			smsCode:            "123456",
			expectedStatusCode: 200,
		},
		{
			loginType:          "smscode",
			phoneNumber:        "19999990003",
			smsCode:            "123455",
			expectedStatusCode: 401,
		},
	}

	for i, testCase := range testCases {
		// 创建 HTTP recorder，记录返回结果
		w := httptest.NewRecorder()
		// 创建测试用 gin context.
		c, _ := gin.CreateTestContext(w)
		// 创建xlog.Logger，记录请求处理过程中的日志（handler函数必需）。
		c.Set(protocol.XLogKey, xlog.New(fmt.Sprintf("test-login-sms-%d", i)))
		// 构造request.
		bodyBuf := &bytes.Buffer{}
		loginReq := &protocol.SMSLoginArgs{
			PhoneNumber: testCase.phoneNumber,
			SMSCode:     testCase.smsCode,
		}
		buf, err := json.Marshal(loginReq)
		assert.Nilf(t, err, "construct request body failed for test case %d, error %v", i, err)
		bodyBuf.Write(buf)
		req, err := http.NewRequest("POST", "/v1/login?logintype="+testCase.loginType, bodyBuf)
		assert.Nilf(t, err, "construct request failed for test case %d, error %v", i, err)
		c.Request = req
		// handler处理请求。
		handler.Login(c)
		assert.Equalf(t, testCase.expectedStatusCode, w.Code, "code is not the same as expected for test case %d", i)
	}
}

func TestUpdateProfile(t *testing.T) {
	handler := &AccountHandler{
		Account: &mockAccount{
			accounts: []*protocol.Account{
				{ID: "user-0"},
			},
		},
		SMSCode: &mockSMSCode{},
	}
	testCases := []struct {
		userID             string
		nickname           string
		gender             string
		expectedStatusCode int
	}{
		{
			userID:             "user-0",
			nickname:           "Alice",
			gender:             "female",
			expectedStatusCode: 200,
		},
		{
			userID:             "user-1",
			nickname:           "Bob",
			gender:             "male",
			expectedStatusCode: 404,
		},
	}
	for i, testCase := range testCases {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		c.Set(protocol.XLogKey, xlog.New(fmt.Sprintf("test-update-profile-%d", i)))
		c.Set(protocol.UserIDContextKey, testCase.userID)
		profileReq := &protocol.UpdateProfileArgs{
			Nickname: testCase.nickname,
			Gender:   testCase.gender,
		}

		buf, err := json.Marshal(profileReq)
		assert.Nilf(t, err, "failed to build request body for case %d", i)
		bodyReader := bytes.NewBuffer(buf)
		req, err := http.NewRequest("POST", "/v1/profile", bodyReader)
		assert.Nilf(t, err, "failed to build request for case %d", i)
		c.Request = req

		handler.UpdateProfile(c)
		assert.Equalf(t, testCase.expectedStatusCode, w.Code, "code is not the same as expected for test case %d", i)
	}

}
