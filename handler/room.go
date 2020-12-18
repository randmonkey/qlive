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
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	qiniuauth "github.com/qiniu/api.v7/v7/auth"
	qiniurtc "github.com/qiniu/api.v7/v7/rtc"
	"github.com/qiniu/x/xlog"

	"github.com/qrtc/qlive/config"
	"github.com/qrtc/qlive/errors"
	"github.com/qrtc/qlive/protocol"
)

// RoomHandler 处理直播间的CRUD，以及进入、退出房间等操作。
type RoomHandler struct {
	Account   AccountInterface
	Room      RoomInterface
	RTCConfig *config.QiniuRTCConfig
	// WSProtocol websocket 协议，ws 或 wss
	WSProtocol string
	// WSAddress 对外返回的websocket 服务地址。
	WSAddress string
	// WSPort websocket 监听端口。
	WSPort int
	// WSPath websocket 服务根路径地址。
	WSPath string
}

// RoomInterface 处理房间相关API的接口。
type RoomInterface interface {
	// 创建房间。当房间的创建者再次调用时，返回房间的当前状态。
	CreateRoom(xl *xlog.Logger, room *protocol.LiveRoom) (*protocol.LiveRoom, error)
	// ListAllRooms 列出全部正在直播的房间列表。
	ListAllRooms(xl *xlog.Logger) ([]protocol.LiveRoom, error)
	// ListRoomsByFields 根据特定条件列出房间。
	ListRoomsByFields(xl *xlog.Logger, fields map[string]interface{}) ([]protocol.LiveRoom, error)
	// CloseRoom 关闭直播间。
	CloseRoom(xl *xlog.Logger, userID string, roomID string) error
	// EnterRoom 进入直播间。
	EnterRoom(xl *xlog.Logger, userID string, roomID string) (*protocol.LiveRoom, error)
	// LeaveRoom 退出直播间。
	LeaveRoom(xl *xlog.Logger, userID string, roomID string) error
	// GetRoomByID 根据ID查找房间。
	GetRoomByID(xl *xlog.Logger, roomID string) (*protocol.LiveRoom, error)
	// UpdateRoom 更新房间信息。
	UpdateRoom(xl *xlog.Logger, id string, room *protocol.LiveRoom) (*protocol.LiveRoom, error)
	// GetAudienceNumber 获取房间内观众人数。
	GetAudienceNumber(xl *xlog.Logger, roomID string) (int, error)
	// GetAllAudiences 获取房间内全部观众信息。
	GetAllAudiences(xl *xlog.Logger, roomID string) ([]*protocol.ActiveUser, error)
}

// @Tags qlive api
// @ID list-rooms
// @Summary List rooms
// @Description if can_pk set in url,server will returns rooms that can pk.Otherwise,all rooms will be returned
// @Accept  json
// @Produce  json
// @Param can_pk query string false "only list rooms that can pk"
// @Success 200 {object} protocol.ListRoomsResponse
// @Failure 500 {object} errors.HTTPError
// @Router /rooms [get]
// ListRooms 列出房间请求。
func (h *RoomHandler) ListRooms(c *gin.Context) {
	if c.Query("can_pk") == "true" {
		h.ListCanPKRooms(c)
		return
	}
	if c.Query("creator") != "" {
		h.ListRoomsByCreator(c)
		return
	}

	h.ListAllRooms(c)
}

func (h *RoomHandler) makeGetRoomResponse(xl *xlog.Logger, room *protocol.LiveRoom) (*protocol.GetRoomResponse, error) {
	if room == nil {
		return nil, fmt.Errorf("nil room")
	}
	creatorInfo, err := h.Account.GetAccountByID(xl, room.Creator)
	if err != nil {
		xl.Errorf("failed to get account info for user %s, creator of room %s", room.Creator, room.ID)
		// TODO：创建者用户信息获取失败，是否要算这个房间? 现在是添加一个模拟的用户信息
		creatorInfo = &protocol.Account{ID: room.Creator, Nickname: "user-" + room.Creator}
	}
	audienceNumber, err := h.Room.GetAudienceNumber(xl, room.ID)
	if err != nil {
		xl.Errorf("failed to get audience number of room %s, error %v", room.ID, err)
	}
	roomType := room.Type
	if string(roomType) == "" {
		// 兼容之前创建的没有type字段的房间。
		roomType = protocol.RoomTypePK
	}
	getRoomResp := protocol.GetRoomResponse{
		ID:   room.ID,
		Name: room.Name,
		Type: string(roomType),
		Creator: protocol.UserInfo{
			ID:        room.Creator,
			Nickname:  creatorInfo.Nickname,
			Gender:    creatorInfo.Gender,
			AvatarURL: creatorInfo.AvatarURL,
		},
		PlayURL:        room.PlayURL,
		AudienceNumber: audienceNumber,
		Status:         string(room.Status),
	}
	if room.Status == protocol.LiveRoomStatusPK {
		pkAnchorInfo, err := h.Account.GetAccountByID(xl, room.PKAnchor)
		if err == nil {
			getRoomResp.PKAnchor = &protocol.UserInfo{
				ID:        room.PKAnchor,
				Nickname:  pkAnchorInfo.Nickname,
				Gender:    pkAnchorInfo.Gender,
				AvatarURL: pkAnchorInfo.AvatarURL,
			}
		} else {
			getRoomResp.PKAnchor = &protocol.UserInfo{
				ID: room.PKAnchor,
			}
		}
	}

	return &getRoomResp, nil
}

