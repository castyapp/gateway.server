package hub

import (
	"errors"
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
func (h *UserHub) FindRoom(name string) (room *UserRoom, err error) {
	if r, ok := h.Get(name); ok {
		return r.(*UserRoom), nil
	}
	return nil, errors.New("user room is missing from cmp")
}

/* If room doesn't exist creates it then returns it */
func (h *UserHub) GetOrCreateRoom(name string) (room *UserRoom, err error) {
	if !h.Has(name) {
		h.Set(name, NewUserRoom(name, h))
	}
	if r, ok := h.Get(name); ok {
		return r.(*UserRoom), nil
	}
	return nil, errors.New("user room is missing from cmp")
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
		room , err := h.GetOrCreateRoom(u.Id)
		if err != nil {
			_ = client.conn.Close()
			log.Println("Error while creating or getting the room from cmp: ", err)
			return nil
		}
		room.AuthToken = string(event.Token)
		room.Join(client)
		return room
	})

	client.OnUnauthorized(func() {

		buffer, err := protobuf.NewMsgProtobuf(enums.EMSG_UNAUTHORIZED, nil)
		if err != nil {
			log.Println(err)
		}

		if err := client.WriteMessage(buffer.Bytes()); err != nil {
			log.Printf("Authentication failed [%d]. disconnected!", client.Id)
		}

		_ = client.conn.Close()
		log.Printf("Authentication failed [%d]. sent `EMSG_UNAUTHORIZED` request to client!", client.Id)
	})

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