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

	// RequestStartKey 存放在gin context中的请求开始的时间戳，单位为纳秒。
	RequestStartKey = "request-start-timestamp-nano"
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
	Token  string `json:"token"`
	Status string `json:"status"`
	Room   string `json:"room,omitempty"`
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
	// 只返回观众人数
	AudienceNumber int `json:"audienceNumber"`
	// 房间状态，单人直播中/PK中。
	Status string `json:"status"`
	// PKAnchor 若房间PK中，返回PK主播的信息。
	PKAnchor *UserInfo `json:"pkAnchor,omitempty"`
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
	// PKAnchorID 若正在PK，返回PK主播的信息。未在PK时该字段为空。
	PKAnchorID *UserInfo `json:"pkAnchor,omitempty"`
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

// UpdateRoomArgs 更新房间信息参数。
type UpdateRoomArgs struct {
	RoomName string `json:"roomName"`
}

// UpdateRoomResponse 更新房间信息返回结果。
type UpdateRoomResponse struct {
	RoomID   string `json:"roomID"`
	RoomName string `json:"roomName"`
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
	// WSURL websocket 服务器地址。
	WSURL string `json:"wsURL"`
}

// IMTokenResponse 获取IM token的回应。
type IMTokenResponse struct {
	UserID string `json:"userID"`
	Token  string `json:"token"`
}

// RongCloudSignature 融云API回调签名。
type RongCloudSignature struct {
	Nonce         string `form:"nonce" json:"nonce"`
	SignTimestamp string `form:"signTimestamp" json:"signTimestamp"`
	Signature     string `form:"signature" json:"signature"`
}

// RongCloudMessage 融云消息路由的消息体。
type RongCloudMessage struct {
	Signature  RongCloudSignature `form:"-"`
	FromUserID string             `form:"fromUserId" json:"fromUserID"`
	ToUserID   string             `form:"toUserId" json:"toUserID"`
	// ObjectName 消息的类型。
	ObjectName     string                  `form:"objectName" json:"objectName"`
	Content        RongCloudMessageContent `form:"content" json:"content"`
	ChannelType    string                  `form:"channelType" json:"channelType"`
	MsgTimestampMS int64                   `form:"msgTimestamp" json:"msgTimestamp"`
	MsgUID         string                  `form:"msgUID" json:"msgUID"`
	SensitiveType  int                     `form:"sensitiveType" json:"sensitiveType"`
	Source         string                  `form:"source" json:"source"`
	GroupUserIDs   string                  `form:"groupUserIds" json:"groupUserIds"`
}

// RongCloudUserInfo 融云消息中的用户信息。
type RongCloudUserInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Portrait string `json:"portrait"`
	Extra    string `json:"extra,omitempty"`
}

// RongCloudMessageContent 融云消息内容。目前仅支持文字消息。
type RongCloudMessageContent struct {
	Content string            `json:"content"`
	User    RongCloudUserInfo `json:"user"`
	Extra   string            `json:"extra,omitempty"`
}

// GetUploadTokenArgs 获取上传文件token的参数。
type GetUploadTokenArgs struct {
	Filename      string `json:"filename"`      // 上传资源的文件名（key）
	ExpireSeconds int    `json:"expireSeconds"` // token的有效期（单位为秒），默认为3600（1小时）。
}

// GetUploadTokenResponse 获取上传文件token的结果。
type GetUploadTokenResponse struct {
	Token    string `json:"token"`
	ExpireAt int64  `json:"expireAt"` // token过期时间，以秒为单位的时间戳。
}

// SendFeedbackArgs 发送反馈信息的参数。
type SendFeedbackArgs struct {
	Content       string `json:"content"`    // 正文
	AttachmentURL string `json:"attachment"` // 附件
}

// SendFeedbackResponse 发送反馈信息成功后的回应。
type SendFeedbackResponse struct {
	FeedbackID string `json:"feedbackID"`
}
