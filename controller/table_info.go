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

package controller

const (
	// AccountCollection 存储账号信息的表。
	AccountCollection = "accounts"
	// ActiveUserCollection 存储已登录用户的表。
	ActiveUserCollection = "active_users"
	// SMSCodeCollection 存储已发送的短信验证码的表。
	SMSCodeCollection = "sms_code"
	// RoomsCollection 存储直播房间信息的表。
	RoomsCollection = "rooms"
	// AudienceCollection (TODO)存储直播间观看者信息的表。
	AudienceCollection = "audiences"
)
