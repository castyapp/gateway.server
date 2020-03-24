package hub

import (
	"context"
	"fmt"
	"github.com/CastyLab/gateway.server/grpc"
	"github.com/CastyLab/gateway.server/hub/protocol"
	"github.com/CastyLab/gateway.server/hub/protocol/protobuf"
	"github.com/CastyLab/gateway.server/hub/protocol/protobuf/enums"
	"github.com/CastyLab/grpc.proto/proto"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	pb "github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"log"
	"net"
	"time"
)

type RoomType int

const (
	UserRoomType       RoomType = 0
	TheaterRoomType    RoomType = 1
)

type Room interface {
	Join(client *Client)
	HandleEvents(client *Client) error
	Leave(client *Client)
}

type Auth struct {
	err           error
	authenticated bool
	token         []byte
	event         pb.Message
	user          *proto.User
}

type Client struct {
	Id             uint32
	conn           net.Conn
	Event          chan *protocol.Packet
	ctx            context.Context
	ctxCancel      context.CancelFunc
	onAuthSuccess  func(e pb.Message, u *proto.User) Room
	onAuthFailed   func()
	onLeaveRoom    func(room Room)
	auth           Auth
	room           Room
	roomType       RoomType
	pingChan       chan struct{}
	lastPingAt     time.Time
}

func (c *Client) GetUser() *proto.User {
	return c.auth.user
}

func (c *Client) OnAuthorized(callback func(e pb.Message, u *proto.User) Room) {
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

func (c *Client) Close() error {
	// call registered on leave function
	c.onLeaveRoom(c.room)
	return errors.New(fmt.Sprintf("Client [%d] disconnected!", c.Id))
}

func (c *Client) Listen() error {

	for {
		select {
		case <-c.ctx.Done():
			return c.Close()
		default:

			data, err := wsutil.ReadClientBinary(c.conn)
			if err != nil {
				c.ctxCancel()
				return c.Close()
			}

			if data != nil {
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
				case enums.EMSG_PING:
					c.lastPingAt = time.Now()
					if buffer, err := protobuf.NewMsgProtobuf(enums.EMSG_PONG, nil); err == nil {
						if err := c.WriteMessage(buffer.Bytes()); err != nil {
							return err
						}
					}
				case enums.EMSG_LOGON:
					if !c.IsAuthenticated() {
						var logOnEvent pb.Message
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

				c.Event <- packet
			}
		}
	}

}

func getTokenFromLogOnEvent(event pb.Message) []byte {
	switch event.(type) {
	case *protobuf.TheaterLogOnEvent:
		return event.(*protobuf.TheaterLogOnEvent).Token
	case *protobuf.LogOnEvent:
		return event.(*protobuf.LogOnEvent).Token
	}
	return nil
}

func (c *Client) Authentication(token []byte, event pb.Message) {
	if !c.IsAuthenticated() {
		response, err := grpc.UserServiceClient.GetUser(c.ctx, &proto.AuthenticateRequest{
			Token: token,
		})
		if err != nil {
			c.auth = Auth{err: err}
			c.onAuthFailed()
			_ = c.conn.Close()
			c.ctxCancel()
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
	err = wsutil.WriteServerMessage(c.conn, ws.OpBinary, msg)
	return
}

func NewClient(ctx context.Context, conn net.Conn, rType RoomType) (client *Client) {
	mCtx, cancelFunc := context.WithCancel(ctx)
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
		if room != nil {
			room.Leave(client)
		}
	}
	return
}
