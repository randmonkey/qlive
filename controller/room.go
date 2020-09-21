package controller

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/qiniu/qmgo"
	"github.com/qiniu/x/xlog"

	"github.com/qrtc/qlive/errors"
	"github.com/qrtc/qlive/protocol"
)

const (
	// DefaultRoomNumberLimit 默认最大的直播间数量。
	DefaultRoomNumberLimit = 20
)

// RoomController 直播房间创建、关闭、查询等操作。
type RoomController struct {
	mongoClient    *qmgo.Client
	roomColl       *qmgo.Collection
	activeUserColl *qmgo.Collection
	// roomNumberLimit 最大的直播间数量。当直播间数量大于等于该数字时无法创建新的直播间，服务端返回503.
	roomNumberLimit int
	xl              *xlog.Logger
}

// NewRoomController 创建 room controller.
func NewRoomController(mongoURI string, database string, xl *xlog.Logger) (*RoomController, error) {
	if xl == nil {
		xl = xlog.New("qlive-room-controller")
	}
	mongoClient, err := qmgo.NewClient(context.Background(), &qmgo.Config{
		Uri:      mongoURI,
		Database: database,
	})
	if err != nil {
		xl.Errorf("failed to create mongo client, error %v", err)
		return nil, err
	}
	roomColl := mongoClient.Database(database).Collection(RoomsCollection)
	activeUserColl := mongoClient.Database(database).Collection(ActiveUserCollection)
	return &RoomController{
		mongoClient:     mongoClient,
		roomColl:        roomColl,
		activeUserColl:  activeUserColl,
		roomNumberLimit: DefaultRoomNumberLimit,
		xl:              xl,
	}, nil
}

// CreateRoom 创建直播房间
func (c *RoomController) CreateRoom(xl *xlog.Logger, room *protocol.LiveRoom) (*protocol.LiveRoom, error) {
	if xl == nil {
		xl = c.xl
	}

	// 查看是否超过房间数量限制。
	n, err := c.roomColl.Find(context.Background(), bson.M{}).Count()
	if err != nil {
		xl.Errorf("failed to get total room number, error %v", err)
		return nil, &errors.ServerError{Code: errors.ServerErrorMongoOpFail}
	}
	if n >= int64(c.roomNumberLimit) {
		xl.Warnf("room number limit exceeded, current %d, max %d", n, c.roomNumberLimit)
		return nil, &errors.ServerError{Code: errors.ServerErrorTooManyRooms}
	}

	// 查看是否已有同名房间。
	existingRoom := protocol.LiveRoom{}
	err = c.roomColl.Find(context.Background(), bson.M{"name": room.Name}).One(&existingRoom)
	if err != nil {
		if !qmgo.IsErrNoDocuments(err) {
			xl.Errorf("failed to find room with name %s, error %v", room.Name, err)
			return nil, &errors.ServerError{Code: errors.ServerErrorMongoOpFail}
		}
	} else {
		if existingRoom.Creator != room.Creator {
			xl.Infof("room name %s is already used", room.Name)
			return nil, &errors.ServerError{Code: errors.ServerErrorRoomNameUsed}
		}
		// 如果是该用户创建的直播间，返回直播间的现有信息。
		return &existingRoom, nil
	}

	// 无同名房间，查看该用户是否已经创建过直播间。
	creatorID := room.Creator
	n, err = c.roomColl.Find(context.Background(), bson.M{"creator": creatorID}).Count()
	if err != nil {
		if !qmgo.IsErrNoDocuments(err) {
			xl.Errorf("failed to count rooms created by %s, error %v", creatorID, err)
			return nil, &errors.ServerError{Code: errors.ServerErrorMongoOpFail}
		}
	}
	if n > 0 {
		xl.Infof("user %s has created at least one room, cannot create room", creatorID)
		return nil, &errors.ServerError{Code: errors.ServerErrorCanOnlyCreateOneRoom}
	}

	_, err = c.roomColl.InsertOne(context.Background(), room)
	if err != nil {
		xl.Errorf("failed to insert room, error %v", err)
		return nil, err
	}
	// 修改创建者状态为单人直播中。

	activeUser := protocol.ActiveUser{}
	err = c.activeUserColl.Find(context.Background(), bson.M{"_id": creatorID}).One(&activeUser)
	if err != nil {
		xl.Errorf("failed to find creator %s in active users, error %v", creatorID, err)
		return nil, err
	}
	activeUser.Status = protocol.UserStatusSingleLive
	activeUser.Room = room.ID
	err = c.activeUserColl.UpdateOne(context.Background(), bson.M{"_id": creatorID}, bson.M{"$set": &activeUser})
	if err != nil {
		xl.Errorf("failed to update user status of room creator %s", creatorID)
	}
	return room, nil
}

