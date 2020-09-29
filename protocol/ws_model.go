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
	MT_StartPKRequest    = "start-pk"
	MT_StartResponse     = "start-pk-res"
	MT_EndPKRequest      = "end-pk"
	MT_EndPKResponse     = "end-pk-res"
	MT_AnswerPKRequest   = "answer-pk"
	MT_AnswerPKResponse  = "answer-pk-res"
	MT_PKOfferNotify     = "on-pk-offer"
	MT_PKAnswerNotify    = "on-pk-answer"
	MT_PKEndNotify       = "on-pk-end"
	MT_DisconnectNotify  = "disconnect"
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
	// PKRoomID 发起PK请求的直播间ID。
	PKRoomID string `json:"pkRoomID"`
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
	// PKRoomID 发起PK请求的直播间ID。
	PKRoomID string `json:"pkRoomID"`
	RPCID    string `json:"rpcID,omitempty"`
	Code     int    `json:"code"`
	Error    string `json:"error"`
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
	// PK 直播间 ID
	PKRoomID string `json:"pkRoomID"`
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

type DisconnectNotify struct {
	RPCID string `json:"rpcID,omitempty"`
}

func (p *DisconnectNotify) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *DisconnectNotify) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}
