package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"

	"github.com/qrtc/qlive/errors"
	"github.com/qrtc/qlive/protocol"
)

// AccountInterface 获取账号信息的接口。
type AccountInterface interface {
	GetAccountByPhoneNumber(phoneNumber string) (*protocol.Account, error)
	GetAccountByID(id string) (*protocol.Account, error)
	CreateAccount(account *protocol.Account) error
	UpdateAccount(id string, account *protocol.Account) (*protocol.Account, error)
}

// SMSCodeInterface 发送短信验证码并记录的接口。
type SMSCodeInterface interface {
	Send(phoneNumber string) (err error)
	Validate(phoneNumber string, smsCode string) (err error)
}

// AccountHandler 处理与账号相关的请求：登录、注册、退出、修改账号信息等
type AccountHandler struct {
	Account AccountInterface
	SMSCode SMSCodeInterface
}

// GetSMSCode 获取短信验证码。
func (h *AccountHandler) GetSMSCode(c *gin.Context) {
	phoneNumber, ok := c.GetQuery("number")
	if !ok {
		c.JSON(http.StatusBadRequest, "empty phone number")
		return
	}
	err := h.SMSCode.Send(phoneNumber)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, nil)
}

const (
	// LoginTypeSMSCode 使用短信验证码登录
	LoginTypeSMSCode = "smscode"
)

// Login 处理登录请求，根据query分不同类型处理。
func (h *AccountHandler) Login(c *gin.Context) {
	loginType, ok := c.GetQuery("logintype")
	if !ok {
		c.JSON(http.StatusBadRequest, fmt.Errorf("empty login type"))
		return
	}
	switch loginType {
	case LoginTypeSMSCode:
		h.LoginBySMS(c)
	default:
		c.JSON(http.StatusBadRequest, fmt.Errorf("login type %s not supported", loginType))
	}
}

// LoginBySMS 使用手机短信验证码登录。
func (h *AccountHandler) LoginBySMS(c *gin.Context) {
	args := protocol.SMSLoginArgs{}
	err := c.BindJSON(&args)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	err = h.SMSCode.Validate(args.PhoneNumber, args.SMSCode)
	if err != nil {
		c.JSON(http.StatusUnauthorized, err)
		return
	}
	account, err := h.Account.GetAccountByPhoneNumber(args.PhoneNumber)
	if err != nil {
		if err.Error() == "not found" {
			newAccount := &protocol.Account{
				ID:          uuid.NewV4().String(),
				PhoneNumber: args.PhoneNumber,
			}
			createErr := h.Account.CreateAccount(newAccount)
			if createErr != nil {
				c.JSON(http.StatusUnauthorized, err)
			}
			res := &protocol.LoginResponse{
				ID:       newAccount.ID,
				Nickname: "",
			}
			h.setLoginCookie(c, newAccount)
			c.JSON(http.StatusOK, res)
			return
		}
		c.JSON(http.StatusUnauthorized, err)
		return
	}
	res := &protocol.LoginResponse{
		ID:       account.ID,
		Nickname: account.Nickname,
	}
	h.setLoginCookie(c, account)
	c.JSON(http.StatusOK, res)
}

// setLoginCookie 设置登录后的cookie。TODO：确定cookie的格式。
func (h *AccountHandler) setLoginCookie(c *gin.Context, account *protocol.Account) {
	token := account.ID + "#" + uuid.NewV4().String()
	c.SetCookie(protocol.LoginCookieKey, token, 0, "/", "qlive.qiniu.com", true, false)
}

// UpdateProfile 修改用户信息。
func (h *AccountHandler) UpdateProfile(c *gin.Context) {

	id := c.GetString(protocol.UserIDContextKey)

	account, err := h.Account.GetAccountByID(id)
	if err != nil {
		httpErr := errors.NewHTTPErrorNotFound().WithMessagef("user %s not found", id)
		c.JSON(httpErr.Code, httpErr)
		c.Abort()
		return
	}

	args := protocol.UpdateProfileArgs{}
	bindErr := c.BindJSON(&args)
	if bindErr != nil {
		httpErr := errors.NewHTTPErrorBadRequest().WithMessage("invalid args")
		c.JSON(httpErr.Code, httpErr)
		c.Abort()
		return
	}

	if args.Nickname != "" {
		account.Nickname = args.Nickname
	}
	if args.Gender != "" {
		account.Gender = args.Gender
	}

	newAccount, err := h.Account.UpdateAccount(id, account)
	if err != nil {
		c.JSON(http.StatusBadRequest, err)
		c.Abort()
		return
	}
	ret := &protocol.UpdateProfileResponse{
		ID:       newAccount.ID,
		Nickname: newAccount.Nickname,
		Gender:   newAccount.Gender,
	}
	c.JSON(http.StatusOK, ret)
}

// Logout 退出登录。
func (h *AccountHandler) Logout(c *gin.Context) {
	c.SetCookie(protocol.LoginCookieKey, "", -1, "/", "qlive.qiniu.com", true, false)
	c.JSON(http.StatusOK, nil)
}
