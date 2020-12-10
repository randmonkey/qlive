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
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qiniu/x/xlog"
	"github.com/stretchr/testify/assert"

	"github.com/qrtc/qlive/config"
	"github.com/qrtc/qlive/protocol"
)

func TestGeneratewsURL(t *testing.T) {
	xlog.SetOutputLevel(0)
	h := &RoomHandler{}
	testCases := []struct {
		wsAddress   string
		wsProtocol  string
		wsPort      int
		wsPath      string
		host        string
		expectedURL string
	}{

		{
			wsAddress:   "wss://localhost:1234/qlive",
			wsProtocol:  "wss",
			wsPort:      8081,
			wsPath:      "/qlive",
			host:        "127.0.0.1",
			expectedURL: "wss://localhost:1234/qlive",
		},
		{
			wsAddress:   "localhost:1234/qlive",
			wsProtocol:  "wss",
			wsPort:      8081,
			wsPath:      "/qlive",
			host:        "127.0.0.1",
			expectedURL: "wss://localhost:1234/qlive",
		},
		{
			wsAddress:   "",
			wsProtocol:  "ws",
			wsPort:      8081,
			wsPath:      "/qlive",
			host:        "127.0.0.1",
			expectedURL: "ws://127.0.0.1:8081/qlive",
		},
		{
			wsAddress:   "",
			wsProtocol:  "wss",
			wsPort:      8081,
			wsPath:      "/qlive",
			host:        "127.0.0.1:8080",
			expectedURL: "wss://127.0.0.1:8081/qlive",
		},
		{
			wsAddress:   "",
			wsProtocol:  "ws",
			wsPort:      80,
			wsPath:      "/qlive",
			host:        "127.0.0.1:8080",
			expectedURL: "ws://127.0.0.1/qlive",
		},
		{
			wsAddress:   "",
			wsProtocol:  "wss",
			wsPort:      443,
			wsPath:      "/qlive",
			host:        "127.0.0.1:80",
			expectedURL: "wss://127.0.0.1/qlive",
		},
	}
	for i, testCase := range testCases {
		h.WSAddress = testCase.wsAddress
		h.WSProtocol = testCase.wsProtocol
		h.WSPort = testCase.wsPort
		h.WSPath = testCase.wsPath
		xl := xlog.New(fmt.Sprintf("test-generatewsURL-%d", i))
		wsURL := h.generateWSURL(xl, testCase.host)
		assert.Equalf(t, testCase.expectedURL, wsURL, "test case %d: URL does not match", i)
	}
}

func TestCreateRoom(t *testing.T) {

	testCases := []struct {
		userID             string
		roomName           string
		roomType           string
		maxRooms           int
		expectedStatusCode int
		expectedRoomStatus string
	}{
		{
			userID:             "user-0",
			roomName:           "room-0",
			roomType:           "pk",
			maxRooms:           10,
			expectedStatusCode: 200,
			expectedRoomStatus: "single",
		},
		{
			userID:             "user-1",
			roomName:           "room-1",
			roomType:           "pk",
			maxRooms:           10,
			expectedStatusCode: 200,
			expectedRoomStatus: "single",
		},
		{
			userID:             "user-1",
			roomName:           "room-1",
			roomType:           "voice",
			maxRooms:           10,
			expectedStatusCode: 200,
			expectedRoomStatus: "voiceLive",
		},
		{
			userID:             "user-1",
			roomName:           "",
			maxRooms:           10,
			expectedStatusCode: 400,
		},
		{
			userID:             "user-1",
			roomName:           "room-2",
			roomType:           "invalid",
			maxRooms:           10,
			expectedStatusCode: 400,
		},
		{
			userID:             "user-1",
			roomName:           "room-1",
			roomType:           "pk",
			maxRooms:           1,
			expectedStatusCode: 503,
		},
		{
			userID:             "user-0",
			roomName:           "room-1",
			roomType:           "pk",
			maxRooms:           10,
			expectedStatusCode: 403,
		},
		{
			userID:             "user-1",
			roomName:           "room-0",
			roomType:           "pk",
			maxRooms:           10,
			expectedStatusCode: 409,
		},
	}

	for i, testCase := range testCases {
		// initialize handler for each case.
		mockRoom := &mockRoom{
			rooms: map[string]*protocol.LiveRoom{
				"room-0": {
					ID:      "room-0",
					Name:    "room-0",
					Creator: "user-0",
					Status:  protocol.LiveRoomStatusSingle,
				},
			},
			roomAudiences: map[string][]string{},
			maxRooms:      testCase.maxRooms,
		}
		mockAccount := &mockAccount{}
		handler := &RoomHandler{
			Account:   mockAccount,
			Room:      mockRoom,
			RTCConfig: &config.QiniuRTCConfig{PublishHost: "test.example.com", PublishHub: "test"},
		}
		// intitialize test recorder and context
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		xl := xlog.New(fmt.Sprintf("test-create-room-%d", i))
		c.Set(protocol.XLogKey, xl)
		c.Set(protocol.UserIDContextKey, testCase.userID)
		// build request
		createRoomReq := &protocol.CreateRoomArgs{
			UserID:   testCase.userID,
			RoomName: testCase.roomName,
			RoomType: testCase.roomType,
		}
		buf, err := json.Marshal(createRoomReq)
		assert.Nilf(t, err, "failed to build request body for case %d, error %v", i, err)
		bodyReader := bytes.NewBuffer(buf)
		req, err := http.NewRequest("POST", "/v1/rooms", bodyReader)
		assert.Nilf(t, err, "failed to craete request for case %d, error %v", i, err)
		c.Request = req

		handler.CreateRoom(c)
		assert.Equalf(t, testCase.expectedStatusCode, w.Code, "code is not the same as expected for test case %d", i)
		if testCase.expectedStatusCode == http.StatusOK {
			roomResp := &protocol.CreateRoomResponse{}
			err := json.Unmarshal(w.Body.Bytes(), &roomResp)
			assert.Nilf(t, err, "should parse create result successfully for test case %d", i)
			room, err := mockRoom.GetRoomByID(xl, roomResp.RoomID)
			assert.Nilf(t, err, "should get room %s successfully for test case %d", roomResp.RoomID, i)
			assert.Equalf(t, testCase.expectedRoomStatus, string(room.Status), "room status should be equal for test case %d", i)
		}
	}
}

