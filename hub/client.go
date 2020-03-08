package hub

import (
	"context"
	"github.com/CastyLab/gateway.server/grpc"
	"github.com/CastyLab/gateway.server/hub/protocol"
	"github.com/CastyLab/gateway.server/hub/protocol/protobuf"
	"github.com/CastyLab/gateway.server/hub/protocol/protobuf/enums"
	proto2 "github.com/CastyLab/grpc.proto"
	"github.com/CastyLab/grpc.proto/messages"
	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"log"
	"time"
)

type Room interface {
	Join(client *Client)
	HandleEvents(client *Client)
	Leave(client *Client)
}

type AuthChan struct {
	err             error
	authenticated   bool
	token           []byte
	event           proto.Message
	user            *messages.User
}

type Client struct {
	Id              uint32
	conn            *websocket.Conn
	Event           chan *protocol.Packet

	onAuthSuccess   func(e proto.Message, u *messages.User) Room
	onAuthFailed    func()
	onLeaveRoom     func(room Room)

	authChan        chan AuthChan
	auth            AuthChan
	room            Room
}

func (c *Client) GetUser() *messages.User {
	return c.auth.user
}

func (c *Client) OnAuthorized(callback func(e proto.Message, u *messages.User) Room) {
	go func() {
		for {
			if auth := <- c.authChan; auth.authenticated && auth.err == nil {
				c.auth = auth
				c.room = callback(auth.event, auth.user)
				break
			} else {
				if c.onAuthFailed != nil {
					c.onAuthFailed()
				} else {
					log.Printf("Authentication failed [%d]. disconnected!", c.Id)
				}
				_ = c.conn.Close()
				return
			}
		}
		c.room.HandleEvents(c)
	}()
}

func (c *Client) OnUnauthorized(cb func()) {
	c.onAuthFailed = cb
}

func (c *Client) IsAuthenticated() bool {
	if c == nil {
		return false
	}
	return c.auth.authenticated
}

func (c *Client) OnLeave(cb func(room Room))  {
	c.onLeaveRoom = cb
}

func (c *Client) Listen() {

	defer func() {
		// call registered on leave function
		c.onLeaveRoom(c.room)

		// closing event channel
		close(c.Event)

		// close websocket connection
		_ = c.conn.Close()
	}()

	for {

		mType, data, err := c.conn.ReadMessage()
		if err != nil {
			log.Println(err)
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

		switch packet.EMsg {
		case enums.EMSG_LOGON:
			if !c.IsAuthenticated() {
				logOnEvent := new(protobuf.LogOnEvent)
				if err := packet.ReadProtoMsg(logOnEvent); err != nil {
					log.Println(err)
					break
				}
				c.Authentication(logOnEvent.Token, logOnEvent)
			}
		case enums.EMSG_THEATER_LOGON:
			if !c.IsAuthenticated() {
				logOnEvent := new(protobuf.TheaterLogOnEvent)
				if err := packet.ReadProtoMsg(logOnEvent); err != nil {
					log.Println(err)
					break
				}
				c.Authentication(logOnEvent.Token, logOnEvent)
			}
		}

		c.Event <- packet
	}

}

func (c *Client) Authentication(token []byte, event proto.Message) {
	if !c.IsAuthenticated() {
		mCtx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
		response, err := grpc.UserServiceClient.GetUser(mCtx, &proto2.AuthenticateRequest{
			Token: token,
		})

		if err != nil {
			c.authChan <- AuthChan{ err: err }
		} else {
			c.authChan <- AuthChan{
				user: response.Result,
				authenticated: true,
				event: event,
				token: token,
			}
		}
	}
}

func (c *Client) WriteMessage(msg []byte) (err error) {
	err = c.conn.WriteMessage(websocket.BinaryMessage, msg)
	return
}

func NewClient(conn *websocket.Conn) *Client {
	return &Client{
		Id:            uuid.New().ID(),
		conn:          conn,
		Event:         make(chan *protocol.Packet),
		authChan:      make(chan AuthChan),
	}
}