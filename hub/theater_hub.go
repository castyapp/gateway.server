package hub

import (
	"context"
	"errors"
	"github.com/CastyLab/grpc.proto/proto"
	"github.com/CastyLab/grpc.proto/protocol"
	"github.com/getsentry/sentry-go"
	"github.com/gobwas/ws"
	"github.com/gorilla/websocket"
	cmap "github.com/orcaman/concurrent-map"
	"log"
	"net/http"
)

/* Controls a bunch of rooms */
type TheaterHub struct {
	ctx      context.Context
	upgrader websocket.Upgrader
	userHub  *UserHub
	cmap     cmap.ConcurrentMap
}

func (h *TheaterHub) GetContext() context.Context {
	return h.ctx
}

func (h *TheaterHub) FindRoom(name string) (room Room, err error) {
	if r, ok := h.cmap.Get(name); ok {
		return r.(*TheaterRoom), nil
	}
	return nil, errors.New("theater room is missing from cmp")
}

/* If room doesn't exist creates it then returns it */
func (h *TheaterHub) GetOrCreateRoom(name string) (room Room) {
	if r, ok := h.cmap.Get(name); ok {
		return r.(*TheaterRoom)
	}
	room, _ = NewTheaterRoom(name, h)
	h.cmap.Set(name, room)
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

func (h *TheaterHub) Close() error {
	return nil
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

	client := NewTheaterClient(h, conn)
	defer client.Close()

	client.OnAuthorized(func(auth Auth) (room Room) {
		event := auth.Event().(*proto.TheaterLogOnEvent)
		room = h.GetOrCreateRoom(string(event.Room))
		room.Join(client)
		return
	})

	client.OnUnauthorized(func() {
		buffer, err := protocol.NewMsgProtobuf(proto.EMSG_UNAUTHORIZED, nil)
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
