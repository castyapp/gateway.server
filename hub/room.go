package hub

type RoomType int

const (
	UserRoomType       RoomType = 0
	TheaterRoomType    RoomType = 1
)

type Room interface {
	Join(*Client)
	HandleEvents(*Client) error
	Leave(*Client)
	GetClients() map[uint32] *Client
}