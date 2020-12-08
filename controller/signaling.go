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

import (
	"fmt"
	"sync"
	"time"

	qiniuauth "github.com/qiniu/api.v7/v7/auth"
	qiniurtc "github.com/qiniu/api.v7/v7/rtc"
	"github.com/qiniu/x/xlog"

	"github.com/qrtc/qlive/config"
	"github.com/qrtc/qlive/errors"
	"github.com/qrtc/qlive/protocol"
)

// MarshallableMessage 可序列化的消息。
type MarshallableMessage interface {
	Marshal() ([]byte, error)
}

type pkRequest struct {
	proposerID string
	receiverID string
}

// SignalingService 处理各种控制信息。
type SignalingService struct {
	xl               *xlog.Logger
	accountCtl       *AccountController
	roomCtl          *RoomController
	pkRequestLock    sync.RWMutex
	pkRequestAnswers map[pkRequest]chan bool
	pkTimeout        time.Duration
	rtcConfig        *config.QiniuRTCConfig
	Notify           func(xl *xlog.Logger, userID string, msgType string, msg MarshallableMessage) error
}

const (
	// DefaultPKRequestTimeout 默认的PK请求超时时间。
	DefaultPKRequestTimeout = 10 * time.Second
)

// NewSignalingService 创建新的控制信息服务。
func NewSignalingService(xl *xlog.Logger, conf *config.Config) (*SignalingService, error) {
	if xl == nil {
		xl = xlog.New("qlive-signaling-service")
	}
	var pkTimeout time.Duration
	if conf.Signaling.PKRequestTimeoutSecond <= 0 {
		pkTimeout = DefaultPKRequestTimeout
	} else {
		pkTimeout = time.Duration(conf.Signaling.PKRequestTimeoutSecond) * time.Second
	}

	accountCtl, err := NewAccountController(conf.Mongo.URI, conf.Mongo.Database, xl)
	if err != nil {
		xl.Errorf("failed to create account controller, error %v", err)
		return nil, err
	}
	roomCtl, err := NewRoomController(conf.Mongo.URI, conf.Mongo.Database, xl)
	if err != nil {
		xl.Errorf("failed to create room controller, error %v", err)
		return nil, err
	}
	return &SignalingService{
		xl:               xl,
		accountCtl:       accountCtl,
		roomCtl:          roomCtl,
		pkRequestAnswers: make(map[pkRequest]chan bool),
		pkTimeout:        pkTimeout,
		rtcConfig:        conf.RTC,
	}, nil
}

// OnMessage 处理[]byte格式的消息。
func (s *SignalingService) OnMessage(xl *xlog.Logger, senderID string, msg []byte) error {
	if xl == nil {
		xl = s.xl
	}
	index := 0
	for i, b := range msg {
		if b == '=' {
			index = i
			break
		}
	}
	if index == 0 || index >= len(msg)-1 {
		return fmt.Errorf("wrong message format, expect type=body")
	}
	msgType := string(msg[0:index])
	msgBody := msg[index+1:]
	switch msgType {
	case protocol.MT_StartPKRequest:
		return s.OnStartPK(xl, senderID, msgBody)
	case protocol.MT_AnswerPKRequest:
		return s.OnAnswerPK(xl, senderID, msgBody)
	case protocol.MT_EndPKRequest:
		return s.OnEndPK(xl, senderID, msgBody)
	case protocol.MT_DisconnectNotify:
		return s.OnUserOffline(xl, senderID)
	}
	return nil
}

