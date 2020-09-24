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

package handler

import (
	"fmt"
	"strings"

	"github.com/qiniu/x/xlog"
	"github.com/qrtc/qlive/errors"
	"github.com/qrtc/qlive/protocol"
)

// MockAccount 模拟的账号服务。
type mockAccount struct {
	accounts []*protocol.Account
}

func (m *mockAccount) GetAccountByPhoneNumber(xl *xlog.Logger, phoneNumber string) (*protocol.Account, error) {
	for _, account := range m.accounts {
		if account.PhoneNumber == phoneNumber {
			return account, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockAccount) GetAccountByID(xl *xlog.Logger, id string) (*protocol.Account, error) {
	for _, account := range m.accounts {
		if account.ID == id {
			return account, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockAccount) CreateAccount(xl *xlog.Logger, account *protocol.Account) error {
	if account.ID == "" || account.PhoneNumber == "" {
		return fmt.Errorf("bad request")
	}
	for _, a := range m.accounts {
		if a.ID == account.ID || a.PhoneNumber == account.PhoneNumber {
			return fmt.Errorf("conflict")
		}
	}
	m.accounts = append(m.accounts, account)
	return nil
}

func (m *mockAccount) UpdateAccount(xl *xlog.Logger, id string, account *protocol.Account) (*protocol.Account, error) {
	if account.ID != "" && account.ID != id {
		return nil, fmt.Errorf("bad request")
	}
	var oldAccount *protocol.Account
	for _, a := range m.accounts {
		if a.ID == id {
			oldAccount = a
			break
		}
	}
	if oldAccount == nil {
		return nil, fmt.Errorf("not found")
	}
	if account.PhoneNumber != "" && account.PhoneNumber != oldAccount.PhoneNumber {
		return nil, fmt.Errorf("bad request")
	}
	oldAccount.Nickname = account.Nickname
	oldAccount.Gender = account.Gender
	return oldAccount, nil
}

// AccountLogin 模拟账号登录，返回token。
func (m *mockAccount) AccountLogin(xl *xlog.Logger, id string) (token string, err error) {
	return id + "#" + "login-token", nil
}

// AccountLogout 模拟账号退出登录。
func (m *mockAccount) AccountLogout(xl *xlog.Logger, id string) error {
	return nil
}

// mockSMSCode 模拟的短信服务。
type mockSMSCode struct {
	// 模拟发送出错的情况。
	NumberToError map[string]error
}

// Send 模拟发送验证码
func (m *mockSMSCode) Send(xl *xlog.Logger, phoneNumber string) error {
	if m.NumberToError != nil {
		return m.NumberToError[phoneNumber]
	}
	return nil
}

// Validate 模拟检查输入的验证码。
func (m *mockSMSCode) Validate(xl *xlog.Logger, phoneNumber string, smsCode string) error {
	if smsCode == "123456" {
		return nil
	}
	return fmt.Errorf("wrong sms code")
}

// mockAuth 模拟的认证服务。
type mockAuth struct{}

// GetIDByToken 从token 中获取用户ID。
func (m *mockAuth) GetIDByToken(xl *xlog.Logger, token string) (string, error) {
	parts := strings.SplitN(token, "#", 2)
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid token")
	}
	return parts[0], nil
}

// mockRoom 模拟的房间管理服务。
type mockRoom struct {
	rooms         []*protocol.LiveRoom
	roomAudiences map[string][]string
	maxRooms      int
}

func (m *mockRoom) CreateRoom(xl *xlog.Logger, newRoom *protocol.LiveRoom) (*protocol.LiveRoom, error) {
	if len(m.rooms) >= m.maxRooms {
		return nil, &errors.ServerError{Code: errors.ServerErrorTooManyRooms}
	}
	for _, room := range m.rooms {
		if room.ID == newRoom.ID {
			return nil, &errors.ServerError{Code: errors.ServerErrorRoomNameUsed}
		}
	}
	m.rooms = append(m.rooms, newRoom)
	return newRoom, nil
}
