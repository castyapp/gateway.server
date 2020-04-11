package hub

import cmap "github.com/orcaman/concurrent-map"

type RoomType int

const (
	UserRoomType RoomType = iota
	TheaterRoomType
)

type Room interface {
	GetName() string
	Join(*Client)
	HandleEvents(*Client) error
	Leave(*Client)
	GetClients() cmap.ConcurrentMap
}