func (h *RoomHandler) getJoinedAudiences(xl *xlog.Logger, room *protocol.LiveRoom) ([]protocol.JoinedAudience, error) {
	ret := []protocol.JoinedAudience{}
	if room.Type == protocol.RoomTypeVoice {
		audiences, err := h.Room.GetAllAudiences(xl, room.ID)
		if err != nil {
			xl.Errorf("failed to get all audiences of room %s, error %v", room.ID, err)
			return nil, err
		}
		for _, audience := range audiences {
			if audience.Status == protocol.UserStatusJoined && audience.JoinPosition != nil {
				audienceAccount, err := h.Account.GetAccountByID(xl, audience.ID)
				if err != nil {
					xl.Errorf("failed to get account info of user %s, error %v", audience.ID, err)
					continue
				}
				joinPosition := *audience.JoinPosition
				ret = append(ret,
					protocol.JoinedAudience{
						Position:  joinPosition,
						ID:        audience.ID,
						Nickname:  audienceAccount.Nickname,
						Gender:    audienceAccount.Gender,
						AvatarURL: audienceAccount.AvatarURL,
					})
			}
		}
	}
	return ret, nil
}

// ListCanPKRooms 列出当前主播可以PK的房间列表。
func (h *RoomHandler) ListCanPKRooms(c *gin.Context) {
	xl := c.MustGet(protocol.XLogKey).(*xlog.Logger)
	requestID := xl.ReqId
	userID := c.GetString(protocol.UserIDContextKey)
	rooms, err := h.Room.ListRoomsByFields(xl, map[string]interface{}{
		"status":  protocol.LiveRoomStatusSingle,
		"creator": map[string]interface{}{"$ne": userID},
	})
	if err != nil {
		xl.Errorf("failed to list rooms which can be PKed, error %v", err)
		httpErr := errors.NewHTTPErrorInternal().WithRequestID(requestID)
		c.JSON(http.StatusInternalServerError, httpErr)
		return
	}
	resp := &protocol.ListRoomsResponse{}
	for _, room := range rooms {
		// 获取房间类型。
		roomType := room.Type
		if string(roomType) == "" {
			// 兼容之前创建的没有type字段的房间。
			roomType = protocol.RoomTypePK
		}
		if roomType != protocol.RoomTypePK {
			continue
		}
		getRoomResp, err := h.makeGetRoomResponse(xl, &room)
		if err != nil {
			xl.Errorf("failed to make get room response for room %s", room.ID)
			continue
		}
		resp.Rooms = append(resp.Rooms, *getRoomResp)
	}
	c.JSON(http.StatusOK, resp)
}

// ListRoomsByCreator 根据创建者的ID列出房间。
func (h *RoomHandler) ListRoomsByCreator(c *gin.Context) {
	xl := c.MustGet(protocol.XLogKey).(*xlog.Logger)
	requestID := xl.ReqId

	creatorID := c.Query("creator")

	rooms, err := h.Room.ListRoomsByFields(xl, map[string]interface{}{
		"creator": creatorID,
	})
	if err != nil {
		xl.Errorf("failed to list rooms created by %s, error %v", creatorID, err)
		httpErr := errors.NewHTTPErrorInternal().WithRequestID(requestID)
		c.JSON(http.StatusInternalServerError, httpErr)
		return
	}
	resp := &protocol.ListRoomsResponse{}
	for _, room := range rooms {
		// 获取房间类型。
		getRoomResp, err := h.makeGetRoomResponse(xl, &room)
		if err != nil {
			xl.Errorf("failed to make get room response for room %s", room.ID)
			continue
		}
		resp.Rooms = append(resp.Rooms, *getRoomResp)
	}
	c.JSON(http.StatusOK, resp)

}

