package service

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"

	qiniuauth "github.com/qiniu/api.v7/v7/auth"
	qiniurtc "github.com/qiniu/api.v7/v7/rtc"
	"github.com/qiniu/x/xlog"
	"github.com/someonegg/msgpump"
	"go.uber.org/atomic"

	"github.com/qrtc/qlive/config"
	"github.com/qrtc/qlive/controller"
	"github.com/qrtc/qlive/errors"
	"github.com/qrtc/qlive/protocol"
	"github.com/qrtc/qlive/service/websocket"
)

type PMessage interface {
	Marshal() ([]byte, error)
}

type WSClient struct {
	s               *WSServer
	p               *msgpump.Pump
	st              time.Time
	xl              *xlog.Logger
	playerID        string
	online          *atomic.Bool
	authorizeChan   chan struct{}
	disconnectChan  chan struct{}
	remoteAddr      string
	remotePort      string
	lastMessageTime time.Time
}

// Start start websocket client. Implement for github.com/qrtc/qlive/service/websocket Client
func (c *WSClient) Start(p *msgpump.Pump) {
	c.p = p
	c.p.Start(c.s.QuitCtx())
	go c.monitor()
}

// Process listen all requests. Implement for github.com/qrtc/qlive/service/websocket Client
func (c *WSClient) Process(ctx context.Context, t string, m msgpump.Message) {
	go func(ctx context.Context, t string, m msgpump.Message) {
		c.parallelProcess(ctx, t, m)
	}(ctx, t, m)
}

// IsOnline get client online status.
func (c *WSClient) IsOnline() bool {
	return c.online.Load()
}

// Close stop client msgpump.
func (c *WSClient) Close() error {
	c.online.Store(false)
	if c.p != nil {
		c.p.Stop()
	}
	return nil
}

// StartTime get client start time.
func (c *WSClient) StartTime() time.Time {
	return c.st
}

// Notify write a notify to client.
func (c *WSClient) Notify(t string, v PMessage) {
	m, err := v.Marshal()
	if err != nil {
		return
	}

	if t != protocol.MT_Ping && t != protocol.MT_Pong {
		c.xl.Infof("message to %v, %v=%v", c.playerID, t, string(m))
	}

	if ok := c.p.TryOutput(t, m); !ok {
		c.xl.Errorf("TryOutput failed %v", c.playerID)
	}

}

func (c *WSClient) monitor() {
	c.xl.Infof("%v:%v connected.", c.remoteAddr, c.remotePort)

	select {
	case <-c.p.StopD():
		c.xl.Infof("%v:%v disconnected.", c.remoteAddr, c.remotePort)
	case <-time.After(time.Millisecond * time.Duration(c.s.conf.WsConf.AuthorizeTimeoutMS)):
		c.xl.Infof("%v:%v authentication failure", c.remoteAddr, c.remotePort)
		c.Close()
		c.xl.Infof("%v:%v disconnected.", c.remoteAddr, c.remotePort)
	case <-c.authorizeChan:
		c.xl.Infof("%v:%v authorized successful as %v", c.remoteAddr, c.remotePort, c.playerID)
		c.s.AddPlayer(c.playerID, c)
		c.online.Store(true)
		ping := &protocol.Ping{}
		c.Notify(protocol.MT_Ping, ping)
		for {
			select {
			case <-time.After(time.Second * time.Duration(c.s.conf.WsConf.PingTickerSecond)):
				c.Notify(protocol.MT_Ping, ping)
				if time.Now().Sub(c.lastMessageTime) > time.Second*time.Duration(c.s.conf.WsConf.PongTimeoutSecond) {
					c.xl.Infof("%v pingpong timeout", c.playerID)
					c.Close()
					c.s.RemovePlayer(c.playerID)
					c.xl.Infof("%v:%v %v disconnected.", c.remoteAddr, c.remotePort, c.playerID)
					return
				}
			case <-c.disconnectChan:
				c.Close()
				c.xl.Infof("%v:%v connection closed.", c.remoteAddr, c.remotePort)
				return
			}
		}
	}
}

