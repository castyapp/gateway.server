package hub

import (
	"context"
	"github.com/CastyLab/grpc.proto/protocol"
	"time"

	"github.com/CastyLab/gateway.server/grpc"
	"github.com/CastyLab/grpc.proto/proto"
	pb "github.com/golang/protobuf/proto"
)

type TheaterRoom struct {
	// authorized theater
	theater   *proto.Theater

	// all clients connected to this room
	clients   map[uint32] *Client

	// members with multiple clients
	members   map[string] *UserWithClients

	// user hub for updating user's activities
	hub       *TheaterHub

	// error channel
	Err       chan error
}

// Get room clients
func (room *TheaterRoom) GetClients() map[uint32]*Client {
	return room.clients
}

// Get room clients
func (room *TheaterRoom) GetContext() context.Context {
	return room.hub.ctx
}

// Get current user's client
func (room *TheaterRoom) AddClient(client *Client) {
	room.clients[client.Id] = client
}

// Get current user's client
func (room *TheaterRoom) GetClient(client *Client) *Client {
	return room.clients[client.Id]
}

func (room *TheaterRoom) AddMember(clients *UserWithClients) {
	room.members[clients.User.Id] = clients
}

func (room *TheaterRoom) RemoveMember(member *proto.User) {
	delete(room.members, member.Id)
}

func (room *TheaterRoom) GetMember(user *proto.User) *UserWithClients {
	return room.members[user.Id]
}

func (room *TheaterRoom) GetMemberFromClient(client *Client) *UserWithClients {
	return room.members[client.GetUser().Id]
}

// Join a client to room
func (room *TheaterRoom) Join(client *Client) {

	// add the client to all room's clients
	room.AddClient(client)

	// Get current client's user object
	mCtx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
	response, err := grpc.UserServiceClient.GetUser(mCtx, &proto.AuthenticateRequest{
		Token: client.Token(),
	})

	if err != nil {
		room.Err <- err
		client.conn.Close()
		return
	}

	member := response.Result

	// Update user's activity to this theater
	// r.updateUserActivity(client)

	// Check if this room already has this member
	if room.HasMember(member) {

		// Add this client to existed member
		room.GetMember(member).AddClient(client)

	} else {

		// create new user with clients
		userWithClients := NewUserWithClients(member)
		userWithClients.AddClient(client)

		// register user with clients to room
		room.AddMember(userWithClients)

		// update this client to others in room
		room.updateClientToFriends(client, &proto.PersonalStateMsgEvent{
			User:  client.GetUser(),
			State: proto.PERSONAL_STATE_ONLINE,
		})

	}

	// sending authentication succeed
	if err := protocol.BrodcastMsgProtobuf(client.conn, proto.EMSG_AUTHORIZED, nil); err != nil {
		room.Err <- err
	}

	// sending members through socket
	members := &proto.TheaterMembers{Members: room.GetMembers()}
	if err := protocol.BrodcastMsgProtobuf(client.conn, proto.EMSG_THEATER_MEMBERS, members); err != nil {
		room.Err <- err
	}

	return
}

// Check member exists in room
func (room *TheaterRoom) HasMember(user *proto.User) (ok bool) {
	_, ok = room.members[user.Id]
	return
}

// Get members the room
func (room *TheaterRoom) GetMembers() (members []*proto.User) {
	for _, member := range room.members {
		members = append(members, member.User)
	}
	return
}

// updae user's activity to watching this theater
func (room *TheaterRoom) updateUserActivity(client *Client) {

	// context with 10 seconds timeout
	mCtx, _ := context.WithTimeout(context.Background(), 10 * time.Second)

	// Update user's activity via grpc
	grpc.UserServiceClient.UpdateActivity(mCtx, &proto.UpdateActivityRequest{
		Activity: &proto.Activity{
			Id:       room.theater.Id,
			Activity: room.theater.Title,
		},
		AuthRequest: &proto.AuthenticateRequest{
			Token: client.Token(),
		},
	})
}

// Remove user's activity
func (room *TheaterRoom) removeUserActivity(client *Client) {

	// context with 10 seconds timeout
	mCtx, _ := context.WithTimeout(context.Background(), 10 * time.Second)

	// remove activity from user
	grpc.UserServiceClient.RemoveActivity(mCtx, &proto.AuthenticateRequest{
		Token: client.Token(),
	})
}

// removing client from room
func (room *TheaterRoom) RemoveClient(client *Client) {
	delete(room.clients, client.Id)
}

