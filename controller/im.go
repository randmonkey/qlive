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

	"github.com/qiniu/x/xlog"
	rcsdk "github.com/rongcloud/server-sdk-go/v3/sdk"

	"github.com/qrtc/qlive/config"
	"github.com/qrtc/qlive/protocol"
)

const (
	// DefaultPortraitURL 默认IM头像地址。
	DefaultPortraitURL = "https://developer.rongcloud.cn/static/images/newversion-logo.png"
)

// RongCloudIMController 融云IM控制器，执行IM用户及聊天室管理。
type RongCloudIMController struct {
	appKey    string
	appSecret string
	// systemUserID 系统用户ID，发送到该ID的IM消息将被当作发送给系统的信令处理。
	systemUserID    string
	rongCloudClient *rcsdk.RongCloud
	xl              *xlog.Logger
}

// IMInterface IM用户管理相关接口。
type IMInterface interface {
	GetUserToken(xl *xlog.Logger, userID string) (*protocol.IMUser, error)
	ProcessMessage(xl *xlog.Logger, msg interface{}) error
}

// NewIMController 生成IM控制器。
func NewIMController(conf *config.IMConfig, xl *xlog.Logger) (IMInterface, error) {
	switch conf.Provider {
	case "rongcloud":
		if conf.RongCloud == nil {
			return nil, fmt.Errorf("empty config for rongcloud IM")
		}
		return NewRongCloudIMController(conf.RongCloud.AppKey, conf.RongCloud.AppSecret, xl)
	case "test":
		return &mockIMController{}, nil
	default:
		return nil, fmt.Errorf("unsupported provider %s", conf.Provider)
	}
}

// NewRongCloudIMController 创建新的融云IM控制器。
func NewRongCloudIMController(appKey string, appSecret string, xl *xlog.Logger) (*RongCloudIMController, error) {
	if xl == nil {
		xl = xlog.New("qlive-rongcloud-im-controller")
	}

	return &RongCloudIMController{
		appKey:          appKey,
		appSecret:       appSecret,
		rongCloudClient: rcsdk.NewRongCloud(appKey, appSecret),
		xl:              xl,
	}, nil
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
	return &protocol.IMUser{
		UserID:   userRes.UserID,
		Username: userRes.UserID,
		Token:    userRes.Token,
	}, nil
}

func (c *RongCloudIMController) validateSignature(sign protocol.RongCloudSignature) bool {
	localSignature := sha1.Sum([]byte(c.appSecret + sign.Nonce + sign.SignTimestamp))
	return string(localSignature[:]) == sign.Signature
}

func (c *RongCloudIMController) processMessage(xl *xlog.Logger, msg *protocol.RongCloudMessage) error {
	if xl == nil {
		xl = c.xl
	}

	if msg.ObjectName == "RC:TxtMsg" && msg.ToUserID == c.systemUserID {
		xl.Debugf("msg content %+v", msg.Content)
	}
	return nil
}

// ProcessMessage 处理通过回调收到的消息。
func (c *RongCloudIMController) ProcessMessage(xl *xlog.Logger, msg interface{}) error {
	rcMsg, ok := msg.(*protocol.RongCloudMessage)
	if !ok {
		return fmt.Errorf("incorrect message type")
	}
	return c.processMessage(xl, rcMsg)
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