func TestListAllRooms(t *testing.T) {
	// test data of rooms and users
	room0 := &protocol.LiveRoom{
		ID:      "room-0",
		Name:    "room0",
		Type:    protocol.RoomTypePK,
		Creator: "user-0",
		Status:  protocol.LiveRoomStatusSingle,
	}
	pkRoom1 := &protocol.LiveRoom{
		ID:       "pkroom-1",
		Name:     "pk1",
		Type:     protocol.RoomTypePK,
		Creator:  "user-1",
		Status:   protocol.LiveRoomStatusPK,
		PKAnchor: "user-2",
	}
	pkRoom2 := &protocol.LiveRoom{
		ID:       "pkroom-2",
		Name:     "pk2",
		Type:     protocol.RoomTypePK,
		Creator:  "user-2",
		Status:   protocol.LiveRoomStatusPK,
		PKAnchor: "user-1",
	}
	users := []*protocol.Account{
		{ID: "user-0", Nickname: "name-0", Gender: "secret"},
		{ID: "user-1", Nickname: "name-1", Gender: "male"},
		{ID: "user-2", Nickname: "name-2", Gender: "female"},
	}

	testCases := []struct {
		rooms                []*protocol.LiveRoom
		userID               string
		expectedLength       int
		expectedCreators     map[string]string
		expectedCreatorNames map[string]string
		expectedPKAnchors    map[string]string
	}{
		{
			rooms:                []*protocol.LiveRoom{room0, pkRoom1, pkRoom2},
			userID:               "user-x",
			expectedLength:       3,
			expectedCreators:     map[string]string{"room-0": "user-0", "pkroom-1": "user-1", "pkroom-2": "user-2"},
			expectedCreatorNames: map[string]string{"room-0": "name-0", "pkroom-1": "name-1", "pkroom-2": "name-2"},
			expectedPKAnchors:    map[string]string{"room-0": "", "pkroom-1": "user-2", "pkroom-2": "user-1"},
		},
		{
			rooms:                []*protocol.LiveRoom{room0, pkRoom1, pkRoom2},
			userID:               "user-0",
			expectedLength:       2,
			expectedCreators:     map[string]string{"pkroom-1": "user-1", "pkroom-2": "user-2"},
			expectedCreatorNames: map[string]string{"pkroom-1": "name-1", "pkroom-2": "name-2"},
			expectedPKAnchors:    map[string]string{"pkroom-1": "user-2", "pkroom-2": "user-1"},
		},
	}

	for i, testCase := range testCases {
		mockRoom := &mockRoom{
			rooms:         map[string]*protocol.LiveRoom{},
			roomAudiences: map[string][]string{},
			maxRooms:      10,
		}
		for _, room := range testCase.rooms {
			mockRoom.rooms[room.ID] = room
		}
		mockAccount := &mockAccount{accounts: users}
		handler := &RoomHandler{
			Account:   mockAccount,
			Room:      mockRoom,
			RTCConfig: &config.QiniuRTCConfig{PublishHost: "test.example.com", PublishHub: "test"},
		}

		// intitialize test recorder and context
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(protocol.XLogKey, xlog.New(fmt.Sprintf("test-list-rooms-%d", i)))
		c.Set(protocol.UserIDContextKey, testCase.userID)
		// build request
		req, err := http.NewRequest("GET", "/v1/rooms", nil)
		assert.Nilf(t, err, "failed to craete request for case %d,error %v", i, err)
		c.Request = req
		// run handler and compare results
		handler.ListAllRooms(c)
		resp := &protocol.ListRoomsResponse{}
		err = json.Unmarshal(w.Body.Bytes(), resp)
		assert.Nilf(t, err, "failed to read response for case %d, error %v", i, err)
		// compare response with expected result
		assert.Lenf(t, resp.Rooms, testCase.expectedLength, "length is not equal for case %d", i)
		for _, room := range resp.Rooms {
			assert.Equalf(t, testCase.expectedCreators[room.ID], room.Creator.ID, "creator ID of room %s is not the same as expected in case %d", room.ID, i)
			assert.Equalf(t, testCase.expectedCreatorNames[room.ID], room.Creator.Nickname, "creator nickname of room %s is not the same as expected in case %d", room.ID, i)
			pkAnchorID := ""
			if room.PKAnchor != nil {
				pkAnchorID = room.PKAnchor.ID
			}
			assert.Equalf(t, testCase.expectedPKAnchors[room.ID], pkAnchorID, "PK anchor ID of room %s is not the same as expected in case %d", room.ID, i)
		}
	}
}