// ListAllRooms 列出全部房间。
func (h *RoomHandler) ListAllRooms(c *gin.Context) {
	xl := c.MustGet(protocol.XLogKey).(*xlog.Logger)
	rooms, err := h.Room.ListAllRooms(xl)
	if err != nil {
		xl.Errorf("failed to list all rooms, error %v", err)
		httpErr := errors.NewHTTPErrorInternal()
		c.JSON(http.StatusInternalServerError, httpErr)
		return
	}
	resp := &protocol.ListRoomsResponse{}
	userID := c.GetString(protocol.UserIDContextKey)
	for _, room := range rooms {
		if userID == room.Creator {
			// 隐藏自己创建的房间
			continue
		}
		getRoomResp, err := h.makeGetRoomResponse(xl, &room)
		if err != nil {
			xl.Errorf("failed to make get room response for room %s", room.ID)
			continue
		}
		resp.Rooms = append(resp.Rooms, *getRoomResp)
	}
	c.JSON(http.StatusOK, resp)
}

// @Tags qlive api
// @ID create-room
// @Summary Anchor creates room
// @Description Anchor creates room
// @Accept  json
// @Produce  json
// @Param CreateRoomArgs body protocol.CreateRoomArgs true "Anchor requests to create room"
// @Success 200 {string} protocol.CreateRoomResponse
// @Failure 400 {object} errors.HTTPError
// @Failure 403 {object} errors.HTTPError
// @Failure 409 {object} errors.HTTPError
// @Failure 500 {object} errors.HTTPError
// @Failure 503 {object} errors.HTTPError
// @Router /rooms [post]
// CreateRoom 创建直播间。
func (h *RoomHandler) CreateRoom(c *gin.Context) {
	xl := c.MustGet(protocol.XLogKey).(*xlog.Logger)
	requestID := xl.ReqId
	userID := c.GetString(protocol.UserIDContextKey)

	args := &protocol.CreateRoomArgs{}
	err := c.BindJSON(args)
	if err != nil {
		xl.Infof("invalid args in body, error %v", err)
		httpErr := errors.NewHTTPErrorBadRequest().WithRequestID(requestID).WithMessage("invalid args in request body")
		c.JSON(http.StatusBadRequest, httpErr)
		return
	}
	if !h.validateRoomName(args.RoomName) {
		xl.Infof("invalid room name %s", args.RoomName)
		httpErr := errors.NewHTTPErrorInvalidRoomName().WithRequestID(requestID).WithMessagef("invalid room name %s", args.RoomName)
		c.JSON(http.StatusBadRequest, httpErr)
		return
	}
	roomType, err := h.convertRoomType(args.RoomType)
	if err != nil {
		xl.Infof("invalid room type %s", args.RoomType)
		httpErr := errors.NewHTTPErrorBadRoomType().WithRequestID(requestID).WithMessagef("unsupported room type: %s", args.RoomType)
		c.JSON(http.StatusBadRequest, httpErr)
		return
	}

	roomID := h.generateRoomID()
	room := &protocol.LiveRoom{
		ID:         roomID,
		Name:       args.RoomName,
		Type:       roomType,
		Creator:    userID,
		PublishURL: h.generatePublishURL(roomID),
		PlayURL:    h.generatePlayURL(roomID),
		RTCRoom:    roomID,
	}
	switch roomType {
	case protocol.RoomTypePK:
		room.Status = protocol.LiveRoomStatusSingle
	case protocol.RoomTypeVoice:
		room.Status = protocol.LiveRoomStatusVoiceLive
		room.MaxJoinAudiences = protocol.DefaultMaxJoinAudiences
	default:
		room.Status = protocol.LiveRoomStatusSingle
	}
	// 若房间之前不存在，返回创建的房间。若房间已存在，返回已经存在的房间。
	roomRes, err := h.Room.CreateRoom(xl, room)
	if err != nil {
		serverErr, ok := err.(*errors.ServerError)
		if ok {
			switch serverErr.Code {
			case errors.ServerErrorRoomNameUsed:
				httpErr := errors.NewHTTPErrorRoomNameused().WithRequestID(requestID)
				c.JSON(http.StatusConflict, httpErr)
				return
			case errors.ServerErrorTooManyRooms:
				httpErr := errors.NewHTTPErrorTooManyRooms().WithRequestID(requestID)
				c.JSON(http.StatusServiceUnavailable, httpErr)
				return
			case errors.ServerErrorCanOnlyCreateOneRoom:
				httpErr := errors.NewHTTPErrorCanOnlyCreateOneRoom().WithRequestID(requestID)
				c.JSON(http.StatusForbidden, httpErr)
				return
			case errors.ServerErrorUserWatching:
				httpErr := errors.NewHTTPErrorUserWatching().WithRequestID(requestID)
				c.JSON(http.StatusConflict, httpErr)
				return
			case errors.ServerErrorUserJoined:
				httpErr := errors.NewHTTPErrorUserJoined().WithRequestID(requestID)
				c.JSON(http.StatusConflict, httpErr)
				return
			}
		}
		httpErr := errors.NewHTTPErrorInternal().WithRequestID(requestID)
		c.JSON(http.StatusInternalServerError, httpErr)
		return
	}

	xl.Infof("user %s created or refreshed room: ID %s, name %s", userID, roomRes.ID, args.RoomName)
	resp := &protocol.CreateRoomResponse{
		RoomID:       roomRes.ID,
		RoomName:     args.RoomName,
		RTCRoom:      roomRes.ID,
		RTCRoomToken: h.generateRTCRoomToken(roomID, userID, "admin"),
		WSURL:        h.generateWSURL(xl, c.Request.Host),
	}
	c.JSON(http.StatusOK, resp)
}

