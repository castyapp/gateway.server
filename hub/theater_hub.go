package hub

import (
	"context"
	"fmt"
	"github.com/CastyLab/gateway.server/redis"
	"github.com/CastyLab/grpc.proto/proto"
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
	clients      cmap.ConcurrentMap
}

func (hub *TheaterHub) cleanUpClients() {
	hub.clients.IterCb(func(key string, c interface{}) {
		if client, ok := c.(ClientWithRoom); ok {
			redis.Client.SRem(context.Background(), fmt.Sprintf("theater:clients:%s", client.Room), client.Id)
		}
	})
	log.Println("Removed all clients from TheaterRooms!")
}

func (hub *TheaterHub) addClientToRoom(client *Client) {
	hub.clients.SetIfAbsent(client.Id, ClientWithRoom{
		Id:   client.Id,
		Room: client.room.GetName(),
	})
	ctx := context.Background()
	clientsKey := fmt.Sprintf("theater:clients:%s", client.room.GetName())
	exists := redis.Client.SIsMember(ctx, clientsKey, client.Id)
	if !exists.Val() {
		redis.Client.SAdd(ctx, clientsKey, client.Id)
	}
}

func (hub *TheaterHub) removeClientFromRoom(client *Client) {
	key := fmt.Sprintf("theater:clients:%s", client.room.GetName())
	if err := redis.Client.SRem(context.Background(), key, client.Id).Err(); err == nil {
		hub.clients.Remove(client.Id)
	}
}

func (hub *TheaterHub) Close() error {
	hub.cleanUpClients()
	return nil
}

/* Get ws conn. and hands it over to correct room */
func (hub *TheaterHub) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	// Upgrade connection to websocket
	conn, _, _, err := ws.UpgradeHTTP(req, w)
	if err != nil {
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

		room = NewTheaterRoom(hub, theater)
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
		upgrader:     newUpgrader(),
		VideoPlayers: cmap.New(),
		clients:      cmap.New(),
	}
}
