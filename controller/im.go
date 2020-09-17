package controller

import (
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
	rongCloudClient *rcsdk.RongCloud
	xl              *xlog.Logger
}

// IMInterface IM用户管理相关接口。
type IMInterface interface {
	GetUserToken(xl *xlog.Logger, userID string) (*protocol.IMUser, error)
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

type mockIMController struct{}

func (m *mockIMController) GetUserToken(xl *xlog.Logger, userID string) (*protocol.IMUser, error) {
	return &protocol.IMUser{
		UserID:   userID,
		Username: userID,
		Token:    "im-token." + userID,
	}, nil
}
