package hub

import (
	"context"
	"errors"
	"github.com/CastyLab/grpc.proto/protocol"
	"log"
	"net/http"
	"time"

	"github.com/CastyLab/gateway.server/grpc"
	"github.com/CastyLab/grpc.proto/proto"
	"github.com/getsentry/sentry-go"
	"github.com/gobwas/ws"
	"github.com/gorilla/websocket"
	cmap "github.com/orcaman/concurrent-map"
)

/* Controls a bunch of rooms */
type UserHub struct {
	upgrader websocket.Upgrader
	cmap     cmap.ConcurrentMap
	ctx      context.Context
}

// set userhub context
func (hub *UserHub) WithContext(ctx context.Context) {
	hub.ctx = ctx
}

// get userhub context
func (hub *UserHub) GetContext() context.Context {
	return hub.ctx
}

// find user's room
func (hub *UserHub) FindRoom(name string) (room Room, err error) {
	if r, ok := hub.cmap.Get(name); ok {
		return r.(*UserRoom), nil
	}
	return nil, errors.New("user room is missing from cmp")
}

// Create or get user's room
func (hub *UserHub) GetOrCreateRoom(name string) (room Room) {
	if r, ok := hub.cmap.Get(name); ok {
		return r.(*UserRoom)
	}
	room = NewUserRoom(name, hub)
	hub.cmap.Set(name, room)
	return
}

// remove user's room from concurrent map
func (hub *UserHub) RemoveRoom(name string) {
	hub.cmap.Remove(name)
	return
}

func (hub *UserHub) RollbackUsersStatesToOffline() {

	log.Println("\r- Rollback all online users to OFFLINE state!")

	// Get user ids connected to server
	usersIds := make([]string, 0)
	for uId := range hub.cmap.Items() {
		usersIds = append(usersIds, uId)
	}

	if len(usersIds) > 0 {
		mCtx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
		response, err := grpc.UserServiceClient.RollbackStates(mCtx, &proto.RollbackStatesRequest{
			UsersIds: usersIds,
		})
		if err != nil {
			sentry.CaptureException(err)
			log.Println(err)
		}
		if response.Code == http.StatusOK {
			log.Println("\r- Rolled back online users state to Offline successfully!")
		}
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

	hub.WithContext(req.Context())

	// Upgrade connection to websocket
	conn, _, _, err := ws.UpgradeHTTP(req, w)
	if err != nil {
		sentry.CaptureException(err)
		log.Println("upgrade:", err)
		return
	}

	// Create a new client for user
	client := NewUserClient(hub, conn)

	log.Printf("[%d] New client connected", client.Id)

	// Close connection after client disconnected
	defer client.Close()

	// Join user room if client recieved authorized
	client.OnAuthorized(func(auth Auth) (room Room) {
		room = hub.GetOrCreateRoom(auth.User().Id)
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

// Create a new userhub
func NewUserHub() *UserHub {
	return &UserHub{
		cmap:     cmap.New(),
		upgrader: newUpgrader(),
	}
}