func (c *WSClient) parallelProcess(ctx context.Context, t string, m msgpump.Message) {
	defer c.recover()

	if t != protocol.MT_Ping && t != protocol.MT_Pong {
		c.xl.Infof("message from %v, %v=%v", c.playerID, t, string(m))
	}

	if !c.IsOnline() && t != protocol.MT_AuthorizeRequest {
		return
	}

	c.lastMessageTime = time.Now()

	switch t {
	case protocol.MT_Ping:
		c.Notify(protocol.MT_Pong, &protocol.Pong{})
	case protocol.MT_Pong:
	case protocol.MT_AuthorizeRequest:
		c.onAuthorize(ctx, m)
	case protocol.MT_StartPKRequest:
		c.onStartPK(ctx, m)
	case protocol.MT_AnswerPKRequest:
		c.onAnswerPK(ctx, m)
	case protocol.MT_EndPKRequest:
		c.onEndPK(ctx, m)
	case protocol.MT_DisconnectNotify:
		c.onDisconnect(ctx, m)
	default:
		c.xl.Errorf("unknown message from %v, %v=%v", c.playerID, t, string(m))
	}
}

func (c *WSClient) recover() {
	if e := recover(); e != nil {
		const size = 16 << 10
		buf := make([]byte, size)
		buf = buf[:runtime.Stack(buf, false)]
		c.xl.Error("process panic: ", c.playerID, e, fmt.Sprintf("\n%s", buf))
	}
}

func (c *WSClient) onAuthorize(ctx context.Context, m msgpump.Message) {
	var req protocol.AuthorizeRequest
	err := req.Unmarshal(m)
	if err != nil {
		res := &protocol.AuthorizeResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorUnknownMessage,
			Error: errors.WSErrorToString[errors.WSErrorUnknownMessage],
		}
		c.Notify(protocol.MT_AuthorizeResponse, res)
		return
	}

	id, err := c.s.authCtl.GetIDByToken(c.xl, req.Token)
	if err != nil {
		res := &protocol.AuthorizeResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorTokenInvalid,
			Error: errors.WSErrorToString[errors.WSErrorTokenInvalid],
		}
		c.Notify(protocol.MT_AuthorizeResponse, res)
		return
	}

	c.playerID = id
	close(c.authorizeChan)

	res := &protocol.AuthorizeResponse{
		RPCID:       req.RPCID,
		Code:        errors.WSErrorOK,
		Error:       errors.WSErrorToString[errors.WSErrorOK],
		PongTimeout: c.s.conf.WsConf.PongTimeoutSecond,
	}
	c.Notify(protocol.MT_AuthorizeResponse, res)
	return
}

