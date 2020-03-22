package hub

import (
	"context"
	"github.com/CastyLab/gateway.server/grpc"
	"github.com/CastyLab/gateway.server/hub/protocol"
	"github.com/CastyLab/gateway.server/hub/protocol/protobuf"
	"github.com/CastyLab/gateway.server/hub/protocol/protobuf/enums"
	gRPCproto "github.com/CastyLab/grpc.proto"
	"github.com/CastyLab/grpc.proto/messages"
	"github.com/getsentry/sentry-go"
	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"log"
	"time"
)

type RoomType int

const (
	UserRoomType       RoomType = 0
	TheaterRoomType    RoomType = 1
)

type Room interface {
	Join(client *Client)
	HandleEvents(client *Client)
	Leave(client *Client)
}

type Auth struct {
	err           error
	authenticated bool
	token         []byte
	event         proto.Message
	user          *messages.User
}

type Client struct {
	Id             uint32
	conn           *websocket.Conn
	Event          chan *protocol.Packet
	ctx            context.Context
	ctxCancel      context.CancelFunc
	onAuthSuccess  func(e proto.Message, u *messages.User) Room
	onAuthFailed   func()
	onLeaveRoom    func(room Room)
	auth           Auth
	room           Room
	roomType       RoomType
	closed         bool
	lastPingAt     time.Time
}

func (c *Client) GetUser() *messages.User {
	return c.auth.user
}

func (c *Client) OnAuthorized(callback func(e proto.Message, u *messages.User) Room) {
	c.onAuthSuccess = callback
}

func (c *Client) OnUnauthorized(cb func()) {
	c.onAuthFailed = cb
}

func (c *Client) IsAuthenticated() bool {
	return c.auth.authenticated
}

func (c *Client) OnLeave(cb func(room Room)) {
	c.onLeaveRoom = cb
}

func (c *Client) close() {
	// call registered on leave function
	c.onLeaveRoom(c.room)

	// closing event channel
	close(c.Event)

	// close websocket connection
	_ = c.conn.Close()
	c.closed = true

	log.Printf("Client [%d] disconnected!", c.Id)
}

func (c *Client) Listen() {

	defer c.close()

	for {
		select {
		case <-c.ctx.Done():
			log.Println("Listen Err: ", c.ctx.Err())
			return
		default:

			if c.closed {
				c.ctxCancel()
				_ = c.conn.Close()
				return
			}

			mType, data, err := c.conn.ReadMessage()
			if err != nil {
				sentry.CaptureException(err)
				log.Println(err)
				continue
			}

			if mType != websocket.BinaryMessage {
				log.Println("Websocket message should be BinaryMessage")
				continue
			}

			packet, err := protocol.NewPacket(data)
			if err != nil {
				log.Println("Error while creating new packet: ", err)
				continue
			}

			if !packet.IsProto {
				log.Println("Packet type should be Protobuf")
				continue
			}

			switch packet.EMsg {
			case enums.EMSG_LOGON:
				if !c.IsAuthenticated() {
					var logOnEvent proto.Message
					switch c.roomType {
					case UserRoomType:
						logOnEvent = new(protobuf.LogOnEvent)
					case TheaterRoomType:
						logOnEvent = new(protobuf.TheaterLogOnEvent)
					}
					if err := packet.ReadProtoMsg(logOnEvent); err != nil {
						log.Println(err)
						break
					}
					c.Authentication(getTokenFromLogOnEvent(logOnEvent), logOnEvent)
				}
			}
			if !c.closed {
				c.Event <- packet
			}
		}
	}

}

func getTokenFromLogOnEvent(event proto.Message) []byte {
	switch event.(type) {
	case *protobuf.TheaterLogOnEvent:
		return event.(*protobuf.TheaterLogOnEvent).Token
	case *protobuf.LogOnEvent:
		return event.(*protobuf.LogOnEvent).Token
	}
	return nil
}

func (c *Client) Authentication(token []byte, event proto.Message) {
	if !c.IsAuthenticated() {
		response, err := grpc.UserServiceClient.GetUser(c.ctx, &gRPCproto.AuthenticateRequest{
			Token: token,
		})
		if err != nil {
			c.auth = Auth{err: err}
			c.onAuthFailed()
			return
		} else {
			c.auth = Auth{
				user:          response.Result,
				authenticated: true,
				event:         event,
				token:         token,
				err:           nil,
			}
			c.room = c.onAuthSuccess(c.auth.event, c.auth.user)
			go c.room.HandleEvents(c)
		}
	}
}

func (c *Client) WriteMessage(msg []byte) (err error) {
	err = c.conn.WriteMessage(websocket.BinaryMessage, msg)
	return
}

func NewClient(ctx context.Context, conn *websocket.Conn, rType RoomType) *Client {
	mCtx, cancelFunc := context.WithCancel(ctx)
	return &Client{
		Id:        uuid.New().ID(),
		conn:      conn,
		ctx:       mCtx,
		ctxCancel: cancelFunc,
		Event:     make(chan *protocol.Packet),
		auth:      Auth{},
		roomType:  rType,
		closed:    false,
	}
}
