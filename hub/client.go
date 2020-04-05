package hub

import (
	"context"
	"fmt"
	"github.com/CastyLab/grpc.proto/protocol"
	"log"
	"net"
	"time"

	"github.com/CastyLab/gateway.server/grpc"
	"github.com/CastyLab/grpc.proto/proto"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	pb "github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type Client struct {
	Id            uint32
	conn          net.Conn
	Event         chan *protocol.Packet
	ctx           context.Context
	ctxCancel     context.CancelFunc
	onAuthSuccess func(auth Auth) Room
	onAuthFailed  func()
	onLeaveRoom   func(room Room)
	auth          Auth
	room          Room
	roomType      RoomType
	pingChan      chan struct{}
	lastPingAt    time.Time
}

// Get client authenticated user
func (c *Client) GetUser() *proto.User {
	return c.auth.User()
}

// Get client authenticated user
func (c *Client) Token() []byte {
	return c.auth.Token()
}

// Set a callback when client authorized
func (c *Client) OnAuthorized(callback func(auth Auth) Room) {
	c.onAuthSuccess = callback
}

// Set a callback when client unauthorized
func (c *Client) OnUnauthorized(cb func()) {
	c.onAuthFailed = cb
}

// Check if client authorized
func (c *Client) IsAuthenticated() bool {
	return c.auth.authenticated
}

// Set a callback when client left
func (c *Client) OnLeave(cb func(room Room)) {
	c.onLeaveRoom = cb
}

// Close client connection
func (c *Client) Close() error {
	if c.room != nil {
		c.onLeaveRoom(c.room)
	}
	_ = c.conn.Close()
	return errors.New(fmt.Sprintf("Client [%d] disconnected!", c.Id))
}

// Handle PingPong
func (c *Client) PingPongHandler() {
	pTicker := time.NewTicker(time.Second)
	for {
		select {
		case <-c.ctx.Done():
			log.Printf("[%d] PingPongHandler Err: %v", c.Id, c.ctx.Err())
			c.Close()
			return
		case <-pTicker.C:
			diff := time.Now().Sub(c.lastPingAt)
			if diff.Round(time.Second) > time.Minute {
				c.ctxCancel()
				c.Close()
				return
			}
		case <-c.pingChan:
			c.lastPingAt = time.Now()
			if buffer, err := protocol.NewMsgProtobuf(proto.EMSG_PONG, nil); err == nil {
				_ = c.WriteMessage(buffer.Bytes())
			}
		}
	}
}

// Listen of client events
func (c *Client) Listen() error {

	go c.PingPongHandler()

	defer func() {
		log.Printf("[%d] End of events!", c.Id)
		close(c.Event)
	}()

	c.pingChan <- struct{}{}

	for {

		data, err := wsutil.ReadClientBinary(c.conn)
		if err != nil {
			c.ctxCancel()
			return c.Close()
		}

		if data != nil {
			packet, err := protocol.NewPacket(data)
			if err != nil {
				log.Printf("[%d] Error while creating new packet: %v", c.Id, err)
				continue
			}

			if !packet.IsProto {
				log.Printf("[%d] Packet type should be Protobuf", c.Id)
				continue
			}

			switch packet.EMsg {
			case proto.EMSG_PING:
				c.pingChan <- struct{}{}
			case proto.EMSG_LOGON:
				log.Printf("[%d] Authorizing...", c.Id)
				if !c.IsAuthenticated() {
					if err := c.Authentication(packet); err != nil {
						c.ctxCancel()
						return c.Close()
					}
					log.Printf("[%d] Authorized!", c.Id)
				}
			}

			c.Event <- packet
		}
	}

}

// Get authentication token from LogOn event for User and Theater rooms
func getTokenFromLogOnEvent(event pb.Message) []byte {
	switch event.(type) {
	case *proto.TheaterLogOnEvent:
		return event.(*proto.TheaterLogOnEvent).Token
	case *proto.LogOnEvent:
		return event.(*proto.LogOnEvent).Token
	}
	return nil
}

// Authenticate client with LogOn event
func (c *Client) Authentication(packet *protocol.Packet) error {

	var event pb.Message
	switch c.roomType {
	case UserRoomType:
		event = new(proto.LogOnEvent)
	case TheaterRoomType:
		event = new(proto.TheaterLogOnEvent)
	}
	if err := packet.ReadProtoMsg(event); err != nil {
		return err
	}

	token := getTokenFromLogOnEvent(event)

	if !c.IsAuthenticated() {

		mCtx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
		response, err := grpc.UserServiceClient.GetUser(mCtx, &proto.AuthenticateRequest{
			Token: token,
		})

		if err != nil {
			c.auth = Auth{err: err}
			return err
		} else {
			c.auth = Auth{
				user:          response.Result,
				authenticated: true,
				event:         event,
				token:         token,
				err:           nil,
			}
			c.room = c.onAuthSuccess(c.auth)
			go c.room.HandleEvents(c)
		}

	}
	return nil
}

// Write message to client
func (c *Client) WriteMessage(msg []byte) (err error) {
	err = wsutil.WriteServerMessage(c.conn, ws.OpBinary, msg)
	return
}

// Create a new theater client
func NewTheaterClient(hub Hub, conn net.Conn) (client *Client) {
	return NewClient(hub, conn, TheaterRoomType)
}

// Create a new user client
func NewUserClient(hub Hub, conn net.Conn) (client *Client) {
	return NewClient(hub, conn, UserRoomType)
}

// Create a new client
func NewClient(hub Hub, conn net.Conn, rType RoomType) (client *Client) {
	mCtx, cancelFunc := context.WithCancel(hub.GetContext())
	client = &Client{
		Id:        uuid.New().ID(),
		conn:      conn,
		ctx:       mCtx,
		ctxCancel: cancelFunc,
		Event:     make(chan *protocol.Packet),
		auth:      Auth{},
		roomType:  rType,
		pingChan:  make(chan struct{}),
	}
	client.onLeaveRoom = func(room Room) {
		room.Leave(client)
	}
	return
}

type UserWithClients struct {
	Clients  map[uint32] *Client
	User     *proto.User
}

// Create a new user client with its clients
func NewUserWithClients(user *proto.User) *UserWithClients {
	return &UserWithClients{
		Clients: make(map[uint32] *Client, 0),
		User: user,
	}
}

func (uwc *UserWithClients) HasClients() bool {
	return len(uwc.Clients) > 0
}

func (uwc *UserWithClients) GetClient(id uint32) *Client {
	return uwc.Clients[id]
}

func (uwc *UserWithClients) GetClients() map[uint32] *Client {
	return uwc.Clients
}

func (uwc *UserWithClients) AddClient(client *Client) {
	uwc.Clients[client.Id] = client
}

func (uwc *UserWithClients) RemoveClient(client *Client) {
	delete(uwc.Clients, client.Id)
}