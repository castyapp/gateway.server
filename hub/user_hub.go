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

/* Controls a bunch of rooms */
type UserHub struct {
	clients  cmap.ConcurrentMap
	upgrader websocket.Upgrader
}

func SendEventToUser(ctx context.Context, event []byte, user *proto.User)  {
	redis.Client.Publish(ctx, fmt.Sprintf("user:events:%s", user.Id), event)
}

func (hub *UserHub) cleanUpClients() {
	hub.clients.IterCb(func(key string, c interface{}) {
		if client, ok := c.(ClientWithRoom); ok {
			redis.Client.SRem(context.Background(), fmt.Sprintf("user:clients:%s", client.Room), client.Id)
		}
	})
	log.Println("Removed all clients from UserRooms!")
}

func (hub *UserHub) addClientToRoom(client *Client) {
	hub.clients.SetIfAbsent(client.Id, ClientWithRoom{
		Id:   client.Id,
		Room: client.room.GetName(),
	})
	ctx := context.Background()
	key := fmt.Sprintf("user:clients:%s", client.room.GetName())
	if exists := redis.Client.SIsMember(ctx, key, client.Id); !exists.Val() {
		redis.Client.SAdd(ctx, key, client.Id)
	}
}

func (hub *UserHub) removeClientFromRoom(client *Client) {
	key := fmt.Sprintf("user:clients:%s", client.room.GetName())
	if err := redis.Client.SRem(context.Background(), key, client.Id).Err(); err == nil {
		hub.clients.Remove(client.Id)
	}
}

// Close user hub
func (hub *UserHub) Close() error {
	hub.cleanUpClients()
	return nil
}

/* Get ws conn. and hands it over to correct room */
func (hub *UserHub) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	// Upgrade connection to websocket
	conn, _, _, err := ws.UpgradeHTTP(req, w)
	if err != nil {
		return
	}

	// Create a new client for user
	client := NewUserClient(req.Context(), conn)

	log.Printf("[%s] New client connected", client.Id)

	// Close connection after client disconnected
	defer client.Close()

	// Join user room if client received authorized
	client.OnAuthorized(func(auth Auth) (room Room) {
		room = NewUserRoom(hub, auth.User().Id)
		room.Join(client)
		return
	})

	// Listen on client events
	client.Listen()
}

// Create a new userhub
func NewUserHub() *UserHub {
	return &UserHub{
		clients:  cmap.New(),
		upgrader: newUpgrader(),
	}
}
