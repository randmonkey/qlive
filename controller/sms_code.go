package controller

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	qiniuauth "github.com/qiniu/api.v7/v7/auth"
	qiniusms "github.com/qiniu/api.v7/v7/sms"
	"github.com/qiniu/qmgo"
	"github.com/qiniu/x/xlog"
	"github.com/qrtc/qlive/config"
	"github.com/qrtc/qlive/errors"
	"github.com/qrtc/qlive/protocol"
)

// SMSCodeCollection 存储已发送的短信验证码的表。
const SMSCodeCollection = "sms_code"

// SMSCodeParamKey 验证码的模板变量名称。
const SMSCodeParamKey = "code"

var (
	// SMSCodeDefaultResendTimeout 重发短信验证码的过期时间。在该时间内已经发送过验证码的手机号不能重发。
	SMSCodeDefaultResendTimeout = time.Minute
	// SMSCodeDefaultValidateTimeout 短信验证码的有效时间。在短信验证码发出后该时间内，验证码有效，过期失效。
	SMSCodeDefaultValidateTimeout = 5 * time.Minute
	// SMSCodeExpireTimeout 短信验证码过期从数据库删除的时间。
	SMSCodeExpireTimeout = 10 * time.Minute
)

// SMSSender 发送短信的接口。
type SMSSender interface {
	SendMessage(xl *xlog.Logger, phoneNumber string, code string) error
}

// SMSCodeController 短信验证码管理。
type SMSCodeController struct {
	mongoClient     *qmgo.Client
	smsCodeColl     *qmgo.Collection
	smsSender       SMSSender
	resendTimeout   time.Duration
	validateTimeout time.Duration
	expireTimeout   time.Duration
	randSource      rand.Source
	xl              *xlog.Logger
}

// NewSMSCodeController 创建 SMSCodeController。
func NewSMSCodeController(mongoURI string, database string, smsConfig *config.SMSConfig, xl *xlog.Logger) (*SMSCodeController, error) {
	if xl == nil {
		xl = xlog.New("qlive-sms-code-controller")
	}
	mongoClient, err := qmgo.NewClient(context.Background(), &qmgo.Config{
		Uri:      mongoURI,
		Database: database,
	})
	if err != nil {
		xl.Errorf("failed to create mongo client, error %v", err)
		return nil, err
	}
	smsCodeColl := mongoClient.Database(database).Collection(SMSCodeCollection)
	c := &SMSCodeController{
		mongoClient:     mongoClient,
		smsCodeColl:     smsCodeColl,
		resendTimeout:   SMSCodeDefaultResendTimeout,
		validateTimeout: SMSCodeDefaultValidateTimeout,
		expireTimeout:   SMSCodeExpireTimeout,
		randSource:      rand.NewSource(time.Now().UnixNano()),
		xl:              xl,
	}
	// 创建短信发送器。
	switch smsConfig.Provider {
	// 模拟的短信发送器，仅供测试使用。
	case "test":
		c.smsSender = &mockSMSSender{}
	case "qiniu":
		sender := NewQiniuSMSSender(smsConfig.QiniuSMS)
		c.smsSender = sender
	default:
		xl.Errorf("unsupported SMS provider %s", smsConfig.Provider)
		return nil, fmt.Errorf("unsupported SMS provider")
	}
	return c, nil
}

type mockSMSSender struct {
}

func (m *mockSMSSender) SendMessage(xl *xlog.Logger, phoneNumber string, code string) error {
	xl.Debugf("mock: send code %s to %s", code, phoneNumber)
	return nil
}

// QiniuSMSSender 七牛云短信发送器，对接七牛云短信平台发送验证码。
type QiniuSMSSender struct {
	conf    *config.QiniuSMSConfig
	manager *qiniusms.Manager
}

