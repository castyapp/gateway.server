package hub

import (
	"context"
	"errors"
	"fmt"
	"github.com/CastyLab/gateway.server/redis"
	"github.com/CastyLab/grpc.proto/protocol"
	"github.com/getsentry/sentry-go"
	"log"
	"time"

	"github.com/CastyLab/gateway.server/grpc"
	"github.com/CastyLab/grpc.proto/proto"
)

type TheaterRoom struct {
	// authorized theater
	theater *proto.Theater
	// Video player configures
	vp *VideoPlayer
}

func (room *TheaterRoom) GetType() RoomType {
	return TheaterRoomType
}

func (room *TheaterRoom) GetName() string {
	return room.theater.MediaSource.Title
}

// Join a client to room
func (room *TheaterRoom) Join(client *Client) {

	if !client.IsGuest() {

		room.SubscribeEvents(client)

		ctx := context.Background()
		clientsKey := fmt.Sprintf("theater:clients:%s", room.theater.Id)
		exists := redis.Client.SIsMember(ctx, clientsKey, client.Id)
		if !exists.Val() {
			redis.Client.SAdd(ctx, clientsKey, client.Id)
		}

		// Store theater members
		//
		//membersKey := fmt.Sprintf("theater:members:%s", room.theater.Id)
		//memberExists := redis.Client.SIsMember(client.ctx, membersKey, client.Id)
		//if !memberExists.Val() {
		//	redis.Client.SAdd(client.ctx, membersKey, client.GetUser().Id)
		//}

		// Get current client's user object
		response, err := grpc.UserServiceClient.GetUser(client.ctx, &proto.AuthenticateRequest{
			Token: client.Token(),
		})
		if err != nil {
			_ = client.conn.Close()
			return
		}

		// check if user has default state
		// if it has, then do nothing with state
		if response.Result.State == proto.PERSONAL_STATE_ONLINE {
			// Update user's activity to this theater
			if err := room.updateUserActivity(client); err != nil {
				sentry.CaptureException(err)
			}
		}
	}

	if client.IsGuest() {
		log.Printf("User [GUEST:%s] Theater[%s]", client.Id, room.theater.Id)
	} else {
		log.Printf("User [%s] Theater[%s]", client.GetUser().Id, room.theater.Id)
	}

	_ = client.send(proto.EMSG_AUTHORIZED, nil)

	// get member from redis
	//_ = client.send(proto.EMSG_THEATER_MEMBERS, &proto.TheaterMembers{
	//	Members: room.GetMembers(),
	//})

	return
}

func (room *TheaterRoom) SubscribeEvents(client *Client) {
	channel := fmt.Sprintf("theater:events:%s", room.theater.Id)
	pubsub := redis.Client.Subscribe(client.ctx, channel)
	go func() {
		defer pubsub.Close()
		for event := range pubsub.Channel() {
			if err := client.WriteMessage([]byte(event.Payload)); err != nil {
				log.Println(fmt.Errorf("could not write message to user's theater client REASON[%v]", err))
				continue
			}
		}
	}()
}

// updae user's activity to watching this theater
func (room *TheaterRoom) updateUserActivity(client *Client) error {

	mCtx := context.Background()

	if room.theater.MediaSource != nil {
		if !client.IsGuest() {
			activity := &proto.Activity{
				Id:       room.theater.Id,
				Activity: room.theater.MediaSource.Title,
			}
			_, err := grpc.UserServiceClient.UpdateActivity(mCtx, &proto.UpdateActivityRequest{
				Activity: activity,
				AuthRequest: &proto.AuthenticateRequest{
					Token: client.Token(),
				},
			})
			if err != nil {
				return err
			}
		}
	} else {
		_, err := grpc.UserServiceClient.RemoveActivity(mCtx, &proto.AuthenticateRequest{Token: client.Token()})
		if err != nil {
			return err
		}
	}

	return nil
}

