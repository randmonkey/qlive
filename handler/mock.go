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
	"strconv"
	"strings"
	"sync"
	"time"

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
func (m *mockAccount) AccountLogin(xl *xlog.Logger, id string) (user *protocol.ActiveUser, err error) {
	for _, a := range m.accounts {
		if a.ID == id {
			return &protocol.ActiveUser{
				ID:     id,
				Token:  id + "#login-token",
				Status: protocol.UserStatusIdle,
			}, nil
		}
	}
	return nil, &errors.ServerError{Code: errors.ServerErrorUserNotfound}
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
	rooms map[string]*protocol.LiveRoom
	// roomID -> slice of userIDs
	roomAudiences map[string][]string
	maxRooms      int
	lock          sync.RWMutex
}

// CreateRoom 模拟创建房间。
func (m *mockRoom) CreateRoom(xl *xlog.Logger, newRoom *protocol.LiveRoom) (*protocol.LiveRoom, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if len(m.rooms) >= m.maxRooms {
		return nil, &errors.ServerError{Code: errors.ServerErrorTooManyRooms}
	}
	for _, room := range m.rooms {
		if room.ID == newRoom.ID {
			return nil, fmt.Errorf("room %s ID used", room.ID)
		}
		if room.Name == newRoom.Name {
			if room.Creator == newRoom.Creator {
				return room, nil
			}
			return nil, &errors.ServerError{Code: errors.ServerErrorRoomNameUsed}
		}
		if room.Creator == newRoom.Creator {
			return nil, &errors.ServerError{Code: errors.ServerErrorCanOnlyCreateOneRoom}
		}
	}
	m.rooms[newRoom.ID] = newRoom
	return newRoom, nil
}

// 列出全部房间。
func (m *mockRoom) ListAllRooms(xl *xlog.Logger) ([]protocol.LiveRoom, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	ret := []protocol.LiveRoom{}
	for _, room := range m.rooms {
		ret = append(ret, *room)
	}
	return ret, nil
}

// TODO：添加根据指定字段列出房间的功能
func (m *mockRoom) ListRoomsByFields(xl *xlog.Logger, fields map[string]interface{}) ([]protocol.LiveRoom, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	ret := []protocol.LiveRoom{}
	for _, room := range m.rooms {
		ret = append(ret, *room)
	}
	return ret, nil
}

// 关闭房间
func (m *mockRoom) CloseRoom(xl *xlog.Logger, userID string, roomID string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	_, ok := m.rooms[roomID]
	if !ok {
		return &errors.ServerError{Code: errors.ServerErrorRoomNotFound}
	}
	delete(m.rooms, roomID)
	delete(m.roomAudiences, roomID)

	return nil
}

func (m *mockRoom) EnterRoom(xl *xlog.Logger, userID string, roomID string) (*protocol.LiveRoom, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	for _, room := range m.rooms {
		if room.Creator == userID {
			return nil, &errors.ServerError{Code: errors.ServerErrorUserBroadcasting}
		}
	}
	room, ok := m.rooms[roomID]
	if !ok {
		return nil, &errors.ServerError{Code: errors.ServerErrorRoomNotFound}
	}
	foundUser := false
	for _, audienceID := range m.roomAudiences[roomID] {
		if audienceID == userID {
			foundUser = true
		}
	}
	if !foundUser {
		m.roomAudiences[roomID] = append(m.roomAudiences[roomID], userID)
	}
	return room, nil
}

func (m *mockRoom) LeaveRoom(xl *xlog.Logger, userID string, roomID string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	for _, room := range m.rooms {
		if room.Creator == userID {
			return &errors.ServerError{Code: errors.ServerErrorUserBroadcasting}
		}
	}
	_, ok := m.rooms[roomID]
	if !ok {
		return &errors.ServerError{Code: errors.ServerErrorRoomNotFound}
	}
	if len(m.roomAudiences[roomID]) > 0 {
		foundUser := false
		var index int
		for i, audienceID := range m.roomAudiences[roomID] {
			if audienceID == userID {
				foundUser = true
				index = i
			}
		}
		if foundUser {
			newAudiences := m.roomAudiences[roomID][0:index]
			if index < len(m.roomAudiences)-1 {
				newAudiences = append(newAudiences, m.roomAudiences[roomID][index+1:]...)
			}
			m.roomAudiences[roomID] = newAudiences
		}
	}
	return nil
}

func (m *mockRoom) GetRoomByID(xl *xlog.Logger, roomID string) (*protocol.LiveRoom, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	room, ok := m.rooms[roomID]
	if !ok {
		return nil, &errors.ServerError{Code: errors.ServerErrorRoomNotFound}
	}
	return room, nil
}

func (m *mockRoom) UpdateRoom(xl *xlog.Logger, id string, newRoom *protocol.LiveRoom) (*protocol.LiveRoom, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	_, ok := m.rooms[id]
	if !ok {
		return nil, &errors.ServerError{Code: errors.ServerErrorRoomNotFound}
	}
	m.rooms[id] = newRoom
	return newRoom, nil
}

func (m *mockRoom) GetAudienceNumber(xl *xlog.Logger, roomID string) (int, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	_, ok := m.rooms[roomID]
	if !ok {
		return 0, &errors.ServerError{Code: errors.ServerErrorRoomNotFound}
	}
	return len(m.roomAudiences[roomID]), nil
}

type mockUpload struct{}

func (m *mockUpload) GetUploadToken(xl *xlog.Logger, userID string, filename string, expireSeconds int) (string, error) {
	return "upload-token:" + userID + ":" + filename, nil
}

type mockTicket struct {
	tickets []*protocol.Ticket
}

func (m *mockTicket) SubmitTicket(xl *xlog.Logger, ticket *protocol.Ticket) (string, error) {
	now := time.Now()
	ticket.ID = "ticket-" + strconv.FormatInt(now.UnixNano(), 36)
	if ticket.SubmitTime.IsZero() {
		ticket.SubmitTime = now
	}
	m.tickets = append(m.tickets, ticket)
	return ticket.ID, nil
}
