package controller

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/qiniu/qmgo"
	"github.com/qiniu/x/xlog"

	"github.com/qrtc/qlive/errors"
	"github.com/qrtc/qlive/handler"
	"github.com/qrtc/qlive/protocol"
)

const (
	// DefaultRoomNumberLimit 默认最大的直播间数量。
	DefaultRoomNumberLimit = 20
)

// RoomController 直播房间创建、关闭、查询等操作。
type RoomController struct {
	mongoClient *qmgo.Client
	roomColl    *qmgo.Collection
	// roomNumberLimit 最大的直播间数量。当直播间数量大于等于该数字时无法创建新的直播间，服务端返回503.
	roomNumberLimit int
	xl              *xlog.Logger
	accountHandler  handler.AccountHandler
}

// NewRoomController 创建 room controller.
func NewRoomController(mongoURI string, database string, account *handler.AccountHandler, xl *xlog.Logger) (*RoomController, error) {
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
	roomColl := mongoClient.Database(database).Collection(RoomsCollectionn)
	return &RoomController{
		mongoClient:     mongoClient,
		roomColl:        roomColl,
		roomNumberLimit: DefaultRoomNumberLimit,
		xl:              xl,
		accountHandler:  *account,
	}, nil
}

// CreateRoom 创建直播房间
func (c *RoomController) CreateRoom(xl *xlog.Logger, room *protocol.LiveRoom) error {
	if xl == nil {
		xl = c.xl
	}

	// 查看是否超过房间数量限制。
	n, err := c.roomColl.Find(context.Background(), bson.M{}).Count()
	if err != nil {
		xl.Errorf("failed to get total room number, error %v", err)
		return &errors.ServerError{Code: errors.ServerErrorMongoOpFail}
	}
	if n >= int64(c.roomNumberLimit) {
		xl.Warnf("room number limit exceeded, current %d, max %d", n, c.roomNumberLimit)
		return &errors.ServerError{Code: errors.ServerErrorTooManyRooms}
	}

	// 查看是否已有同名房间。
	err = c.roomColl.Find(context.Background(), bson.M{"name": room.Name}).One(&protocol.LiveRoom{})
	if err != nil {
		if !qmgo.IsErrNoDocuments(err) {
			xl.Errorf("failed to find room with name %s, error %v", room.Name, err)
			return &errors.ServerError{Code: errors.ServerErrorMongoOpFail}
		}
	} else {
		xl.Infof("room name %s is already used", room.Name)
		return &errors.ServerError{Code: errors.ServerErrorRoomNameUsed}
	}
	// TODO:限制一个用户只能开一个直播间？

	_, err = c.roomColl.InsertOne(context.Background(), room)
	if err != nil {
		xl.Errorf("failed to insert room, error %v", err)
		return err
	}
	return nil
}

// CloseRoom 关闭直播房间
func (c *RoomController) CloseRoom(xl *xlog.Logger, userID string, roomID string) error {
	if xl == nil {
		xl = c.xl
	}

	err := c.roomColl.Find(context.Background(), bson.M{"_id": roomID, "creator": userID}).One(&protocol.LiveRoom{})
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
			return nil, fmt.Errorf("not found")
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
	err := c.roomColl.Find(context.Background(), bson.M{"status": "single"}).All(&rooms)
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
	if newRoom.Status != room.Status {
		room.Status = newRoom.Status
	}
	if newRoom.RTCRoom != "" {
		room.RTCRoom = newRoom.RTCRoom
	}
	if newRoom.PlayURL != "" {
		room.PlayURL = newRoom.PlayURL
	}
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

	// 用户进入房间
	room.Audiences = append(room.Audiences, userID)
	updatedRoom, err := c.UpdateRoom(xl, room.ID, room)
	if err != nil {
		xl.Infof("error when updating room %v", err)
		return nil, err
	}

	return updatedRoom, nil
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

	//TODO 用户退出房间，不在房间是否需要err？
	for index, audience := range room.Audiences {
		if audience == userID {
			room.Audiences = append(room.Audiences[:index], room.Audiences[index+1:]...)
		}
	}
	_, err = c.UpdateRoom(xl, room.ID, room)
	if err != nil {
		xl.Infof("error when updating room %v", err)
		return err
	}

	return nil
}

// ListRooms 获取直播间列表
func (c *RoomController) ListRooms(xl *xlog.Logger, onlyListPkRooms string, userID string) ([]protocol.LiveRoom, error) {
	var roomLists []protocol.LiveRoom
	var err error
	if onlyListPkRooms == "true" {
		roomLists, err = c.ListPKRooms(xl, userID)
	} else {
		roomLists, err = c.ListAllRooms(xl)
	}
	return roomLists, err
}
