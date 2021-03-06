package protocol

import "time"

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
	AvartarURL string `json:"avartarURL,omitempty" bson:"avartarURL,omitempty"`
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
)

// IMUserInfo 对应IM 用户信息。TODO：根据对接的IM厂商，填充此结构体。
type IMUserInfo struct{}

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
	IMUser IMUserInfo `json:"imUser" bson:"imUser"`
}

// SMSCodeRecord 已发送的验证码记录。
type SMSCodeRecord struct {
	PhoneNumber string    `json:"phoneNumber" bson:"phoneNumber"`
	SMSCode     string    `json:"smsCode" bson:"smsCode"`
	SendTime    time.Time `json:"sendTime" bson:"sendTime"`
}

// LiveRoomStatus 直播间状态。
type LiveRoomStatus string

const (
	// LiveRoomStatusSingle 单人直播中。
	LiveRoomStatusSingle LiveRoomStatus = "single"
	// LiveRoomStatusPK PK连麦直播中。
	LiveRoomStatusPK LiveRoomStatus = "PK"
	// LiveRoomStatusWaitPK 直播间有PK请求，等待响应中
	LiveRoomStatusWaitPK = "waitPK"
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
	// WatchURL 观看直播的拉流地址。
	WatchURL string `json:"watchURL" bson:"watchURL"`
	// RTCRoom 对应的RTC房间名。
	RTCRoom string `json:"rtcRoom" bson:"rtcRoom"`
	// Status 该直播间的当前状态。(单人直播中、PK中、等待PK)
	Status LiveRoomStatus `json:"status" bson:"status"`
	// PKStreamer 正在该直播间参与PK的另一主播的ID。
	PKStreamer string `json:"pkStreamer,omitempty" bson:"pkStreamer,omitempty"`
	// Audiences 观众ID列表。
	Audiences []string `json:"audiences" bson:"audiences"`
	// IMGroup 该直播间关联聊天群组。
	IMGroup string `json:"imGroup" bson:"imGroup"`
}
