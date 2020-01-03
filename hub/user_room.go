package hub

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/golang/protobuf/ptypes"
	"log"
	"movie.night.ws.server/grpc"
	"movie.night.ws.server/hub/protocol/protobuf"
	"movie.night.ws.server/hub/protocol/protobuf/enums"
	"movie.night.ws.server/proto"
	"movie.night.ws.server/proto/messages"
	"time"
)

/* Has a name, clients, count which holds the actual coutn and index which acts as the unique id */
type UserRoom struct {
	name       string
	hub        *UserHub

	id         uint32
	clients    map[uint32] *Client

	AuthToken  string

	Friends    []string
	// friends is an array full with friends hash
	// so we can find the friends room with the hash_id
}

func (r *UserRoom) ChangeState(state messages.PERSONAL_STATE) bool {
	mCtx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
	_, err := grpc.UserServiceClient.UpdateState(mCtx, &proto.UpdateStateRequest{
		State: state,
		AuthRequest: &proto.AuthenticateRequest{
			Token: []byte(r.AuthToken),
		},
	})
	if err != nil {
		return false
	}
	return true
}

/* Add a conn to clients map so that it can be managed */
func (r *UserRoom) Join(client *Client) {

	client.State = JoinedRoomState
	r.clients[client.Id] = client

	if len(r.clients) <= 1 {
		r.ChangeState(messages.PERSONAL_STATE_ONLINE)
		if err := r.fetchFriends(); err != nil {
			log.Println(err)
		}
		r.updateMeOnFriendsList(ActivityState{
			State: enums.EMSG_PERSONAL_STATE_ONLINE,
			UserId: client.user.Id,
		})
	}

	err := protobuf.BrodcastMsgProtobuf(client.conn, enums.EMSG_AUTHORIZED, nil)
	if err != nil {
		log.Println(err)
	}
}

/* Removes client from room */
func (r *UserRoom) Leave(id uint32) {
	client := r.clients[id]
	delete(r.clients, id)
	if len(r.clients) == 0 {
		r.ChangeState(messages.PERSONAL_STATE_OFFLINE)
		r.updateMeOnFriendsList(ActivityState{
			State: enums.EMSG_PERSONAL_STATE_OFFLINE,
			UserId: client.user.Id,
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

func (r *UserRoom) sendMessageTo(message *messages.Message) error {

	if fc, ok := r.hub.Get(message.Reciever.Id); ok {

		from, err := json.Marshal(message.Sender)
		if err != nil {
			return err
		}

		createdAt, _ := ptypes.TimestampProto(time.Now())

		entry := &protobuf.ChatMsgEvent{
			Message: []byte(message.Content),
			From: string(from),
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

func (r *UserRoom) updateMeOnFriendsList(as ActivityState) {

	for _, fr := range r.Friends {
		if fc, ok := r.hub.Get(fr); ok {

			friendRoom := fc.(*UserRoom)

			buffer, err := protobuf.NewMsgProtobuf(enums.EMSG_PERSONAL_STATE_CHANGED, &protobuf.PersonalStateMsgEvent{
				State:    as.State,
				Activity: as.Activity,
				UserId:   as.UserId,
			})
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

	mCtx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
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

					mCtx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
					response, err := grpc.MessagesServiceClient.CreateMessage(mCtx, &proto.CreateMessageRequest{
						RecieverId: chatMessage.To,
						Content: string(chatMessage.Message),
						AuthRequest: &proto.AuthenticateRequest{
							Token: client.token,
						},
					})

					if err != nil {
						log.Println(err)
						break
					}

					_ = r.sendMessageTo(response.Result)
				}
			}
		}
	}
}

/* Constructor */
func NewUserRoom(name string) *UserRoom {
	return &UserRoom{
		name:     name,
		clients:  make(map[uint32] *Client),
		Friends:  make([]string, 0),
	}
}