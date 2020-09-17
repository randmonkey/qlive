package errors

import "encoding/json"

// ServerError 服务端内部错误与非正常返回结果定义
type ServerError struct {
	Code    int    `json:"code"`
	Summary string `json:"summary"`
}

func (e *ServerError) Error() string {
	buf, _ := json.Marshal(e)
	return string(buf)
}

// 各种服务端内部错误的错误码定义。错误码为5位数字。
const (
	// 1开头表示服务端内部，或数据库访问相关的错误。
	ServerErrorUserNotLoggedin      = 10001
	ServerErrorUserLoggedin         = 10002
	ServerErrorUserNoPermission     = 10003
	ServerErrorUserNotfound         = 10004
	ServerErrorRoomNotFound         = 10005
	ServerErrorRoomNameUsed         = 10006
	ServerErrorTooManyRooms         = 10007
	ServerErrorCanOnlyCreateOneRoom = 10008
	ServerErrorSMSSendTooFrequent   = 10011
	ServerErrorMongoOpFail          = 11000
	// 2开头表示外部服务错误。
	ServerErrorSMSSendFail = 20001
)
