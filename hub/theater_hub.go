package hub

import (
	"context"
	"errors"
	"github.com/CastyLab/gateway.server/hub/protocol/protobuf"
	"github.com/CastyLab/gateway.server/hub/protocol/protobuf/enums"
	"github.com/CastyLab/grpc.proto/proto"
	"github.com/getsentry/sentry-go"
	"github.com/gobwas/ws"
	pb "github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/orcaman/concurrent-map"
	"log"
	"net/http"
)

/* Controls a bunch of rooms */
type TheaterHub struct {
	ctx       context.Context
	upgrader  websocket.Upgrader
	userHub   *UserHub
	cmap      cmap.ConcurrentMap
}

/* If room doesn't exist creates it then returns it */
func (h *TheaterHub) GetOrCreateRoom(name string) (room *TheaterRoom, err error) {
	if r, ok := h.cmap.Get(name); ok {
		return r.(*TheaterRoom), nil
	}
	room, err = NewTheaterRoom(name, h)
	return
}

/* If room doesn't exist creates it then returns it */
func (h *TheaterHub) GetRoom(name string) (*TheaterRoom, error) {
	if !h.cmap.Has(name) {
		return nil, errors.New("room not found")
	}
	if r, ok := h.cmap.Get(name); ok {
		return r.(*TheaterRoom), nil
	}
	return nil, errors.New("room is missing from cmp")
}

func (h *TheaterHub) RemoveRoom(name string) {
	h.cmap.Remove(name)
	return
}

/* Get ws conn. and hands it over to correct room */
func (h *TheaterHub) Handler(w http.ResponseWriter, req *http.Request) {

	h.ctx = req.Context()
	conn, _, _, err := ws.UpgradeHTTP(req, w)
	if err != nil {
		sentry.CaptureException(err)
		log.Println("upgrade:", err)
		return
	}

	client := NewClient(h.ctx, conn, TheaterRoomType)
	defer client.Close()

	client.OnAuthorized(func(e pb.Message, u *proto.User) Room {
		event := e.(*protobuf.TheaterLogOnEvent)
		room , err := h.GetOrCreateRoom(string(event.Room))
		if err != nil {
			_ = client.conn.Close()
			log.Println("Error while creating or getting the room from cmp: ", err)
			return nil
		}
		room.Join(client)
		return room
	})

	client.OnUnauthorized(func() {
		buffer, err := protobuf.NewMsgProtobuf(enums.EMSG_UNAUTHORIZED, nil)
		if err == nil {
			_ = client.WriteMessage(buffer.Bytes())
		}
	})

	client.Listen()
	return
}

/* Constructor */
func NewTheaterHub(uhub *UserHub) *TheaterHub {
	return &TheaterHub{
		upgrader: newUpgrader(),
		userHub:  uhub,
		cmap:     cmap.New(),
	}
}