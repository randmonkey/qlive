package service

import (
	"encoding/base64"
	"encoding/binary"
	"net/http"
	"strings"
	"time"
)

var pid = uint32(time.Now().UnixNano() % 4294967291)

func NewREQID() string {
	var b [12]byte
	binary.LittleEndian.PutUint32(b[:], pid)
	binary.LittleEndian.PutUint64(b[4:], uint64(time.Now().UnixNano()))
	return base64.URLEncoding.EncodeToString(b[:])
}

func GetRemoteAddr(r *http.Request) string {

	if addr := r.Header.Get("X-Forwarded-For"); addr != "" {
		if idx := strings.Index(addr, ","); idx != -1 {
			addr = addr[:idx]
		}
		return addr
	}

	if addr := r.Header.Get("X-Real-IP"); addr != "" {
		if port := r.Header.Get("X-Real-PORT"); port != "" {
			addr += ":" + port
		}
		return addr
	}
	return r.RemoteAddr
}
