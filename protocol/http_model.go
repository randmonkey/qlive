package protocol

/*
	protocol.go: 规定API的参数与返回值的定义，***Args 表示 *** 接口的参数，***Response表示 *** 接口的返回体格式。
*/

const (
	// RequestIDHeader 七牛 request ID 头部。
	RequestIDHeader = "X-Reqid"
	// XLogKey gin context中，用于获取记录请求相关日志的 xlog logger的key。
	XLogKey = "xlog-logger"

	// LoginTokenKey 登录用的token。
	LoginTokenKey = "qlive-login-token"

	// UserIDContextKey 存放在请求context 中的用户ID。
	UserIDContextKey = "userID"
)

// UserInfo 用户的信息，包括ID、昵称等。
type UserInfo struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
	Gender   string `json:"gender"`
}

// SMSLoginArgs 通过短信登录的参数
type SMSLoginArgs struct {
	PhoneNumber string `json:"phoneNumber"`
	SMSCode     string `json:"smsCode"`
}

// LoginResponse 登录的返回结果。
type LoginResponse struct {
	UserInfo
	Token string `json:"token"`
}

// UpdateProfileArgs 修改用户信息接口。
type UpdateProfileArgs struct {
	Nickname string `json:"nickname"`
	Gender   string `json:"gender"`
}

// UpdateProfileResponse 修改用户信息的返回结果。
type UpdateProfileResponse UserInfo

// GetRoomResponse 获取房间信息结果。
type GetRoomResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// 创建者的用户ID
	Creator UserInfo `json:"creator"`
	// TODO：添加创建者的昵称/性别等信息？
	// WatchURL 观看地址
	PlayURL string `json:"playURL"`
	// TODO：添加观众详情？or只返回一个观众人数
	Audiences []string `json:"audiences"`
	// 房间状态，单人直播中/PK中。
	Status string `json:"status"`
	// PKStreamer 若房间PK中，返回PK主播的信息。
	PKStreamer *UserInfo `json:"pkStreamer,omitempty"`
}

// ListRoomsResponse 列出房间的返回结果。
type ListRoomsResponse struct {
	Rooms []GetRoomResponse `json:"rooms"`
}

// EnterRoomRequest 进入房间请求。
type EnterRoomRequest struct {
	UserID string `json:"userID"`
	RoomID string `json:"roomID"`
}

//IMChatRoom  IM聊天室信息。
type IMChatRoom struct{}

// EnterRoomResponse 进入房间返回结果。
type EnterRoomResponse struct {
	RoomID   string `json:"roomID"`
	RoomName string `json:"roomName"`
	// 直播间的拉流观看地址。
	PlayURL string `json:"playURL"`
	// 直播间创建者信息。
	Creator UserInfo `json:"creator"`
	// TODO：添加创建者的昵称/性别等信息？
	// Status 房间的状态，单人直播中/PK中
	Status string `json:"status"`
	// PKStreamerID 若正在PK，返回PK主播的信息。未在PK时该字段为空。
	PKStreamerID *UserInfo `json:"pkStreamer,omitempty"`
	// IMUser IM聊天用户信息。
	IMUser IMUser `json:"imUser"`
	// IMGroup IM聊天室信息。
	IMChatRoom IMChatRoom `json:"imGroup"`
}

// LeaveRoomArgs 离开房间的请求。
type LeaveRoomArgs struct {
	UserID string `json:"userID"`
	RoomID string `json:"roomID"`
}

// CreateRoomArgs 创建直播间的请求参数。
type CreateRoomArgs struct {
	UserID   string `json:"userID"`
	RoomName string `json:"roomName"`
}

// CreateRoomResponse 创建直播间返回结果。
type CreateRoomResponse struct {
	RoomID   string `json:"roomID"`
	RoomName string `json:"roomName"`
	// RTCRoom 对应的RTC房间。
	RTCRoom string `json:"rtcRoom"`
	// RTCRoomToken 创建/加入RTC房间的token。
	RTCRoomToken string `json:"rtcRoomToken"`
	// WSURL websocket 信令连接的地址。
	WSURL string `json:"wsURL"`
	// IMUser
	IMUser IMUser `json:"imUser"`
	// IMGroup
	IMChatRoom IMChatRoom `json:"imChatRoom"`
}

// CloseRoomArgs 关闭直播间参数。
type CloseRoomArgs struct {
	UserID string `json:"userID"`
	RoomID string `json:"roomID"`
}

// RefreshRoomArgs 主播返回直播间（例如断线重连，PK结束等）
type RefreshRoomArgs struct {
	RoomID string `json:"roomID"`
}

// RefreshRoomResponse 主播返回直播间的返回结果，包含新的RTC token。
type RefreshRoomResponse struct {
	RoomID   string `json:"roomID"`
	RoomName string `json:"roomName"`
	// RTCRoom 对应的RTC房间。
	RTCRoom string `json:"rtcRoom"`
	// RTCRoomToken 创建/加入RTC房间的token。
	RTCRoomToken string `json:"rtcRoomToken"`
}

// IMTokenResponse 获取IM token的回应。
type IMTokenResponse struct {
	UserID string `json:"userID"`
	Token  string `json:"token"`
}
