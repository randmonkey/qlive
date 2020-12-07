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
	"crypto/sha1"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/qiniu/x/xlog"
	rcsdk "github.com/rongcloud/server-sdk-go/v3/sdk"

	"github.com/qrtc/qlive/config"
	"github.com/qrtc/qlive/protocol"
)

const (
	// DefaultPortraitURL 默认IM头像地址。
	DefaultPortraitURL = "https://developer.rongcloud.cn/static/images/newversion-logo.png"
	// DefaultPingPeriod 轮询用户是否在线的时间间隔。
	DefaultPingPeriod = 5 * time.Second
	// DefaultInactiveTimeout 判定用户掉线的超时时间。
	DefaultInactiveTimeout = 20 * time.Second
)

// RongCloudIMController 融云IM控制器，执行IM用户及聊天室管理。
type RongCloudIMController struct {
	appKey    string
	appSecret string
	// systemUserID 系统用户ID，发送到该ID的IM消息将被当作发送给系统的信令处理。
	systemUserID string
	// pingPeriod 发送ping消息并清理非活跃用户的时间。
	pingPeriod time.Duration
	// inactiveTimeout 清理不活跃用户的超时时间，该段时间内为发送过消息的用户将被清理。
	inactiveTimeout  time.Duration
	userLock         sync.RWMutex
	userMap          map[string]*protocol.IMUser
	signalingService *SignalingService
	rongCloudClient  *rcsdk.RongCloud
	xl               *xlog.Logger
	stopCh           chan struct{}
}

// 融云的IM消息类型。
const (
	RongCloudMessageTypeText = "RC:TxtMsg"
)

// IMInterface IM用户与消息管理相关接口。
type IMInterface interface {
	GetUserToken(xl *xlog.Logger, userID string) (*protocol.IMUser, error)
	ProcessMessage(xl *xlog.Logger, msg interface{}) error
	SendTextMessage(xl *xlog.Logger, userID string, msg string) error
	UserOnline(xl *xlog.Logger, userID string, onlineTime time.Time) error
	UserOffline(xl *xlog.Logger, userID string, offlineTime time.Time) error
	WithSignalingService(s *SignalingService) IMInterface
}

// NewIMController 生成IM控制器。
func NewIMController(conf *config.IMConfig, xl *xlog.Logger) (IMInterface, error) {
	switch conf.Provider {
	case "rongcloud":
		if conf.RongCloud == nil {
			return nil, fmt.Errorf("empty config for rongcloud IM")
		}
		return NewRongCloudIMController(conf, xl)
	case "test":
		return &mockIMController{}, nil
	default:
		return nil, fmt.Errorf("unsupported provider %s", conf.Provider)
	}
}

// NewRongCloudIMController 创建新的融云IM控制器。
func NewRongCloudIMController(conf *config.IMConfig, xl *xlog.Logger) (*RongCloudIMController, error) {
	if xl == nil {
		xl = xlog.New("qlive-rongcloud-im-controller")
	}
	appKey := conf.RongCloud.AppKey
	appSecret := conf.RongCloud.AppSecret
	systemUserID := conf.SystemUserID

	c := &RongCloudIMController{
		appKey:          appKey,
		appSecret:       appSecret,
		systemUserID:    systemUserID,
		userMap:         map[string]*protocol.IMUser{},
		rongCloudClient: rcsdk.NewRongCloud(appKey, appSecret),
		xl:              xl,
		stopCh:          make(chan struct{}),
	}
	if conf.PingTickerSecond == 0 {
		c.pingPeriod = DefaultPingPeriod
	} else {
		c.pingPeriod = time.Duration(conf.PingTickerSecond) * time.Second
	}

	if conf.PongTimeoutSecond == 0 {
		c.inactiveTimeout = DefaultInactiveTimeout
	} else {
		c.inactiveTimeout = time.Duration(conf.PongTimeoutSecond) * time.Second
	}

	_, err := c.GetUserToken(xl, systemUserID)
	if err != nil {
		xl.Errorf("failed to get user token for system user %s, error %v", systemUserID, err)
		return nil, err
	}
	return c, nil
}

