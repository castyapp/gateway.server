package hub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/CastyLab/gateway.server/redis"
	"github.com/CastyLab/grpc.proto/protocol"
	"github.com/getsentry/sentry-go"
	cmap "github.com/orcaman/concurrent-map"
	"log"
	"time"

	"github.com/CastyLab/gateway.server/grpc"
	"github.com/CastyLab/grpc.proto/proto"
	pb "github.com/golang/protobuf/proto"
)

type TheaterRoom struct {
	// authorized theater
	theater *proto.Theater

	// all clients connected to this room
	clients cmap.ConcurrentMap

	// members with multiple clients
	members cmap.ConcurrentMap

	// user hub for updating user's activities
	hub *TheaterHub

	// Video player configures
	vp *VideoPlayer
}

func (room *TheaterRoom) GetName() string {
	return room.theater.MediaSource.Title
}

// Get room clients
func (room *TheaterRoom) GetClients() cmap.ConcurrentMap {
	return room.clients
}

// Get current user's client
func (room *TheaterRoom) AddClient(client *Client) {
	room.clients.Set(client.Id, client)
}

// Get current user's client
func (room *TheaterRoom) GetClient(client *Client) *Client {
	cl, _ := room.clients.Get(client.Id)
	return cl.(*Client)
}

func (room *TheaterRoom) AddMember(member *MemberWithClients) {
	room.members.Set(member.User.Id, member)
}

func (room *TheaterRoom) RemoveMember(member *proto.User) {
	room.members.Remove(member.Id)
}

func (room *TheaterRoom) GetMember(user *proto.User) *MemberWithClients {
	cl, _ := room.members.Get(user.Id)
	return cl.(*MemberWithClients)
}

func (room *TheaterRoom) GetMemberFromClient(client *Client) *MemberWithClients {
	mem, _ := room.members.Get(client.GetUser().Id)
	return mem.(*MemberWithClients)
}

// Join a client to room
func (room *TheaterRoom) Join(client *Client) {

	// add the client to all room's clients
	room.AddClient(client)

	if !client.IsGuest() {

		// Get current client's user object
		mCtx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
		response, err := grpc.UserServiceClient.GetUser(mCtx, &proto.AuthenticateRequest{
			Token: client.Token(),
		})

		if err != nil {
			_ = client.conn.Close()
			return
		}

		member := response.Result

		// Update user's activity to this theater
		if err := room.updateUserActivity(client); err != nil {
			sentry.CaptureException(err)
		}

		// Check if this room already has this member
		if room.HasMember(member) {

			// Add this client to existed member
			room.GetMember(member).AddClient(client)

		} else {

			// create new user with clients
			memberRoom := NewMemberWithClients(member)
			memberRoom.AddClient(client)

			// register user with clients to room
			room.AddMember(memberRoom)

			// update this client to others in room
			_ = room.updateClientToFriends(client, &proto.PersonalStateMsgEvent{
				User:  client.GetUser(),
				State: proto.PERSONAL_STATE_ONLINE,
			})

		}

	}

	if client.IsGuest() {
		log.Printf("User [GUEST:%s] Theater[%s]", client.Id, room.theater.Id)
	} else {
		log.Printf("User [%s] Theater[%s]", client.GetUser().Id, room.theater.Id)
	}

	_ = client.send(proto.EMSG_AUTHORIZED, nil)

	_ = client.send(proto.EMSG_THEATER_MEMBERS, &proto.TheaterMembers{
		Members: room.GetMembers(),
	})

	return
}

func (room *TheaterRoom) UpdateMediaSource(mediaSourceId, token string) error {

	theater, err := GetTheater([]byte(room.theater.Id), []byte(token))
	if err != nil {
		sentry.CaptureException(fmt.Errorf("could not get theater :%v", err))
	}

	room.theater = theater
	room.vp.End()

	event := &proto.MediaSourceChangedEvent{
		TheaterId: room.theater.Id,
		MediaSourceId: mediaSourceId,
	}

	room.GetClients().IterCb(func(_ string, uc interface{}) {
		client := uc.(*Client)
		buffer, err := protocol.NewMsgProtobuf(proto.EMSG_THEATER_MEDIA_SOURCE_CHANGED, event)
		if err == nil {
			_ = client.WriteMessage(buffer.Bytes())
		}
		if err := room.updateUserActivity(client); err != nil {
			sentry.CaptureException(err)
		}
	})

	return nil
}

// Check member exists in room
func (room *TheaterRoom) HasMember(member *proto.User) (ok bool) {
	return room.members.Has(member.Id)
}

// Get members the room
func (room *TheaterRoom) GetMembers() (members []*proto.User) {
	for _, member := range room.members.Items() {
		members = append(members, member.(*MemberWithClients).User)
	}
	return
}

// updae user's activity to watching this theater
func (room *TheaterRoom) updateUserActivity(client *Client) error {

	mCtx, cancel := context.WithTimeout(client.ctx, time.Second * 10)
	defer cancel()

	hKey := fmt.Sprintf("user:%s", client.GetUser().Id)

	activity := &proto.Activity{
		Id:       room.theater.Id,
		Activity: room.theater.MediaSource.Title,
	}

	activityJson, err := json.Marshal(activity)
	if err != nil {
		sentry.CaptureException(fmt.Errorf("could not marshal user's activity to json: %v", err))
		return err
	}

	cmd := redis.Client.HSet(mCtx, hKey, "activity", string(activityJson))
	if err := cmd.Err(); err != nil {
		sentry.CaptureException(fmt.Errorf("could not update user's activity: %v", err))
		return err
	}

	pa := &proto.PersonalActivityMsgEvent{
		User:  client.GetUser(),
		Activity: &proto.Activity{
			Id:       room.theater.Id,
			Activity: room.theater.MediaSource.Title,
		},
	}

	// update this client to others in room
	if err := room.updateMyActivity(client, pa); err != nil {
		sentry.CaptureException(fmt.Errorf("could not send user's activity to friends: %v", err))
		return err
	}

	return nil
}