// CloseRoom 关闭直播房间
func (c *RoomController) CloseRoom(xl *xlog.Logger, userID string, roomID string) error {
	if xl == nil {
		xl = c.xl
	}

	// 查找mongo中是否有此房间。
	room := protocol.LiveRoom{}
	err := c.roomColl.Find(context.Background(), bson.M{"_id": roomID, "creator": userID}).One(&room)
	if err != nil {
		if qmgo.IsErrNoDocuments(err) {
			xl.Infof("cannot found room %s created by user %s", roomID, userID)
			return &errors.ServerError{Code: errors.ServerErrorRoomNotFound}
		}
		return err
	}
	err = c.roomColl.RemoveId(context.Background(), roomID)
	if err != nil {
		xl.Errorf("failed to remove room ID %s, error %v", roomID, err)
		return err
	}
	// 修改创建者状态为空闲。
	activeUser := protocol.ActiveUser{}
	err = c.activeUserColl.Find(context.Background(), bson.M{"_id": userID}).One(&activeUser)
	if err != nil {
		xl.Errorf("failed to find creator %s in active users, error %v", userID, err)
		return err
	}
	activeUser.Status = protocol.UserStatusIdle
	activeUser.Room = ""
	err = c.activeUserColl.UpdateOne(context.Background(), bson.M{"_id": userID}, bson.M{"$set": &activeUser})
	if err != nil {
		xl.Errorf("failed to update user status of room creator %s", userID)
	}
	// 修改所有观众状态为空闲。
	for _, audienceID := range room.Audiences {
		activeUser := protocol.ActiveUser{}
		err = c.activeUserColl.Find(context.Background(), bson.M{"_id": audienceID}).One(&activeUser)
		if err != nil {
			xl.Errorf("failed to find audience %s in active users, error %v", audienceID, err)
			continue
		} else {
			activeUser.Status = protocol.UserStatusIdle
			activeUser.Room = ""
			err = c.activeUserColl.UpdateOne(context.Background(), bson.M{"_id": audienceID}, bson.M{"$set": &activeUser})
			if err != nil {
				xl.Errorf("failed to update status of audience %s in active users, error %v", audienceID, err)
			}
		}
	}
	return nil
}

// GetRoomByFields 根据一组 key/value 关系查找直播房间。
func (c *RoomController) GetRoomByFields(xl *xlog.Logger, fields map[string]interface{}) (*protocol.LiveRoom, error) {
	if xl == nil {
		xl = c.xl
	}
	room := protocol.LiveRoom{}
	err := c.roomColl.Find(context.Background(), fields).One(&room)
	if err != nil {
		if qmgo.IsErrNoDocuments(err) {
			xl.Infof("no such room for fields %v", fields)
			return nil, &errors.ServerError{Code: errors.ServerErrorRoomNotFound}
		}
		xl.Errorf("failed to get room, error %v", fields)
		return nil, err
	}
	return &room, nil
}

// GetRoomByID 使用 ID 查找直播房间。
func (c *RoomController) GetRoomByID(xl *xlog.Logger, id string) (*protocol.LiveRoom, error) {
	return c.GetRoomByFields(xl, map[string]interface{}{"_id": id})
}

// ListAllRooms 获取所有直播房间列表
func (c *RoomController) ListAllRooms(xl *xlog.Logger) ([]protocol.LiveRoom, error) {
	if xl == nil {
		xl = c.xl
	}
	rooms := []protocol.LiveRoom{}
	err := c.roomColl.Find(context.Background(), bson.M{}).All(&rooms)
	return rooms, err
}

// ListPKRooms 获取可与某一主播PK的直播房间列表
func (c *RoomController) ListPKRooms(xl *xlog.Logger, userID string) ([]protocol.LiveRoom, error) {
	if xl == nil {
		xl = c.xl
	}
	rooms := []protocol.LiveRoom{}
	err := c.roomColl.Find(context.Background(), bson.M{
		"status":  "single",
		"creator": bson.M{"$ne": userID},
	}).All(&rooms)
	if err != nil {
		xl.Errorf("failed to list PK rooms, error %v", err)
	}
	return rooms, err
}

