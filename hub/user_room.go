package hub

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/CastyLab/gateway.server/redis"
	"github.com/CastyLab/grpc.proto/protocol"
	"github.com/getsentry/sentry-go"
	"github.com/golang/protobuf/ptypes"

	"github.com/CastyLab/gateway.server/grpc"
	"github.com/CastyLab/grpc.proto/proto"
)

/* Has a name, clients, count which holds the actual coutn and index which acts as the unique id */
type UserRoom struct {
	hub     *UserHub
	name    string
	session *Session
}

func (room *UserRoom) GetType() RoomType {
	return UserRoomType
}

func (room *UserRoom) GetName() string {
	return room.name
}

func (room *UserRoom) GetContext() context.Context {
	return room.session.c.ctx
}

func (room *UserRoom) UpdateState(client *Client, state proto.PERSONAL_STATE) {
	if !client.IsGuest() {
		_, err := grpc.UserServiceClient.UpdateState(context.Background(), &proto.UpdateStateRequest{
			State:       state,
			AuthRequest: &proto.AuthenticateRequest{Token: client.Token()},
		})
		if err != nil {
			sentry.CaptureException(err)
		}
	}
}

func (room *UserRoom) SubscribeEvents(client *Client) {
	if !client.IsGuest() {
		channel := fmt.Sprintf("user:events:%s", client.GetUser().Id)
		pubsub := redis.Client.Subscribe(context.Background(), channel)
		go func() {
			for {
				select {
				case <-client.ctx.Done():
					if err := pubsub.Unsubscribe(context.Background(), channel); err != nil {
						log.Println(fmt.Errorf("could not unsubscribe user from its redis events REASON[%v]", err))
					}
					pubsub.Close() // close redis pubsub for user
					return
				case event := <-pubsub.Channel():
					if err := client.WriteMessage([]byte(event.Payload)); err != nil {
						log.Println(fmt.Errorf("could not write message to user client REASON[%v]", err))
						continue
					}
				}
			}
		}()
	}
}

func (room *UserRoom) Join(client *Client) {

	client.room = room
	room.session = NewSession(client)

	if !client.IsGuest() {

		// subscribe to user's events on redis
		room.SubscribeEvents(client)
		room.hub.addClientToRoom(client)

		if err := room.FeatchFriendsState(client); err != nil {
			sentry.CaptureException(fmt.Errorf("could not GetAndFeatchFriendsState : %v", err))
		}

		room.UpdateState(client, proto.PERSONAL_STATE_ONLINE)
	}

	if err := protocol.BrodcastMsgProtobuf(client.conn, proto.EMSG_AUTHORIZED, nil); err != nil {
		log.Println(err)
		sentry.CaptureException(fmt.Errorf("could not send Authorized message to user: %v", err))
	}
}

func (room *UserRoom) Leave(client *Client) {

	// removing client from redis and User's ConccurentMap
	room.hub.removeClientFromRoom(client)

	key := fmt.Sprintf("user:clients:%s", client.GetUser().Id)
	if clients := redis.Client.SMembers(context.Background(), key).Val(); len(clients) == 0 {
		// Set a OFFLINE state for user if there's no client left
		room.UpdateState(client, proto.PERSONAL_STATE_OFFLINE)
	}
}

func (room *UserRoom) FeatchFriends(client *Client) ([]*proto.User, error) {
	response, err := grpc.UserServiceClient.GetFriends(client.ctx, &proto.AuthenticateRequest{
		Token: client.Token(),
	})
	if err != nil {
		return nil, err
	}
	return response.Result, nil
}

func (room *UserRoom) FeatchFriendsState(client *Client) error {
	friends, err := room.FeatchFriends(client)
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
			if err != nil {
				return err
			}
			SendEventToUser(client.ctx, buffer.Bytes(), client.GetUser())
		}
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

				case proto.EMSG_FRIEND_REQUEST_ACCEPTED:
					if client.IsAuthenticated() {
						protoMessage := new(proto.FriendRequestAcceptedMsgEvent)
						if err := event.ReadProtoMsg(protoMessage); err != nil {
							log.Println(err)
							continue
						}
					}

				case proto.EMSG_GET_FRIEND_STATE:
					if client.IsAuthenticated() {
						protoMessage := new(proto.User)
						if err := event.ReadProtoMsg(protoMessage); err != nil {
							log.Println(err)
							continue
						}

					}
					break

				// when user sending a new message
				case proto.EMSG_NEW_CHAT_MESSAGE:
					if client.IsAuthenticated() {

						chatMessage := new(proto.ChatMsgEvent)
						if err := event.ReadProtoMsg(chatMessage); err != nil {
							log.Println(err)
							continue
						}

						chatMessage.CreatedAt = ptypes.TimestampNow()

						mCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
						defer cancel()
						_, err := grpc.MessagesServiceClient.CreateMessage(mCtx, &proto.MessageRequest{
							Message: &proto.Message{
								Reciever: chatMessage.Reciever,
								Content:  string(chatMessage.Message),
							},
							AuthRequest: &proto.AuthenticateRequest{
								Token: client.Token(),
							},
						})
						if err != nil {
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
func NewUserRoom(hub *UserHub, name string) (room *UserRoom) {
	return &UserRoom{hub: hub, name: name}
}
