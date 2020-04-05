package hub

import (
	"context"
	"fmt"
	"github.com/CastyLab/grpc.proto/protocol"
	cmap "github.com/orcaman/concurrent-map"
	"log"
	"net"
	"strconv"
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
	Id            string
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
	return errors.New(fmt.Sprintf("Client [%s] disconnected!", c.Id))
}

// Handle PingPong
func (c *Client) PingPongHandler() {
	pTicker := time.NewTicker(time.Second)
	for {
		select {
		case <-c.ctx.Done():
			log.Printf("[%s] PingPongHandler Err: %v", c.Id, c.ctx.Err())
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
		log.Printf("[%s] End of events!", c.Id)
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
				log.Printf("[%s] Error while creating new packet: %v", c.Id, err)
				continue
			}

			if !packet.IsProto {
				log.Printf("[%s] Packet type should be Protobuf", c.Id)
				continue
			}

			switch packet.EMsg {
			case proto.EMSG_PING:
				c.pingChan <- struct{}{}
			case proto.EMSG_LOGON:
				log.Printf("[%s] Authorizing...", c.Id)
				if !c.IsAuthenticated() {
					if err := c.Authentication(packet); err != nil {
						c.ctxCancel()
						return c.Close()
					}
					log.Printf("[%s] Authorized!", c.Id)
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
func NewClient(hub Hub, conn net.Conn, rType RoomType) *Client {
	mCtx, cancelFunc := context.WithCancel(hub.GetContext())
	clientID := strconv.Itoa(int(uuid.New().ID()))
	client := &Client{
		Id:        clientID,
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
	return client
}

type MemberWithClients struct {
	Clients  cmap.ConcurrentMap
	User     *proto.User
}

// Create a new user client with its clients
func NewMemberWithClients(user *proto.User) *MemberWithClients {
	return &MemberWithClients{
		Clients: cmap.New(),
		User: user,
	}
}

func (uwc *MemberWithClients) HasClients() bool {
	return len(uwc.Clients) > 0
}

func (uwc *MemberWithClients) GetClient(id string) *Client {
	client, _ := uwc.Clients.Get(id)
	return client.(*Client)
}

func (uwc *MemberWithClients) GetClients() cmap.ConcurrentMap {
	return uwc.Clients
}

func (uwc *MemberWithClients) AddClient(client *Client) {
	uwc.Clients.Set(client.Id, client)
}

func (uwc *MemberWithClients) RemoveClient(client *Client) {
	uwc.Clients.Remove(client.Id)
}