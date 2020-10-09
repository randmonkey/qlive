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
	"math/rand"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/qiniu/x/xlog"

	"github.com/qrtc/qlive/errors"
	"github.com/qrtc/qlive/protocol"
)

// AccountInterface 获取账号信息的接口。
type AccountInterface interface {
	GetAccountByPhoneNumber(xl *xlog.Logger, phoneNumber string) (*protocol.Account, error)
	GetAccountByID(xl *xlog.Logger, id string) (*protocol.Account, error)
	CreateAccount(xl *xlog.Logger, account *protocol.Account) error
	UpdateAccount(xl *xlog.Logger, id string, account *protocol.Account) (*protocol.Account, error)
	AccountLogin(xl *xlog.Logger, id string) (user *protocol.ActiveUser, err error)
	AccountLogout(xl *xlog.Logger, id string) error
}

// SMSCodeInterface 发送短信验证码并记录的接口。
type SMSCodeInterface interface {
	Send(xl *xlog.Logger, phoneNumber string) (err error)
	Validate(xl *xlog.Logger, phoneNumber string, smsCode string) (err error)
}

// AccountHandler 处理与账号相关的请求：登录、注册、退出、修改账号信息等
type AccountHandler struct {
	Account AccountInterface
	SMSCode SMSCodeInterface
}

// validatePhoneNumber 检查手机号码是否符合规则。
func validatePhoneNumber(phoneNumber string) bool {
	phoneNumberRegExp := regexp.MustCompile(`1[3-9][0-9]{9}`)
	return phoneNumberRegExp.MatchString(phoneNumber)
}

// @Tags qlive api
// @ID send-sms-code
// @Summary Send SMS code to user
// @Description Send SMS code to user and the code will survive for ten minutes
// @Accept  json
// @Produce  json
// @Param phone_number query string true "Send sms code to user's phone number"
// @Success 200 {string} ok
// @Failure 400 {object} errors.HTTPError
// @Failure 429 {object} errors.HTTPError
// @Failure 500 {object} errors.HTTPError
// @Router /send_sms_code [post]
// SendSMSCode 发送短信验证码。
func (h *AccountHandler) SendSMSCode(c *gin.Context) {
	xl := c.MustGet(protocol.XLogKey).(*xlog.Logger)
	requestID := xl.ReqId
	phoneNumber, ok := c.GetQuery("phone_number")
	if !ok {
		httpErr := errors.NewHTTPErrorInvalidPhoneNumber().WithRequestID(requestID).WithMessage("empty phone number")
		c.JSON(http.StatusBadRequest, httpErr)
		return
	}
	if !validatePhoneNumber(phoneNumber) {
		httpErr := errors.NewHTTPErrorInvalidPhoneNumber().WithRequestID(requestID).WithMessage("invalid phone number")
		c.JSON(http.StatusBadRequest, httpErr)
		return
	}
	err := h.SMSCode.Send(xl, phoneNumber)
	if err != nil {
		serverErr, ok := err.(*errors.ServerError)
		if ok && serverErr.Code == errors.ServerErrorSMSSendTooFrequent {
			xl.Infof("SMS code has been sent to %s, cannot resend in short time", phoneNumber)
			httpErr := errors.NewHTTPErrorSMSSendTooFrequent().WithRequestID(requestID)
			c.JSON(http.StatusTooManyRequests, httpErr)
			return
		}
		xl.Errorf("failed to send sms code to phone number %s, error %v", phoneNumber, err)
		c.JSON(http.StatusInternalServerError, err)
		return
	}
	xl.Infof("SMS code sent to number %s", phoneNumber)
	c.JSON(http.StatusOK, "")
}

const (
	// LoginTypeSMSCode 使用短信验证码登录
	LoginTypeSMSCode = "smscode"
)