// OnStartPK 处理开始PK消息。
func (s *SignalingService) OnStartPK(xl *xlog.Logger, senderID string, msgBody []byte) error {
	if xl == nil {
		xl = s.xl
	}
	req := &protocol.StartPKRequest{}
	err := req.Unmarshal(msgBody)
	if err != nil {
		res := &protocol.StartPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorUnknownMessage,
			Error: errors.WSErrorToString[errors.WSErrorUnknownMessage],
		}
		s.Notify(xl, senderID, protocol.MT_StartResponse, res)
		return err
	}
	xl.Debugf("start pk: user %s, pk room id %s", senderID, req.PKRoomID)
	// 判断是否满足PK条件。
	// 获取对方的房间。
	pkRoom, err := s.roomCtl.GetRoomByID(xl, req.PKRoomID)
	if err != nil {
		res := &protocol.StartPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorRoomNoExist,
			Error: errors.WSErrorToString[errors.WSErrorRoomNoExist],
		}
		s.Notify(xl, senderID, protocol.MT_StartResponse, res)
		return err
	}
	if pkRoom.Status != protocol.LiveRoomStatusSingle {
		res := &protocol.StartPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorRoomInPK,
			Error: errors.WSErrorToString[errors.WSErrorRoomInPK],
		}
		s.Notify(xl, senderID, protocol.MT_StartResponse, res)
		return err
	}
	// 获取自己的房间。
	selfRoom, err := s.roomCtl.GetRoomByFields(xl, map[string]interface{}{"creator": senderID})
	if err != nil {
		res := &protocol.StartPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		s.Notify(xl, senderID, protocol.MT_StartResponse, res)
		return err
	}
	if selfRoom.Status != protocol.LiveRoomStatusSingle {
		res := &protocol.StartPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorRoomInPK,
			Error: errors.WSErrorToString[errors.WSErrorRoomInPK],
		}
		s.Notify(xl, senderID, protocol.MT_StartResponse, res)
		return err
	}
	// 获取自己和对方的账户信息。
	pkPlayer, err := s.accountCtl.GetAccountByID(xl, pkRoom.Creator)
	if err != nil {
		res := &protocol.StartPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorPlayerNoExist,
			Error: errors.WSErrorToString[errors.WSErrorPlayerNoExist],
		}
		s.Notify(xl, senderID, protocol.MT_StartResponse, res)
		return err
	}
	selfPlayer, err := s.accountCtl.GetAccountByID(xl, senderID)
	if err != nil {
		res := &protocol.StartPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		s.Notify(xl, senderID, protocol.MT_StartResponse, res)
		return err
	}
	pkActiveUser, err := s.accountCtl.GetActiveUserByID(xl, pkPlayer.ID)
	if err != nil {
		res := &protocol.StartPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		s.Notify(xl, senderID, protocol.MT_StartResponse, res)
		return err
	}
	selfActiveUser, err := s.accountCtl.GetActiveUserByID(s.xl, selfPlayer.ID)
	if err != nil {
		res := &protocol.StartPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		s.Notify(xl, senderID, protocol.MT_StartResponse, res)
		return err
	}

	// 发送 PK offer 通知
	pkMessage := &protocol.PKOfferNotify{
		RPCID:    "",
		UserID:   selfPlayer.ID,
		Nickname: selfPlayer.Nickname,
		RoomID:   selfRoom.ID,
		RoomName: selfRoom.Name,
	}
	err = s.Notify(xl, pkPlayer.ID, protocol.MT_PKOfferNotify, pkMessage)
	// 修改状态
	selfRoom.Status = protocol.LiveRoomStatusWaitPK
	_, err = s.roomCtl.UpdateRoom(xl, selfRoom.ID, selfRoom)
	if err != nil {
		res := &protocol.StartPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		s.Notify(xl, senderID, protocol.MT_StartResponse, res)
		return err
	}
	pkRoom.Status = protocol.LiveRoomStatusWaitPK
	_, err = s.roomCtl.UpdateRoom(xl, pkRoom.ID, pkRoom)
	if err != nil {
		res := &protocol.StartPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		s.Notify(xl, senderID, protocol.MT_StartResponse, res)
		return err
	}
	// 修改用户状态
	selfActiveUser.Status = protocol.UserStatusPKWait
	pkActiveUser.Status = protocol.UserStatusPKWait

	_, err = s.accountCtl.UpdateActiveUser(xl, selfPlayer.ID, selfActiveUser)
	if err != nil {
		res := &protocol.StartPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		s.Notify(xl, senderID, protocol.MT_StartResponse, res)
		return err
	}
	_, err = s.accountCtl.UpdateActiveUser(xl, pkPlayer.ID, pkActiveUser)
	if err != nil {
		res := &protocol.StartPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		s.Notify(xl, senderID, protocol.MT_StartResponse, res)
		return err
	}

	// 成功返回
	res := &protocol.StartPKResponse{
		RPCID: req.RPCID,
		Code:  errors.WSErrorOK,
		Error: errors.WSErrorToString[errors.WSErrorOK],
	}
	s.Notify(xl, senderID, protocol.MT_StartResponse, res)
	go s.waitPKTimeout(senderID, selfRoom.ID, pkActiveUser.ID, pkRoom.ID)
	return nil
}

