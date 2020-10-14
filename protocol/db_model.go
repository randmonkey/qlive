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

import (
	"time"
)

/*
	model.go: 规定数据存储的格式。
*/

// Account 用户账号信息。
type Account struct {
	// 用户ID，作为数据库唯一标识。
	ID string `json:"id" bson:"_id"`
	// 手机号，目前要求全局唯一。
	PhoneNumber string `json:"phoneNumber" bson:"phoneNumber"`
	// TODO：支持账号密码登录。
	Password string `json:"password" bson:"password"`
	// 用户昵称（TODO:是否要求全局唯一？）
	Nickname string `json:"nickname" bson:"nickname"`
	// 用户显示性别。
	Gender string `json:"gender" bson:"gender"`
	// AvartarURL 头像URL地址，暂时留空（SDK提供头像）
	AvatarURL string `json:"avatarURL,omitempty" bson:"avatarURL,omitempty"`
	// RegisterIP 用户注册（首次登录）时使用的IP。
	RegisterIP string `json:"registerIP" bson:"registerIP"`
	// RegisterTime 用户注册（首次登录）时间。
	RegisterTime time.Time `json:"registerTime" bson:"registerTime"`
	// LastLoginIP 上次登录IP。
	LastLoginIP string `json:"lastLoginIP" bson:"lastLoginIP"`
	// LastLoginTime 上次登录时间。
	LastLoginTime time.Time `json:"lastLoginTime" bson:"lastLoginTime"`
}

// UserStatus 用户的当前状态。
type UserStatus string

const (
	// UserStatusIdle 用户已登录，未观看或创建直播，为空闲状态。
	UserStatusIdle UserStatus = "idle"
	// UserStatusWatching 用户正在观看直播。
	UserStatusWatching UserStatus = "watching"
	// UserStatusSingleLive 用户正在单人直播中。
	UserStatusSingleLive UserStatus = "singleLive"
	// UserStatusPKLive 用户正在PK连麦直播。
	UserStatusPKLive UserStatus = "pkLive"
	// UserStatusPKPending 用户已发起PK请求,正在等待响应
	UserStatusPKWait UserStatus = "pkWait"
)

// IMUser 对应IM 用户信息。
type IMUser struct {
	UserID   string `json:"id"`
	Username string `json:"name"`
	Token    string `json:"token"`
}

// ActiveUser 已登录用户的信息。
type ActiveUser struct {
	ID       string `json:"id" bson:"_id"`
	Nickname string `json:"nickname" bson:"nickname"`
	// Token 本次登录使用的token。
	Token string `json:"token" bson:"token"`
	// Status 用户状态。包括：空闲，观看中，单人直播中，PK连麦直播中（等待PK被响应、等待响应他人PK）
	Status UserStatus `json:"status" bson:"status"`
	// Room 所在直播间。PK连麦直播中，发起PK一方主播的所在直播间为其PK对手主播的直播间。
	Room string `json:"room,omitempty" bson:"room,omitempty"`
	// IMUser 关联IM用户信息。
	IMUser IMUser `json:"imUser" bson:"imUser"`
}

// SMSCodeRecord 已发送的验证码记录。
type SMSCodeRecord struct {
	PhoneNumber string    `json:"phoneNumber" bson:"phoneNumber"`
	SMSCode     string    `json:"smsCode" bson:"smsCode"`
	SendTime    time.Time `json:"sendTime" bson:"sendTime"`
	ExpireAt    time.Time `json:"-" bson:"expireAt"`
}

// LiveRoomStatus 直播间状态。
type LiveRoomStatus string

const (
	// LiveRoomStatusSingle 单人直播中。
	LiveRoomStatusSingle LiveRoomStatus = "single"
	// LiveRoomStatusPK PK连麦直播中。
	LiveRoomStatusPK LiveRoomStatus = "PK"
	// LiveRoomStatusWaitPK 直播间有PK请求，等待响应中
	LiveRoomStatusWaitPK LiveRoomStatus = "waitPK"
)

// LiveRoom 直播间信息。
type LiveRoom struct {
	ID string `json:"id" bson:"_id"`
	// Name 直播间显示的名称。
	Name string `json:"name" bson:"name"`
	// CoverURL 直播间的封面地址。
	CoverURL string `json:"coverURL" bson:"coverURL"`
	// Creator 直播间创建者的ID。
	Creator string `json:"creator" bson:"creator"`
	// playURL 观看直播的拉流地址。
	PlayURL string `json:"playURL" bson:"playURL"`
	// RTCRoom 对应的RTC房间名。
	RTCRoom string `json:"rtcRoom" bson:"rtcRoom"`
	// Status 该直播间的当前状态。(单人直播中、PK中、等待PK)
	Status LiveRoomStatus `json:"status" bson:"status"`
	// PKAnchor 正在该直播间参与PK的另一主播的ID。
	PKAnchor string `json:"pkAnchor,omitempty" bson:"pkAnchor,omitempty"`
	// IMGroup 该直播间关联聊天群组。
	IMChatRoom string `json:"imGroup" bson:"imGroup"`
}

// PKRequest PK请求信息。
type PKRequest struct {
	Proposer string `json:"id" bson:"_id"`
	
}

// Feedback 反馈信息。
type Feedback struct {
	ID            string    `json:"id" bson:"_id"`
	Sender        string    `json:"sender" bson:"sender"`
	Content       string    `json:"content" bson:"content"`
	AttachmentURL string    `json:"attachment" bson:"attachment"`
	SendTime      time.Time `json:"sendTime" bson:"sendTime"`
}

// ObjectCounter 对象计数，用于生成自增的ID/序列号。
type ObjectCounter struct {
	ID             string `json:"id" bson:"_id"`
	SequenceNumber int64  `json:"seq" bson:"seq"`
}
