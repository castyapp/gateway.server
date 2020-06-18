package hub

import (
	"context"
	"errors"
	"github.com/CastyLab/gateway.server/grpc"
	"github.com/CastyLab/grpc.proto/proto"
	"github.com/getsentry/sentry-go"
	"github.com/gobwas/ws"
	"github.com/gorilla/websocket"
	cmap "github.com/orcaman/concurrent-map"
	"log"
	"net/http"
	"time"
)

/* Controls a bunch of rooms */
type UserHub struct {
	upgrader websocket.Upgrader
	cmap.ConcurrentMap
}

// find user's room
func (hub *UserHub) FindRoom(name string) (room Room, err error) {
	if r, ok := hub.Get(name); ok {
		return r.(*UserRoom), nil
	}
	return nil, errors.New("user room is missing from cmp")
}

// Create or get user's room
func (hub *UserHub) GetOrCreateRoom(name string) (room Room) {
	if r, ok := hub.Get(name); ok {
		return r.(*UserRoom)
	}
	room = NewUserRoom(name, hub)
	hub.SetIfAbsent(name, room)
	return
}

// remove user's room from concurrent map
func (hub *UserHub) RemoveRoom(name string) {
	hub.Remove(name)
	return
}

func (hub *UserHub) RollbackUsersStatesToOffline() {

	log.Println("\r- Rollback all online users to OFFLINE state!")

	mCtx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
	response, err := grpc.UserServiceClient.RollbackStates(mCtx, &proto.RollbackStatesRequest{})
	if err != nil {
		sentry.CaptureException(err)
		log.Println(err)
	}
	if response.Code == http.StatusOK {
		log.Println("\r- Rolled back online users state to Offline successfully!")
	}
}

// Close user hub
func (hub *UserHub) Close() error {

	// roll back all users back to OFFLINE state if user hub closed
	hub.RollbackUsersStatesToOffline()

	return nil
}

/* Get ws conn. and hands it over to correct room */
func (hub *UserHub) Handler(w http.ResponseWriter, req *http.Request) {

	// Upgrade connection to websocket
	conn, _, _, err := ws.UpgradeHTTP(req, w)
	if err != nil {
		sentry.CaptureException(err)
		return
	}

	// Create a new client for user
	client := NewUserClient(req.Context(), conn)

	log.Printf("[%s] New client connected", client.Id)

	// Close connection after client disconnected
	defer client.Close()

	// Join user room if client received authorized
	client.OnAuthorized(func(auth Auth) (room Room) {
		room = hub.GetOrCreateRoom(auth.User().Id)
		room.Join(client)
		return
	})

	// Listen on client events
	client.Listen()
}

// Create a new userhub
func NewUserHub() *UserHub {
	return &UserHub{
		ConcurrentMap: cmap.New(),
		upgrader:      newUpgrader(),
	}
}
