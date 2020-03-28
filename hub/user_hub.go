package hub

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/CastyLab/gateway.server/grpc"
	"github.com/CastyLab/gateway.server/hub/protocol/protobuf"
	"github.com/CastyLab/gateway.server/hub/protocol/protobuf/enums"
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

func (h *UserHub) GetContext() context.Context {
	return h.ctx
}

func (h *UserHub) FindRoom(name string) (room Room, err error) {
	if r, ok := h.cmap.Get(name); ok {
		return r.(*UserRoom), nil
	}
	return nil, errors.New("user room is missing from cmp")
}

/* If room doesn't exist creates it then returns it */
func (h *UserHub) GetOrCreateRoom(name string) (room Room) {
	if r, ok := h.cmap.Get(name); ok {
		return r.(*UserRoom)
	}
	return NewUserRoom(name, h)
}

func (h *UserHub) RemoveRoom(name string) {
	h.cmap.Remove(name)
	return
}

func (h *UserHub) RollbackUsersStatesToOffline() {
	log.Println("\r- Rollback all online users to OFFLINE state!")
	usersIds := make([]string, 0)
	for uId := range h.cmap.Items() {
		usersIds = append(usersIds, uId)
	}
	if len(usersIds) > 0 {
		response, err := grpc.UserServiceClient.RollbackStates(h.ctx, &proto.RollbackStatesRequest{
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

func (h *UserHub) Close() error {
	h.RollbackUsersStatesToOffline()
	return nil
}

/* Get ws conn. and hands it over to correct room */
func (h *UserHub) Handler(w http.ResponseWriter, req *http.Request) {

	h.ctx = req.Context()
	conn, _, _, err := ws.UpgradeHTTP(req, w)
	if err != nil {
		sentry.CaptureException(err)
		log.Println("upgrade:", err)
		return
	}

	client := NewUserClient(h, conn)
	defer client.Close()

	client.OnAuthorized(func(auth Auth) (room Room) {
		event := auth.event.(*protobuf.LogOnEvent)
		room = h.GetOrCreateRoom(auth.user.Id)
		room.SetAuthToken(string(event.Token))
		room.Join(client)
		return
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
func NewUserHub() *UserHub {
	return &UserHub{
		cmap:     cmap.New(),
		upgrader: newUpgrader(),
	}
}