// @Tags qlive api
// @ID log-in
// @Summary User log in
// @Description User log in with sms code
// @Accept  json
// @Produce  json
// @Param logintype query string true "type of user logs in"
// @Param SMSLoginArgs body protocol.SMSLoginArgs true "user's phone number and sms code"
// @Success 200 {object} protocol.LoginResponse
// @Failure 400 {object} errors.HTTPError
// @Failure 401 {object} errors.HTTPError
// @Router /login [post]
// Login 处理登录请求，根据query分不同类型处理。
func (h *AccountHandler) Login(c *gin.Context) {
	xl := c.MustGet(protocol.XLogKey).(*xlog.Logger)
	requestID := xl.ReqId
	loginType, ok := c.GetQuery("logintype")
	if !ok {
		httpErr := errors.NewHTTPErrorBadLoginType().WithRequestID(requestID).WithMessage("empty login type")
		c.JSON(http.StatusBadRequest, httpErr)
		return
	}
	switch loginType {
	case LoginTypeSMSCode:
		h.LoginBySMS(c)
	default:
		httpErr := errors.NewHTTPErrorBadLoginType().WithRequestID(requestID).WithMessagef("login type %s not supported", loginType)
		c.JSON(http.StatusBadRequest, httpErr)
	}
}

// generateUserID 生成新的用户ID。
func (h *AccountHandler) generateUserID() string {
	alphaNum := "0123456789abcdefghijklmnopqrstuvwxyz"
	idLength := 12
	id := ""
	for i := 0; i < idLength; i++ {
		index := rand.Intn(len(alphaNum))
		id = id + string(alphaNum[index])
	}
	return id
}

func (h *AccountHandler) generateNicknameByPhoneNumber(phoneNumber string) string {
	namePrefix := "用户_"
	if len(phoneNumber) < 4 {
		return namePrefix + phoneNumber
	}
	return namePrefix + phoneNumber[len(phoneNumber)-4:]
}

// LoginBySMS 使用手机短信验证码登录。
func (h *AccountHandler) LoginBySMS(c *gin.Context) {
	xl := c.MustGet(protocol.XLogKey).(*xlog.Logger)
	requestID := xl.ReqId
	args := protocol.SMSLoginArgs{}
	err := c.BindJSON(&args)
	if err != nil {
		xl.Infof("invalid args in body, error %v", err)
		httpError := errors.NewHTTPErrorBadRequest().WithRequestID(requestID).WithMessage("invalid args in request body")
		c.JSON(http.StatusBadRequest, httpError)
		return
	}

	err = h.SMSCode.Validate(xl, args.PhoneNumber, args.SMSCode)
	if err != nil {
		xl.Infof("validate SMS code failed, error %v", err)
		httpErr := errors.NewHTTPErrorWrongSMSCode().WithRequestID(requestID)
		c.JSON(http.StatusUnauthorized, httpErr)
		return
	}
	account, err := h.Account.GetAccountByPhoneNumber(xl, args.PhoneNumber)
	if err != nil {
		if err.Error() == "not found" {
			xl.Infof("phone number %s not found, create new account", args.PhoneNumber)
			newAccount := &protocol.Account{
				ID:          h.generateUserID(),
				Nickname:    h.generateNicknameByPhoneNumber(args.PhoneNumber),
				PhoneNumber: args.PhoneNumber,
			}
			createErr := h.Account.CreateAccount(xl, newAccount)
			if createErr != nil {
				xl.Errorf("failed to craete account, error %v", err)
				httpErr := errors.NewHTTPErrorUnauthorized().WithRequestID(requestID)
				c.JSON(http.StatusUnauthorized, httpErr)
				return
			}
			account = newAccount
		} else {
			xl.Errorf("get account by phone number failed, error %v", err)
			httpErr := errors.NewHTTPErrorUnauthorized().WithRequestID(requestID)
			c.JSON(http.StatusUnauthorized, httpErr)
			return
		}
	}
	// 更新该账号状态为已登录。
	user, err := h.Account.AccountLogin(xl, account.ID)
	if err != nil {
		serverErr, ok := err.(*errors.ServerError)
		if ok && serverErr.Code == errors.ServerErrorUserLoggedin {
			xl.Infof("user %s already logged in", account.ID)
			httpErr := errors.NewHTTPErrorAlreadyLoggedin().WithRequestID(requestID)
			c.JSON(http.StatusUnauthorized, httpErr)
			return
		}
		xl.Errorf("failed to set account %s to status logged in, error %v", account.ID, err)
		httpErr := errors.NewHTTPErrorUnauthorized().WithRequestID(requestID)
		c.JSON(http.StatusUnauthorized, httpErr)
		return
	}

	res := &protocol.LoginResponse{
		UserInfo: protocol.UserInfo{
			ID:       account.ID,
			Nickname: account.Nickname,
			Gender:   account.Gender,
		},
		Token:  user.Token,
		Status: string(user.Status),
		Room:   user.Room,
	}
	c.SetCookie(protocol.LoginTokenKey, user.Token, 0, "/", "qlive.qiniu.com", true, false)
	c.JSON(http.StatusOK, res)
}

