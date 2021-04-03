package hub

import (
	"net/http"
)

type Hub interface {
	FindRoom(name string) (room Room, err error)
	GetOrCreateRoom(name string) (room Room)
	RemoveRoom(name string)
	Close() error
	Handler(w http.ResponseWriter, req *http.Request)
}
