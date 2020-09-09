package protocol

/*
	protocol.go: 规定API的参数与返回值的定义，***Args 表示 *** 接口的参数，***Response表示 *** 接口的返回体格式。
*/

// SMSLoginArgs 通过短信登录的参数
type SMSLoginArgs struct {
	PhoneNumber string `json:"phoneNumber"`
	SMSCode     string `json:"smsCode"`
}

// LoginResponse 登录的返回结果。
type LoginResponse struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
	Gender   string `json:"gender"`
}

// LoginCookieKey 登录用的token，存放在cookie中。
const LoginCookieKey = "qlive-login-token"

// UserIDContextKey 存放在请求context 中的用户ID。
const UserIDContextKey = "userID"

// UpdateProfileArgs 修改用户信息接口。
type UpdateProfileArgs struct {
	Nickname string `json:"nickname"`
	Gender   string `json:"gender"`
}

// UpdateProfileResponse 修改用户信息的返回结果。
type UpdateProfileResponse struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
	Gender   string `json:"gender"`
}