// Stop 停止循环运行的goroutine的运行。
func (c *RongCloudIMController) Stop() {
	c.stopCh <- struct{}{}
}

// GetUserToken 用户注册，生成User token
func (c *RongCloudIMController) GetUserToken(xl *xlog.Logger, userID string) (*protocol.IMUser, error) {
	if xl == nil {
		xl = c.xl
	}
	userRes, err := c.rongCloudClient.UserRegister(userID, userID, DefaultPortraitURL)
	if err != nil {
		xl.Errorf("failed to get user token from rongcloud, error %v", err)
		return nil, err
	}
	now := time.Now()
	imUser := &protocol.IMUser{
		UserID:           userRes.UserID,
		Username:         userRes.UserID,
		Token:            userRes.Token,
		LastRegisterTime: now,
		LastOnlineTime:   now,
	}
	c.userLock.Lock()
	c.userMap[userID] = imUser
	c.userLock.Unlock()
	return imUser, nil
}

func (c *RongCloudIMController) validateSignature(sign protocol.RongCloudSignature) bool {
	localSignature := sha1.Sum([]byte(c.appSecret + sign.Nonce + sign.SignTimestamp))
	return string(localSignature[:]) == sign.Signature
}

func (c *RongCloudIMController) processMessage(xl *xlog.Logger, msg *protocol.RongCloudMessage) error {
	if xl == nil {
		xl = c.xl
	}

	if msg.ObjectName == RongCloudMessageTypeText && msg.ToUserID == c.systemUserID {
		textContent := msg.Content.Content
		msgTime := time.Unix(msg.MsgTimestampMS/1000, (msg.MsgTimestampMS%1000)*1000*1000)
		c.setUserOnlineTime(xl, msg.FromUserID, msgTime)
		// 当信令服务使用im时，处理信令消息。
		if c.isSignalingMessage(textContent) && c.signalingService != nil {
			xl.Debugf("signaling message %s", textContent)
			c.signalingService.OnMessage(xl, msg.FromUserID, []byte(textContent))
		}
	}
	return nil
}

func (c *RongCloudIMController) isSignalingMessage(msg string) bool {
	parts := strings.SplitN(msg, "=", 2)
	if len(parts) < 2 {
		return false
	}
	msgType := parts[0]
	msgBody := parts[1]
	return (len(msgType) > 0) && (msgBody[0] == '{' && msgBody[len(msgBody)-1] == '}')
}

// ProcessMessage 处理通过回调收到的消息。
func (c *RongCloudIMController) ProcessMessage(xl *xlog.Logger, msg interface{}) error {
	rcMsg, ok := msg.(*protocol.RongCloudMessage)
	if !ok {
		return fmt.Errorf("incorrect message type")
	}
	return c.processMessage(xl, rcMsg)
}

// WithSignalingService 设置信令处理服务。
func (c *RongCloudIMController) WithSignalingService(s *SignalingService) IMInterface {
	if s != nil {
		c.signalingService = s
		s.Notify = c.sendSignalingMessage
		go c.pingUserLoop(c.stopCh)
		return c
	}
	return c
}

func (c *RongCloudIMController) pingUserLoop(stopCh chan struct{}) {
	t := time.NewTicker(c.pingPeriod)
	for {
		select {
		case <-t.C:
			c.pingUsers()
			c.removeInactiveUsers()
		case <-stopCh:
			c.xl.Infof("ping user loop stopped.")
			return
		}
	}
}

func (c *RongCloudIMController) pingUsers() {
	c.userLock.RLock()
	defer c.userLock.RUnlock()
	for userID := range c.userMap {
		if userID == c.systemUserID {
			continue
		}
		c.sendSignalingMessage(c.xl, userID, protocol.MT_Ping, &protocol.Ping{})
	}
}

