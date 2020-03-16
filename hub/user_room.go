package hub

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/CastyLab/gateway.server/grpc"
	"github.com/CastyLab/gateway.server/hub/protocol/protobuf"
	"github.com/CastyLab/gateway.server/hub/protocol/protobuf/enums"
	proto "github.com/CastyLab/grpc.proto"
	"github.com/CastyLab/grpc.proto/messages"
	"github.com/golang/protobuf/ptypes"
)

/* Has a name, clients, count which holds the actual coutn and index which acts as the unique id */
type UserRoom struct {
	name      string
	hub       *UserHub
	clients   map[uint32]*Client
	AuthToken string
	Friends   []string
}

func (r *UserRoom) ChangeState(state messages.PERSONAL_STATE) {
	go func() {
		mCtx, _ := context.WithTimeout(context.Background(), 10*time.Second)
		_, _ = grpc.UserServiceClient.UpdateState(mCtx, &proto.UpdateStateRequest{
			State: state,
			AuthRequest: &proto.AuthenticateRequest{
				Token: []byte(r.AuthToken),
			},
		})
	}()
}

/* Add a conn to clients map so that it can be managed */
func (r *UserRoom) Join(client *Client) {

	r.clients[client.Id] = client

	if len(r.clients) <= 1 {
		r.ChangeState(messages.PERSONAL_STATE_ONLINE)
		if err := r.fetchFriends(); err != nil {
			log.Println(err)
		}
		r.updateMeOnFriendsList(&protobuf.PersonalStateMsgEvent{
			State: enums.EMSG_PERSONAL_STATE_ONLINE,
			User:  client.auth.user,
		})
	}

	if err := protobuf.BrodcastMsgProtobuf(client.conn, enums.EMSG_AUTHORIZED, nil); err != nil {
		log.Println(err)
	}
}

/* Removes client from room */
func (r *UserRoom) Leave(client *Client) {
	delete(r.clients, client.Id)
	if len(r.clients) == 0 {
		r.ChangeState(messages.PERSONAL_STATE_OFFLINE)
		r.updateMeOnFriendsList(&protobuf.PersonalStateMsgEvent{
			State: enums.EMSG_PERSONAL_STATE_OFFLINE,
			User:  client.auth.user,
		})
		r.hub.RemoveRoom(r.name)
	}
}

func (r *UserRoom) Send(msg []byte) (err error) {
	for _, client := range r.clients {
		err = client.WriteMessage(msg)
	}
	return
}

func (r *UserRoom) SendMessage(message *messages.Message) error {

	if fc, ok := r.hub.cmap.Get(message.Reciever.Id); ok {

		from, err := json.Marshal(message.Sender)
		if err != nil {
			return err
		}

		createdAt, _ := ptypes.TimestampProto(time.Now())

		entry := &protobuf.ChatMsgEvent{
			Message:   []byte(message.Content),
			From:      string(from),
			CreatedAt: createdAt,
		}

		buffer, err := protobuf.NewMsgProtobuf(enums.EMSG_CHAT_MESSAGE, entry)
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

func (r *UserRoom) updateMyActivityOnFriendsList(psme *protobuf.PersonalActivityMsgEvent) {

	for _, fr := range r.Friends {
		if fc, ok := r.hub.cmap.Get(fr); ok {

			friendRoom := fc.(*UserRoom)

			buffer, err := protobuf.NewMsgProtobuf(enums.EMSG_PERSONAL_ACTIVITY_CHANGED, psme)
			if err != nil {
				log.Println(err)
				continue
			}

			_ = friendRoom.Send(buffer.Bytes())
		}
	}

}

func (r *UserRoom) updateMeOnFriendsList(psme *protobuf.PersonalStateMsgEvent) {

	for _, fr := range r.Friends {
		if fc, ok := r.hub.cmap.Get(fr); ok {

			friendRoom := fc.(*UserRoom)

			buffer, err := protobuf.NewMsgProtobuf(enums.EMSG_PERSONAL_STATE_CHANGED, psme)
			if err != nil {
				log.Println(err)
				continue
			}

			_ = friendRoom.Send(buffer.Bytes())
		}
	}

}

func (r *UserRoom) fetchFriends() error {

	r.Friends = make([]string, 0)

	mCtx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	response, err := grpc.UserServiceClient.GetFriends(mCtx, &proto.AuthenticateRequest{
		Token: []byte(r.AuthToken),
	})
	if err != nil {
		return err
	}

	for _, friend := range response.Result {
		r.Friends = append(r.Friends, friend.Id)
	}

	return nil
}

/* Handle messages */
func (r *UserRoom) HandleEvents(client *Client) {
	for {
		if event := <-client.Event; event != nil {
			switch event.EMsg {
			case enums.EMSG_NEW_CHAT_MESSAGE:
				if client.IsAuthenticated() {
					chatMessage := new(protobuf.ChatMsgEvent)
					if err := event.ReadProtoMsg(chatMessage); err != nil {
						log.Println(err)
						break
					}

					mCtx, _ := context.WithTimeout(context.Background(), 10*time.Second)
					response, err := grpc.MessagesServiceClient.CreateMessage(mCtx, &proto.CreateMessageRequest{
						RecieverId: chatMessage.To,
						Content:    string(chatMessage.Message),
						AuthRequest: &proto.AuthenticateRequest{
							Token: client.auth.token,
						},
					})

					if err != nil {
						log.Println(err)
						break
					}

					_ = r.SendMessage(response.Result)
				}
			}
		}
	}
}

/* Constructor */
func NewUserRoom(name string, hub *UserHub) (newRoom *UserRoom) {
	newRoom = &UserRoom{
		name:    name,
		clients: make(map[uint32]*Client),
		Friends: make([]string, 0),
		hub:     hub,
	}
	hub.cmap.Set(name, newRoom)
	return
}