func (c *WSClient) onStartPK(ctx context.Context, m msgpump.Message) {
	var req protocol.StartPKRequest
	err := req.Unmarshal(m)
	if err != nil {
		res := &protocol.StartPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorUnknownMessage,
			Error: errors.WSErrorToString[errors.WSErrorUnknownMessage],
		}
		c.Notify(protocol.MT_StartResponse, res)
		return
	}
	// 判断房间是否满足 PK 条件
	pkRoom, err := c.s.roomCtl.GetRoomByID(c.xl, req.PKRoomID)
	if err != nil {
		res := &protocol.StartPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorRoomNoExist,
			Error: errors.WSErrorToString[errors.WSErrorRoomNoExist],
		}
		c.Notify(protocol.MT_StartResponse, res)
		return
	}
	if pkRoom.Status != protocol.LiveRoomStatusSingle {
		res := &protocol.StartPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorRoomInPK,
			Error: errors.WSErrorToString[errors.WSErrorRoomInPK],
		}
		c.Notify(protocol.MT_StartResponse, res)
		return
	}
	selfRoom, err := c.s.roomCtl.GetRoomByFields(c.xl, map[string]interface{}{"creator": c.playerID})
	if err != nil {
		res := &protocol.StartPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		c.Notify(protocol.MT_StartResponse, res)
		return
	}
	if selfRoom.Status != protocol.LiveRoomStatusSingle {
		res := &protocol.StartPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorRoomInPK,
			Error: errors.WSErrorToString[errors.WSErrorRoomInPK],
		}
		c.Notify(protocol.MT_StartResponse, res)
		return
	}

	pkPlayer, err := c.s.accountCtl.GetAccountByID(c.xl, pkRoom.Creator)
	if err != nil {
		res := &protocol.StartPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorPlayerNoExist,
			Error: errors.WSErrorToString[errors.WSErrorPlayerNoExist],
		}
		c.Notify(protocol.MT_StartResponse, res)
		return
	}

	selfPlayer, err := c.s.accountCtl.GetAccountByID(c.xl, c.playerID)
	if err != nil {
		res := &protocol.StartPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		c.Notify(protocol.MT_StartResponse, res)
		return
	}
	pkActiveUser, err := c.s.accountCtl.GetActiveUserByID(c.xl, pkPlayer.ID)
	if err != nil {
		res := &protocol.StartPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		c.Notify(protocol.MT_StartResponse, res)
		return
	}
	selfActiveUser, err := c.s.accountCtl.GetActiveUserByID(c.xl, selfPlayer.ID)
	if err != nil {
		res := &protocol.StartPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		c.Notify(protocol.MT_StartResponse, res)
		return
	}

	// 发送 PK Offer 通知
	pkMessage := &protocol.PKOfferNotify{
		RPCID:    NewReqID(),
		UserID:   selfPlayer.ID,
		Nickname: selfPlayer.Nickname,
		RoomID:   selfRoom.ID,
		RoomName: selfRoom.Name,
	}
	if err := c.s.NotifyPlayer(pkPlayer.ID, protocol.MT_PKOfferNotify, pkMessage); err != nil {
		res := &protocol.StartPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorPlayerOffline,
			Error: errors.WSErrorToString[errors.WSErrorPlayerOffline],
		}
		c.Notify(protocol.MT_StartResponse, res)
		return
	}

	// 修改状态
	selfRoom.Status = protocol.LiveRoomStatusWaitPK
	pkRoom.Status = protocol.LiveRoomStatusWaitPK

	_, err = c.s.roomCtl.UpdateRoom(c.xl, selfRoom.ID, selfRoom)
	if err != nil {
		res := &protocol.StartPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		c.Notify(protocol.MT_StartResponse, res)
		return
	}
	_, err = c.s.roomCtl.UpdateRoom(c.xl, pkRoom.ID, pkRoom)
	if err != nil {
		res := &protocol.StartPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		c.Notify(protocol.MT_StartResponse, res)
		return
	}

	selfActiveUser.Status = protocol.UserStatusPKWait
	pkActiveUser.Status = protocol.UserStatusPKWait

	_, err = c.s.accountCtl.UpdateActiveUser(c.xl, selfPlayer.ID, selfActiveUser)
	if err != nil {
		res := &protocol.StartPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		c.Notify(protocol.MT_StartResponse, res)
		return
	}
	_, err = c.s.accountCtl.UpdateActiveUser(c.xl, pkPlayer.ID, pkActiveUser)
	if err != nil {
		res := &protocol.StartPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		c.Notify(protocol.MT_StartResponse, res)
		return
	}

	// 成功返回
	res := &protocol.StartPKResponse{
		RPCID: req.RPCID,
		Code:  errors.WSErrorOK,
		Error: errors.WSErrorToString[errors.WSErrorOK],
	}
	c.Notify(protocol.MT_StartResponse, res)
	return
}