// validateRoomName 校验直播间名称。
func (h *RoomHandler) validateRoomName(roomName string) bool {
	roomNameMaxLength := 100
	if len(roomName) == 0 || len(roomName) > roomNameMaxLength {
		return false
	}
	return true
}

// convertRoomType 校验并转换房间类型。
func (h *RoomHandler) convertRoomType(roomType string) (protocol.RoomType, error) {
	switch roomType {
	case string(protocol.RoomTypePK):
		return protocol.RoomTypePK, nil
	case string(protocol.RoomTypeVoice):
		return protocol.RoomTypeVoice, nil
	}
	return protocol.RoomType(""), fmt.Errorf("unsupported room type %s", roomType)
}

// generateRoomID 生成直播间ID。
func (h *RoomHandler) generateRoomID() string {
	alphaNum := "0123456789abcdefghijklmnopqrstuvwxyz"
	roomID := ""
	idLength := 16
	for i := 0; i < idLength; i++ {
		index := rand.Intn(len(alphaNum))
		roomID = roomID + string(alphaNum[index])
	}
	return roomID
}

func (h *RoomHandler) generateWSURL(xl *xlog.Logger, host string) string {
	// 优先使用配置的websocket 地址。
	if h.WSAddress != "" {
		xl.Debugf("websocket address has been specified to %s", h.WSAddress)
		if strings.HasPrefix(h.WSAddress, "ws://") || strings.HasPrefix(h.WSAddress, "wss://") {
			return h.WSAddress
		}
		return h.WSProtocol + "://" + h.WSAddress
	}
	hostPart, _, err := net.SplitHostPort(host)
	if err != nil {
		xl.Debugf("cannot split into host and port, error %v, directly use host", err)
	} else {
		host = hostPart
	}
	var portPart string
	if (h.WSPort == 80 && h.WSProtocol == "ws") || (h.WSPort == 443 && h.WSProtocol == "wss") {
	} else {
		portPart = ":" + strconv.Itoa(h.WSPort)
	}
	return h.WSProtocol + "://" + host + portPart + h.WSPath
}

func (h *RoomHandler) generatePlayURL(roomID string) string {
	return "rtmp://" + h.RTCConfig.PlayHost + "/" + h.RTCConfig.PublishHub + "/" + roomID
}

func (h *RoomHandler) generatePublishURL(roomID string) string {
	return "rtmp://" + h.RTCConfig.PublishHost + "/" + h.RTCConfig.PublishHub + "/" + roomID
}

const (
	// DefaultRTCRoomTokenTimeout 默认的RTC加入房间用token的过期时间。
	DefaultRTCRoomTokenTimeout = 60 * time.Second
)

