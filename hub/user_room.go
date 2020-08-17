package hub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/CastyLab/gateway.server/redis"
	"github.com/CastyLab/grpc.proto/protocol"
	"github.com/getsentry/sentry-go"
	redis2 "github.com/go-redis/redis/v8"
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
	friends   cmap.ConcurrentMap
	session   *Session
}

func (r *UserRoom) GetName() string {
	return r.name
}

func (r *UserRoom) GetContext() context.Context {
	return r.session.c.ctx
}

func (r *UserRoom) AddFriend(friend *proto.User) {
	r.friends.Set(friend.Id, friend)
}

func (r *UserRoom) GetClients() cmap.ConcurrentMap {
	return r.clients
}

func (r *UserRoom) UpdateState(client *Client, state proto.PERSONAL_STATE) {

	r.updateMeOnFriendsList(&proto.PersonalStateMsgEvent{
		State: state,
		User:  client.GetUser(),
	})

	mCtx, cancel := context.WithTimeout(client.ctx, time.Second * 10)
	defer cancel()

	hKey := fmt.Sprintf("user:%s", client.GetUser().Id)

	switch state {
	case proto.PERSONAL_STATE_ONLINE:
		cmd := redis.Client.HSet(mCtx, hKey, "state", state.String())
		if err := cmd.Err(); err != nil {
			sentry.CaptureException(err)
		}
	case proto.PERSONAL_STATE_OFFLINE:
		cmd := redis.Client.HDel(mCtx, hKey, "state")
		if err := cmd.Err(); err != nil {
			sentry.CaptureException(err)
		}
	}

}

/* Add a conn to clients map so that it can be managed */
func (r *UserRoom) Join(client *Client) {

	r.clients.SetIfAbsent(client.Id, client)
	r.session = NewSession(client)

	if err := r.GetAndFeatchFriendsState(client.ctx); err != nil {
		sentry.CaptureException(fmt.Errorf("could not GetAndFeatchFriendsState : %v", err))
	}

	if r.clients.Count() == 1 {
		r.UpdateState(client, proto.PERSONAL_STATE_ONLINE)
	}

	if err := protocol.BrodcastMsgProtobuf(client.conn, proto.EMSG_AUTHORIZED, nil); err != nil {
		log.Println(err)
		sentry.CaptureException(fmt.Errorf("could not send Authorized message to user: %v", err))
	}
}

func (r *UserRoom) UpdateUserEvent(token string) error {

	ctx, cancel := context.WithTimeout(context.Background(), 20 * time.Second)
	defer cancel()

	response, err := grpc.UserServiceClient.GetUser(ctx, &proto.AuthenticateRequest{
		Token: []byte(token),
	})
	if err != nil {
		return err
	}

	// sending updated user to user's clients
	r.GetClients().IterCb(func(_ string, uc interface{}) {
		client := uc.(*Client)
		buffer, err := protocol.NewMsgProtobuf(proto.EMSG_USER_UPDATED, response.Result)
		if err == nil {
			_ = client.WriteMessage(buffer.Bytes())
		}
	})

	// sending updated user to friends clients
	r.friends.IterCb(func(key string, val interface{}) {
		friend := val.(*proto.User)
		friendRoom, err := r.hub.FindRoom(friend.Id)
		if err == nil {
			buffer, err := protocol.NewMsgProtobuf(proto.EMSG_USER_UPDATED, response.Result)
			if err == nil {
				if err = friendRoom.(*UserRoom).Send(buffer.Bytes()); err != nil {
					sentry.CaptureException(fmt.Errorf("could not send updated user to friends: %v", err))
				}
			}
		}
	})
	return nil
}

/* Removes client from room */
func (r *UserRoom) Leave(client *Client) {
	r.clients.Remove(client.Id)
	if r.clients.Count() == 0 {
		r.UpdateState(client, proto.PERSONAL_STATE_OFFLINE)
		r.hub.RemoveRoom(r.name)
	}
}

func (r *UserRoom) Send(msg []byte) (err error) {
	r.clients.IterCb(func(key string, v interface{}) {
		err = v.(*Client).WriteMessage(msg)
	})
	return
}