func (c *WSClient) onAnswerPK(ctx context.Context, m msgpump.Message) {
	var req protocol.AnswerPKRequest
	err := req.Unmarshal(m)
	if err != nil {
		res := &protocol.AnswerPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorUnknownMessage,
			Error: errors.WSErrorToString[errors.WSErrorUnknownMessage],
		}
		c.Notify(protocol.MT_AnswerPKResponse, res)
		return
	}

	pkRoom, err := c.s.roomCtl.GetRoomByID(c.xl, req.PKRoomID)
	if err != nil {
		res := &protocol.AnswerPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorRoomNoExist,
			Error: errors.WSErrorToString[errors.WSErrorRoomNoExist],
		}
		c.Notify(protocol.MT_AnswerPKResponse, res)
		return
	}
	selfRoom, err := c.s.roomCtl.GetRoomByFields(c.xl, map[string]interface{}{"creator": c.playerID})
	if err != nil {
		res := &protocol.AnswerPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		c.Notify(protocol.MT_AnswerPKResponse, res)
		return
	}
	pkPlayer, err := c.s.accountCtl.GetAccountByID(c.xl, pkRoom.Creator)
	if err != nil {
		res := &protocol.AnswerPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorPlayerNoExist,
			Error: errors.WSErrorToString[errors.WSErrorPlayerNoExist],
		}
		c.Notify(protocol.MT_AnswerPKResponse, res)
		return
	}
	selfPlayer, err := c.s.accountCtl.GetAccountByID(c.xl, c.playerID)
	if err != nil {
		res := &protocol.AnswerPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		c.Notify(protocol.MT_AnswerPKResponse, res)
		return
	}
	pkActiveUser, err := c.s.accountCtl.GetActiveUserByID(c.xl, pkPlayer.ID)
	if err != nil {
		res := &protocol.AnswerPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		c.Notify(protocol.MT_AnswerPKResponse, res)
		return
	}
	selfActiveUser, err := c.s.accountCtl.GetActiveUserByID(c.xl, selfPlayer.ID)
	if err != nil {
		res := &protocol.AnswerPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		c.Notify(protocol.MT_AnswerPKResponse, res)
		return
	}

	if req.Accept {
		if selfRoom.Status != protocol.LiveRoomStatusWaitPK {
			res := &protocol.AnswerPKResponse{
				RPCID: req.RPCID,
				Code:  errors.WSErrorRoomNotInPK,
				Error: errors.WSErrorToString[errors.WSErrorRoomNotInPK],
			}
			c.Notify(protocol.MT_AnswerPKResponse, res)
			return
		}
		if pkRoom.Status != protocol.LiveRoomStatusWaitPK {
			res := &protocol.AnswerPKResponse{
				RPCID: req.RPCID,
				Code:  errors.WSErrorRoomNotInPK,
				Error: errors.WSErrorToString[errors.WSErrorRoomNotInPK],
			}
			c.Notify(protocol.MT_AnswerPKResponse, res)
			return
		}
	}

	// 通知发起者
	answerMessage := &protocol.PKAnswerNotify{
		RPCID:    NewReqID(),
		PKRoomID: req.PKRoomID,
		Accepted: req.Accept,
	}
	if req.Accept {
		answerMessage.RTCRoom = selfRoom.ID
		answerMessage.RTCRoomToken = c.s.generateRTCRoomToken(selfRoom.ID, pkPlayer.ID, "user")
	}
	if err := c.s.NotifyPlayer(pkPlayer.ID, protocol.MT_PKAnswerNotify, answerMessage); err != nil {
		res := &protocol.AnswerPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorPlayerOffline,
			Error: errors.WSErrorToString[errors.WSErrorPlayerOffline],
		}
		c.Notify(protocol.MT_AnswerPKResponse, res)
		return
	}

	// 修改状态
	if req.Accept {
		selfRoom.Status = protocol.LiveRoomStatusPK
		selfRoom.PKAnchor = pkPlayer.ID
		pkRoom.Status = protocol.LiveRoomStatusPK
		pkRoom.PKAnchor = selfPlayer.ID
		selfActiveUser.Status = protocol.UserStatusPKLive
		pkActiveUser.Status = protocol.UserStatusPKLive
		pkActiveUser.Room = selfRoom.ID
	} else {
		selfRoom.Status = protocol.LiveRoomStatusSingle
		pkRoom.Status = protocol.LiveRoomStatusSingle
		selfActiveUser.Status = protocol.UserStatusSingleLive
		pkActiveUser.Status = protocol.UserStatusSingleLive
	}

	_, err = c.s.roomCtl.UpdateRoom(c.xl, selfRoom.ID, selfRoom)
	if err != nil {
		res := &protocol.AnswerPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		c.Notify(protocol.MT_AnswerPKResponse, res)
		return
	}
	_, err = c.s.roomCtl.UpdateRoom(c.xl, pkRoom.ID, pkRoom)
	if err != nil {
		res := &protocol.AnswerPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		c.Notify(protocol.MT_AnswerPKResponse, res)
		return
	}

	_, err = c.s.accountCtl.UpdateActiveUser(c.xl, selfPlayer.ID, selfActiveUser)
	if err != nil {
		res := &protocol.AnswerPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		c.Notify(protocol.MT_AnswerPKResponse, res)
		return
	}
	_, err = c.s.accountCtl.UpdateActiveUser(c.xl, pkPlayer.ID, pkActiveUser)
	if err != nil {
		res := &protocol.AnswerPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		c.Notify(protocol.MT_AnswerPKResponse, res)
		return
	}

	// 成功返回
	res := &protocol.AnswerPKResponse{
		PKRoomID: req.PKRoomID,
		RPCID:    req.RPCID,
		Code:     errors.WSErrorOK,
		Error:    errors.WSErrorToString[errors.WSErrorOK],
	}
	c.Notify(protocol.MT_AnswerPKResponse, res)
	return
}

