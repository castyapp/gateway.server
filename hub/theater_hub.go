package hub

import (
	"errors"
	"github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/orcaman/concurrent-map"
	"gitlab.com/movienight1/grpc.proto/messages"
	"log"
	"movie.night.ws.server/hub/protocol/protobuf"
	"net/http"
)

/* Controls a bunch of rooms */
type TheaterHub struct {
	upgrader  websocket.Upgrader
	userHub   *UserHub
	cmap cmap.ConcurrentMap
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

	conn, err := h.upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}

	client := NewClient(conn)

	client.OnAuthorized(func(e proto.Message, u *messages.User) Room {

		event := e.(*protobuf.TheaterLogOnEvent)

		room , err := h.GetOrCreateRoom(string(event.Room))
		if err != nil {
			_ = client.conn.Close()
			log.Println("Error while creating or getting the room from cmp: ", err)
			return nil
		}

		client.AuthToken = string(event.Token)

		room.Join(client)

		return room
	})

	client.OnUnauthorized(func() {
		_ = client.conn.Close()
		log.Printf("Authentication failed [%d]. disconnected!", client.Id)
	})

	/* If Listen breaks then client disconnected. */
	client.OnLeave(func(room Room) {
		if client.State != DisconnectedState {
			if room == nil {
				return
			}
			room.Leave(client)
		}
	})

	client.Listen()
}

/* Constructor */
func NewTheaterHub(uhub *UserHub) *TheaterHub {
	hub := new(TheaterHub)
	hub.userHub = uhub
	hub.cmap = cmap.New()
	hub.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	return hub
}