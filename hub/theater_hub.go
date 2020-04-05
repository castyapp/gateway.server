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

func (hub *TheaterHub) WithContext(ctx context.Context) {
	hub.ctx = ctx
}

func (hub *TheaterHub) GetContext() context.Context {
	return hub.ctx
}

func (hub *TheaterHub) FindRoom(name string) (room Room, err error) {
	if r, ok := hub.cmap.Get(name); ok {
		return r.(*TheaterRoom), nil
	}
	return nil, errors.New("theater room is missing from cmp")
}

/* If room doesn't exist creates it then returns it */
func (hub *TheaterHub) GetOrCreateRoom(name string) (room Room) {
	if r, ok := hub.cmap.Get(name); ok {
		return r.(*TheaterRoom)
	}
	room, _ = NewTheaterRoom(name, hub)
	hub.cmap.Set(name, room)
	return
}

/* If room doesn't exist creates it then returns it */
func (hub *TheaterHub) GetRoom(name string) (*TheaterRoom, error) {
	if !hub.cmap.Has(name) {
		return nil, errors.New("room not found")
	}
	if r, ok := hub.cmap.Get(name); ok {
		return r.(*TheaterRoom), nil
	}
	return nil, errors.New("room is missing from cmp")
}

func (hub *TheaterHub) RemoveRoom(name string) {
	hub.cmap.Remove(name)
	return
}

func (hub *TheaterHub) Close() error {
	return nil
}

/* Get ws conn. and hands it over to correct room */
func (hub *TheaterHub) Handler(w http.ResponseWriter, req *http.Request) {

	hub.WithContext(req.Context())

	// Upgrade connection to websocket
	conn, _, _, err := ws.UpgradeHTTP(req, w)
	if err != nil {
		sentry.CaptureException(err)
		log.Println("upgrade:", err)
		return
	}

	// Create a new client for user
	client := NewTheaterClient(hub, conn)

	log.Printf("[%d] New client connected", client.Id)

	// Close connection after client disconnected
	defer client.Close()

	// Join user room if client recieved authorized
	client.OnAuthorized(func(auth Auth) (room Room) {
		event := auth.Event().(*proto.TheaterLogOnEvent)
		room = hub.GetOrCreateRoom(string(event.Room))
		room.Join(client)
		return
	})

	// Send Unauthorized message to client if client Unauthorized and close connection
	client.OnUnauthorized(func() {
		buffer, err := protocol.NewMsgProtobuf(proto.EMSG_UNAUTHORIZED, nil)
		if err == nil {
			_ = client.WriteMessage(buffer.Bytes())
		}
	})

	// Listen on client events
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