func (s *SignalingService) addPKRequest(proposerID string, proposerRoomID string, receiverID string, receiverRoomID string) chan bool {
	s.pkRequestLock.Lock()
	defer s.pkRequestLock.Unlock()
	req := pkRequest{proposerID: proposerID, receiverID: receiverID}
	answerChan := make(chan bool)
	s.pkRequestAnswers[req] = answerChan
	return answerChan
}

func (s *SignalingService) removePKRequest(proposerID string, proposerRoomID string, receiverID string, receiverRoomID string) {
	s.pkRequestLock.Lock()
	defer s.pkRequestLock.Unlock()
	req := pkRequest{proposerID: proposerID, receiverID: receiverID}
	delete(s.pkRequestAnswers, req)
}

func (s *SignalingService) answerPKRequest(proposerID string, proposerRoomID string, receiverID string, receiverRoomID string, accept bool) {
	s.pkRequestLock.RLock()
	defer s.pkRequestLock.RUnlock()
	req := pkRequest{proposerID: proposerID, receiverID: receiverID}
	answerChan, ok := s.pkRequestAnswers[req]
	if ok {
		s.xl.Debugf("PK answered: proposer %s, room %s, receiver %s, room %s, accept %v", proposerID, proposerRoomID, receiverID, receiverRoomID, accept)
		answerChan <- accept
	}
}

func (s *SignalingService) waitPKTimeout(proposerID string, proposerRoomID string, receiverID string, receiverRoomID string) {
	t := time.NewTimer(s.pkTimeout)

	pkAnswer := s.addPKRequest(proposerID, proposerRoomID, receiverID, receiverRoomID)
	select {
	case <-t.C:
		err := s.onPKTimeout(proposerID, proposerRoomID, receiverID, receiverRoomID)
		if err != nil {
			s.xl.Warnf("failed to process pk request timeout, error %v", err)
			return
		}
	case accept := <-pkAnswer:
		s.removePKRequest(proposerID, proposerRoomID, receiverID, receiverRoomID)
		s.xl.Debugf("PK answered: proposer %s, room %s, receiver %s, room %s, accept %v", proposerID, proposerRoomID, receiverID, receiverRoomID, accept)
		return
	}

	return
}

