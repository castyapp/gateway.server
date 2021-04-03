package hub

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/castyapp/libcasty-protocol-go/protocol"

	"github.com/castyapp/gateway.server/grpc"
	"github.com/castyapp/libcasty-protocol-go/proto"
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
	onGuess       func(auth Auth) Room
	onAuthFailed  func()
	onLeaveRoom   func(room Room)
	auth          Auth
	room          Room
	roomType      RoomType
	pingChan      chan struct{}
}

type ClientWithRoom struct {
	Id   string
	Room string
}

// Get client authenticated user
func (c *Client) IsGuest() bool {
	return c.auth.guest
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

// Set a callback when client authorized
func (c *Client) OnGuest(callback func(auth Auth) Room) {
	c.onGuess = callback
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
	for {
		select {
		case <-c.ctx.Done():
			log.Printf("[%s] Ping Pong Handler Err: %v", c.Id, c.ctx.Err())
			_ = c.Close()
			return
		case <-c.pingChan:
			if buffer, err := protocol.NewMsgProtobuf(proto.EMSG_PONG, nil); err == nil {
				_ = c.WriteMessage(buffer.Bytes())
			}
		}
	}
}

// Listen of client events
func (c *Client) Listen() {

	defer func() {
		log.Printf("[%s] End of events!", c.Id)
		close(c.Event)
	}()

	go c.PingPongHandler()

	for {
		select {
		case <-c.ctx.Done():
			// end of connection
			return
		default:

			data, err := wsutil.ReadClientBinary(c.conn)
			if err != nil {
				return
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
					if err := c.authenticate(packet); err != nil {
						log.Println(err)
						c.ctxCancel()
						_ = c.Close()
						return
					}
				}

				c.Event <- packet
			}
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

func (c *Client) send(eMsg proto.EMSG, body pb.Message) (err error) {
	return protocol.BrodcastMsgProtobuf(c.conn, eMsg, body)
}

// Authenticate client with LogOn event
func (c *Client) authenticate(packet *protocol.Packet) error {

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
	if token == nil {

		c.auth = Auth{
			user:          new(proto.User),
			authenticated: false,
			guest:         true,
			token:         nil,
			event:         event,
			err:           nil,
		}

		c.room = c.onAuthSuccess(c.auth)
		if c.room == nil {
			return errors.New("could not find theater room")
		}

		go c.room.HandleEvents(c)

		return nil
	}

	if !c.IsAuthenticated() {

		mCtx, _ := context.WithTimeout(context.Background(), 10*time.Second)
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
			if c.room == nil {
				return errors.New("could not find theater room")
			}
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
func NewTheaterClient(ctx context.Context, conn net.Conn) (client *Client) {
	return NewClient(ctx, conn, TheaterRoomType)
}

// Create a new user client
func NewUserClient(ctx context.Context, conn net.Conn) (client *Client) {
	return NewClient(ctx, conn, UserRoomType)
}

// Create a new client
func NewClient(ctx context.Context, conn net.Conn, rType RoomType) *Client {
	mCtx, cancelFunc := context.WithCancel(ctx)
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
	client.onAuthFailed = func() {
		buffer, err := protocol.NewMsgProtobuf(proto.EMSG_UNAUTHORIZED, nil)
		if err == nil {
			_ = client.WriteMessage(buffer.Bytes())
		}
	}
	return client
}
