package hub

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/CastyLab/grpc.proto/protocol"
	"log"
	"time"

	"github.com/CastyLab/gateway.server/grpc"
	"github.com/CastyLab/grpc.proto/proto"
	"github.com/golang/protobuf/ptypes"
)

/* Has a name, clients, count which holds the actual coutn and index which acts as the unique id */
type UserRoom struct {
	name      string
	hub       *UserHub
	clients   map[uint32]*Client
	friends   map[string] *proto.User
	Err       chan error
	client    *Client
}

func (r *UserRoom) GetClients() map[uint32]*Client {
	return r.clients
}

func (r *UserRoom) UpdateState(client *Client, state proto.PERSONAL_STATE) {
	r.updateMeOnFriendsList(&proto.PersonalStateMsgEvent{
		State: state,
		User:  client.GetUser(),
	})
	mCtx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
	_, _ = grpc.UserServiceClient.UpdateState(mCtx, &proto.UpdateStateRequest{
		State: state,
		AuthRequest: &proto.AuthenticateRequest{
			Token: client.auth.Token(),
		},
	})
}

/* Add a conn to clients map so that it can be managed */
func (r *UserRoom) Join(client *Client) {

	r.clients[client.Id] = client
	r.client = client

	r.fetchFriends()

	if len(r.clients) == 1 {
		r.UpdateState(client, proto.PERSONAL_STATE_ONLINE)
	}

	if err := protocol.BrodcastMsgProtobuf(client.conn, proto.EMSG_AUTHORIZED, nil); err != nil {
		r.Err <- err
	}
}

/* Removes client from room */
func (r *UserRoom) Leave(client *Client) {
	delete(r.clients, client.Id)
	if len(r.clients) <= 1 {
		r.UpdateState(client, proto.PERSONAL_STATE_OFFLINE)
		r.hub.RemoveRoom(r.name)
	}
}

func (r *UserRoom) Send(msg []byte) (err error) {
	for _, client := range r.clients {
		err = client.WriteMessage(msg)
	}
	return
}

func (r *UserRoom) SendMessage(message *proto.Message) error {

	if fc, ok := r.hub.cmap.Get(message.Reciever.Id); ok {

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

func (r *UserRoom) updateMyActivityOnFriendsList(psme *proto.PersonalActivityMsgEvent) {

	for _, fr := range r.friends {
		if fc, ok := r.hub.cmap.Get(fr.Id); ok {

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

func (r *UserRoom) updateMeOnFriendsList(psme *proto.PersonalStateMsgEvent) {
	for _, fr := range r.friends {
		if fc, ok := r.hub.cmap.Get(fr.Id); ok {
			friendRoom := fc.(*UserRoom)
			buffer, err := protocol.NewMsgProtobuf(proto.EMSG_PERSONAL_STATE_CHANGED, psme)
			if err != nil {
				r.Err <- err
				continue
			}
			if err := friendRoom.Send(buffer.Bytes()); err != nil {
				r.Err <- err
				continue
			}
		}
	}
}

func (r *UserRoom) fetchFriends() {
	mCtx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
	response, err := grpc.UserServiceClient.GetFriends(mCtx, &proto.AuthenticateRequest{
		Token: r.client.auth.Token(),
	})
	if err != nil {
		r.Err <- err
		return
	}
	for _, friend := range response.Result {
		r.friends[friend.Id] = friend
	}
	return
}

/* Handle messages */
func (r *UserRoom) HandleEvents(client *Client) error {
	for {
		select {
		case <-r.hub.GetContext().Done():
			return errors.New("context closed")
		default:
			if event := <-client.Event; event != nil {

				log.Printf("[%d] Recieved new packet <- [%s]", client.Id,  event.EMsg)

				switch event.EMsg {
				case proto.EMSG_NEW_CHAT_MESSAGE:
					if client.IsAuthenticated() {
						chatMessage := new(proto.ChatMsgEvent)
						if err := event.ReadProtoMsg(chatMessage); err != nil {
							r.Err <- err
							break
						}
						mCtx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
						response, err := grpc.MessagesServiceClient.CreateMessage(mCtx, &proto.CreateMessageRequest{
							RecieverId: chatMessage.To,
							Content:    string(chatMessage.Message),
							AuthRequest: &proto.AuthenticateRequest{
								Token: client.auth.Token(),
							},
						})

						if err != nil {
							r.Err <- err
							break
						}
						r.Err <- r.SendMessage(response.Result)
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
		clients: make(map[uint32] *Client),
		friends: make(map[string] *proto.User, 0),
		hub:     hub,
		Err:     make(chan error),
	}
}