func (c *WSClient) onEndPK(ctx context.Context, m msgpump.Message) {
	var req protocol.EndPKRequest
	err := req.Unmarshal(m)
	if err != nil {
		res := &protocol.EndPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorUnknownMessage,
			Error: errors.WSErrorToString[errors.WSErrorUnknownMessage],
		}
		c.Notify(protocol.MT_EndPKResponse, res)
		return
	}

	// 获取信息
	pkRoom, err := c.s.roomCtl.GetRoomByID(c.xl, req.PKRoomID)
	if err != nil {
		res := &protocol.EndPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorRoomNoExist,
			Error: errors.WSErrorToString[errors.WSErrorRoomNoExist],
		}
		c.Notify(protocol.MT_EndPKResponse, res)
		return
	}
	selfPlayer, err := c.s.accountCtl.GetAccountByID(c.xl, c.playerID)
	if err != nil {
		res := &protocol.EndPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		c.Notify(protocol.MT_EndPKResponse, res)
		return
	}
	selfActiveUser, err := c.s.accountCtl.GetActiveUserByID(c.xl, selfPlayer.ID)
	if err != nil {
		res := &protocol.EndPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		c.Notify(protocol.MT_EndPKResponse, res)
		return
	}

	var otherPlayerID string
	if selfPlayer.ID == pkRoom.Creator {
		otherPlayerID = pkRoom.PKAnchor
	} else {
		otherPlayerID = pkRoom.Creator
	}
	otherPlayer, err := c.s.accountCtl.GetAccountByID(c.xl, otherPlayerID)
	if err != nil {
		res := &protocol.EndPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		c.Notify(protocol.MT_EndPKResponse, res)
		return
	}
	otherActiveUser, err := c.s.accountCtl.GetActiveUserByID(c.xl, otherPlayerID)
	if err != nil {
		res := &protocol.EndPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		c.Notify(protocol.MT_EndPKResponse, res)
		return
	}

	otherRoom, err := c.s.roomCtl.GetRoomByFields(c.xl, map[string]interface{}{"creator": pkRoom.PKAnchor})
	if err != nil {
		res := &protocol.EndPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		c.Notify(protocol.MT_EndPKResponse, res)
		return
	}

	// 权限检查
	if req.PKRoomID != selfActiveUser.Room {
		res := &protocol.EndPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorNoPermission,
			Error: errors.WSErrorToString[errors.WSErrorNoPermission],
		}
		c.Notify(protocol.MT_EndPKResponse, res)
		return
	}

	// 状态检查
	if pkRoom.Status != protocol.LiveRoomStatusPK {
		res := &protocol.EndPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorRoomNotInPK,
			Error: errors.WSErrorToString[errors.WSErrorRoomNotInPK],
		}
		c.Notify(protocol.MT_EndPKResponse, res)
		return
	}

	// 通知 PK 另一方
	endMessage := &protocol.PKEndNotify{
		RPCID:    NewReqID(),
		PKRoomID: req.PKRoomID,
	}
	if err := c.s.NotifyPlayer(otherPlayer.ID, protocol.MT_PKEndNotify, endMessage); err != nil {
		res := &protocol.EndPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorPlayerOffline,
			Error: errors.WSErrorToString[errors.WSErrorPlayerOffline],
		}
		c.Notify(protocol.MT_EndPKResponse, res)
		return
	}

	// 修改状态
	pkRoom.PKAnchor = ""
	pkRoom.Status = protocol.LiveRoomStatusSingle
	otherRoom.Status = protocol.LiveRoomStatusSingle
	otherRoom.PKAnchor = ""
	selfActiveUser.Status = protocol.UserStatusSingleLive
	otherActiveUser.Status = protocol.UserStatusSingleLive
	if selfPlayer.ID == pkRoom.Creator {
		otherActiveUser.Room = otherRoom.ID
	} else {
		selfActiveUser.Room = otherRoom.ID
	}

	_, err = c.s.roomCtl.UpdateRoom(c.xl, pkRoom.ID, pkRoom)
	if err != nil {
		res := &protocol.EndPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		c.Notify(protocol.MT_EndPKResponse, res)
		return
	}
	_, err = c.s.roomCtl.UpdateRoom(c.xl, otherRoom.ID, otherRoom)
	if err != nil {
		res := &protocol.EndPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		c.Notify(protocol.MT_EndPKResponse, res)
		return
	}

	_, err = c.s.accountCtl.UpdateActiveUser(c.xl, selfPlayer.ID, selfActiveUser)
	if err != nil {
		res := &protocol.EndPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		c.Notify(protocol.MT_EndPKResponse, res)
		return
	}
	_, err = c.s.accountCtl.UpdateActiveUser(c.xl, otherPlayer.ID, otherActiveUser)
	if err != nil {
		res := &protocol.EndPKResponse{
			RPCID: req.RPCID,
			Code:  errors.WSErrorInvalidParameter,
			Error: errors.WSErrorToString[errors.WSErrorInvalidParameter],
		}
		c.Notify(protocol.MT_EndPKResponse, res)
		return
	}

	// 成功返回
	res := &protocol.EndPKResponse{
		RPCID: req.RPCID,
		Code:  errors.WSErrorOK,
		Error: errors.WSErrorToString[errors.WSErrorOK],
	}
	c.Notify(protocol.MT_EndPKResponse, res)
	return
}