// Remove user's activity
func (room *TheaterRoom) removeUserActivity(client *Client) error {
	if !client.IsGuest() {
		_, err := grpc.UserServiceClient.RemoveActivity(context.Background(), &proto.AuthenticateRequest{
			Token: client.Token(),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

/* Removes client from room */
func (room *TheaterRoom) Leave(client *Client) {
	mCtx := context.Background()
	if !client.IsGuest() {
		// Remove user's activity
		_ = room.removeUserActivity(client)
	}
	clientsKey := fmt.Sprintf("theater:clients:%s", room.theater.Id)
	redis.Client.SRem(mCtx, clientsKey, client.Id)
	clients := redis.Client.SMembers(mCtx, clientsKey)
	if len(clients.Val()) == 0 {
		room.vp.Pause()
	}
}

func (room *TheaterRoom) Sync(client *Client) {

	log.Printf("[%s] Syncing client...", client.Id)

	var state proto.TheaterVideoPlayer_State

	log.Println("InProgress: ", room.vp.InProgress())

	if room.vp.InProgress() {
		state = proto.TheaterVideoPlayer_PLAYING
	} else {
		state = proto.TheaterVideoPlayer_PAUSED
	}

	tvp := &proto.TheaterVideoPlayer{
		CurrentTime: room.vp.CurrentTime(),
		State: state,
	}

	log.Println("TVP: ", tvp)

	_ = client.send(proto.EMSG_SYNCED, tvp)

}

func (room *TheaterRoom) SendEventToTheaterMembers(ctx context.Context, event []byte)  {
	redis.Client.Publish(ctx, fmt.Sprintf("theater:events:%s", room.theater.Id), event)
}

// Handle client events
func (room *TheaterRoom) HandleEvents(client *Client) error {

	for {
		select {

		// check if context closed
		case <-client.ctx.Done():
			return client.ctx.Err()

		// on new events
		case event := <-client.Event:

			if event != nil {

				log.Printf("NEW EVENT: [%s]", event.EMsg)

				switch event.EMsg {

				// syncing client to theater video player
				case proto.EMSG_SYNC_ME:
					room.Sync(client)

				// when theater play requested
				case proto.EMSG_THEATER_PLAY:
					if client.IsAuthenticated() {
						mCtx := context.Background()
						theaterVideoPlayer := new(proto.TheaterVideoPlayer)
						if err := event.ReadProtoMsg(theaterVideoPlayer); err == nil {

							room.vp.SetCurrentTime(theaterVideoPlayer.CurrentTime)

							log.Println("CurrentTime: ", room.vp.CurrentTime())

							log.Println("PLAY: ", theaterVideoPlayer)
							room.vp.Play()

							event, err := protocol.NewMsgProtobuf(proto.EMSG_THEATER_PLAY, theaterVideoPlayer)
							if err == nil {
								room.SendEventToTheaterMembers(mCtx, event.Bytes())
							}
						}
					}
					break

				// when theater pause requested
				case proto.EMSG_THEATER_PAUSE:
					if client.IsAuthenticated() {
						mCtx := context.Background()
						theaterVideoPlayer := new(proto.TheaterVideoPlayer)
						if err := event.ReadProtoMsg(theaterVideoPlayer); err == nil {

							room.vp.SetCurrentTime(theaterVideoPlayer.CurrentTime)

							log.Println("PAUSE: ", theaterVideoPlayer)
							room.vp.Pause()

							event, err := protocol.NewMsgProtobuf(proto.EMSG_THEATER_PAUSE, theaterVideoPlayer)
							if err == nil {
								room.SendEventToTheaterMembers(mCtx, event.Bytes())
							}
						}
					}
					break

				// when new message chat recieved
				case proto.EMSG_NEW_CHAT_MESSAGE:
					if client.IsAuthenticated() {
						mCtx := context.Background()
						chatMessage := new(proto.ChatMsgEvent)
						if err := event.ReadProtoMsg(chatMessage); err == nil {
							chatMessage.User = client.GetUser()
							event, err := protocol.NewMsgProtobuf(proto.EMSG_CHAT_MESSAGES, chatMessage)
							if err == nil {
								room.SendEventToTheaterMembers(mCtx, event.Bytes())
							}
						}
					}
					break
				}
			}

		}
	}
}

func GetTheater(theaterId, token []byte) (*proto.Theater, error) {
	req := &proto.GetTheaterRequest{
		TheaterId: string(theaterId),
	}
	if token != nil {
		req.AuthRequest = &proto.AuthenticateRequest{
			Token: token,
		}
	}
	mCtx, cancel := context.WithTimeout(context.Background(), 10 * time.Second)
	defer cancel()
	response, err := grpc.TheaterServiceClient.GetTheater(mCtx, req)
	if err != nil {
		return nil, err
	}
	if response.Result == nil {
		return nil, errors.New("could not find theater")
	}
	return response.Result, nil
}

// create a new theater room
func NewTheaterRoom(theater *proto.Theater) *TheaterRoom {
	return &TheaterRoom{
		theater: theater,
		vp:      NewVideoPlayer(),
	}
}