func (r *UserRoom) SendProtoMessage(enum proto.EMSG, message *proto.Message) (err error) {
	buffer, err := protocol.NewMsgProtobuf(enum, message)
	if err != nil {
		return err
	}
	if err := r.Send(buffer.Bytes()); err != nil {
		sentry.CaptureException(fmt.Errorf("could not send proto message : %v", err))
		return err
	}
	return nil
}

func (r *UserRoom) SendMessage(message *proto.Message) error {

	room, err := r.hub.FindRoom(message.Reciever.Id)
	if err != nil {
		return errors.New("could not find friend's room")
	}

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

	if err := room.(*UserRoom).Send(buffer.Bytes()); err != nil {
		return err
	}

	return nil
}

func (r *UserRoom) updateMyActivityOnFriendsList(psme *proto.PersonalActivityMsgEvent) {

	r.friends.IterCb(func(key string, val interface{}) {
		friend := val.(*proto.User)
		if friendRoom, err := r.hub.FindRoom(friend.Id); err == nil {
			buffer, err := protocol.NewMsgProtobuf(proto.EMSG_PERSONAL_ACTIVITY_CHANGED, psme)
			if err == nil {
				_ = friendRoom.(*UserRoom).Send(buffer.Bytes())
			}
		}
	})

}

func (r *UserRoom) updateMeOnFriendsList(psme *proto.PersonalStateMsgEvent) {
	r.friends.IterCb(func(key string, val interface{}) {
		friend := val.(*proto.User)
		friendRoom, err := r.hub.FindRoom(friend.Id)
		if err == nil {
			buffer, err := protocol.NewMsgProtobuf(proto.EMSG_PERSONAL_STATE_CHANGED, psme)
			if err == nil {
				if err = friendRoom.(*UserRoom).Send(buffer.Bytes()); err != nil {
					sentry.CaptureException(fmt.Errorf("could not send user's state to friends: %v", err))
				}
			}
		}
	})
}

func (r *UserRoom) FeatchFriends() error {
	mCtx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
	response, err := grpc.UserServiceClient.GetFriends(mCtx, &proto.AuthenticateRequest{
		Token: r.session.Token(),
	})
	if err != nil {
		return err
	}
	for _, friend := range response.Result {
		r.friends.Set(friend.Id, friend)
	}
	return nil
}

func (r *UserRoom) GetAndFeatchFriendsState(ctx context.Context) error {
	if err := r.FeatchFriends(); err != nil {
		return err
	}
	r.friends.IterCb(func(key string, v interface{}) {
		friend, ok := v.(*proto.User)
		if ok {
			cmd := redis.Client.HGet(ctx, fmt.Sprintf("user:%s", friend.Id), "state")
			if err := cmd.Err(); err != nil {
				sentry.CaptureException(fmt.Errorf("could not get friend from redis: %v", err))
				return
			}
			activityCmd := redis.Client.HGet(ctx, fmt.Sprintf("user:%s", friend.Id), "activity")
			if err := activityCmd.Err(); err != nil {
				if err != redis2.Nil {
					log.Println(err)
					sentry.CaptureException(fmt.Errorf("could not get friend's activuty from redis: %v", err))
					return
				}
			}
			if sState := cmd.Val(); sState != proto.PERSONAL_STATE_OFFLINE.String() {
				var (
					activity = new(proto.Activity)
					state    = proto.PERSONAL_STATE(proto.PERSONAL_STATE_value[sState])
					psm      = &proto.PersonalStateMsgEvent{
						User: friend,
						State: state,
					}
				)

				redisActivity := activityCmd.Val()
				if redisActivity != "" {
					err := json.Unmarshal([]byte(redisActivity), activity)
					if err == nil {
						psm.Activity = activity
					}
				}

				buffer, err := protocol.NewMsgProtobuf(proto.EMSG_PERSONAL_STATE_CHANGED, psm)
				if err == nil {
					if err := r.Send(buffer.Bytes()); err != nil {
						sentry.CaptureException(fmt.Errorf("could not send friend's psm to user! : %v", err))
					}
				}
			}
		}
	})
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
						r.AddFriend(protoMessage.Friend)
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
func NewUserRoom(name string, hub *UserHub) (room *UserRoom) {
	return &UserRoom{
		name:    name,
		clients: cmap.New(),
		friends: cmap.New(),
		hub:     hub,
	}
}