// @Tags qlive api
// @ID update-profile
// @Summary User updates profile
// @Description User updates personal profile
// @Accept  json
// @Produce  json
// @Param updatedProfile body protocol.UpdateProfileArgs true "User updates personal profile"
// @Success 200 {string} ok
// @Failure 400 {object} errors.HTTPError
// @Failure 404 {object} errors.HTTPError
// @Failure 500 {object} errors.HTTPError
// @Router /profile [post]
// UpdateProfile 修改用户信息。
func (h *AccountHandler) UpdateProfile(c *gin.Context) {
	xl := c.MustGet(protocol.XLogKey).(*xlog.Logger)
	requestID := xl.ReqId
	id := c.GetString(protocol.UserIDContextKey)

	args := protocol.UpdateProfileArgs{}
	bindErr := c.BindJSON(&args)
	if bindErr != nil {
		xl.Infof("invalid args in request body, error %v", bindErr)
		httpErr := errors.NewHTTPErrorBadRequest().WithRequestID(requestID).WithMessage("invalid args in request body")
		c.JSON(http.StatusBadRequest, httpErr)
		return
	}

	account, err := h.Account.GetAccountByID(xl, id)
	if err != nil {
		xl.Infof("cannot find account, error %v", err)
		httpErr := errors.NewHTTPErrorNoSuchUser().WithRequestID(requestID).WithMessagef("user %s not found", id)
		c.JSON(http.StatusNotFound, httpErr)
		return
	}
	if account.ID != "" && account.ID != id {
		xl.Infof("user %s try to update profile of other user %s", id, account.ID)
		httpErr := errors.NewHTTPErrorNoSuchUser().WithRequestID(requestID).WithMessagef("user %s not found", id)
		c.JSON(http.StatusNotFound, httpErr)
		return
	}

	// TODO: validate updated profile.
	if args.Nickname != "" {
		account.Nickname = args.Nickname
	}
	if args.Gender != "" {
		account.Gender = args.Gender
	}

	newAccount, err := h.Account.UpdateAccount(xl, id, account)
	if err != nil {
		httpErr := errors.NewHTTPErrorInternal().WithRequestID(requestID).WithMessagef("update account failed: %v", err)
		c.JSON(http.StatusInternalServerError, httpErr)
		return
	}
	ret := &protocol.UpdateProfileResponse{
		ID:       newAccount.ID,
		Nickname: newAccount.Nickname,
		Gender:   newAccount.Gender,
	}
	c.JSON(http.StatusOK, ret)
}

// @Tags qlive api
// @ID log-out
// @Summary User logs out
// @Description Online user logs out
// @Accept  json
// @Produce  json
// @Success 200 {string} ok
// @Failure 401 {object} errors.HTTPError
// @Router /logout [post]
// Logout 退出登录。
func (h *AccountHandler) Logout(c *gin.Context) {
	xl := c.MustGet(protocol.XLogKey).(*xlog.Logger)
	requestID := xl.ReqId
	id, exist := c.Get(protocol.UserIDContextKey)
	if !exist {
		xl.Infof("cannot find ID in context")
		httpErr := errors.NewHTTPErrorNotLoggedIn().WithRequestID(requestID)
		c.JSON(http.StatusUnauthorized, httpErr)
	}
	err := h.Account.AccountLogout(xl, id.(string))
	if err != nil {
		xl.Errorf("user %s log out error: %v", id, err)
		c.JSON(http.StatusUnauthorized, "")
	}
	xl.Infof("user %s logged out", id)
	c.SetCookie(protocol.LoginTokenKey, "", -1, "/", "qlive.qiniu.com", true, false)
	c.JSON(http.StatusOK, "")
}