func (c *WSClient) onDisconnect(ctx context.Context, m msgpump.Message) {
	var notify protocol.DisconnectNotify
	err := notify.Unmarshal(m)
	if err != nil {
		c.xl.Errorf("unknown disconnect message: %v", m)
		return
	}
	close(c.disconnectChan)
	c.s.RemovePlayer(c.playerID)
	c.xl.Infof("%v take the initiative to disconnect.", c.playerID)
}

// WebSocket Server
type WSServer struct {
	conf config.Config
	xl   *xlog.Logger

	cl    sync.RWMutex
	conns map[string]*WSClient

	accountCtl *controller.AccountController
	authCtl    *controller.AuthController
	roomCtl    *controller.RoomController

	*websocket.Service
}

// CreateClient Implement for github.com/qrtc/qlive/service/websocket ClientCreator
func (s *WSServer) CreateClient(r *http.Request, rAddr, rPort string) (websocket.Client, error) {

	return &WSClient{
		s:              s,
		st:             time.Now(),
		xl:             xlog.New(NewReqID()),
		online:         atomic.NewBool(false),
		authorizeChan:  make(chan struct{}),
		disconnectChan: make(chan struct{}),
		remoteAddr:     rAddr,
		remotePort:     rPort,
	}, nil
}

// AddPlayer add player on player list
func (s *WSServer) AddPlayer(id string, c *WSClient) error {
	s.cl.Lock()
	defer s.cl.Unlock()

	if client, ok := s.conns[id]; ok && client != nil {
		s.xl.Infof("%v reconnect", id)
		close(client.disconnectChan)
	}

	s.conns[id] = c
	return nil
}