func (c *RongCloudIMController) removeInactiveUsers() {
	c.userLock.Lock()
	defer c.userLock.Unlock()
	for userID, user := range c.userMap {
		if userID == c.systemUserID {
			continue
		}
		if user.LastOnlineTime.Before(time.Now().Add(-c.inactiveTimeout)) {
			c.xl.Infof("user %s last online time %v, seems to be offlined", userID, user.LastOnlineTime)
			if c.signalingService != nil {
				c.signalingService.OnUserOffline(c.xl, userID)
			}
			delete(c.userMap, userID)
		}
	}
}

func (c *RongCloudIMController) getIMUser(userID string) (user *protocol.IMUser, ok bool) {
	c.userLock.RLock()
	defer c.userLock.RUnlock()

	user, ok = c.userMap[userID]
	return user, ok
}

func (c *RongCloudIMController) setUserOnlineTime(xl *xlog.Logger, userID string, t time.Time) {
	if xl == nil {
		xl = c.xl
	}

	user, ok := c.getIMUser(userID)
	if ok && (user.LastOnlineTime.IsZero() || user.LastOnlineTime.Before(t)) {
		xl.Debugf("user %s, last online time %v", userID, t)
		user.LastOnlineTime = t
	}
}

func (c *RongCloudIMController) sendSignalingMessage(xl *xlog.Logger, userID string, msgType string, msg MarshallableMessage) error {
	buf, err := msg.Marshal()
	if err != nil {
		return err
	}
	err = c.SendTextMessage(xl, userID, msgType+"="+string(buf))
	return err
}

// SendTextMessage 发送文字消息。
func (c *RongCloudIMController) SendTextMessage(xl *xlog.Logger, userID string, content string) error {
	if xl == nil {
		xl = c.xl
	}
	rcTXTMsg := &rcsdk.TXTMsg{
		Content: content,
	}
	err := c.rongCloudClient.PrivateSend(c.systemUserID, []string{userID}, RongCloudMessageTypeText, rcTXTMsg, "", "",
		0, 0, 0, 0, 0)
	if err != nil {
		xl.Infof("failed to send message to %s, error %v", userID, err)
		return err
	}
	xl.Debugf("send message %s to %s", content, userID)
	return nil
}

// UserOnline 用户上线。
func (c *RongCloudIMController) UserOnline(xl *xlog.Logger, userID string, onlineTime time.Time) error {
	if xl == nil {
		xl = c.xl
	}
	xl.Infof("user %s IM online", userID)
	c.setUserOnlineTime(xl, userID, onlineTime)
	return nil
}

// UserOffline 用户下线。
func (c *RongCloudIMController) UserOffline(xl *xlog.Logger, userID string, offlineTime time.Time) error {
	if xl == nil {
		xl = c.xl
	}
	xl.Infof("user %s IM offline at %v", userID, offlineTime)
	user, ok := c.getIMUser(userID)
	if ok {
		user.LastOfflineTime = offlineTime
	}
	return nil
}

type mockIMController struct{}

func (m *mockIMController) GetUserToken(xl *xlog.Logger, userID string) (*protocol.IMUser, error) {
	return &protocol.IMUser{
		UserID:   userID,
		Username: userID,
		Token:    "im-token." + userID,
	}, nil
}

func (m *mockIMController) ProcessMessage(xl *xlog.Logger, msg interface{}) error {
	return nil
}

func (m *mockIMController) SendTextMessage(xl *xlog.Logger, userID string, msg string) error {
	return nil
}

func (m *mockIMController) WithSignalingService(s *SignalingService) IMInterface {
	return m
}

func (m *mockIMController) UserOnline(xl *xlog.Logger, userID string, onlineTime time.Time) error {
	return nil
}

func (m *mockIMController) UserOffline(xl *xlog.Logger, userID string, offlineTime time.Time) error {
	return nil
}
