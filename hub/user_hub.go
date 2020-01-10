package hub

import (
	"github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/orcaman/concurrent-map"
	"gitlab.com/movienight1/grpc.proto/messages"
	"log"
	"movie.night.ws.server/hub/protocol/protobuf"
	"movie.night.ws.server/hub/protocol/protobuf/enums"
	"net/http"
)

/* Controls a bunch of rooms */
type UserHub struct {
	upgrader  websocket.Upgrader
	cmap.ConcurrentMap
}

/* If room doesn't exist creates it then returns it */
func (h *UserHub) GetOrCreateRoom(name string) (room *UserRoom) {
	if !h.Has(name) {
		h.Set(name, NewUserRoom(name))
	}
	r, _ := h.Get(name)
	room = r.(*UserRoom)
	room.hub = h
	return
}

func (h *UserHub) RemoveRoom(name string) {
	h.Remove(name)
	return
}

/* Get ws conn. and hands it over to correct room */
func (h *UserHub) Handler(w http.ResponseWriter, req *http.Request) {

	conn, err := h.upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}

	client := NewClient(conn)

	client.OnAuthorized(func(e proto.Message, u *messages.User) Room {
		event := e.(*protobuf.LogOnEvent)
		room := h.GetOrCreateRoom(u.Id)
		room.AuthToken = string(event.Token)
		room.Join(client)
		return room
	})

	client.OnAuthorizedFailed(func() {

		buffer, err := protobuf.NewMsgProtobuf(enums.EMSG_UNAUTHORIZED, nil)
		if err != nil {
			log.Println(err)
		}

		if err := client.WriteMessage(buffer.Bytes()); err != nil {
			log.Printf("Authentication failed [%d]. disconnected!", client.Id)
			_ = client.conn.Close()
			return
		}

		log.Printf("Authentication failed [%d]. sent `EMSG_UNAUTHORIZED` request to client!", client.Id)
	})

	client.ReadLoop()

	/* If ReadLoop breaks then client disconnected. */
	client.OnLeave(func(room Room) {
		if client.State != DisconnectedState {
			if room == nil {
				log.Println("Could not find room.")
				return
			}
			room.Leave(client.Id)
			log.Printf("Client [%d] disconnected!", client.Id)
		}
	})
}

/* Constructor */
func NewUserHub() *UserHub {
	hub := new(UserHub)
	hub.ConcurrentMap = cmap.New()
	hub.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	return hub
}