package hub

import (
	"context"
	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	proto2 "gitlab.com/movienight1/grpc.proto"
	"gitlab.com/movienight1/grpc.proto/messages"
	"log"
	"movie.night.ws.server/grpc"
	"movie.night.ws.server/hub/protocol"
	"movie.night.ws.server/hub/protocol/protobuf"
	"movie.night.ws.server/hub/protocol/protobuf/enums"
	"time"
)

type Room interface {
	Join(client *Client)
	HandleEvents(client *Client)
	Leave(client *Client)
}

type State uint

const (
	Initialized       State = 0
	DisconnectedState State = 1
	ConnectedState    State = 2
	JoinedRoomState   State = 3
)

type Client struct {
	Id              uint32
	token           []byte
	conn            *websocket.Conn
	Event           chan *protocol.Packet
	user            *messages.User
	onAuthSuccess   func(e proto.Message, u *messages.User) Room
	onAuthFailed    func()
	onLeaveRoom     func(room Room)
	authenticated   bool
	room            Room
	State           State
	AuthToken       string
}

func (c *Client) GetUser() *messages.User {
	return c.user
}

func (c *Client) OnAuthorized(callback func(e proto.Message, u *messages.User) Room) {
	c.onAuthSuccess = callback
}

func (c *Client) OnUnauthorized(cb func()) {
	c.onAuthFailed = cb
}

func (c *Client) IsAuthenticated() bool {
	if c == nil {
		return false
	}
	return c.authenticated
}

func (c *Client) OnLeave(cb func(room Room))  {
	c.onLeaveRoom = cb
}

func (c *Client) Listen() {

	defer func() {
		// call registered on leave function
		c.onLeaveRoom(c.room)

		// set client as disconnected
		c.State = DisconnectedState

		// closing event channel
		close(c.Event)

		// close websocket connection
		_ = c.conn.Close()
	}()

	for {

		mType, data, err := c.conn.ReadMessage()
		if err != nil {
			break
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

		c.State = ConnectedState

		switch packet.EMsg {
		case enums.EMSG_LOGON:
			if !c.IsAuthenticated() {
				logOnEvent := new(protobuf.LogOnEvent)
				if err := packet.ReadProtoMsg(logOnEvent); err != nil {
					log.Println(err)
					break
				}
				if err := c.Authentication(logOnEvent.Token, logOnEvent); err != nil {
					log.Println(err)
					break
				}
			}
		case enums.EMSG_THEATER_LOGON:
			if !c.IsAuthenticated() {
				logOnEvent := new(protobuf.TheaterLogOnEvent)
				if err := packet.ReadProtoMsg(logOnEvent); err != nil {
					log.Println(err)
					break
				}
				if err := c.Authentication(logOnEvent.Token, logOnEvent); err != nil {
					log.Println(err)
					break
				}
			}
		}

		c.Event <- packet

	}
}

func (c *Client) Authentication(token []byte, event proto.Message) error {

	if !c.IsAuthenticated() {

		mCtx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
		response, err := grpc.UserServiceClient.GetUser(mCtx, &proto2.AuthenticateRequest{
			Token: token,
		})
		if err != nil {
			c.onAuthFailed()
			c.authenticated  = false
			return err
		}

		user := response.Result
		c.user = user

		if room := c.onAuthSuccess(event, user); room != nil {
			c.room = room
			c.authenticated = true
			c.token = token
			go room.HandleEvents(c)
		}

	}

	return nil
}

func (c *Client) WriteMessage(msg []byte) error {
	err := c.conn.WriteMessage(websocket.BinaryMessage, msg)
	if err != nil {
		return err
	}
	return nil
}

func NewClient(conn *websocket.Conn) *Client {
	return &Client{
		Id:            uuid.New().ID(),
		conn:          conn,
		Event:         make(chan *protocol.Packet),
		authenticated: false,
		State:         Initialized,
	}
}