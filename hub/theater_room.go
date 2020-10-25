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

		clientsKey := fmt.Sprintf("theater:clients:%s", room.theater.Id)
		exists := redis.Client.SIsMember(client.ctx, clientsKey, client.Id)
		if !exists.Val() {
			redis.Client.SAdd(client.ctx, clientsKey, client.Id)
		}

		// Store theater members
		//
		//membersKey := fmt.Sprintf("theater:members:%s", room.theater.Id)
		//memberExists := redis.Client.SIsMember(client.ctx, membersKey, client.Id)
		//if !memberExists.Val() {
		//	redis.Client.SAdd(client.ctx, membersKey, client.GetUser().Id)
		//}

		// Get current client's user object
		mCtx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
		response, err := grpc.UserServiceClient.GetUser(mCtx, &proto.AuthenticateRequest{
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

	go room.SubscribeEvents(client)

	_ = client.send(proto.EMSG_AUTHORIZED, nil)

	// get member from redis
	//_ = client.send(proto.EMSG_THEATER_MEMBERS, &proto.TheaterMembers{
	//	Members: room.GetMembers(),
	//})

	return
}

func (room *TheaterRoom) SubscribeEvents(client *Client) {
	mCtx, _ := context.WithTimeout(client.ctx, 10 *time.Second)
	channel := fmt.Sprintf("theater:events:%s", room.theater.Id)
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

// updae user's activity to watching this theater
func (room *TheaterRoom) updateUserActivity(client *Client) error {

	mCtx, cancel := context.WithTimeout(client.ctx, time.Second * 10)
	defer cancel()

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
	mCtx, cancel := context.WithTimeout(client.ctx, time.Second * 10)
	defer cancel()
	if !client.IsGuest() {
		_, err := grpc.UserServiceClient.RemoveActivity(mCtx, &proto.AuthenticateRequest{Token: client.Token()})
		if err != nil {
			return err
		}
	}
	return nil
}

/* Removes client from room */
func (room *TheaterRoom) Leave(client *Client) {
	if !client.IsGuest() {
		// Remove user's activity
		_ = room.removeUserActivity(client)
	}
	clientsKey := fmt.Sprintf("theater:clients:%s", room.theater.Id)
	redis.Client.SRem(client.ctx, clientsKey, client.Id)
	clients := redis.Client.SMembers(client.ctx, clientsKey)
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
						theaterVideoPlayer := new(proto.TheaterVideoPlayer)
						if err := event.ReadProtoMsg(theaterVideoPlayer); err == nil {

							room.vp.SetCurrentTime(theaterVideoPlayer.CurrentTime)

							log.Println("CurrentTime: ", room.vp.CurrentTime())

							log.Println("PLAY: ", theaterVideoPlayer)
							room.vp.Play()

							event, err := protocol.NewMsgProtobuf(proto.EMSG_THEATER_PLAY, theaterVideoPlayer)
							if err == nil {
								room.SendEventToTheaterMembers(client.ctx, event.Bytes())
							}
						}
					}
					break

				// when theater pause requested
				case proto.EMSG_THEATER_PAUSE:
					if client.IsAuthenticated() {
						theaterVideoPlayer := new(proto.TheaterVideoPlayer)
						if err := event.ReadProtoMsg(theaterVideoPlayer); err == nil {

							room.vp.SetCurrentTime(theaterVideoPlayer.CurrentTime)

							log.Println("PAUSE: ", theaterVideoPlayer)
							room.vp.Pause()

							event, err := protocol.NewMsgProtobuf(proto.EMSG_THEATER_PAUSE, theaterVideoPlayer)
							if err == nil {
								room.SendEventToTheaterMembers(client.ctx, event.Bytes())
							}
						}
					}
					break

				// when new message chat recieved
				case proto.EMSG_NEW_CHAT_MESSAGE:
					if client.IsAuthenticated() {
						chatMessage := new(proto.ChatMsgEvent)
						if err := event.ReadProtoMsg(chatMessage); err == nil {
							chatMessage.User = client.GetUser()
							event, err := protocol.NewMsgProtobuf(proto.EMSG_CHAT_MESSAGES, chatMessage)
							if err == nil {
								room.SendEventToTheaterMembers(client.ctx, event.Bytes())
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
	mCtx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
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
