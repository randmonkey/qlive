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

package errors

const (
	WSErrorOK                  = 0
	WSErrorUnknownMessage      = 10001
	WSErrorTokenInvalid        = 10002
	WSErrorNoPermission        = 10003
	WSErrorRoomNoExist         = 10011
	WSErrorRoomInPK            = 10012
	WSErrorRoomNotInPK         = 10013
	WSErrorRoomTypeWrong       = 10014 // 房间类型不符合，不能发起PK/连麦请求
	WSErrorInvalidJoinPosition = 10015 // 不正确的连麦位置
	WSErrorJoinPositionBusy    = 10016 // 连麦位置被占用（已有人上麦或发起上麦请求）
	WSErrorPlayerNoExist       = 10021
	WSErrorPlayerOffline       = 10022
	WSErrorPlayerNotInRoom     = 10023 // 观众不在房间中，不能发起连麦
	WSErrorPlayerJoined        = 10024 // 观众已经加入连麦，不能重复加入
	WSErrorPlayerNotJoined     = 10025 // 观众未加入连麦，不能发起结束连麦请求
	WSErrorInvalidParameter    = 10031
)

var WSErrorToString = map[int]string{
	WSErrorOK:                  "",
	WSErrorUnknownMessage:      "unknown message",
	WSErrorTokenInvalid:        "token invalid",
	WSErrorNoPermission:        "no permission",
	WSErrorRoomNoExist:         "room no exist",
	WSErrorRoomInPK:            "room in PK",
	WSErrorRoomNotInPK:         "room not in PK",
	WSErrorRoomTypeWrong:       "room type does not support the request",
	WSErrorInvalidJoinPosition: "invalid join position",
	WSErrorJoinPositionBusy:    "join position already ocuppied or requested by another audience",
	WSErrorPlayerNoExist:       "player no exist",
	WSErrorPlayerOffline:       "player offline",
	WSErrorPlayerNotInRoom:     "player not in room",
	WSErrorPlayerJoined:        "player already joined or requested to join",
	WSErrorPlayerNotJoined:     "player not joined",
	WSErrorInvalidParameter:    "invalid parameter",
}

type WSError struct {
	s string
}

func (e WSError) Error() string {
	return e.s
}

func NewWSError(errString string) error {
	return &WSError{errString}
}