// UpdateRoom 更新直播房间信息。
func (c *RoomController) UpdateRoom(xl *xlog.Logger, id string, newRoom *protocol.LiveRoom) (*protocol.LiveRoom, error) {
	if xl == nil {
		xl = c.xl
	}
	room, err := c.GetRoomByID(xl, id)
	if err != nil {
		return nil, err
	}
	// 需要修改房间名称的情况，先校验是否有同名不同ID的房间。
	if newRoom.Name != "" && newRoom.Name != room.Name {
		_, err = c.GetRoomByFields(xl, bson.M{"_id": bson.M{"$ne": newRoom.Name}, "name": newRoom.Name})
		if err != nil {
			serverErr, ok := err.(*errors.ServerError)
			if !ok || serverErr.Code != errors.ServerErrorRoomNotFound {
				xl.Errorf("failed to get rooms with name %s, error %v", id, err)
				return nil, &errors.ServerError{Code: errors.ServerErrorMongoOpFail}
			}
		} else {
			xl.Infof("room name %s is used by another rooms", newRoom.Name)
			return nil, &errors.ServerError{Code: errors.ServerErrorRoomNameUsed}
		}
		room.Name = newRoom.Name
	}
	if newRoom.Status != room.Status {
		room.Status = newRoom.Status
	}
	if newRoom.RTCRoom != "" {
		room.RTCRoom = newRoom.RTCRoom
	}
	if newRoom.PlayURL != "" {
		room.PlayURL = newRoom.PlayURL
	}
	room.PKAnchor = newRoom.PKAnchor
	room.Audiences = newRoom.Audiences
	err = c.roomColl.UpdateOne(context.Background(), bson.M{"_id": id}, bson.M{"$set": room})
	if err != nil {
		xl.Errorf("failed to update room %s,error %v", id, err)
		return nil, err
	}
	return room, nil
}

// EnterRoom 进入直播房间。
func (c *RoomController) EnterRoom(xl *xlog.Logger, userID string, roomID string) (*protocol.LiveRoom, error) {
	if xl == nil {
		xl = c.xl
	}
	room, err := c.GetRoomByID(xl, roomID)
	if err != nil {
		return nil, err
	}

	// 更新房间观众列表。
	found := false
	for _, audience := range room.Audiences {
		if audience == userID {
			found = true
			xl.Infof("user %s already in room %s", userID, roomID)
			break
		}
	}
	if !found {
		room.Audiences = append(room.Audiences, userID)
	}

	updatedRoom, err := c.UpdateRoom(xl, room.ID, room)
	if err != nil {
		xl.Infof("error when updating room %v", err)
		return nil, err
	}
	// 修改用户状态为观看中。
	activeUser := protocol.ActiveUser{}
	err = c.activeUserColl.Find(context.Background(), bson.M{"_id": userID}).One(&activeUser)
	if err != nil {
		xl.Errorf("failed to find user %s in active users, error %v", userID, err)
		return nil, err
	}
	activeUser.Status = protocol.UserStatusWatching
	activeUser.Room = roomID
	err = c.activeUserColl.UpdateOne(context.Background(), bson.M{"_id": userID}, bson.M{"$set": &activeUser})
	if err != nil {
		xl.Errorf("failed to update user status of user %s, error %v", userID, err)
		return nil, err
	}
	return updatedRoom, nil
}

// ListRoomsByFields 根据字段列出房间。
func (c *RoomController) ListRoomsByFields(xl *xlog.Logger, fields map[string]interface{}) ([]protocol.LiveRoom, error) {
	if xl == nil {
		xl = c.xl
	}
	if fields == nil {
		fields = map[string]interface{}{}
	}

	rooms := []protocol.LiveRoom{}
	err := c.roomColl.Find(context.Background(), fields).All(&rooms)
	if err != nil {
		xl.Errorf("failed to list rooms, error %v", err)
	}
	return rooms, nil
}

// LeaveRoom 退出直播房间。
func (c *RoomController) LeaveRoom(xl *xlog.Logger, userID string, roomID string) error {
	if xl == nil {
		xl = c.xl
	}
	room, err := c.GetRoomByID(xl, roomID)
	if err != nil {
		return err
	}

	//查看用户是否在当前房间，若在当前房间，从观众列表中移除此用户。
	found := false
	audiences := []string{}
	for _, audience := range room.Audiences {
		if audience == userID {
			found = true
		} else {
			audiences = append(audiences, audience)
		}
	}
	if !found {
		xl.Errorf("user %s not found in room %s", userID, roomID)
	}
	room.Audiences = audiences

	_, err = c.UpdateRoom(xl, room.ID, room)
	if err != nil {
		xl.Infof("error when updating room %v", err)
		return err
	}

	// 修改用户状态为空闲。
	activeUser := protocol.ActiveUser{}
	err = c.activeUserColl.Find(context.Background(), bson.M{"_id": userID}).One(&activeUser)
	if err != nil {
		xl.Errorf("failed to find user %s in active users, error %v", userID, err)
		return err
	}
	activeUser.Status = protocol.UserStatusIdle
	activeUser.Room = ""
	err = c.activeUserColl.UpdateOne(context.Background(), bson.M{"_id": userID}, bson.M{"$set": &activeUser})
	if err != nil {
		xl.Errorf("failed to update user status of user %s, error %v", userID, err)
	}
	return nil
}
