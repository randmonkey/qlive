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

import "encoding/json"

// WebSocket Message Type
const (
	MT_Ping              = "ping"
	MT_Pong              = "pong"
	MT_AuthorizeRequest  = "auth"
	MT_AuthorizeResponse = "auth-res"
	// 主播PK相关信令。
	MT_StartPKRequest   = "start-pk"
	MT_StartResponse    = "start-pk-res"
	MT_EndPKRequest     = "end-pk"
	MT_EndPKResponse    = "end-pk-res"
	MT_AnswerPKRequest  = "answer-pk"
	MT_AnswerPKResponse = "answer-pk-res"
	MT_PKOfferNotify    = "on-pk-offer"
	MT_PKAnswerNotify   = "on-pk-answer"
	MT_PKEndNotify      = "on-pk-end"
	MT_PKTimeoutNotify  = "on-pk-timeout"
	// 观众连麦相关信令。
	MTStartJoinRequest     = "start-join"       // 申请连麦
	MTStartJoinResponse    = "start-join-res"   // 申请连麦处理结果
	MTAnswerJoinRequest    = "answer-join"      // 主播回应观众连麦请求
	MTAnswerJoinResponse   = "answer-join-res"  // 回应连麦请求处理结果
	MTEndJoinRequest       = "end-join"         // 观众结束连麦
	MTEndJoinResponse      = "end-join-res"     // 结束连麦处理结果
	MTRequestJoinNotify    = "on-join-reqeust"  // 通知主播有观众申请连麦
	MTAnswerJoinNotify     = "on-join-answer"   // 通知观众连麦请求已被应答（接受/拒绝）
	MTAudienceJoinedNotify = "on-audience-join" // 通知同房间内其他观众有观众上麦
	MTEndJoinNotify        = "on-join-end"      // 通知主播与观众连麦结束
	MTJoinTimeoutNotify    = "on-join-timeout"  //观众连麦请求超时
	MTRoomCloseNotify      = "on-room-close"    // 通知观众房间关闭
	MT_DisconnectNotify    = "disconnect"
)

// Ping 服务端发送给客户端的心跳消息，客户端收到后应当回复pong表明自己在线。
// @generate-json-marshaller
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

type AuthorizeRequest struct {
	RPCID string `json:"rpcID,omitempty"`
	Token string `json:"token"`
}

func (p *AuthorizeRequest) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *AuthorizeRequest) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

type AuthorizeResponse struct {
	RPCID       string `json:"rpcID,omitempty"`
	Code        int    `json:"code"`
	Error       string `json:"error"`
	PongTimeout int    `json:"pongTimeout,omitempty"`
}

func (p *AuthorizeResponse) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *AuthorizeResponse) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

type StartPKRequest struct {
	RPCID string `json:"rpcID,omitempty"`
	// PKRoomID 请求开启PK的直播间ID。
	PKRoomID string `json:"pkRoomID"`
}

func (p *StartPKRequest) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *StartPKRequest) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

type StartPKResponse struct {
	RPCID string `json:"rpcID,omitempty"`
	Code  int    `json:"code"`
	Error string `json:"error"`
}

func (p *StartPKResponse) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *StartPKResponse) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

type EndPKRequest struct {
	RPCID string `json:"rpcID,omitempty"`
	// PKRoomID PK中的直播间ID。
	PKRoomID string `json:"pkRoomID"`
}

func (p *EndPKRequest) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *EndPKRequest) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

type EndPKResponse struct {
	RPCID string `json:"rpcID,omitempty"`
	Code  int    `json:"code"`
	Error string `json:"error"`
}

func (p *EndPKResponse) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *EndPKResponse) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

type AnswerPKRequest struct {
	RPCID string `json:"rpcID,omitempty"`
	// ReqRoomID 发起 PK 请求的直播间ID。
	ReqRoomID string `json:"reqRoomID"`
	// 是否接受PK
	Accept bool `json:"accept"`
}

func (p *AnswerPKRequest) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *AnswerPKRequest) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

type AnswerPKResponse struct {
	// ReqRoomID 发起PK请求的直播间ID。
	ReqRoomID string `json:"ReqRoomID"`
	RPCID     string `json:"rpcID,omitempty"`
	Code      int    `json:"code"`
	Error     string `json:"error"`
}

func (p *AnswerPKResponse) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *AnswerPKResponse) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

type PKOfferNotify struct {
	RPCID string `json:"rpcID,omitempty"`
	// 发起PK的主播的用户ID
	UserID string `json:"userID"`
	// 发起PK的主播的用户昵称
	Nickname string `json:"nickname"`
	// 发起PK的主播的直播间ID
	RoomID string `json:"roomID"`
	// 发起PK的主播的直播间名称
	RoomName string `json:"roomName"`
}

func (p *PKOfferNotify) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *PKOfferNotify) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

type PKAnswerNotify struct {
	RPCID string `json:"rpcID,omitempty"`
	// 发起 PK 请求的直播间 ID
	ReqRoomID string `json:"reqRoomID"`
	// 是否接受 PK
	Accepted bool `json:"accepted"`
	// PK被接受时才有该字段，表示被PK直播间对应的RTC房间
	RTCRoom string `json:"rtcRoom,omitempty"`
	// PK被接受时才有该字段，表示加入被PK直播间对应的RTC房间使用的token
	RTCRoomToken string `json:"rtcRoomToken,omitempty"`
}

