package errors

import (
	"encoding/json"
	"fmt"
)

// HTTPError HTTP 请求错误。
type HTTPError struct {
	// 自定义错误码。
	Code int `json:"code"`
	// 请求ID。
	RequestID string `json:"requestID"`
	// Summary
	Summary string `json:"summary"`
	// Message 错误消息。
	Message string `json:"message"`
}

// HTTPError 系列，规定HTTP请求出错时的错误码。
const (
	HTTPErrorBadRequest         = 400000
	HTTPErrorInvalidPhoneNumber = 400001
	HTTPErrorBadLoginType       = 400005
	HTTPErrorUnauthorized       = 401000
	HTTPErrorNotLoggedIn        = 401001
	HTTPErrorWrongSMSCode       = 401002
	HTTPErrorBadToken           = 401003
	HTTPErrorAlreadyLoggedIn    = 401004
	HTTPErrorNotFound           = 404000
	HTTPErrorNoSuchUser         = 404001
	HTTPErrorSMSSendTooFrequent = 429001
	HTTPErrorInternal           = 500000
)

// WithMessage 为HTTP错误添加详细消息。
func (e *HTTPError) WithMessage(msg string) *HTTPError {
	e.Message = msg
	return e
}

// WithMessagef 使用printf的方式为HTTP错误添加格式化输出的消息。
func (e *HTTPError) WithMessagef(format string, args ...interface{}) *HTTPError {
	msg := fmt.Sprintf(format, args...)
	return e.WithMessage(msg)
}

// WithRequestID 设置request ID。
func (e *HTTPError) WithRequestID(requestID string) *HTTPError {
	e.RequestID = requestID
	return e
}

func (e *HTTPError) Error() string {
	buf, err := json.Marshal(e)
	if err != nil {
		return err.Error()
	}
	return string(buf)
}

// NewHTTPErrorBadRequest 一般的HTTP bad request 错误。
func NewHTTPErrorBadRequest() *HTTPError {
	return &HTTPError{
		Code:    HTTPErrorBadRequest,
		Summary: "bad request",
	}
}

// NewHTTPErrorInvalidPhoneNumber 不符合规则的手机号码。
func NewHTTPErrorInvalidPhoneNumber() *HTTPError {
	return &HTTPError{
		Code:    HTTPErrorInvalidPhoneNumber,
		Summary: "invalid phone number",
	}
}

// NewHTTPErrorBadLoginType 不支持的登录方式。
func NewHTTPErrorBadLoginType() *HTTPError {
	return &HTTPError{
		Code:    HTTPErrorBadLoginType,
		Summary: "unsupported login type",
	}
}

// NewHTTPErrorUnauthorized 一般的HTTP Unauthorized 错误。
func NewHTTPErrorUnauthorized() *HTTPError {
	return &HTTPError{
		Code:    HTTPErrorUnauthorized,
		Summary: "unauthorized",
	}
}

// NewHTTPErrorNotLoggedIn 用户未登录。
func NewHTTPErrorNotLoggedIn() *HTTPError {
	return &HTTPError{
		Code:    HTTPErrorNotLoggedIn,
		Summary: "not logged in",
	}
}

// NewHTTPErrorBadToken 登录token错误。
func NewHTTPErrorBadToken() *HTTPError {
	return &HTTPError{
		Code:    HTTPErrorBadToken,
		Summary: "bad token",
	}
}

// NewHTTPErrorWrongSMSCode 用户短信验证码错误。
func NewHTTPErrorWrongSMSCode() *HTTPError {
	return &HTTPError{
		Code:    HTTPErrorWrongSMSCode,
		Summary: "wrong sms code",
	}
}

// NewHTTPErrorAlreadyLoggedin 用户已经登录，此为重复登录
func NewHTTPErrorAlreadyLoggedin() *HTTPError {
	return &HTTPError{
		Code:    HTTPErrorAlreadyLoggedIn,
		Summary: "already logged in",
	}
}

// NewHTTPErrorNotFound 一般的HTTP not found 错误。
func NewHTTPErrorNotFound() *HTTPError {
	return &HTTPError{
		Code:    HTTPErrorNotFound,
		Summary: "not found",
	}
}

// NewHTTPErrorNoSuchUser 无此用户。
func NewHTTPErrorNoSuchUser() *HTTPError {
	return &HTTPError{
		Code:    HTTPErrorNoSuchUser,
		Summary: "no such user",
	}
}

// NewHTTPErrorSMSSendTooFrequent 短信验证码已发送，短时间内不能重复发送。
func NewHTTPErrorSMSSendTooFrequent() *HTTPError {
	return &HTTPError{
		Code:    HTTPErrorSMSSendTooFrequent,
		Summary: "send sms code request limited",
	}
}

// NewHTTPErrorInternal 其他内部服务错误。
func NewHTTPErrorInternal() *HTTPError {
	return &HTTPError{
		Code:    HTTPErrorInternal,
		Summary: "internal server error",
	}
}
