package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// HTTPError HTTP 请求错误。
type HTTPError struct {
	// HTTP 状态码。
	Code int `json:"code"`
	// 请求ID。
	RequestID string `json:"requestID"`
	// Summary
	Summary string `json:"summary"`
	// Message 错误消息。
	Message string `json:"message"`
}

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
		Code:    http.StatusBadRequest,
		Summary: "bad request",
	}
}

// NewHTTPErrorUnauthorized 一般的HTTP Unauthorized 错误。
func NewHTTPErrorUnauthorized() *HTTPError {
	return &HTTPError{
		Code:    http.StatusUnauthorized,
		Summary: "unauthorized",
	}
}

// NewHTTPErrorNotFound 一般的HTTP not found 错误。
func NewHTTPErrorNotFound() *HTTPError {
	return &HTTPError{
		Code:    http.StatusNotFound,
		Summary: "not found",
	}
}