// 若PK请求已发送超过一定时间还未被响应，恢复PK发起者与接收者的用户与直播间状态。
func (s *SignalingService) onPKTimeout(proposerID string, proposerRoomID string, receiverID string, receiverRoomID string) error {
	shouldNoticeProposer := false
	shouldNoticeReceiver := false
	// 删除PK请求记录。
	s.removePKRequest(proposerID, proposerRoomID, receiverID, receiverRoomID)
	// 恢复状态至单人直播中。
	// 恢复PK发起者状态。
	proposer, err := s.accountCtl.GetActiveUserByID(s.xl, proposerID)
	if err != nil {
		s.xl.Infof("proposer not found, error %v", err)
	} else {
		if proposer.Status == protocol.UserStatusPKWait {
			shouldNoticeProposer = true
			proposer.Status = protocol.UserStatusSingleLive
			_, updateErr := s.accountCtl.UpdateActiveUser(s.xl, proposerID, proposer)
			if updateErr != nil {
				s.xl.Errorf("failed to update proposer %s,error %v", proposerID, updateErr)
				return updateErr
			}
		}
	}
	// 恢复PK发起者的房间状态。
	proposerRoom, err := s.roomCtl.GetRoomByID(s.xl, proposerRoomID)
	if err != nil {
		s.xl.Infof("proposer's room %s not found, error %v", proposerRoomID, err)
	} else {
		if proposerRoom.Status == protocol.LiveRoomStatusWaitPK {
			shouldNoticeProposer = true
			proposerRoom.Status = protocol.LiveRoomStatusSingle
			proposerRoom.PKAnchor = ""
			_, updateErr := s.roomCtl.UpdateRoom(s.xl, proposerRoom.ID, proposerRoom)
			if updateErr != nil {
				s.xl.Errorf("failed to update proposer's room %s, error %v", proposerRoom.ID, updateErr)
				return updateErr
			}
		}
	}
	// 恢复PK接收者的用户状态。
	receiver, err := s.accountCtl.GetActiveUserByID(s.xl, receiverID)
	if err != nil {
		s.xl.Infof("receiver %s not found", receiverID)
	} else {
		if receiver.Status == protocol.UserStatusPKWait {
			shouldNoticeReceiver = true
			receiver.Status = protocol.UserStatusSingleLive
			_, updateErr := s.accountCtl.UpdateActiveUser(s.xl, receiverID, receiver)
			if updateErr != nil {
				s.xl.Errorf("failed to update status of receiver %s, error %v", receiverID, updateErr)
				return updateErr
			}
		}
	}
	// 恢复PK接收者的房间状态。
	receiverRoom, err := s.roomCtl.GetRoomByID(s.xl, receiverRoomID)
	if err != nil {
		s.xl.Infof("receiver's room %s not found, error %v", receiverRoomID, err)
	} else {
		if receiverRoom.Status == protocol.LiveRoomStatusWaitPK {
			shouldNoticeReceiver = true
			receiverRoom.Status = protocol.LiveRoomStatusSingle
			receiverRoom.PKAnchor = ""
			_, updateErr := s.roomCtl.UpdateRoom(s.xl, receiverRoom.ID, receiverRoom)
			if updateErr != nil {
				s.xl.Errorf("failed to update receiver's room %s, error %v", receiverRoom.ID, err)
				return updateErr
			}
		}
	}
	// 发送通知。
	if shouldNoticeProposer {
		msg := &protocol.PKTimeoutNotify{
			PKRoomID:   receiverRoomID,
			PKAnchorID: receiverID,
		}
		err := s.Notify(s.xl, proposerID, protocol.MT_PKTimeoutNotify, msg)
		if err != nil {
			s.xl.Warnf("failed to notice proposer %s, error %v", proposerID, err)
		}
	}

	if shouldNoticeReceiver {
		msg := &protocol.PKTimeoutNotify{
			PKRoomID:   proposerRoomID,
			PKAnchorID: proposerID,
		}
		s.Notify(s.xl, receiverID, protocol.MT_PKTimeoutNotify, msg)
		if err != nil {
			s.xl.Warnf("failed to notice receiver %s,error %v", receiver, err)
		}
	}
	return nil
}
func (s *SignalingService) generateRTCRoomToken(roomID string, userID string, permission string) string {
	rtcClient := qiniurtc.NewManager(&qiniuauth.Credentials{
		AccessKey: s.rtcConfig.KeyPair.AccessKey,
		SecretKey: []byte(s.rtcConfig.KeyPair.SecretKey),
	})
	rtcRoomTokenTimeout := time.Duration(s.rtcConfig.RoomTokenExpireSecond) * time.Second
	roomAccess := qiniurtc.RoomAccess{
		AppID:    s.rtcConfig.AppID,
		RoomName: roomID,
		UserID:   userID,
		ExpireAt: time.Now().Add(rtcRoomTokenTimeout).Unix(),
		// Permission分admin/user，直播间创建者需要admin权限。
		Permission: permission,
	}
	token, _ := rtcClient.GetRoomToken(roomAccess)
	return token
}