func (p *PKAnswerNotify) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *PKAnswerNotify) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

type PKEndNotify struct {
	RPCID string `json:"rpcID,omitempty"`
	// PK 直播间 ID
	PKRoomID string `json:"pkRoomID"`
}

func (p *PKEndNotify) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *PKEndNotify) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

// PKTimeoutNotify 通知PK请求超时
type PKTimeoutNotify struct {
	RPCID string `json:"rpcID,omitempty"`
	// PKAnchorID PK 主播ID，PK请求的另一方主播的用户ID。
	PKAnchorID string `json:"pkAnchorID"`
	// PKRoomID PK 直播间ID,PK请求的另一方的直播间ID。
	PKRoomID string `json:"pkRoomID"`
}

func (p *PKTimeoutNotify) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *PKTimeoutNotify) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

type DisconnectNotify struct {
	RPCID string `json:"rpcID,omitempty"`
}

func (p *DisconnectNotify) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *DisconnectNotify) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

// StartJoinRequest 观众申请加入连麦。
type StartJoinRequest struct {
	RPCID    string `json:"rpcID,omitempty"`
	RoomID   string `json:"roomID"`   // 加入的房间ID
	Position int    `json:"position"` // 上麦位置
	// TODO:申请连麦时是否可以附带消息？
	Message string `json:"message,omitempty"`
}

func (p *StartJoinRequest) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *StartJoinRequest) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

// StartJoinResponse 观众申请加入连麦的返回结果。
type StartJoinResponse struct {
	RPCID string `json:"rpcID,omitempty"`
	Code  int    `json:"code"`
	Error string `json:"error"`
}

func (p *StartJoinResponse) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *StartJoinResponse) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

// JoinRequestNotify 通知主播有观众申请上麦。
type JoinRequestNotify struct {
	RoomID    string `json:"roomID"`
	ReqUserID string `json:"reqUserID"`         // 用户ID
	Nickname  string `json:"nickname"`          // 用户昵称
	Gender    string `json:"gender"`            // 用户性别
	AvatarURL string `json:"avatar"`            // 用户头像地址
	Position  int    `json:"position"`          // 上麦位置
	Message   string `json:"message,omitempty"` // TODO:上麦请求的附带消息
}

func (p *JoinRequestNotify) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *JoinRequestNotify) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

// AnswerJoinRequest 主播应答观众上麦请求。
type AnswerJoinRequest struct {
	RPCID     string `json:"rpcID,omitempty"`
	RoomID    string `json:"roomID"`
	ReqUserID string `json:"reqUserID"` // 用户ID
	Accept    bool   `json:"accept"`
	Message   string `json:"message,omitempty"` // TODO:回应上麦的附带消息
}

func (p *AnswerJoinRequest) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *AnswerJoinRequest) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

// AnswerJoinResponse 对主播发来应答连麦请求的回应消息。
type AnswerJoinResponse struct {
	RPCID string `json:"rpcID,omitempty"`
	Code  int    `json:"code"`
	Error string `json:"error"`
}

func (p *AnswerJoinResponse) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *AnswerJoinResponse) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

// JoinAnswerNotify 主播回应连麦请求时，通知发起请求的观众。
type JoinAnswerNotify struct {
	RoomID    string `json:"roomID"`
	ReqUserID string `json:"reqUserID"`
	Accept    bool   `json:"accept"`
}

func (p *JoinAnswerNotify) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *JoinAnswerNotify) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

// AudienceJoinNotify 当主播同意观众连麦时，通知所有观众（包括发起者）。
type AudienceJoinNotify struct {
	RoomID    string `json:"roomID"`
	ReqUserID string `json:"reqUserID"`
	Position  int    `json:"position"`
	Gender    string `json:"gender"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar"`
}

func (p *AudienceJoinNotify) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *AudienceJoinNotify) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

// EndJoinRequest 观众结束连麦请求。
type EndJoinRequest struct {
	RPCID     string `json:"rpcID"`
	RoomID    string `json:"roomID"`
	ReqUserID string `json:"reqUserID"`
}

func (p *EndJoinRequest) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *EndJoinRequest) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

// EndJoinResponse 结束连麦请求的返回结果。
type EndJoinResponse struct {
	RPCID string `json:"rpcID,omitempty"`
	Code  int    `json:"code"`
	Error string `json:"error"`
}

func (p *EndJoinResponse) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *EndJoinResponse) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

// EndJoinNotify 结束连麦通知
type EndJoinNotify struct {
	RoomID    string `json:"roomID"`
	ReqUserID string `json:"reqUserID"`
	Position  int    `json:"position"`
	Gender    string `json:"gender"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar"`
}

func (p *EndJoinNotify) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *EndJoinNotify) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

// JoinTimeoutNotify 连麦请求超时通知
type JoinTimeoutNotify struct {
	RoomID    string `json:"roomID"`
	ReqUserID string `json:"reqUserID"`
}

func (p *JoinTimeoutNotify) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *JoinTimeoutNotify) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

// RoomCloseNotify 房间关闭通知
type RoomCloseNotify struct {
	RoomID string `json:"roomID"`
}

func (p *RoomCloseNotify) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *RoomCloseNotify) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}