// NewQiniuSMSSender 创建七牛云短信发送器。
func NewQiniuSMSSender(conf *config.QiniuSMSConfig) *QiniuSMSSender {
	manager := qiniusms.NewManager(&qiniuauth.Credentials{
		AccessKey: conf.KeyPair.AccessKey,
		SecretKey: []byte(conf.KeyPair.SecretKey),
	})
	return &QiniuSMSSender{
		conf:    conf,
		manager: manager,
	}
}

// SendMessage 发送验证码为code的短信。
func (s *QiniuSMSSender) SendMessage(xl *xlog.Logger, phoneNumber string, code string) error {
	_, err := s.manager.SendMessage(qiniusms.MessagesRequest{
		SignatureID: s.conf.SignatureID,
		TemplateID:  s.conf.TemplateID,
		Mobiles:     []string{phoneNumber},
		Parameters:  map[string]interface{}{SMSCodeParamKey: code},
	})
	if err != nil {
		xl.Errorf("failed to send message, error %v", err)
		return err
	}
	return nil
}

// Send 对给定手机号发送验证码。
func (c *SMSCodeController) Send(xl *xlog.Logger, phoneNumber string) error {
	if xl == nil {
		xl = c.xl
	}
	// TODO: 校验手机号码。
	// 首先查找是否有1分钟内发送给该手机号的记录。
	now := time.Now()
	filter := map[string]interface{}{
		"phoneNumber": phoneNumber,
		"sendTime": map[string]interface{}{
			"$gt": now.Add(-c.resendTimeout),
		},
	}
	sendCount, err := c.smsCodeColl.Find(context.Background(), filter).Count()
	if err != nil && !qmgo.IsErrNoDocuments(err) {
		xl.Errorf("failed to find sms code record in mongo, error %v", err)
		return err
	}
	if err == nil && sendCount > 0 {
		xl.Infof("phone number %s has already been sent to in 1 minute", phoneNumber)
		return &errors.ServerError{Code: errors.ServerErrorSMSSendTooFrequent, Summary: ""}
	}

	code := fmt.Sprintf("%06d", c.randSource.Int63()%1000000)

	// TODO: use transaction to save sms code record(need mongo >= 4.0).
	smsCodeRecord := &protocol.SMSCodeRecord{
		PhoneNumber: phoneNumber,
		SMSCode:     code,
		SendTime:    time.Now(),
		ExpireAt:    time.Now().Add(c.expireTimeout),
	}
	insertRes, err := c.smsCodeColl.InsertOne(context.Background(), smsCodeRecord)
	if err != nil {
		xl.Errorf("failed to insert SMS code record, error %v", err)
		return err
	}
	err = c.smsSender.SendMessage(xl, phoneNumber, code)
	if err != nil {
		xl.Errorf("failed to send SMS code, error %v", err)
		// 删除已插入的发送记录。
		deleteErr := c.smsCodeColl.Remove(context.Background(), map[string]interface{}{"_id": insertRes.InsertedID})
		if deleteErr != nil {
			xl.Errorf("failed to delete sms code record in mongo, error %v", deleteErr)
		}
		return err
	}
	xl.Debugf("sent code %s to phone number %s", code, phoneNumber)
	return nil
}

// Validate 检验手机号与验证码是否符合。
func (c *SMSCodeController) Validate(xl *xlog.Logger, phoneNumber string, code string) error {
	if xl == nil {
		xl = c.xl
	}
	now := time.Now()
	filter := map[string]interface{}{
		"phoneNumber": phoneNumber,
		"smsCode":     code,
		"sendTime": map[string]interface{}{
			"$gt": now.Add(-c.validateTimeout),
		},
	}
	smsCodeRecord := protocol.SMSCodeRecord{}
	err := c.smsCodeColl.Find(context.Background(), filter).One(&smsCodeRecord)
	if err != nil {
		if qmgo.IsErrNoDocuments(err) {
			xl.Infof("sms code is not found or expired")
		} else {
			xl.Errorf("failed to find sms code record, error %v", err)
		}
		return err
	}
	return nil
}