// OnAnswerPK 处理应答PK消息。
func (s *SignalingService) OnAnswerPK(xl *xlog.Logger, senderID string, msgBody []byte) error {
	if xl == nil {
		xl = s.xl
	}

	req := &protocol.AnswerPKRequest{}
	err := req.Unmarshal(msgBody)
	if err != nil {
		res := &protocol.AnswerPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorUnknownMessage,
			Error: errors.WSErrorToString[errors.WSErrorUnknownMessage],
		}
		s.Notify(xl, senderID, protocol.MT_AnswerPKResponse, res)
		return err
	}

	selfRoom, err := s.roomCtl.GetRoomByFields(xl, map[string]interface{}{"creator": senderID})
	if err != nil {
		res := &protocol.AnswerPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		s.Notify(xl, senderID, protocol.MT_AnswerPKResponse, res)
		return err
	}
	selfPlayer, err := s.accountCtl.GetAccountByID(xl, senderID)
	if err != nil {
		res := &protocol.AnswerPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		s.Notify(xl, senderID, protocol.MT_AnswerPKResponse, res)
		return err
	}
	selfActiveUser, err := s.accountCtl.GetActiveUserByID(xl, selfPlayer.ID)
	if err != nil {
		res := &protocol.AnswerPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		s.Notify(xl, senderID, protocol.MT_AnswerPKResponse, res)
		return err
	}
	// 检查自身的房间状态。
	if selfRoom.Status != protocol.LiveRoomStatusWaitPK {
		res := &protocol.AnswerPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorRoomNotInPK,
			Error: errors.WSErrorToString[errors.WSErrorRoomNotInPK],
		}
		s.Notify(xl, senderID, protocol.MT_AnswerPKResponse, res)
		return err
	}
	// shouldResetStatus 如果响应PK时，出现对方房间不存在（已下播）、状态不对等异常情况，应重置当前直播间与用户状态为单人直播中。
	shouldResetStatus := false
	defer func(err error) {
		if shouldResetStatus {
			xl.Debugf("answer PK: error %v, reset room and user status", err)
			selfActiveUser.Status = protocol.UserStatusSingleLive
			selfRoom.Status = protocol.LiveRoomStatusSingle
			_, updateErr := s.roomCtl.UpdateRoom(xl, selfRoom.ID, selfRoom)
			if updateErr != nil {
				xl.Warnf("failed to reset room %s, error %v", selfRoom.ID, updateErr)
			}
			_, updateErr = s.accountCtl.UpdateActiveUser(xl, selfPlayer.ID, selfActiveUser)
			if updateErr != nil {
				xl.Warnf("failed to reset user status of user %s, error %v", selfPlayer.ID, updateErr)
			}
		}
	}(err)
	pkRoom, err := s.roomCtl.GetRoomByID(xl, req.ReqRoomID)
	if err != nil {
		res := &protocol.AnswerPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorRoomNoExist,
			Error: errors.WSErrorToString[errors.WSErrorRoomNoExist],
		}
		s.Notify(xl, senderID, protocol.MT_AnswerPKResponse, res)
		shouldResetStatus = true
		return err
	}
	if pkRoom.Status != protocol.LiveRoomStatusWaitPK {
		res := &protocol.AnswerPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorRoomNotInPK,
			Error: errors.WSErrorToString[errors.WSErrorRoomNotInPK],
		}
		s.Notify(xl, senderID, protocol.MT_AnswerPKResponse, res)
	}
	pkPlayer, err := s.accountCtl.GetAccountByID(xl, pkRoom.Creator)
	if err != nil {
		res := &protocol.AnswerPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorPlayerNoExist,
			Error: errors.WSErrorToString[errors.WSErrorPlayerNoExist],
		}
		shouldResetStatus = true
		s.Notify(xl, senderID, protocol.MT_AnswerPKResponse, res)
		return err
	}

	pkActiveUser, err := s.accountCtl.GetActiveUserByID(xl, pkPlayer.ID)
	if err != nil {
		res := &protocol.AnswerPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		shouldResetStatus = true
		s.Notify(xl, senderID, protocol.MT_AnswerPKResponse, res)
		return err
	}

	// 检查PK对方的房间状态。
	if pkRoom.Status != protocol.LiveRoomStatusWaitPK {
		res := &protocol.AnswerPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorRoomNotInPK,
			Error: errors.WSErrorToString[errors.WSErrorRoomNotInPK],
		}
		shouldResetStatus = true
		s.Notify(xl, senderID, protocol.MT_AnswerPKResponse, res)
		return err
	}
	// 通知发起者
	answerMessage := &protocol.PKAnswerNotify{
		RPCID:     "",
		ReqRoomID: req.ReqRoomID,
		Accepted:  req.Accept,
	}
	if req.Accept {
		answerMessage.RTCRoom = selfRoom.ID
		answerMessage.RTCRoomToken = s.generateRTCRoomToken(selfRoom.ID, pkPlayer.ID, "user")
	}
	err = s.Notify(xl, pkPlayer.ID, protocol.MT_PKAnswerNotify, answerMessage)
	if err != nil {
		res := &protocol.AnswerPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorPlayerOffline,
			Error: errors.WSErrorToString[errors.WSErrorPlayerOffline],
		}
		shouldResetStatus = true
		s.Notify(xl, senderID, protocol.MT_AnswerPKResponse, res)
		return err
	}
	// 修改房间与用户状态。
	if req.Accept {
		selfRoom.Status = protocol.LiveRoomStatusPK
		selfRoom.PKAnchor = pkPlayer.ID
		pkRoom.Status = protocol.LiveRoomStatusPK
		pkRoom.PKAnchor = selfPlayer.ID
		selfActiveUser.Status = protocol.UserStatusPKLive
		selfActiveUser.Room = selfRoom.ID
		pkActiveUser.Status = protocol.UserStatusPKLive
		pkActiveUser.Room = selfRoom.ID
	} else {
		selfRoom.Status = protocol.LiveRoomStatusSingle
		pkRoom.Status = protocol.LiveRoomStatusSingle
		selfActiveUser.Status = protocol.UserStatusSingleLive
		pkActiveUser.Status = protocol.UserStatusSingleLive
	}
	// 通过controller更新状态。
	_, err = s.accountCtl.UpdateActiveUser(xl, selfPlayer.ID, selfActiveUser)
	if err != nil {
		res := &protocol.AnswerPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		s.Notify(xl, senderID, protocol.MT_AnswerPKResponse, res)
	}
	_, err = s.accountCtl.UpdateActiveUser(xl, pkPlayer.ID, pkActiveUser)
	if err != nil {
		res := &protocol.AnswerPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		s.Notify(xl, senderID, protocol.MT_AnswerPKResponse, res)
	}
	_, err = s.roomCtl.UpdateRoom(xl, selfRoom.ID, selfRoom)
	if err != nil {
		res := &protocol.AnswerPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		s.Notify(xl, senderID, protocol.MT_AnswerPKResponse, res)
	}
	_, err = s.roomCtl.UpdateRoom(xl, pkRoom.ID, pkRoom)
	if err != nil {
		res := &protocol.AnswerPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		s.Notify(xl, senderID, protocol.MT_AnswerPKResponse, res)
	}
	s.answerPKRequest(pkPlayer.ID, pkRoom.ID, senderID, selfRoom.ID, req.Accept)
	// 成功返回。
	res := &protocol.AnswerPKResponse{
		ReqRoomID: req.ReqRoomID,
		RPCID:     req.RPCID,
		Code:      errors.WSErrorOK,
		Error:     errors.WSErrorToString[errors.WSErrorOK],
	}
	s.Notify(xl, senderID, protocol.MT_AnswerPKResponse, res)
	return nil
}

