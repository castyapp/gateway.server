package hub

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/CastyLab/grpc.proto/protocol"
	"github.com/golang/protobuf/ptypes"
	cmap "github.com/orcaman/concurrent-map"
	"log"
	"time"

	"github.com/CastyLab/gateway.server/grpc"
	"github.com/CastyLab/grpc.proto/proto"
)

/* Has a name, clients, count which holds the actual coutn and index which acts as the unique id */
type UserRoom struct {
	name      string
	hub       *UserHub
	clients   cmap.ConcurrentMap
	friends   map[string] *proto.User
	client    *Client
}

// Get room clients
func (room *UserRoom) GetContext() context.Context {
	return room.client.ctx
}

func (room *UserRoom) GetClients() (clients []*Client) {
	clients = make([]*Client, 0)
	room.clients.IterCb(func(key string, val interface{}) {
		clients = append(clients, val.(*Client))
	})
	return
}

func (room *UserRoom) UpdateState(client *Client, state proto.PERSONAL_STATE) {
	room.updateMeOnFriendsList(&proto.PersonalStateMsgEvent{
		State: state,
		User:  client.GetUser(),
	})
	mCtx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
	grpc.UserServiceClient.UpdateState(mCtx, &proto.UpdateStateRequest{
		State: state,
		AuthRequest: &proto.AuthenticateRequest{
			Token: client.Token(),
		},
	})
}

/* Add a conn to clients map so that it can be managed */
func (room *UserRoom) Join(client *Client) {

	room.clients.SetIfAbsent(client.Id, client)
	room.client = client

	if err := room.GetFriendsFromGRPC(); err != nil {
		log.Println(err)
	}

	if room.clients.Count() == 1 {
		room.UpdateState(client, proto.PERSONAL_STATE_ONLINE)
	}

	if err := protocol.BrodcastMsgProtobuf(client.conn, proto.EMSG_AUTHORIZED, nil); err != nil {
		log.Println(err)
	}
}

/* Removes client from room */
func (room *UserRoom) Leave(client *Client) {
	room.clients.Remove(client.Id)
	if room.clients.Count() <= 1 {
		room.UpdateState(client, proto.PERSONAL_STATE_OFFLINE)
		room.hub.RemoveRoom(room.name)
	}
}

func (room *UserRoom) Send(msg []byte) (err error) {
	for _, client := range room.GetClients() {
		err = client.WriteMessage(msg)
	}
	return
}

func (room *UserRoom) SendMessage(message *proto.Message) error {

	if fc, ok := room.hub.cmap.Get(message.Reciever.Id); ok {

		log.Println(fc.(*UserRoom).clients.Count())
		log.Println("Found user's room", fc.(*UserRoom).name)

		from, err := json.Marshal(message.Sender)
		if err != nil {
			return err
		}

		createdAt, _ := ptypes.TimestampProto(time.Now())

		entry := &proto.ChatMsgEvent{
			Message:   []byte(message.Content),
			From:      string(from),
			CreatedAt: createdAt,
		}

		buffer, err := protocol.NewMsgProtobuf(proto.EMSG_CHAT_MESSAGES, entry)
		if err != nil {
			return err
		}

		if err := fc.(*UserRoom).Send(buffer.Bytes()); err != nil {
			return err
		}

		return nil
	}

	return errors.New("could not find friend's room")
}

func (room *UserRoom) updateMyActivityOnFriendsList(psme *proto.PersonalActivityMsgEvent) {

	for _, fr := range room.friends {
		if fc, ok := room.hub.cmap.Get(fr.Id); ok {

			friendRoom := fc.(*UserRoom)

			buffer, err := protocol.NewMsgProtobuf(proto.EMSG_PERSONAL_ACTIVITY_CHANGED, psme)
			if err != nil {
				log.Println(err)
				continue
			}

			_ = friendRoom.Send(buffer.Bytes())
		}
	}

}

func (room *UserRoom) updateMeOnFriendsList(psme *proto.PersonalStateMsgEvent) {
	for _, fr := range room.friends {
		if fc, ok := room.hub.cmap.Get(fr.Id); ok {
			friendRoom := fc.(*UserRoom)
			buffer, err := protocol.NewMsgProtobuf(proto.EMSG_PERSONAL_STATE_CHANGED, psme)
			if err != nil {
				log.Println(err)
				continue
			}
			if err := friendRoom.Send(buffer.Bytes()); err != nil {
				log.Println(err)
				continue
			}
		}
	}
}

func (room *UserRoom) GetFriendsFromGRPC() error {
	mCtx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
	response, err := grpc.UserServiceClient.GetFriends(mCtx, &proto.AuthenticateRequest{
		Token: room.client.Token(),
	})
	if err != nil {
		return err
	}
	for _, friend := range response.Result {
		room.friends[friend.Id] = friend
	}
	return nil
}

/* Handle messages */
func (room *UserRoom) HandleEvents(client *Client) error {
	for {
		select {

		// check if context closed
		case <-room.GetContext().Done():
			return room.GetContext().Err()

		// on new events
		case event := <-client.Event:
			if event != nil {
				switch event.EMsg {
				// when user sending a new message
				case proto.EMSG_NEW_CHAT_MESSAGE:
					if client.IsAuthenticated() {

						chatMessage := new(proto.ChatMsgEvent)
						if err := event.ReadProtoMsg(chatMessage); err != nil {
							log.Println(err)
							continue
						}

						mCtx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
						response, err := grpc.MessagesServiceClient.CreateMessage(mCtx, &proto.CreateMessageRequest{
							RecieverId: chatMessage.To,
							Content:    string(chatMessage.Message),
							AuthRequest: &proto.AuthenticateRequest{
								Token: client.Token(),
							},
						})

						if err != nil {
							log.Println(err)
							continue
						}

						if err = room.SendMessage(response.Result); err != nil {
							log.Println(err)
							continue
						}

					}
				}
			}
		}
	}
}

/* Constructor */
func NewUserRoom(name string, hub *UserHub) (room *UserRoom) {
	return &UserRoom{
		name:    name,
		clients: cmap.New(),
		friends: make(map[string] *proto.User, 0),
		hub:     hub,
	}
}
