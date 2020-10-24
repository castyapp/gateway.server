package hub

import (
	"github.com/CastyLab/grpc.proto/proto"
	"github.com/getsentry/sentry-go"
	"github.com/gobwas/ws"
	"github.com/gorilla/websocket"
	cmap "github.com/orcaman/concurrent-map"
	"log"
	"net/http"
)

// TheaterHub holds theater rooms
type TheaterHub struct {
	upgrader     websocket.Upgrader
	VideoPlayers cmap.ConcurrentMap
	cmap.ConcurrentMap
}

/* If room doesn't exist creates it then returns it */
func (hub *TheaterHub) GetOrCreateRoom(theater *proto.Theater) (room Room) {
	if r, ok := hub.Get(theater.Id); ok {
		return r.(*TheaterRoom)
	}
	hub.Set(theater.Id, NewTheaterRoom(theater))
	return
}

func (hub *TheaterHub) Close() error {
	return nil
}

/* Get ws conn. and hands it over to correct room */
func (hub *TheaterHub) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	// Upgrade connection to websocket
	conn, _, _, err := ws.UpgradeHTTP(req, w)
	if err != nil {
		sentry.CaptureException(err)
		return
	}

	// Create a new client for user
	client := NewTheaterClient(req.Context(), conn)

	log.Printf("[%s] New client connected", client.Id)

	// Close connection after client disconnected
	defer client.Close()

	// Join user room if client recieved authorized
	client.OnAuthorized(func(auth Auth) (room Room) {

		var (
			err     error
			theater = new(proto.Theater)
			event   = auth.Event().(*proto.TheaterLogOnEvent)
		)

		// getting theater from grpc service
		theater, err = GetTheater(event.Room, auth.token)

		if err != nil {
			client.ctxCancel()
			_ = client.Close()
			return
		}

		room = hub.GetOrCreateRoom(theater)
		room.Join(client)
		return
	})

	// Listen on client events
	client.Listen()
	return
}

/* Constructor */
func NewTheaterHub() *TheaterHub {
	return &TheaterHub{
		upgrader:       newUpgrader(),
		VideoPlayers:   cmap.New(),
		ConcurrentMap:  cmap.New(),
	}
}