// OnEndPK 结束PK。
func (s *SignalingService) OnEndPK(xl *xlog.Logger, senderID string, msgBody []byte) error {
	if xl == nil {
		xl = s.xl
	}
	req := &protocol.EndPKRequest{}
	err := req.Unmarshal(msgBody)
	if err != nil {
		res := &protocol.EndPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorUnknownMessage,
			Error: errors.WSErrorToString[errors.WSErrorUnknownMessage],
		}
		s.Notify(xl, senderID, protocol.MT_EndPKResponse, res)
		return err
	}

	// 获取当前用户信息。
	selfActiveUser, err := s.accountCtl.GetActiveUserByID(xl, senderID)
	if err != nil {
		res := &protocol.EndPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		s.Notify(xl, senderID, protocol.MT_EndPKResponse, res)
		return err
	}
	if selfActiveUser.Status != protocol.UserStatusPKLive {
		res := &protocol.EndPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorRoomInPK,
			Error: errors.WSErrorToString[errors.WSErrorRoomInPK],
		}
		s.Notify(xl, senderID, protocol.MT_EndPKResponse, res)
		return err
	}
	// 找到该用户创建的房间。
	selfRoom, err := s.roomCtl.GetRoomByFields(xl, map[string]interface{}{"creator": senderID})
	if err != nil {
		res := &protocol.EndPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		s.Notify(xl, senderID, protocol.MT_EndPKResponse, res)
		return err
	}
	// 找到对方主播的ID。
	pkAnchorID := selfRoom.PKAnchor
	pkActiveUser, err := s.accountCtl.GetActiveUserByID(xl, pkAnchorID)
	if err != nil {
		res := &protocol.EndPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		s.Notify(xl, senderID, protocol.MT_EndPKResponse, res)
		return err
	}
	// 找到对方主播的房间。
	pkRoom, err := s.roomCtl.GetRoomByFields(xl, map[string]interface{}{"creator": pkAnchorID})
	if err != nil {
		res := &protocol.EndPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorRoomInPK,
			Error: errors.WSErrorToString[errors.WSErrorRoomInPK],
		}
		s.Notify(xl, senderID, protocol.MT_EndPKResponse, res)
		return err
	}
	// 状态检查
	if selfRoom.Status != protocol.LiveRoomStatusPK || pkRoom.Status != protocol.LiveRoomStatusPK {
		res := &protocol.EndPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorRoomNotInPK,
			Error: errors.WSErrorToString[errors.WSErrorRoomNotInPK],
		}
		s.Notify(xl, senderID, protocol.MT_EndPKResponse, res)
		return fmt.Errorf("room status not in PK")
	}
	// 检查房间ID是否匹配。
	if selfRoom.ID != req.PKRoomID && pkRoom.ID != req.PKRoomID {
		res := &protocol.EndPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorNoPermission,
			Error: errors.WSErrorToString[errors.WSErrorNoPermission],
		}
		s.Notify(xl, senderID, protocol.MT_EndPKResponse, res)
		return fmt.Errorf("user %s does not have permission to end PK with room %s", senderID, req.PKRoomID)
	}
	// 向对方发送结束PK推送。
	endMessage := &protocol.PKEndNotify{
		RPCID:    "",
		PKRoomID: req.PKRoomID,
	}
	err = s.Notify(xl, pkAnchorID, protocol.MT_PKEndNotify, endMessage)
	if err != nil {
		res := &protocol.EndPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorPlayerOffline,
			Error: errors.WSErrorToString[errors.WSErrorPlayerOffline],
		}
		s.Notify(xl, senderID, protocol.MT_EndPKResponse, res)
		return err
	}
	// 修改状态。TODO:使用事务修改。
	var updateErr error
	selfActiveUser.Room = selfRoom.ID
	selfActiveUser.Status = protocol.UserStatusSingleLive
	selfRoom.Status = protocol.LiveRoomStatusSingle
	selfRoom.PKAnchor = ""
	pkActiveUser.Room = pkRoom.ID
	pkActiveUser.Status = protocol.UserStatusSingleLive
	pkRoom.Status = protocol.LiveRoomStatusSingle
	pkRoom.PKAnchor = ""
	// 更新状态。
	_, err = s.accountCtl.UpdateActiveUser(xl, selfActiveUser.ID, selfActiveUser)
	if err != nil {
		xl.Errorf("failed to update active user %s, error %v", selfActiveUser.ID, err)
		updateErr = err
	}
	_, err = s.accountCtl.UpdateActiveUser(xl, pkActiveUser.ID, pkActiveUser)
	if err != nil {
		xl.Errorf("failed to update active user %s, error %v", pkActiveUser.ID, err)
		updateErr = err
	}
	_, err = s.roomCtl.UpdateRoom(xl, selfRoom.ID, selfRoom)
	if err != nil {
		xl.Errorf("failed to update room %s, error %v", selfRoom.ID, err)
		updateErr = err
	}
	_, err = s.roomCtl.UpdateRoom(xl, pkRoom.ID, pkRoom)
	if err != nil {
		xl.Errorf("failed to update room %s, error %v", pkRoom.ID, err)
		updateErr = err
	}
	// 处理更新出错的情况。
	if updateErr != nil {
		res := &protocol.EndPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		s.Notify(xl, senderID, protocol.MT_EndPKResponse, res)
		// TODO：目前仅包含最后一次出错的错误。
		return updateErr
	}
	// 成功返回。
	res := &protocol.EndPKResponse{
		RPCID: req.RPCID,
		Code:  errors.WSErrorOK,
		Error: errors.WSErrorToString[errors.WSErrorOK],
	}
	s.Notify(xl, senderID, protocol.MT_EndPKResponse, res)
	return nil
}

