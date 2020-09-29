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
	WSErrorOK               = 0
	WSErrorUnknownMessage   = 10001
	WSErrorTokenInvalid     = 10002
	WSErrorNoPermission     = 10003
	WSErrorRoomNoExist      = 10011
	WSErrorRoomInPK         = 10012
	WSErrorRoomNotInPK      = 10013
	WSErrorPlayerNoExist    = 10021
	WSErrorPlayerOffline    = 10022
	WSErrorInvalidParameter = 10031
)

var WSErrorToString = map[int]string{
	WSErrorOK:               "",
	WSErrorUnknownMessage:   "unknown message",
	WSErrorTokenInvalid:     "token invalid",
	WSErrorNoPermission:     "no permission",
	WSErrorRoomNoExist:      "room no exist",
	WSErrorRoomInPK:         "room in PK",
	WSErrorRoomNotInPK:      "room not in PK",
	WSErrorPlayerNoExist:    "player no exist",
	WSErrorPlayerOffline:    "player offline",
	WSErrorInvalidParameter: "invalid parameter",
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
