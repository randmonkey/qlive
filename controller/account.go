package controller

import (
	"context"
	"fmt"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/qiniu/qmgo"
	"github.com/qiniu/x/xlog"
	"github.com/qrtc/qlive/protocol"
)

// AccountController 用户注册、更新信息、登录、退出登录等操作。
type AccountController struct {
	mongoClient    *qmgo.Client
	accountColl    *qmgo.Collection
	activeUserColl *qmgo.Collection
	xl             *xlog.Logger
}

// NewAccountController 创建account controller.
func NewAccountController(mongoURI string, database string, xl *xlog.Logger) (*AccountController, error) {
	if xl == nil {
		xl = xlog.New("qlive-account-controller")
	}
	mongoClient, err := qmgo.NewClient(context.Background(), &qmgo.Config{
		Uri:      mongoURI,
		Database: database,
	})
	if err != nil {
		xl.Errorf("failed to create mongo client, error %v", err)
		return nil, err
	}
	accountColl := mongoClient.Database(database).Collection(AccountCollection)
	activeUserColl := mongoClient.Database(database).Collection(ActiveUserCollection)
	return &AccountController{
		mongoClient:    mongoClient,
		accountColl:    accountColl,
		activeUserColl: activeUserColl,
		xl:             xl,
	}, nil
}

// CreateAccount 创建用户账号。
func (c *AccountController) CreateAccount(xl *xlog.Logger, account *protocol.Account) error {
	if xl == nil {
		xl = c.xl
	}
	account.RegisterTime = time.Now()
	_, err := c.accountColl.InsertOne(context.Background(), account)
	if err != nil {
		xl.Errorf("failed to insert user, error %v", err)
		return err
	}
	return nil
}

// GetAccountByFields 根据一组key/value关系查找用户账号。
func (c *AccountController) GetAccountByFields(xl *xlog.Logger, fields map[string]interface{}) (*protocol.Account, error) {
	if xl == nil {
		xl = c.xl
	}
	account := protocol.Account{}
	err := c.accountColl.Find(context.Background(), fields).One(&account)
	if err != nil {
		if qmgo.IsErrNoDocuments(err) {
			xl.Infof("no such user for fields %v", fields)
			return nil, fmt.Errorf("not found")
		}
		xl.Errorf("failed to get user, error %v", fields)
		return nil, err
	}
	return &account, nil
}

// GetAccountByID 使用ID查找账号。
func (c *AccountController) GetAccountByID(xl *xlog.Logger, id string) (*protocol.Account, error) {
	return c.GetAccountByFields(xl, map[string]interface{}{"_id": id})
}

// GetAccountByPhoneNumber 使用电话号码查找账号。
func (c *AccountController) GetAccountByPhoneNumber(xl *xlog.Logger, phoneNumber string) (*protocol.Account, error) {
	return c.GetAccountByFields(xl, map[string]interface{}{"phoneNumber": phoneNumber})
}

// UpdateAccount 更新用户信息。
func (c *AccountController) UpdateAccount(xl *xlog.Logger, id string, newAccount *protocol.Account) (*protocol.Account, error) {
	if xl == nil {
		xl = c.xl
	}
	account, err := c.GetAccountByID(xl, id)
	if err != nil {
		return nil, err
	}
	if newAccount.Nickname != "" {
		account.Nickname = newAccount.Nickname
	}
	if newAccount.Gender != "" {
		account.Gender = newAccount.Gender
	}
	err = c.accountColl.UpdateOne(context.Background(), bson.M{"_id": id}, bson.M{"$set": account})
	if err != nil {
		xl.Errorf("failed to update account %s,error %v", id, err)
		return nil, err
	}
	// 同步更新已登录用户信息。
	activeUser := &protocol.ActiveUser{}
	err = c.activeUserColl.Find(context.Background(), bson.M{"_id": id}).One(&activeUser)
	if err == nil {
		activeUser.Nickname = newAccount.Nickname
		updateErr := c.activeUserColl.UpdateOne(context.Background(), bson.M{"_id": id}, bson.M{"$set": activeUser})
		if updateErr != nil {
			xl.Errorf("failed to update active user info, error %v", err)
		}
	} else {
		if qmgo.IsErrNoDocuments(err) {
			xl.Errorf("cannot find active user info for user %s", id)
		} else {
			xl.Errorf("failed to find active user info, error %v", err)
		}
	}
	return account, nil
}

