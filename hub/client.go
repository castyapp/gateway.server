package hub

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

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
)

type Auth struct {
	err           error
	authenticated bool
	token         []byte
	event         pb.Message
	user          *proto.User
}

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

func (c *Client) GetUser() *proto.User {
	return c.auth.user
}

func (c *Client) OnAuthorized(callback func(auth Auth) Room) {
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

	if c.IsAuthenticated() {
		if r := c.room; r != nil {
			c.onLeaveRoom(r)
		}
	}

	_ = c.conn.Close()

	return errors.New(fmt.Sprintf("Client [%d] disconnected!", c.Id))
}

func (c *Client) PingPongHandler() error {

	pTicker := time.NewTicker(time.Second)

	for {
		select {
		case <-c.ctx.Done():
			log.Printf("PingPongHandler Err: %v", c.ctx.Err())
			return c.Close()
		case <-pTicker.C:
			diff := time.Now().Sub(c.lastPingAt)
			if diff.Round(time.Second) > time.Minute {
				c.ctxCancel()
				return c.Close()
			}
		case <-c.pingChan:
			c.lastPingAt = time.Now()
			if buffer, err := protobuf.NewMsgProtobuf(enums.EMSG_PONG, nil); err == nil {
				_ = c.WriteMessage(buffer.Bytes())
			}
		}
	}
}

func (c *Client) Listen() error {

	go func() {
		log.Println(c.PingPongHandler())
	}()

	c.pingChan <- struct{}{}

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
					c.pingChan <- struct{}{}
				case enums.EMSG_LOGON:
					if !c.IsAuthenticated() {
						if err := c.Authentication(packet); err != nil {
							log.Println(err)
							break
						}
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

func (c *Client) Authentication(packet *protocol.Packet) error {

	var event pb.Message
	switch c.roomType {
	case UserRoomType:
		event = new(protobuf.LogOnEvent)
	case TheaterRoomType:
		event = new(protobuf.TheaterLogOnEvent)
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

func (c *Client) WriteMessage(msg []byte) (err error) {
	err = wsutil.WriteServerMessage(c.conn, ws.OpBinary, msg)
	return
}

func NewTheaterClient(hub Hub, conn net.Conn) (client *Client) {
	return NewClient(hub, conn, TheaterRoomType)
}

func NewUserClient(hub Hub, conn net.Conn) (client *Client) {
	return NewClient(hub, conn, UserRoomType)
}

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