/* Removes client from room */
func (room *TheaterRoom) Leave(client *Client) {

	// remove current clinet from all clients
	room.RemoveClient(client)

	// get current member from client
	member := client.GetUser()

	// check if this member exists
	if room.HasMember(member) {

		// get member clients from room
		memberClients := room.GetMemberFromClient(client)

		// check if member has clients
		if memberClients.HasClients() {

			// then removes it
			memberClients.RemoveClient(client)

		} else {

			// remove member from room
			room.RemoveMember(member)

			// Update client to others
			room.updateClientToFriends(client, &proto.PersonalStateMsgEvent{
				User:  client.GetUser(),
				State: proto.PERSONAL_STATE_OFFLINE,
			})
		}

	}

	// Remove user's activity
	//room.removeUserActivity(client)

	// check if room clients are empty then removing room from cmp
	if len(room.clients) == 0 {
		room.hub.RemoveRoom(room.theater.Id)
	}
}

/* Send to specific client */
func (room *TheaterRoom) SendTo(id uint32, msg []byte) (err error) {
	if client := room.clients[id]; client != nil {
		err = client.WriteMessage(msg)
	}
	return
}

/* Broadcast to every client */
func (room *TheaterRoom) BroadcastAll(msg []byte) (err error) {
	for _, client := range room.clients {
		err = client.WriteMessage(msg)
	}
	return
}

func (room *TheaterRoom) SendAll(msg []byte) (err error) {
	for _, client := range room.clients {
		err = client.WriteMessage(msg)
	}
	return
}

/* Broadcast to all except */
func (room *TheaterRoom) BroadcastEx(senderid uint32, msg []byte) (err error) {
	for _, client := range room.clients {
		if client.Id != senderid {
			err = client.WriteMessage(msg)
		}
	}
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
	pmae := &proto.PersonalActivityMsgEvent{
		User: client.GetUser(),
	}
	if msg.State == proto.PERSONAL_STATE_ONLINE {
		if room.theater != nil {
			pmae.Activity = &proto.Activity{
				Id:       room.theater.Id,
				Activity: room.theater.Title,
			}
		}
	}
	//_ = r.updateMyActivity(client, pmae)
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

// Handle client events
func (room *TheaterRoom) HandleEvents(client *Client) error {

	for {
		select {

		// check if context closed
		case <-room.GetContext().Done():
			return room.GetContext().Err()

		// on new events
		case event := <-client.Event:

			if event != nil {
				switch event.EMsg {
				// when theater play requested
				case proto.EMSG_THEATER_PLAY:
					if client.IsAuthenticated() {
						theaterVideoPlayer := new(proto.TheaterVideoPlayer)
						if err := event.ReadProtoMsg(theaterVideoPlayer); err != nil {
							room.Err <- err
						} else {
							room.BroadcastProtoToAllEx(client, proto.EMSG_THEATER_PLAY, theaterVideoPlayer)
						}
					}
					break

				// when theater pause requested
				case proto.EMSG_THEATER_PAUSE:
					if client.IsAuthenticated() {
						theaterVideoPlayer := new(proto.TheaterVideoPlayer)
						if err := event.ReadProtoMsg(theaterVideoPlayer); err != nil {
							room.Err <- err
						} else {
							room.BroadcastProtoToAllEx(client, proto.EMSG_THEATER_PAUSE, theaterVideoPlayer)
						}
					}
					break

				// when new message chat recieved
				case proto.EMSG_NEW_CHAT_MESSAGE:
					if client.IsAuthenticated() {
						chatMessage := new(proto.ChatMsgEvent)
						if err := event.ReadProtoMsg(chatMessage); err != nil {
							room.Err <- err
						} else {
							chatMessage.User = client.GetUser()
							room.sendMessageToMemebers(client, chatMessage)
						}
					}
					break
				}
			}

		}
	}
}

// create a new theater room
func NewTheaterRoom(name string, hub *TheaterHub) (room *TheaterRoom, err error) {
	response, err := grpc.TheaterServiceClient.GetTheater(hub.ctx, &proto.Theater{
		Id: name,
	})
	if err != nil {
		return nil, err
	}
	return &TheaterRoom{
		clients: make(map[uint32]*Client, 0),
		members: make(map[string]*UserWithClients, 0),
		theater: response.Result,
		hub:     hub,
		Err:     make(chan error),
	}, nil
}