func TestGetRoom(t *testing.T) {
	// test data of rooms and users
	room0 := &protocol.LiveRoom{
		ID:      "room-0",
		Name:    "room0",
		Creator: "user-0",
		Status:  protocol.LiveRoomStatusSingle,
	}
	users := []*protocol.Account{
		{ID: "user-0", Nickname: "name-0", Gender: "secret"},
	}

	testCases := []struct {
		roomID              string
		expectedStatusCode  int
		expectedRoomName    string
		expectedCreator     string
		expectedCreatorName string
	}{
		{
			roomID:              "room-0",
			expectedStatusCode:  200,
			expectedRoomName:    "room0",
			expectedCreator:     "user-0",
			expectedCreatorName: "name-0",
		},
		{
			roomID:             "room-1",
			expectedStatusCode: 404,
		},
	}

	for i, testCase := range testCases {
		mockRoom := &mockRoom{
			rooms:         map[string]*protocol.LiveRoom{"room-0": room0},
			roomAudiences: map[string][]string{},
			maxRooms:      10,
		}
		mockAccount := &mockAccount{accounts: users}
		handler := &RoomHandler{
			Account:   mockAccount,
			Room:      mockRoom,
			RTCConfig: &config.QiniuRTCConfig{PublishHost: "test.example.com", PublishHub: "test"},
		}
		// intitialize test recorder and context
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(protocol.XLogKey, xlog.New(fmt.Sprintf("test-get-room-%d", i)))
		// build request
		req, err := http.NewRequest("GET", "/v1/rooms/"+testCase.roomID, nil)
		assert.Nilf(t, err, "failed to craete request for case %d,error %v", i, err)
		c.Request = req
		c.Params = append(c.Params, gin.Param{Key: "roomID", Value: testCase.roomID})
		// run handler and compare results
		handler.GetRoom(c)
		// compare status code
		assert.Equalf(t, testCase.expectedStatusCode, w.Code, "status code is not equal to expected for case %d", i)
		if testCase.expectedStatusCode == http.StatusOK {
			resp := &protocol.GetRoomResponse{}
			err = json.Unmarshal(w.Body.Bytes(), resp)
			assert.Nilf(t, err, "failed to parse response for case %d, error %v", i, err)
			// compare response with expected result
			assert.Equalf(t, testCase.expectedRoomName, resp.Name, "room name is not the same as expected for case %d", i)
			assert.Equalf(t, testCase.expectedCreator, resp.Creator.ID, "creator ID is not the same as expected for case %d", i)
			assert.Equalf(t, testCase.expectedCreatorName, resp.Creator.Nickname, "creator nickname is not the same as expected for case %d", i)
		}
	}
}
