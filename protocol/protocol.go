package protocol

import "encoding/json"

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

// WebSocket Message Type
const (
	MT_Ping       = "ping"
	MT_Pong       = "pong"
	MT_Authorize  = "authorize"
	MT_Disconnect = "disconnect"
)

type Ping struct {
}

func (p *Ping) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *Ping) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

type Pong struct {
}

func (p *Pong) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *Pong) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

type Authorize struct {
	RPCID          string `json:"rpcID,omitempty"`
	Token          string `json:"token"`
	ReconnectToken int    `json:"reconnectToken"`
	MessageSN      int    `json:"messageSN"`
	PlayerData     string `json:"playerData"`
	Agent          string `json:"agent"`
	SDKVersion     string `json:"sdkVersion"`
}

func (p *Authorize) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *Authorize) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

type AuthorizeResponse struct {
	RPCID          string `json:"rpcID,omitempty"`
	Code           int    `json:"code"`
	Error          string `json:"error"`
	ReconnectToken int    `json:"reconnectToken"`
}

func (p *AuthorizeResponse) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *AuthorizeResponse) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

type Disconnect struct {
	Code     int    `json:"code"`
	Error    string `json:"error"`
	KickedID string `json:"kickedID"`
}

func (p *Disconnect) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *Disconnect) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}