// Remove user's activity
func (room *TheaterRoom) removeUserActivity(client *Client) error {

	mCtx, cancel := context.WithTimeout(client.ctx, time.Second * 10)
	defer cancel()

	hKey := fmt.Sprintf("user:%s", client.GetUser().Id)

	cmd := redis.Client.HDel(mCtx, hKey, "activity")
	if err := cmd.Err(); err != nil {
		sentry.CaptureException(fmt.Errorf("could not update user's activity: %v", err))
		return err
	}

	pa := &proto.PersonalActivityMsgEvent{
		User:  client.GetUser(),
	}

	// update this client to others in room
	if err := room.updateMyActivity(client, pa); err != nil {
		sentry.CaptureException(fmt.Errorf("could not send user's activity to friends: %v", err))
		return err
	}

	return nil
}

// removing client from room
func (room *TheaterRoom) RemoveClient(client *Client) {
	room.clients.Remove(client.Id)
}

/* Removes client from room */
func (room *TheaterRoom) Leave(client *Client) {

	// remove current clinet from all clients
	room.RemoveClient(client)

	if !client.IsGuest() {
		// get current member from client
		mClient := client.GetUser()

		// check if this member exists
		if room.HasMember(mClient) {

			// get member clients from room
			member := room.GetMemberFromClient(client)

			// check if member has clients
			if member.HasClients() {

				// then removes it
				member.RemoveClient(client)

			} else {

				// remove member from room
				room.RemoveMember(mClient)

				// Update client to others
				_ = room.updateClientToFriends(client, &proto.PersonalStateMsgEvent{
					User:  client.GetUser(),
					State: proto.PERSONAL_STATE_OFFLINE,
				})
			}

		}

		// Remove user's activity
		_ = room.removeUserActivity(client)
	}

	// check if room clients are empty then removing room from cmp
	if room.clients.Count() == 0 {
		room.vp.Pause()
	}

}

/* Send to specific client */
func (room *TheaterRoom) SendTo(client *Client, msg []byte) error {
	return client.WriteMessage(msg)
}

/* Broadcast to every client */
func (room *TheaterRoom) BroadcastAll(msg []byte) (err error) {
	room.GetClients().IterCb(func(key string, v interface{}) {
		err = v.(*Client).WriteMessage(msg)
	})
	return
}

func (room *TheaterRoom) SendAll(msg []byte) (err error) {
	room.GetClients().IterCb(func(key string, v interface{}) {
		err = v.(*Client).WriteMessage(msg)
	})
	return
}

/* Broadcast to all except */
func (room *TheaterRoom) BroadcastEx(senderid string, msg []byte) (err error) {
	room.GetClients().IterCb(func(key string, v interface{}) {
		client := v.(*Client)
		if client.Id != senderid {
			err = client.WriteMessage(msg)
		}
	})
	return
}

func (room *TheaterRoom) BroadcastProtoToAllEx(client *Client, enum proto.EMSG, pMsg pb.Message) error {
	buffer, err := protocol.NewMsgProtobuf(enum, pMsg)
	if err != nil {
		return err
	}
	return room.BroadcastEx(client.Id, buffer.Bytes())
}

func (room *TheaterRoom) updateClientToFriends(client *Client, msg *proto.PersonalStateMsgEvent) error {
	buffer, err := protocol.NewMsgProtobuf(proto.EMSG_MEMBER_STATE_CHANGED, msg)
	if err != nil {
		return err
	}
	return room.BroadcastEx(client.Id, buffer.Bytes())
}

func (room *TheaterRoom) sendMessageToMemebers(client *Client, event *proto.ChatMsgEvent) error {
	buffer, err := protocol.NewMsgProtobuf(proto.EMSG_CHAT_MESSAGES, event)
	if err != nil {
		return err
	}
	return room.BroadcastEx(client.Id, buffer.Bytes())
}

func (room *TheaterRoom) updateMyActivity(client *Client, msg *proto.PersonalActivityMsgEvent) error {
	uroom, err := room.hub.userHub.FindRoom(client.GetUser().Id)
	if err != nil {
		return err
	}
	uroom.(*UserRoom).updateMyActivityOnFriendsList(msg)
	return nil
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

							_ = room.BroadcastProtoToAllEx(client, proto.EMSG_THEATER_PLAY, theaterVideoPlayer)
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

							_ = room.BroadcastProtoToAllEx(client, proto.EMSG_THEATER_PAUSE, theaterVideoPlayer)
						}
					}
					break

				// when new message chat recieved
				case proto.EMSG_NEW_CHAT_MESSAGE:
					if client.IsAuthenticated() {
						chatMessage := new(proto.ChatMsgEvent)
						if err := event.ReadProtoMsg(chatMessage); err == nil {
							chatMessage.User = client.GetUser()
							_ = room.sendMessageToMemebers(client, chatMessage)
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
func NewTheaterRoom(theater *proto.Theater, hub *TheaterHub) (room *TheaterRoom, err error) {
	return &TheaterRoom{
		clients: cmap.New(),
		members: cmap.New(),
		theater: theater,
		hub:     hub,
		vp:      NewVideoPlayer(),
	}, nil
}