// 生成加入RTC房间的room token。
func (h *RoomHandler) generateRTCRoomToken(roomID string, userID string, permission string) string {
	rtcClient := qiniurtc.NewManager(&qiniuauth.Credentials{
		AccessKey: h.RTCConfig.KeyPair.AccessKey,
		SecretKey: []byte(h.RTCConfig.KeyPair.SecretKey),
	})
	rtcRoomTokenTimeout := DefaultRTCRoomTokenTimeout
	if h.RTCConfig.RoomTokenExpireSecond > 0 {
		rtcRoomTokenTimeout = time.Duration(h.RTCConfig.RoomTokenExpireSecond) * time.Second
	}
	roomAccess := qiniurtc.RoomAccess{
		AppID:    h.RTCConfig.AppID,
		RoomName: roomID,
		UserID:   userID,
		ExpireAt: time.Now().Add(rtcRoomTokenTimeout).Unix(),
		// Permission分admin/user，直播间创建者需要admin权限。
		Permission: permission,
	}
	token, _ := rtcClient.GetRoomToken(roomAccess)
	return token
}

// @Tags qlive api
// @ID get-room-by-id
// @Summary Get room by id
// @Description Get room by id
// @Accept  json
// @Produce  json
// @Param roomID path string true " Get room by id"
// @Success 200 {object} protocol.GetRoomResponse
// @Failure 404 {object} errors.HTTPError
// @Failure 500 {object} errors.HTTPError
// @Router /rooms/{roomID} [get]
// GetRoom 根据房间ID获取房间信息。
func (h *RoomHandler) GetRoom(c *gin.Context) {
	xl := c.MustGet(protocol.XLogKey).(*xlog.Logger)
	requestID := xl.ReqId
	userID := c.GetString(protocol.UserIDContextKey)
	roomID := c.Param("roomID")

	room, err := h.Room.GetRoomByID(xl, roomID)
	if err != nil {
		serverErr, ok := err.(*errors.ServerError)
		if ok {
			switch serverErr.Code {
			case errors.ServerErrorRoomNotFound:
				httpErr := errors.NewHTTPErrorNoSuchRoom().WithRequestID(requestID)
				c.JSON(http.StatusNotFound, httpErr)
				return
			}
		}
		httpErr := errors.NewHTTPErrorInternal().WithRequestID(requestID)
		c.JSON(http.StatusInternalServerError, httpErr)
		return
	}
	resp, err := h.makeGetRoomResponse(xl, room)
	if err != nil {
		httpErr := errors.NewHTTPErrorInternal().WithRequestID(requestID).WithMessagef("failed to get room info")
		xl.Errorf("failed to get make get room response, error %v", err)
		c.JSON(http.StatusInternalServerError, httpErr)
		return
	}
	if room.Type == protocol.RoomTypeVoice {
		joinedAudiences, err := h.getJoinedAudiences(xl, room)
		if err != nil {
			xl.Errorf("failed to get joined audiences of room %s,error %v", room.ID, err)
		} else {
			resp.JoinedAudiences = joinedAudiences
		}
	}
	xl.Debugf("user %s get info of room %s", userID, roomID)
	c.JSON(http.StatusOK, resp)
}