// GetActiveUserByFields 根据一组key/value关系查找活跃用户信息。
func (c *AccountController) GetActiveUserByFields(xl *xlog.Logger, fields map[string]interface{}) (*protocol.ActiveUser, error) {
	if xl == nil {
		xl = c.xl
	}
	activeUser := protocol.ActiveUser{}
	err := c.activeUserColl.Find(context.Background(), fields).One(&activeUser)
	if err != nil {
		if qmgo.IsErrNoDocuments(err) {
			xl.Infof("no such active user for fields %v", fields)
			return nil, fmt.Errorf("not found")
		}
		xl.Errorf("failed to get active user, error %v", fields)
		return nil, err
	}
	return &activeUser, nil
}

// GetAccountByID 使用ID查找活跃用户信息。
func (c *AccountController) GetActiveUserByID(xl *xlog.Logger, id string) (*protocol.ActiveUser, error) {
	return c.GetActiveUserByFields(xl, bson.M{"_id": id})
}

// UpdateActiveUser 更新活跃用户的状态信息
func (c *AccountController) UpdateActiveUser(xl *xlog.Logger, userID string, newActiveUser *protocol.ActiveUser) (*protocol.ActiveUser, error) {
	if xl == nil {
		xl = c.xl
	}
	activeUserRecord := &protocol.ActiveUser{}
	err := c.activeUserColl.Find(context.Background(), bson.M{"_id": userID}).
		One(activeUserRecord)
	if err != nil {
		c.xl.Errorf("UpdateActiveUser: failed to find account %s", userID)
		return nil, err
	}

	if newActiveUser.Status != "" {
		activeUserRecord.Status = newActiveUser.Status
	}
	if newActiveUser.Room != "" {
		activeUserRecord.Room = newActiveUser.Room
	}
	err = c.activeUserColl.UpdateOne(context.Background(), bson.M{"_id": userID}, bson.M{"$set": activeUserRecord})
	if err != nil {
		xl.Errorf("failed to update active user status record, error %v", err)
		return nil, err
	}
	return activeUserRecord, nil
}

// AccountLogin 设置某个账号为已登录状态。
func (c *AccountController) AccountLogin(xl *xlog.Logger, userID string) (token string, err error) {
	if xl == nil {
		xl = c.xl
	}
	account, err := c.GetAccountByID(xl, userID)
	if err != nil {
		c.xl.Errorf("AccountLogin: failed to find account %s", userID)
		return "", err
	}
	// 查看是否已经登录。
	activeUserRecord := &protocol.ActiveUser{}
	err = c.activeUserColl.Find(context.Background(), map[string]interface{}{"_id": userID}).
		One(activeUserRecord)
	if err != nil {
		if !qmgo.IsErrNoDocuments(err) {
			c.xl.Errorf("failed to check logged in users in mongo,error %v", err)
			return "", err
		}
	} else {
		c.xl.Infof("user %s has been already logged in, the old session will be invalid", userID)
	}
	activeUserRecord.ID = userID
	activeUserRecord.Nickname = account.Nickname
	activeUserRecord.Status = protocol.UserStatusIdle
	token = c.makeLoginToken(xl, account)
	activeUserRecord.Token = token
	// update or insert login record.
	_, err = c.activeUserColl.Upsert(context.Background(), bson.M{"_id": userID}, activeUserRecord)
	if err != nil {
		xl.Errorf("failed to update or insert user login record, error %v", err)
		return "", err
	}
	return token, nil
}

func (c *AccountController) makeLoginToken(xl *xlog.Logger, account *protocol.Account) string {
	if xl == nil {
		xl = c.xl
	}
	timestamp := time.Now().UnixNano()
	// TODO: add more secret things for token?
	claims := jwt.MapClaims{
		"userID":    account.ID,
		"timestamp": timestamp,
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, _ := t.SignedString([]byte(""))
	return token
}

// AccountLogout 用户退出登录。
func (c *AccountController) AccountLogout(xl *xlog.Logger, userID string) error {
	if xl == nil {
		xl = c.xl
	}
	// 删除用户登录记录。
	err := c.activeUserColl.RemoveId(context.Background(), userID)
	if err != nil {
		xl.Errorf("failed to remove user ID %s in logged in users, error %v", userID, err)
		return err
	}
	return nil
}