// RemovePlayer remove player from player list
func (s *WSServer) RemovePlayer(id string) error {
	s.cl.Lock()
	defer s.cl.Unlock()

	if _, ok := s.conns[id]; !ok {
		return errors.NewWSError("player not online")
	}

	// if on pk, send other anchor end pk notify
	selfActiveUser, err := s.accountCtl.GetActiveUserByID(s.xl, id)
	if err == nil {
		if selfActiveUser.Status == protocol.UserStatusPKLive {
			pkRoom, err := s.roomCtl.GetRoomByID(s.xl, selfActiveUser.Room)
			if err == nil {
				endMessage := &protocol.PKEndNotify{
					RPCID:    NewReqID(),
					PKRoomID: pkRoom.ID,
				}
				if id == pkRoom.PKAnchor {
					s.notifyPlayer(pkRoom.Creator, protocol.MT_PKEndNotify, endMessage)
					otherActiveUser, err := s.accountCtl.GetActiveUserByID(s.xl, pkRoom.Creator)
					if err == nil {
						otherActiveUser.Status = protocol.UserStatusSingleLive
						s.accountCtl.UpdateActiveUser(s.xl, pkRoom.Creator, otherActiveUser)

						pkRoom.PKAnchor = ""
						pkRoom.Status = protocol.LiveRoomStatusSingle
						s.roomCtl.UpdateRoom(s.xl, pkRoom.ID, pkRoom)
					}

				} else if id == pkRoom.Creator {
					s.notifyPlayer(pkRoom.PKAnchor, protocol.MT_PKEndNotify, endMessage)
					otherActiveUser, err := s.accountCtl.GetActiveUserByID(s.xl, pkRoom.PKAnchor)
					if err == nil {
						otherRoom, err := s.roomCtl.GetRoomByFields(s.xl, map[string]interface{}{"creator": pkRoom.PKAnchor})
						if err == nil {
							otherActiveUser.Status = protocol.UserStatusSingleLive
							otherActiveUser.Room = otherRoom.ID
							s.accountCtl.UpdateActiveUser(s.xl, pkRoom.Creator, otherActiveUser)

							otherRoom.PKAnchor = ""
							otherRoom.Status = protocol.LiveRoomStatusSingle
							s.roomCtl.UpdateRoom(s.xl, otherRoom.ID, otherRoom)
						}
					}
				}
			}
		}
		selfActiveUser.Status = protocol.UserStatusIdle
		s.accountCtl.UpdateActiveUser(s.xl, id, selfActiveUser)
	}

	// close room create by player
	room, err := s.roomCtl.GetRoomByFields(s.xl, map[string]interface{}{"creator": id})
	if err != nil {
		s.xl.Errorf("can not find room create by %v", id)
	}
	if room != nil {
		err := s.roomCtl.CloseRoom(s.xl, id, room.ID)
		if err != nil {
			s.xl.Errorf("close room %v create by %v faild", room.ID, id)
		} else {
			s.xl.Infof("room %v create by %v has been closed", room.ID, id)
		}
	}

	delete(s.conns, id)
	return nil
}

// NotifyPlayer send player notify message
func (s *WSServer) NotifyPlayer(id string, t string, v PMessage) error {
	s.cl.RLock()
	defer s.cl.RUnlock()

	return s.notifyPlayer(id, t, v)
}

func (s *WSServer) notifyPlayer(id string, t string, v PMessage) error {
	player, ok := s.conns[id]
	if !ok || !player.IsOnline() {
		return errors.NewWSError("player not online")
	}
	player.Notify(t, v)

	return nil
}

// FindPlayer find player by ID
func (s *WSServer) FindPlayer(id string) (c *WSClient, err error) {
	s.cl.RLock()
	defer s.cl.RUnlock()

	player, ok := s.conns[id]
	if !ok || !player.IsOnline() {
		return nil, errors.NewWSError("player not online")
	}
	return player, nil
}

func (s *WSServer) generateRTCRoomToken(roomID string, userID string, permission string) string {
	rtcClient := qiniurtc.NewManager(&qiniuauth.Credentials{
		AccessKey: s.conf.RTC.KeyPair.AccessKey,
		SecretKey: []byte(s.conf.RTC.KeyPair.SecretKey),
	})
	rtcRoomTokenTimeout := time.Duration(s.conf.RTC.RoomTokenExpireSecond) * time.Second
	roomAccess := qiniurtc.RoomAccess{
		AppID:    s.conf.RTC.AppID,
		RoomName: roomID,
		UserID:   userID,
		ExpireAt: time.Now().Add(rtcRoomTokenTimeout).Unix(),
		// Permission分admin/user，直播间创建者需要admin权限。
		Permission: permission,
	}
	token, _ := rtcClient.GetRoomToken(roomAccess)
	return token
}

// NewWSServer return a new websocket server
func NewWSServer(conf *config.Config) (s *WSServer, err error) {
	s = &WSServer{
		conf:  *conf,
		xl:    xlog.New(NewReqID()),
		conns: make(map[string]*WSClient),
	}

	s.accountCtl, err = controller.NewAccountController(conf.Mongo.URI, conf.Mongo.Database, nil)
	if err != nil {
		return nil, err
	}
	s.authCtl, err = controller.NewAuthController(conf.Mongo.URI, conf.Mongo.Database, nil)
	if err != nil {
		return nil, err
	}
	s.roomCtl, err = controller.NewRoomController(conf.Mongo.URI, conf.Mongo.Database, nil)
	if err != nil {
		return nil, err
	}

	s.Service = websocket.NewService(&websocket.Config{
		ListenAddr: conf.WsConf.ListenAddr,
		ServeURI:   conf.WsConf.ServeURI,
	}, s)

	return s, err
}