// @Tags qlive api
// @ID update-room
// @Summary Anchor updates room
// @Description Anchor updates room
// @Accept  json
// @Produce  json
// @Param roomID path string true "room id"
// @Param UpdateRoomArgs body protocol.UpdateRoomArgs true "Anchor requests to update room"
// @Success 200 {string} protocol.UpdateRoomResponse
// @Failure 400 {object} errors.HTTPError
// @Failure 404 {object} errors.HTTPError
// @Failure 409 {object} errors.HTTPError
// @Failure 500 {object} errors.HTTPError
// @Router /rooms/{roomID} [put]
// UpdateRoom 更新直播间信息。
func (h *RoomHandler) UpdateRoom(c *gin.Context) {
	xl := c.MustGet(protocol.XLogKey).(*xlog.Logger)
	requestID := xl.ReqId
	userID := c.GetString(protocol.UserIDContextKey)
	roomID := c.Param("roomID")
	args := protocol.UpdateRoomArgs{}
	err := c.BindJSON(&args)
	if err != nil {
		xl.Infof("invalid args in body, error %v", err)
		httpErr := errors.NewHTTPErrorBadRequest().WithRequestID(requestID).WithMessage("invalid args in request body")
		c.JSON(http.StatusBadRequest, httpErr)
		return
	}
	if !h.validateRoomName(args.RoomName) {
		xl.Infof("invalid room name %s", args.RoomName)
		httpErr := errors.NewHTTPErrorInvalidRoomName().WithRequestID(requestID).WithMessagef("invalid room name %s", args.RoomName)
		c.JSON(http.StatusBadRequest, httpErr)
		return
	}

	room, err := h.Room.GetRoomByID(xl, roomID)
	if err != nil {
		serverErr, ok := err.(*errors.ServerError)
		if !ok {
			xl.Errorf("failed to get current room %s, error %v", roomID, err)
			httpErr := errors.NewHTTPErrorInternal().WithRequestID(requestID)
			c.JSON(http.StatusInternalServerError, httpErr)
			return
		}
		switch serverErr.Code {
		case errors.ServerErrorRoomNotFound:
			xl.Infof("room %s not found", roomID)
			httpErr := errors.NewHTTPErrorNoSuchRoom().WithRequestID(requestID)
			c.JSON(http.StatusNotFound, httpErr)
			return
		default:
			xl.Errorf("failed to get current room %s, error %v", roomID, err)
			httpErr := errors.NewHTTPErrorInternal().WithRequestID(requestID)
			c.JSON(http.StatusInternalServerError, httpErr)
			return
		}
	}

	if room.Creator != userID {
		xl.Infof("user %s try to update room %s, no permission", userID, roomID)
		httpErr := errors.NewHTTPErrorNoSuchRoom().WithRequestID(requestID)
		c.JSON(http.StatusNotFound, httpErr)
		return
	}

	needUpdate := false
	if args.RoomName != room.Name {
		room.Name = args.RoomName
		needUpdate = true
	}
	if needUpdate {
		room, err = h.Room.UpdateRoom(xl, room.ID, room)
		if err != nil {
			serverErr, ok := err.(*errors.ServerError)
			if ok {
				switch serverErr.Code {
				case errors.ServerErrorRoomNameUsed:
					xl.Infof("room name %s used", args.RoomName)
					httpErr := errors.NewHTTPErrorRoomNameused().WithRequestID(requestID)
					c.JSON(http.StatusConflict, httpErr)
					return
				}
			}
			xl.Errorf("failed to update room, error %v", err)
			httpErr := errors.NewHTTPErrorInternal().WithRequestID(requestID)
			c.JSON(http.StatusInternalServerError, httpErr)
			return
		}
	}
	resp := &protocol.UpdateRoomResponse{
		RoomID:   room.ID,
		RoomName: room.Name,
	}
	xl.Infof("room %s updated by user %s", roomID, userID)
	c.JSON(http.StatusOK, resp)
}

// @Tags qlive api
// @ID close-room
// @Summary Anchor closes room
// @Description Anchor closes room
// @Accept  json
// @Produce  json
// @Param CloseRoomArgs body protocol.CloseRoomArgs true "Anchor requests to close room"
// @Success 200 {string} ok
// @Failure 400 {object} errors.HTTPError
// @Failure 500 {object} errors.HTTPError
// @Router /rooms [post]
// CloseRoom 关闭直播间。
func (h *RoomHandler) CloseRoom(c *gin.Context) {
	xl := c.MustGet(protocol.XLogKey).(*xlog.Logger)
	requestID := xl.ReqId
	userID := c.GetString(protocol.UserIDContextKey)

	args := &protocol.CloseRoomArgs{}
	err := c.BindJSON(args)
	if err != nil {
		xl.Infof("invalid args in body, error %v", err)
		httpErr := errors.NewHTTPErrorBadRequest().WithRequestID(requestID).WithMessage("invalid args in request body")
		c.JSON(http.StatusBadRequest, httpErr)
		return
	}

	err = h.Room.CloseRoom(xl, userID, args.RoomID)
	if err != nil {
		serverErr, ok := err.(*errors.ServerError)
		if !ok {
			xl.Errorf("close room error %v", err)
			httpErr := errors.NewHTTPErrorInternal().WithRequestID(requestID)
			c.JSON(http.StatusInternalServerError, httpErr)
			return
		}
		switch serverErr.Code {
		case errors.ServerErrorRoomNotFound:
			httpErr := errors.NewHTTPErrorNoSuchRoom().WithRequestID(requestID)
			c.JSON(http.StatusNotFound, httpErr)
			return
		default:
			httpErr := errors.NewHTTPErrorInternal().WithRequestID(requestID)
			c.JSON(http.StatusInternalServerError, httpErr)
			return
		}
	}
	xl.Infof("user %s closed room: ID %s", userID, args.RoomID)
	c.JSON(http.StatusOK, "")
	// return OK
}

