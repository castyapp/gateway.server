package hub

import cmap "github.com/orcaman/concurrent-map"

type RoomType int

const (
	UserRoomType RoomType = iota
	TheaterRoomType
)

type Room interface {
	GetType() RoomType
	GetName() string
	Join(c *Client)
	HandleEvents(c *Client) error
	Leave(c *Client)
	GetClients() cmap.ConcurrentMap
}