// OnUserOffline 处理客户端下线。
func (s *SignalingService) OnUserOffline(xl *xlog.Logger, userID string) error {
	if xl == nil {
		xl = s.xl
	}
	user, err := s.accountCtl.GetActiveUserByID(xl, userID)
	if err != nil {
		xl.Debugf("user %s not logged in but offlined", userID)
		return err
	}
	xl.Debugf("user %s offline:processing start, current status %v, in room %s", userID, user.Status, user.Room)
	// 找出用户的房间。
	var room *protocol.LiveRoom
	if protocol.IsUserBroadCasting(user.Status) {
		room, err = s.roomCtl.GetRoomByFields(xl, map[string]interface{}{"creator": userID})
		if err != nil {
			xl.Debugf("cannot find user %s's room, user status is %v, error %v", userID, user.Status, err)
		}
		if room != nil {
			xl.Debugf("will close room %s created by user %s", room.ID, userID)
		}
	}

	// 如果是PK状态，向其PK对方发送消息。
	if user.Status == protocol.UserStatusPKLive {
		if room != nil {
			xl.Debugf("user %s's room %s is in PK, notify PK anchor %s", userID, room.ID, room.PKAnchor)
			pkAnchorID := room.PKAnchor
			endMessage := &protocol.PKEndNotify{
				PKRoomID: room.ID,
			}
			s.Notify(xl, pkAnchorID, protocol.MT_PKEndNotify, endMessage)
			// 更新PK对方主播的用户状态和房间状态。
			pkActiveUser, err := s.accountCtl.GetActiveUserByID(xl, pkAnchorID)
			if err == nil {
				pkRoom, err := s.roomCtl.GetRoomByFields(xl, map[string]interface{}{"creator": pkAnchorID})
				if err == nil {
					pkActiveUser.Status = protocol.UserStatusSingleLive
					pkActiveUser.Room = pkRoom.ID
					s.accountCtl.UpdateActiveUser(xl, pkAnchorID, pkActiveUser)
					pkRoom.Status = protocol.LiveRoomStatusSingle
					pkRoom.PKAnchor = ""
					s.roomCtl.UpdateRoom(xl, pkRoom.ID, pkRoom)
				}
			}
		}
	}
	// 关闭该房间。
	if room != nil {
		err := s.roomCtl.CloseRoom(xl, userID, room.ID)
		if err != nil {
			xl.Errorf("close room %s created by %s failed, error %v", room.ID, userID, err)
			return err
		}
		xl.Infof("room %s created by %s has been closed", room.ID, userID)
	}
	xl.Debugf("user %s offline:processing end", userID)
	return nil
}