// @Tags qlive api
// @ID refresh-room
// @Summary Anchor refreshes room
// @Description Anchor refreshes room
// @Accept  json
// @Produce  json
// @Param RefreshRoomArgs body protocol.RefreshRoomArgs true "Anchor requests to refresh room"
// @Success 200 {object} protocol.RefreshRoomResponse
// @Failure 400 {object} errors.HTTPError
// @Failure 404 {object} errors.HTTPError
// @Failure 500 {object} errors.HTTPError
// @Router /refresh_room [post]
// RefreshRoom 主播重新回到房间。
func (h *RoomHandler) RefreshRoom(c *gin.Context) {
	xl := c.MustGet(protocol.XLogKey).(*xlog.Logger)
	requestID := xl.ReqId
	userID := c.GetString(protocol.UserIDContextKey)

	args := protocol.RefreshRoomArgs{}
	err := c.BindJSON(&args)
	if err != nil {
		xl.Infof("invalid args in body, error %v", err)
		httpErr := errors.NewHTTPErrorBadRequest().WithRequestID(requestID).WithMessage("invalid args in request body")
		c.JSON(http.StatusBadRequest, httpErr)
		return
	}
	roomID := args.RoomID
	room, err := h.Room.GetRoomByID(xl, roomID)
	if err != nil {
		serverErr, ok := err.(*errors.ServerError)
		if !ok {
			xl.Errorf("get room with ID %s failed, error %v", roomID, err)
			httpErr := errors.NewHTTPErrorInternal().WithRequestID(requestID)
			c.JSON(http.StatusInternalServerError, httpErr)
			return
		}
		switch serverErr.Code {
		case errors.ServerErrorRoomNotFound:
			xl.Infof("room %s not found", roomID)
			httpErr := errors.NewHTTPErrorNoSuchRoom().WithRequestID(requestID).WithMessagef("room %s not found", roomID)
			c.JSON(http.StatusNotFound, httpErr)
			return
		default:
			xl.Errorf("get room with ID %s failed, error %v", roomID, err)
			httpErr := errors.NewHTTPErrorInternal().WithRequestID(requestID)
			c.JSON(http.StatusInternalServerError, httpErr)
			return
		}
	}
	if room.Creator != userID {
		xl.Infof("room %s is not created by user %s, cannot do refresh", roomID, userID)
		httpErr := errors.NewHTTPErrorNoSuchRoom().WithRequestID(requestID).WithMessagef("room %s not found", roomID)
		c.JSON(http.StatusNotFound, httpErr)
		return
	}

	xl.Infof("user %s refresh room %s, generated new RTC room token", userID, roomID)

	resp := &protocol.RefreshRoomResponse{
		RoomID:       roomID,
		RoomName:     room.Name,
		RTCRoom:      roomID,
		RTCRoomToken: h.generateRTCRoomToken(roomID, userID, "admin"),
		WSURL:        h.generateWSURL(xl, c.Request.Host),
	}
	c.JSON(http.StatusOK, resp)
}

