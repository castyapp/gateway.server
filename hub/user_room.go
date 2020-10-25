package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/CastyLab/gateway.server/redis"
	"github.com/CastyLab/grpc.proto/protocol"
	"github.com/getsentry/sentry-go"
	"github.com/golang/protobuf/ptypes"
	"log"
	"time"

	"github.com/CastyLab/gateway.server/grpc"
	"github.com/CastyLab/grpc.proto/proto"
)

/* Has a name, clients, count which holds the actual coutn and index which acts as the unique id */
type UserRoom struct {
	name      string
	session   *Session
}

func (r *UserRoom) GetType() RoomType {
	return UserRoomType
}

func (r *UserRoom) GetName() string {
	return r.name
}

func (r *UserRoom) GetContext() context.Context {
	return r.session.c.ctx
}

func (r *UserRoom) UpdateState(client *Client, state proto.PERSONAL_STATE) {
	if !client.IsGuest() {
		_, err := grpc.UserServiceClient.UpdateState(context.Background(), &proto.UpdateStateRequest{
			State: state,
			AuthRequest: &proto.AuthenticateRequest{Token: client.Token()},
		})
		if err != nil {
			sentry.CaptureException(err)
		}
	}
}

func (r *UserRoom) SubscribeEvents(client *Client) {
	if !client.IsGuest() {
		mCtx := context.Background()
		channel := fmt.Sprintf("user:events:%s", client.GetUser().Id)
		sub := redis.Client.Subscribe(mCtx, channel)
		for {
			select {
			case <-client.ctx.Done():
				if err := sub.Unsubscribe(mCtx, channel); err != nil {
					sentry.CaptureException(err)
				}
				return
			case event := <-sub.Channel():
				if err := client.WriteMessage([]byte(event.Payload)); err != nil {
					sentry.CaptureException(err)
					continue
				}
			}
		}
	}
}

func (r *UserRoom) Join(client *Client) {

	r.session = NewSession(client)

	if !client.IsGuest() {

		uClientsKey := fmt.Sprintf("user:clients:%s", client.GetUser().Id)
		exists := redis.Client.SIsMember(client.ctx, uClientsKey, client.Id)
		if !exists.Val() {
			redis.Client.SAdd(client.ctx, uClientsKey, client.Id)
		}
		clients := redis.Client.SMembers(client.ctx, uClientsKey).Val()

		if err := r.FeatchFriendsState(client); err != nil {
			sentry.CaptureException(fmt.Errorf("could not GetAndFeatchFriendsState : %v", err))
		}

		if len(clients) == 1 {
			r.UpdateState(client, proto.PERSONAL_STATE_ONLINE)
		}

		// subscribe to user's events on redis
		go r.SubscribeEvents(client)
	}

	if err := protocol.BrodcastMsgProtobuf(client.conn, proto.EMSG_AUTHORIZED, nil); err != nil {
		log.Println(err)
		sentry.CaptureException(fmt.Errorf("could not send Authorized message to user: %v", err))
	}
}

func (r *UserRoom) Leave(client *Client) {
	uClientsKey := fmt.Sprintf("user:clients:%s", client.GetUser().Id)
	redis.Client.SRem(client.ctx, uClientsKey, client.Id)
	clients := redis.Client.SMembers(client.ctx, uClientsKey).Val()
	if len(clients) == 0 {
		r.UpdateState(client, proto.PERSONAL_STATE_OFFLINE)
	}
}

func (r *UserRoom) SendMessage(message *proto.Message) error {

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

	SendEventToUser(r.GetContext(), buffer.Bytes(), message.Reciever)
	return nil
}

func (r *UserRoom) FeatchFriends(client *Client) ([]*proto.User, error) {
	mCtx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
	response, err := grpc.UserServiceClient.GetFriends(mCtx, &proto.AuthenticateRequest{
		Token: client.Token(),
	})
	if err != nil {
		return nil, err
	}
	return response.Result, nil
}

func (r *UserRoom) FeatchFriendsState(client *Client) error {
	friends, err := r.FeatchFriends(client)
	if err != nil {
		return err
	}
	for _, friend := range friends {
		if friend.State != proto.PERSONAL_STATE_OFFLINE && friend.State != proto.PERSONAL_STATE_INVISIBLE {
			psm := &proto.PersonalStateMsgEvent{
				User:  friend,
				State: friend.State,
			}
			buffer, err := protocol.NewMsgProtobuf(proto.EMSG_PERSONAL_STATE_CHANGED, psm)
			if err == nil {
				SendEventToUser(client.ctx, buffer.Bytes(), client.GetUser())
			}
		}
	}
	return nil
}

/* Handle messages */
func (r *UserRoom) HandleEvents(client *Client) error {
	for {
		select {

		// check if context closed
		case <-r.GetContext().Done():
			return r.GetContext().Err()

		// on new events
		case event := <-client.Event:
			if event != nil {
				switch event.EMsg {

				case proto.EMSG_FRIEND_REQUEST_ACCEPTED:
					if client.IsAuthenticated() {
						protoMessage := new(proto.FriendRequestAcceptedMsgEvent)
						if err := event.ReadProtoMsg(protoMessage); err != nil {
							log.Println(err)
							continue
						}
					}

				// when user sending a new message
				case proto.EMSG_NEW_CHAT_MESSAGE:
					if client.IsAuthenticated() {

						chatMessage := new(proto.ChatMsgEvent)
						if err := event.ReadProtoMsg(chatMessage); err != nil {
							log.Println(err)
							continue
						}

						chatMessage.CreatedAt = ptypes.TimestampNow()

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

						if err = r.SendMessage(response.Result); err != nil {
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
func NewUserRoom(name string) (room *UserRoom) {
	return &UserRoom{name: name}
}