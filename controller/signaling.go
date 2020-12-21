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

type joinRequest struct {
	roomID    string
	reqUserID string
}

// SignalingService 处理各种控制信息。
type SignalingService struct {
	xl                 *xlog.Logger
	accountCtl         *AccountController
	roomCtl            *RoomController
	pkRequestLock      sync.RWMutex
	pkRequestAnswers   map[pkRequest]chan bool
	pkTimeout          time.Duration
	joinRequestLock    sync.RWMutex
	joinRequestAnswers map[joinRequest]chan bool
	joinTimeout        time.Duration
	rtcConfig          *config.QiniuRTCConfig
	Notify             func(xl *xlog.Logger, userID string, msgType string, msg MarshallableMessage) error
}

const (
	// DefaultPKRequestTimeout 默认的PK请求超时时间。
	DefaultPKRequestTimeout = 10 * time.Second
	// DefaultJoinRequestTimeout 默认的连麦请求超时时间。
	DefaultJoinRequestTimeout = 10 * time.Second
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
	var joinTimeout time.Duration
	if conf.Signaling.JoinRequestTimeoutSecond <= 0 {
		joinTimeout = DefaultJoinRequestTimeout
	} else {
		joinTimeout = time.Duration(conf.Signaling.JoinRequestTimeoutSecond) * time.Second
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
		xl:                 xl,
		accountCtl:         accountCtl,
		roomCtl:            roomCtl,
		pkRequestAnswers:   make(map[pkRequest]chan bool),
		pkTimeout:          pkTimeout,
		joinRequestAnswers: make(map[joinRequest]chan bool),
		joinTimeout:        joinTimeout,
		rtcConfig:          conf.RTC,
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
	case protocol.MTStartJoinRequest:
		return s.OnStartJoin(xl, senderID, msgBody)
	case protocol.MTAnswerJoinRequest:
		return s.OnAnswerJoin(xl, senderID, msgBody)
	case protocol.MTEndJoinRequest:
		return s.OnEndJoin(xl, senderID, msgBody)
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
	res := &protocol.StartPKResponse{}
	defer func() {
		res.RPCID = req.RPCID
		if res.Error == "" {
			res.Error = errors.WSErrorToString[res.Code]
		}
		s.Notify(xl, senderID, protocol.MT_StartResponse, res)
	}()
	err := req.Unmarshal(msgBody)
	if err != nil {
		res.Code = errors.WSErrorUnknownMessage
		return err
	}
	xl.Debugf("start pk: user %s, pk room id %s", senderID, req.PKRoomID)
	// 判断是否满足PK条件。
	// 获取对方的房间。
	pkRoom, err := s.roomCtl.GetRoomByID(xl, req.PKRoomID)
	if err != nil {
		res.Code = errors.WSErrorRoomNoExist
		return err
	}
	// 检查房间是否为主播PK房。
	if pkRoom.Type != protocol.RoomTypePK {
		// 兼容没有指定type的旧房间
		if string(pkRoom.Type) != "" {
			res.Code = errors.WSErrorRoomTypeWrong
			return fmt.Errorf("invalid room type %s", pkRoom.Type)
		}
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
		res.Code = errors.WSErrorInvalidParameter
		return err
	}
	if selfRoom.Status != protocol.LiveRoomStatusSingle {
		res.Code = errors.WSErrorRoomInPK
		return err
	}
	// 获取自己和对方的账户信息。
	pkPlayer, err := s.accountCtl.GetAccountByID(xl, pkRoom.Creator)
	if err != nil {
		res.Code = errors.WSErrorPlayerNoExist
		return err
	}
	selfPlayer, err := s.accountCtl.GetAccountByID(xl, senderID)
	if err != nil {
		res.Code = errors.WSErrorInvalidParameter
		return err
	}
	pkActiveUser, err := s.accountCtl.GetActiveUserByID(xl, pkPlayer.ID)
	if err != nil {
		res.Code = errors.WSErrorPlayerNoExist
		return err
	}
	selfActiveUser, err := s.accountCtl.GetActiveUserByID(s.xl, selfPlayer.ID)
	if err != nil {
		res.Code = errors.WSErrorInvalidParameter
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
		res.Code = errors.WSErrorInvalidParameter
		return err
	}
	pkRoom.Status = protocol.LiveRoomStatusWaitPK
	_, err = s.roomCtl.UpdateRoom(xl, pkRoom.ID, pkRoom)
	if err != nil {
		res.Code = errors.WSErrorInvalidParameter
		return err
	}
	// 修改用户状态
	selfActiveUser.Status = protocol.UserStatusPKWait
	pkActiveUser.Status = protocol.UserStatusPKWait

	_, err = s.accountCtl.UpdateActiveUser(xl, selfPlayer.ID, selfActiveUser)
	if err != nil {
		res.Code = errors.WSErrorInvalidParameter
		return err
	}
	_, err = s.accountCtl.UpdateActiveUser(xl, pkPlayer.ID, pkActiveUser)
	if err != nil {
		res.Code = errors.WSErrorInvalidParameter
		return err
	}

	// 成功返回
	res.Code = errors.WSErrorOK
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
			s.xl.Warnf("failed to notice receiver %s,error %v", receiverID, err)
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
	res := &protocol.AnswerPKResponse{}
	defer func() {
		res.RPCID = req.RPCID
		if res.Error == "" {
			res.Error = errors.WSErrorToString[res.Code]
		}
		s.Notify(xl, senderID, protocol.MT_AnswerPKResponse, res)
	}()
	err := req.Unmarshal(msgBody)
	if err != nil {
		res.Code = errors.WSErrorUnknownMessage
		return err
	}

	selfRoom, err := s.roomCtl.GetRoomByFields(xl, map[string]interface{}{"creator": senderID})
	if err != nil {
		res.Code = errors.WSErrorRoomNoExist
		return err
	}
	selfPlayer, err := s.accountCtl.GetAccountByID(xl, senderID)
	if err != nil {
		res.Code = errors.WSErrorPlayerNoExist
		return err
	}
	selfActiveUser, err := s.accountCtl.GetActiveUserByID(xl, selfPlayer.ID)
	if err != nil {
		res.Code = errors.WSErrorPlayerNoExist
		return err
	}
	// 检查自身的房间状态。
	if selfRoom.Status != protocol.LiveRoomStatusWaitPK {
		res.Code = errors.WSErrorRoomNotInPK
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
		res.Code = errors.WSErrorRoomNoExist
		shouldResetStatus = true
		return err
	}
	if pkRoom.Status != protocol.LiveRoomStatusWaitPK {
		res.Code = errors.WSErrorRoomNotInPK
		shouldResetStatus = true
		return nil
	}
	pkPlayer, err := s.accountCtl.GetAccountByID(xl, pkRoom.Creator)
	if err != nil {
		shouldResetStatus = true
		res.Code = errors.WSErrorPlayerNoExist
		return err
	}

	pkActiveUser, err := s.accountCtl.GetActiveUserByID(xl, pkPlayer.ID)
	if err != nil {
		shouldResetStatus = true
		res.Code = errors.WSErrorPlayerNoExist
		return err
	}

	// 检查PK对方的房间状态。
	if pkRoom.Status != protocol.LiveRoomStatusWaitPK {
		shouldResetStatus = true
		res.Code = errors.WSErrorRoomNotInPK
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
		shouldResetStatus = true
		res.Code = errors.WSErrorPlayerOffline
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
		res.Code = errors.WSErrorInvalidParameter
		return err
	}
	_, err = s.accountCtl.UpdateActiveUser(xl, pkPlayer.ID, pkActiveUser)
	if err != nil {
		res.Code = errors.WSErrorInvalidParameter
		return err
	}
	_, err = s.roomCtl.UpdateRoom(xl, selfRoom.ID, selfRoom)
	if err != nil {
		res.Code = errors.WSErrorInvalidParameter
		return err
	}
	_, err = s.roomCtl.UpdateRoom(xl, pkRoom.ID, pkRoom)
	if err != nil {
		res.Code = errors.WSErrorInvalidParameter
		return err
	}
	s.answerPKRequest(pkPlayer.ID, pkRoom.ID, senderID, selfRoom.ID, req.Accept)
	// 成功返回。
	res.ReqRoomID = req.ReqRoomID
	res.Code = errors.WSErrorOK
	return nil
}

// OnEndPK 结束PK。
func (s *SignalingService) OnEndPK(xl *xlog.Logger, senderID string, msgBody []byte) error {
	if xl == nil {
		xl = s.xl
	}
	req := &protocol.EndPKRequest{}
	res := &protocol.EndPKResponse{}
	defer func() {
		res.RPCID = req.RPCID
		if res.Error == "" {
			res.Error = errors.WSErrorToString[res.Code]
		}
		s.Notify(xl, senderID, protocol.MT_EndPKResponse, res)
	}()
	err := req.Unmarshal(msgBody)
	if err != nil {
		res.Code = errors.WSErrorUnknownMessage
		return err
	}

	// 获取当前用户信息。
	selfActiveUser, err := s.accountCtl.GetActiveUserByID(xl, senderID)
	if err != nil {
		res.Code = errors.WSErrorInvalidParameter
		return err
	}
	if selfActiveUser.Status != protocol.UserStatusPKLive {
		res.Code = errors.WSErrorRoomNotInPK
		return err
	}
	// 找到该用户创建的房间。
	selfRoom, err := s.roomCtl.GetRoomByFields(xl, map[string]interface{}{"creator": senderID})
	if err != nil {
		res.Code = errors.WSErrorInvalidParameter
		return err
	}
	// 找到对方主播的ID。
	pkAnchorID := selfRoom.PKAnchor
	pkActiveUser, err := s.accountCtl.GetActiveUserByID(xl, pkAnchorID)
	if err != nil {
		res.Code = errors.WSErrorInvalidParameter
		return err
	}
	// 找到对方主播的房间。
	pkRoom, err := s.roomCtl.GetRoomByFields(xl, map[string]interface{}{"creator": pkAnchorID})
	if err != nil {
		res.Code = errors.WSErrorRoomNoExist
		return err
	}
	// 状态检查
	if selfRoom.Status != protocol.LiveRoomStatusPK || pkRoom.Status != protocol.LiveRoomStatusPK {
		res.Code = errors.WSErrorRoomNotInPK
		return fmt.Errorf("room status not in PK")
	}
	// 检查房间ID是否匹配。
	if selfRoom.ID != req.PKRoomID && pkRoom.ID != req.PKRoomID {
		res.Code = errors.WSErrorNoPermission
		return fmt.Errorf("user %s does not have permission to end PK with room %s", senderID, req.PKRoomID)
	}
	// 向对方发送结束PK推送。
	endMessage := &protocol.PKEndNotify{
		RPCID:    "",
		PKRoomID: req.PKRoomID,
	}
	err = s.Notify(xl, pkAnchorID, protocol.MT_PKEndNotify, endMessage)
	if err != nil {
		res.Code = errors.WSErrorPlayerOffline
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

		res.Code = errors.WSErrorInvalidParameter
		// TODO：目前仅包含最后一次出错的错误。
		return updateErr
	}
	// 成功返回。
	res.Code = errors.WSErrorOK
	s.Notify(xl, senderID, protocol.MT_EndPKResponse, res)
	return nil
}

func (s *SignalingService) addJoinRequest(roomID string, reqUserID string) chan bool {
	s.joinRequestLock.Lock()
	defer s.joinRequestLock.Unlock()
	req := joinRequest{roomID: roomID, reqUserID: reqUserID}
	answerChan := make(chan bool)
	s.joinRequestAnswers[req] = answerChan
	return answerChan
}

func (s *SignalingService) removeJoinRequest(roomID string, reqUserID string) {
	s.joinRequestLock.Lock()
	defer s.joinRequestLock.Unlock()
	req := joinRequest{roomID: roomID, reqUserID: reqUserID}
	delete(s.joinRequestAnswers, req)
}

func (s *SignalingService) answerJoinRequest(roomID string, reqUserID string, accept bool) {
	s.joinRequestLock.RLock()
	defer s.joinRequestLock.RUnlock()
	req := joinRequest{roomID: roomID, reqUserID: reqUserID}
	answerChan, ok := s.joinRequestAnswers[req]
	if ok {
		answerChan <- accept
	}
}

func (s *SignalingService) waitJoinTimeout(xl *xlog.Logger, roomID string, reqUserID string) {
	t := time.NewTimer(s.joinTimeout)
	joinAnswer := s.addJoinRequest(roomID, reqUserID)
	if xl == nil {
		xl = s.xl
	}
	xl = xl.Spawn("wait-join-timeout")
	select {
	case <-t.C:
		err := s.onJoinTimeout(xl, roomID, reqUserID)
		if err != nil {
			xl.Errorf("failed to process join request time out, user %s, room %s, error %v", reqUserID, roomID, err)
		}
	case accept := <-joinAnswer:
		xl.Debugf("user %s join room %s, accept %v", reqUserID, roomID, accept)
	}
}

func (s *SignalingService) onJoinTimeout(xl *xlog.Logger, roomID string, reqUserID string) error {
	timeoutNotice := &protocol.JoinTimeoutNotify{
		RoomID:    roomID,
		ReqUserID: reqUserID,
	}

	user, err := s.accountCtl.GetActiveUserByID(xl, reqUserID)
	if err != nil {
		xl.Infof("cannot find user %s, error %v", reqUserID, err)
	} else {
		// 恢复观众状态至观看中。
		if user.Status == protocol.UserStatusJoinWait {
			user.Status = protocol.UserStatusWatching
			user.JoinPosition = nil
			_, err = s.accountCtl.UpdateActiveUser(xl, user.ID, user)
			if err != nil {
				xl.Errorf("failed to update status of user %s, error %v", user.ID, err)
				return err
			}
		}
		// 通知观众。
		s.Notify(xl, user.ID, protocol.MTJoinTimeoutNotify, timeoutNotice)
	}
	room, err := s.roomCtl.GetRoomByID(xl, roomID)
	if err != nil {
		xl.Infof("cannot find room %s, error %v", roomID, err)
	} else {
		// 通知主播。
		if room.Type == protocol.RoomTypeVoice {
			s.Notify(xl, room.Creator, protocol.MTJoinTimeoutNotify, timeoutNotice)
		}
	}

	return nil
}

// OnStartJoin 观众发出连麦请求。
func (s *SignalingService) OnStartJoin(xl *xlog.Logger, senderID string, msgBody []byte) error {
	if xl == nil {
		xl = s.xl
	}
	req := &protocol.StartJoinRequest{}
	res := &protocol.StartJoinResponse{}
	// 最后给发起者发送回应。
	defer func() {
		res.RPCID = req.RPCID
		if res.Error == "" {
			res.Error = errors.WSErrorToString[res.Code]
		}
		s.Notify(xl, senderID, protocol.MTStartJoinResponse, res)
	}()
	err := req.Unmarshal(msgBody)
	if err != nil {
		res.Code = errors.WSErrorUnknownMessage
		return err
	}
	// 未找到房间。
	room, err := s.roomCtl.GetRoomByID(xl, req.RoomID)
	if err != nil {
		res.Code = errors.WSErrorRoomNoExist
		return err
	}
	// 房间类型不是语音房。
	if room.Type != protocol.RoomTypeVoice || room.Status != protocol.LiveRoomStatusVoiceLive {
		res.Code = errors.WSErrorRoomTypeWrong
		return err
	}
	// 查看该观众账号信息。
	user, err := s.accountCtl.GetActiveUserByID(xl, senderID)
	if err != nil {
		res.Code = errors.WSErrorPlayerNoExist
		return err
	}
	account, err := s.accountCtl.GetAccountByID(xl, senderID)
	if err != nil {
		res.Code = errors.WSErrorPlayerNoExist
		return err
	}
	// 查看该用户是否在房间中，并且状态为观看中。
	if user.Room != room.ID {
		res.Code = errors.WSErrorPlayerNotInRoom
		return fmt.Errorf("user %s not in room %s, cannot join", senderID, room.ID)
	}
	if user.Status != protocol.UserStatusWatching {
		xl.Debugf("user %s in room %s status %v, cannot join", senderID, room.ID, user.Status)
		res.Code = errors.WSErrorPlayerJoined
		return fmt.Errorf("user %s joined, cannot join again", senderID)
	}
	joinPosition := req.Position
	if joinPosition < 0 || joinPosition >= room.MaxJoinAudiences {
		xl.Debugf("user %s requested to join room %s at invalid position: %d", senderID, room.ID, joinPosition)
		res.Code = errors.WSErrorInvalidJoinPosition
		return fmt.Errorf("user %s join at invalid position %d", senderID, joinPosition)
	}
	// 检查该位置是否已经有其他观众已经上麦/请求上麦。
	filter := map[string]interface{}{
		"room":         room.ID,
		"joinPosition": joinPosition,
		"status":       map[string]interface{}{"$in": []string{"joined", "joinWait"}},
	}
	positionUser, err := s.accountCtl.GetActiveUserByFields(xl, filter)
	if err != nil {
		if err.Error() != "not found" {
			res.Code = errors.WSErrorJoinPositionBusy
			return err
		}
	} else {
		xl.Debugf("user %s join room %s at position %d: occupied by %s", senderID, room.ID, joinPosition, positionUser.ID)
		res.Code = errors.WSErrorJoinPositionBusy
		return fmt.Errorf("room %s position %d occupied by user %s", room.ID, joinPosition, positionUser.ID)
	}

	// 更新用户状态。
	user.Status = protocol.UserStatusJoinWait
	user.JoinPosition = &joinPosition
	_, err = s.accountCtl.UpdateActiveUser(xl, senderID, user)
	if err != nil {
		xl.Errorf("failed to update user %s status, error %v", senderID, err)
		res.Code = errors.WSErrorInvalidParameter
		return err
	}
	// 通知主播有观众申请连麦。
	joinNotice := &protocol.JoinRequestNotify{
		RoomID:    room.ID,
		ReqUserID: senderID,
		Nickname:  account.Nickname,
		Gender:    account.Gender,
		AvatarURL: account.AvatarURL,
		Position:  req.Position,
	}
	s.Notify(xl, room.Creator, protocol.MTRequestJoinNotify, joinNotice)
	// 等待连麦请求超时或被响应。
	go s.waitJoinTimeout(xl, req.RoomID, senderID)
	// 回复观众申请连麦成功。
	res.Code = errors.WSErrorOK
	return nil
}

// OnAnswerJoin 处理主播应答连麦。
func (s *SignalingService) OnAnswerJoin(xl *xlog.Logger, senderID string, msgBody []byte) error {
	if xl == nil {
		xl = s.xl
	}
	// 0.解析请求。
	req := &protocol.AnswerJoinRequest{}
	res := &protocol.AnswerJoinResponse{}
	// 最后给请求者发送回应。
	defer func() {
		res.RPCID = req.RPCID
		if res.Error == "" {
			res.Error = errors.WSErrorToString[res.Code]
		}
		s.Notify(xl, senderID, protocol.MTAnswerJoinResponse, res)
	}()

	err := req.Unmarshal(msgBody)
	if err != nil {
		res.Code = errors.WSErrorUnknownMessage
		return err
	}
	// 1. 检查房间是否存在
	room, err := s.roomCtl.GetRoomByID(xl, req.RoomID)
	if err != nil {
		res.Code = errors.WSErrorRoomNoExist
		return err
	}
	// 1.1 检查房间是否是语音房
	if room.Type != protocol.RoomTypeVoice {
		res.Code = errors.WSErrorRoomTypeWrong
		return err
	}
	// 2. 检查用户是否是主播
	user, err := s.accountCtl.GetActiveUserByID(xl, senderID)
	if err != nil {
		res.Code = errors.WSErrorPlayerNoExist
		return err
	}
	if user.ID != room.Creator {
		res.Code = errors.WSErrorNoPermission
		return err
	}
	// 3. 检查申请上麦的观众是否存在
	joinAudience, err := s.accountCtl.GetActiveUserByID(xl, req.ReqUserID)
	if err != nil {
		res.Code = errors.WSErrorPlayerNoExist
		return err
	}
	joinAudienceAccount, err := s.accountCtl.GetAccountByID(xl, req.ReqUserID)
	if err != nil {
		res.Code = errors.WSErrorPlayerNoExist
		return err
	}
	// 4. 检查申请上麦的观众是否在房间及当前状态
	if joinAudience.Room != room.ID {
		res.Code = errors.WSErrorPlayerNotInRoom
		return fmt.Errorf("user %s not in room %s, cannot join", req.ReqUserID, req.RoomID)
	}
	if joinAudience.Status != protocol.UserStatusJoinWait || joinAudience.JoinPosition == nil {
		xl.Debugf("user %s status %v, not %v, cannot answer join", joinAudience.ID, joinAudience.Status, protocol.UserStatusJoinWait)
		res.Code = errors.WSErrorPlayerNotJoined
		return fmt.Errorf("user %s status %s, cannot answer join", req.ReqUserID, joinAudience.Status)
	}
	// 5. 更新房间与观众状态。
	if req.Accept {
		// 更新观众状态为连麦中。
		joinAudience.Status = protocol.UserStatusJoined
		_, err = s.accountCtl.UpdateActiveUser(xl, joinAudience.ID, joinAudience)
		if err != nil {
			xl.Errorf("failed to change user %s status to joined", joinAudience.ID)
			res.Code = errors.WSErrorInvalidParameter
			return err
		}
		// 获取观众列表。
		audiences, err := s.roomCtl.GetAllAudiences(xl, room.ID)
		if err != nil {
			xl.Errorf("failed to get all audiences in room %s", room.ID)
			res.Code = errors.WSErrorInvalidParameter
			return err
		}
		// 通知请求者连麦被接受。
		answerNotice := &protocol.JoinAnswerNotify{
			RoomID:    room.ID,
			ReqUserID: req.ReqUserID,
			Accept:    true,
			Position:  *joinAudience.JoinPosition,
		}
		s.Notify(xl, req.ReqUserID, protocol.MTAnswerJoinNotify, answerNotice)
		// 通知所有观众有人加入连麦。
		joinNotice := &protocol.AudienceJoinNotify{
			RoomID:    room.ID,
			ReqUserID: req.ReqUserID,
			Position:  *joinAudience.JoinPosition,
			Nickname:  joinAudienceAccount.Nickname,
			Gender:    joinAudienceAccount.Gender,
			AvatarURL: joinAudienceAccount.AvatarURL,
		}
		for _, audience := range audiences {
			if audience.ID != req.ReqUserID {
				s.Notify(xl, audience.ID, protocol.MTAudienceJoinedNotify, joinNotice)
			}
		}
		// 通知主播有观众加入连麦。
		s.Notify(xl, senderID, protocol.MTAudienceJoinedNotify, joinNotice)
	} else {
		// 更新观众状态为观看中。
		joinAudience.Status = protocol.UserStatusWatching
		joinAudience.JoinPosition = nil
		_, err = s.accountCtl.UpdateActiveUser(xl, joinAudience.ID, joinAudience)
		if err != nil {
			xl.Errorf("failed to change user %s status to watching", joinAudience.ID)
			res.Code = errors.WSErrorInvalidParameter
			return err
		}
		// 通知观众。
		answerNotice := &protocol.JoinAnswerNotify{
			RoomID:    room.ID,
			ReqUserID: req.ReqUserID,
			Accept:    false,
		}
		s.Notify(xl, req.ReqUserID, protocol.MTAnswerJoinNotify, answerNotice)
	}
	// 6. 通知主播请求成功。
	res.Code = errors.WSErrorOK
	// 通知等待超时的goroutine连麦请求被响应。
	s.answerJoinRequest(req.RoomID, req.ReqUserID, req.Accept)
	return nil
}

// OnEndJoin 处理观众结束连麦。
func (s *SignalingService) OnEndJoin(xl *xlog.Logger, senderID string, msgBody []byte) error {

	req := &protocol.EndJoinRequest{}
	res := &protocol.EndJoinResponse{}
	// 最后向请求者发送回应。
	defer func() {
		res.RPCID = req.RPCID
		if res.Error == "" {
			res.Error = errors.WSErrorToString[res.Code]
		}
		s.Notify(xl, senderID, protocol.MTEndJoinResponse, res)
	}()
	// 0.解析请求。
	err := req.Unmarshal(msgBody)
	if err != nil {
		res.Code = errors.WSErrorUnknownMessage
		return err
	}
	// 1. 检查用户状态。
	user, err := s.accountCtl.GetActiveUserByID(xl, senderID)
	if err != nil {
		res.Code = errors.WSErrorPlayerNoExist
		return err
	}
	account, err := s.accountCtl.GetAccountByID(xl, senderID)
	if err != nil {
		res.Code = errors.WSErrorPlayerNoExist
		return err
	}
	if user.Status != protocol.UserStatusJoined || user.JoinPosition == nil {
		res.Code = errors.WSErrorPlayerNotJoined
		return fmt.Errorf("user %s status %v, cannot end join", user.ID, user.Status)
	}
	joinPosition := *(user.JoinPosition)
	if user.Room != req.RoomID {
		res.Code = errors.WSErrorPlayerNotInRoom
		return fmt.Errorf("user %s not in room %s, cannot end join", user.ID, req.RoomID)
	}
	// 检查房间是否存在。
	room, err := s.roomCtl.GetRoomByID(xl, user.Room)
	if err != nil {
		res.Code = errors.WSErrorRoomNoExist
		return err
	}
	// 2. 更新观众的用户状态。
	user.Status = protocol.UserStatusWatching
	user.JoinPosition = nil
	_, err = s.accountCtl.UpdateActiveUser(xl, senderID, user)
	if err != nil {
		res.Code = errors.WSErrorInvalidParameter
		return err
	}

	// 3. 通知主播与其他观众。
	// 3.0 获取观众列表
	audiences, err := s.roomCtl.GetAllAudiences(xl, user.Room)
	if err != nil {
		xl.Errorf("failed to list audiences in room %s", user.Room)
		res.Code = errors.WSErrorInvalidParameter
		return err
	}
	endNotice := &protocol.EndJoinNotify{
		RoomID:    user.Room,
		ReqUserID: senderID,
		Position:  joinPosition,
		Nickname:  account.Nickname,
		Gender:    account.Gender,
		AvatarURL: account.AvatarURL,
	}
	// 3.1 通知主播。
	s.Notify(xl, room.Creator, protocol.MTEndJoinNotify, endNotice)
	// 3.2 通知其他观众。
	for _, audience := range audiences {
		if audience.ID != senderID {
			s.Notify(xl, audience.ID, protocol.MTEndJoinNotify, endNotice)
		}
	}
	res.Code = errors.WSErrorOK
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
	// 如果用户在直播中，找出用户创建的的房间。
	if protocol.IsUserBroadCasting(user.Status) {
		room, err := s.roomCtl.GetRoomByFields(xl, map[string]interface{}{"creator": userID})
		if err != nil {
			xl.Infof("cannot find user %s's room, user status is %v, error %v", userID, user.Status, err)
		}
		if room != nil {
			xl.Debugf("will close room %s created by user %s", room.ID, userID)
			err := s.processAnchorLeave(xl, user, room)
			if err != nil {
				return err
			}
		}
	} else if user.Room != "" {
		room, err := s.roomCtl.GetRoomByID(xl, user.Room)
		if err != nil {
			xl.Infof("cannot find user %s's room, user status %v, error %v", userID, user.Status, err)
			return err
		}
		xl.Debugf("user %s will leave room %s", user.ID, room.ID)
		err = s.processAudienceLeave(xl, user, room)
		if err != nil {
			return err
		}
	}

	xl.Debugf("user %s offline:processing end", userID)
	return nil
}

// processAnchorLeave 处理主播离开。
func (s *SignalingService) processAnchorLeave(xl *xlog.Logger, user *protocol.ActiveUser, room *protocol.LiveRoom) error {
	if xl == nil {
		xl = s.xl
	}
	// 如果是PK状态，向其PK对方发送消息。
	if user.Status == protocol.UserStatusPKLive {
		xl.Debugf("user %s's room %s is in PK, notify PK anchor %s", user.ID, room.ID, room.PKAnchor)
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

	audiences, err := s.roomCtl.GetAllAudiences(xl, room.ID)
	if err != nil {
		xl.Errorf("failed to get all audiences of room %s, error %v", room.ID, err)
		return err
	}

	// 关闭该房间。
	if room != nil {
		err := s.roomCtl.CloseRoom(xl, user.ID, room.ID)
		if err != nil {
			xl.Errorf("close room %s created by %s failed, error %v", room.ID, user.ID, err)
			return err
		}
		xl.Infof("room %s created by %s has been closed", room.ID, user.ID)
	}
	// 通知观众房间已关闭。
	closeNotice := &protocol.RoomCloseNotify{RoomID: room.ID}
	for _, audience := range audiences {
		s.Notify(xl, audience.ID, protocol.MTRoomCloseNotify, closeNotice)
	}
	return nil
}

// processAudienceLeave 处理观众离开。
func (s *SignalingService) processAudienceLeave(xl *xlog.Logger, user *protocol.ActiveUser, room *protocol.LiveRoom) error {
	if xl == nil {
		xl = s.xl
	}
	err := s.roomCtl.LeaveRoom(xl, user.ID, room.ID)
	if err != nil {
		xl.Errorf("failed to leave room, user %s, room %s", user.ID, room.ID)
		return err
	}
	if room.Type == protocol.RoomTypeVoice {
		// 如果为连麦观众，通知主播与其他观众连麦已结束。
		if user.Status == protocol.UserStatusJoined && user.JoinPosition != nil {
			joinPosition := *user.JoinPosition
			xl.Debugf("user %s joined room %s at position %d, now leave", user.ID, room.ID, joinPosition)
			endNotice := &protocol.EndJoinNotify{
				RoomID:    user.Room,
				ReqUserID: user.ID,
				Position:  joinPosition,
			}
			// 通知主播。
			s.Notify(xl, room.Creator, protocol.MTEndJoinNotify, endNotice)
			// 获取观众列表。
			audiences, err := s.roomCtl.GetAllAudiences(xl, user.Room)
			if err != nil {
				xl.Errorf("failed to list audiences in room %s,error %v", user.Room, err)
			} else {
				for _, audience := range audiences {
					if audience.ID != user.ID {
						s.Notify(xl, audience.ID, protocol.MTEndJoinNotify, endNotice)
					}
				}
			}
		}
	}
	return nil
}