// @Tags qlive api
// @ID enter-room
// @Summary Audience enters room
// @Description Audience enters room
// @Accept  json
// @Produce  json
// @Param EnterRoomRequest body protocol.EnterRoomRequest true "Audience enters room"
// @Success 200 {object} protocol.EnterRoomResponse
// @Failure 400 {object} errors.HTTPError
// @Failure 404 {object} errors.HTTPError
// @Failure 409 {object} errors.HTTPError
// @Failure 500 {object} errors.HTTPError
// @Router /enter_room [post]
// EnterRoom 进入直播间。
func (h *RoomHandler) EnterRoom(c *gin.Context) {
	xl := c.MustGet(protocol.XLogKey).(*xlog.Logger)
	requestID := xl.ReqId
	userID := c.GetString(protocol.UserIDContextKey)

	args := &protocol.EnterRoomRequest{}
	bindErr := c.BindJSON(&args)
	if bindErr != nil {
		xl.Infof("invalid args in request body, error %v", bindErr)
		httpError := errors.NewHTTPErrorBadRequest().WithRequestID(requestID).WithMessage("invalid args in request body")
		c.JSON(http.StatusBadRequest, httpError)
		return
	}

	updatedRoom, err := h.Room.EnterRoom(xl, userID, args.RoomID)
	if err != nil {
		serverErr, ok := err.(*errors.ServerError)
		if ok {
			switch serverErr.Code {
			case errors.ServerErrorRoomNotFound:
				xl.Infof("enter room failed: room %s not found", args.RoomID)
				httpErr := errors.NewHTTPErrorNoSuchRoom().WithRequestID(requestID)
				c.JSON(http.StatusNotFound, httpErr)
				return
			case errors.ServerErrorUserBroadcasting:
				xl.Infof("enter room failed: user %s is broadcasting", userID)
				httpErr := errors.NewHTTPErrorUserBroadcasting().WithRequestID(requestID)
				c.JSON(http.StatusConflict, httpErr)
				return
			case errors.ServerErrorUserJoined:
				xl.Infof("enter room failed: user %s joined", userID)
				httpErr := errors.NewHTTPErrorUserJoined().WithRequestID(requestID)
				c.JSON(http.StatusConflict, httpErr)
				return
			}
		}
		xl.Errorf("enter room failed, enter room request: %v, error: %v", args, err)
		httpErr := errors.NewHTTPErrorInternal().WithRequestID(requestID)
		c.JSON(http.StatusInternalServerError, httpErr)
		return
	}

	ret := &protocol.EnterRoomResponse{}
	// 获取creator的userInfo
	creator, err := h.Account.GetAccountByID(xl, updatedRoom.Creator)
	if err != nil {
		xl.Errorf("creator %v is not found", creator)
		httpErr := errors.NewHTTPErrorNoSuchUser().WithRequestID(requestID)
		c.JSON(http.StatusBadRequest, httpErr)
		return
	}
	creatorInfo := protocol.UserInfo{
		ID:        creator.ID,
		Nickname:  creator.Nickname,
		Gender:    creator.Gender,
		AvatarURL: creator.AvatarURL,
	}

	ret = &protocol.EnterRoomResponse{
		RoomID:   updatedRoom.ID,
		RoomName: updatedRoom.Name,
		RoomType: string(updatedRoom.Type),
		PlayURL:  updatedRoom.PlayURL,
		Creator:  creatorInfo,
		Status:   string(updatedRoom.Status),
	}

	// 获取pkAnchor的userInfo
	if updatedRoom.Status == protocol.LiveRoomStatusPK {
		pkAnchorInfo := &protocol.UserInfo{}
		pkAnchor, err := h.Account.GetAccountByID(xl, updatedRoom.PKAnchor)
		if err != nil {
			xl.Errorf("pkAnchor %v is not found", pkAnchor)
			httpErr := errors.NewHTTPErrorNoSuchUser().WithRequestID(requestID)
			c.JSON(http.StatusInternalServerError, httpErr)
			return
		}
		pkAnchorInfo = &protocol.UserInfo{
			ID:        pkAnchor.ID,
			Nickname:  pkAnchor.Nickname,
			Gender:    pkAnchor.Gender,
			AvatarURL: pkAnchor.AvatarURL,
		}
		ret.PKAnchor = pkAnchorInfo
	}
	// 若为语音房，为用户生成一个具有user权限的RTC房间token。
	if updatedRoom.Type == protocol.RoomTypeVoice {
		rtcRoomToken := h.generateRTCRoomToken(updatedRoom.ID, userID, "user")
		ret.RTCRoomToken = rtcRoomToken
		joinedAudieces, err := h.getJoinedAudiences(xl, updatedRoom)
		if err != nil {
			xl.Errorf("failed to get joined audiences of room %s, error %v", args.RoomID, err)
		} else {
			ret.JoinedAudiences = joinedAudieces
		}
		// 生成websocket的连接地址。
		ret.WSURL = h.generateWSURL(xl, c.Request.Host)
	}

	c.JSON(http.StatusOK, ret)
}

// @Tags qlive api
// @ID leave-room
// @Summary Audience leaves room
// @Description Audience leaves room
// @Accept  json
// @Produce  json
// @Param LeaveRoomArgs body protocol.LeaveRoomArgs true "Audience leaves room"
// @Success 200 {string} ok
// @Failure 400 {object} errors.HTTPError
// @Router /leave_room [post]
// LeaveRoom 离开直播间。
func (h *RoomHandler) LeaveRoom(c *gin.Context) {
	xl := c.MustGet(protocol.XLogKey).(*xlog.Logger)
	requestID := xl.ReqId
	userID := c.GetString(protocol.UserIDContextKey)

	args := &protocol.LeaveRoomArgs{}
	bindErr := c.BindJSON(&args)
	if bindErr != nil {
		xl.Infof("invalid args in request body, error %v", bindErr)
		httpErr := errors.NewHTTPErrorBadRequest().WithRequestID(requestID).WithMessage("invalid args in request body")
		c.JSON(http.StatusBadRequest, httpErr)
		return
	}

	err := h.Room.LeaveRoom(xl, userID, args.RoomID)
	if err != nil {
		xl.Infof("error when leaving room, error: %v", err)
		httpErr := errors.NewHTTPErrorBadRequest().WithRequestID(requestID)
		c.JSON(http.StatusBadRequest, httpErr)
	}

	c.JSON(http.StatusOK, "")
	return
}
