package handler

import (
	"bytes"
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
		Account: &MockAccount{},
		SMSCode: &MockSMSCode{
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
		c.Set(protocol.XLogKey, xlog.New(fmt.Sprintf("test-login-sms-%d", i)))
		// 构造request.
		bodyBuf := &bytes.Buffer{}
		req, err := http.NewRequest("POST", "/v1/send_sms_code?phone_number="+testCase.phoneNumber, bodyBuf)
		assert.Nilf(t, err, "construct request failed for test case %d, error %v", i, err)
		c.Request = req
		// handler处理请求。
		handler.SendSMSCode(c)
		assert.Equalf(t, testCase.expectedStatusCode, w.Code, "code is not the same as expectrd for test case %d", i)
	}
}
