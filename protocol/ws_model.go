package protocol

import "encoding/json"

// WebSocket Message Type
const (
	MT_Ping              = "ping"
	MT_Pong              = "pong"
	MT_Authorize         = "auth"
	MT_AuthorizeResponse = "auth-res"
	MT_Disconnect        = "disconnect"
)

type Ping struct {
}

func (p *Ping) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *Ping) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

type Pong struct {
}

func (p *Pong) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *Pong) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

type AuthorizeRequest struct {
	RPCID string `json:"rpcID,omitempty"`
	Token string `json:"token"`
}

func (p *AuthorizeRequest) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *AuthorizeRequest) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

type AuthorizeResponse struct {
	RPCID string `json:"rpcID,omitempty"`
	Code  int    `json:"code"`
	Error string `json:"error"`
}

func (p *AuthorizeResponse) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *AuthorizeResponse) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}

type Disconnect struct {
	Code     int    `json:"code"`
	Error    string `json:"error"`
	KickedID string `json:"kickedID"`
}

func (p *Disconnect) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *Disconnect) Unmarshal(b []byte) error {
	return json.Unmarshal(b, p)
}
