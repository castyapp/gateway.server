package hub

import (
	"context"
	"github.com/google/uuid"
	"gitlab.com/movienight1/grpc.proto"
	"gitlab.com/movienight1/grpc.proto/messages"
	"log"
	"movie.night.ws.server/grpc"
	"movie.night.ws.server/hub/protocol/protobuf"
	"movie.night.ws.server/hub/protocol/protobuf/enums"
	"time"
)

/* Has a name, clients, count which holds the actual coutn and index which acts as the unique id */
type TheaterRoom struct {
	name       string
	theater    *messages.Theater
	clients    map[uint32] *Client
	members    map[string] *UserWithClients
	hub        *TheaterHub
}

func (r *TheaterRoom) GetClients() map[uint32] *Client {
	return r.clients
}

func (r *TheaterRoom) GetMembers() (members []messages.User) {
	for _, member := range r.members {
		members = append(members, member.User)
	}
	return
}

func (r *TheaterRoom) generateRandomClientId() uint32 {
	return uuid.New().ID()
}

/* Add a conn to clients map so that it can be managed */
func (r *TheaterRoom) Join(client *Client) {

	r.clients[client.Id] = client

	mCtx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
	response, err := grpc.UserServiceClient.GetUser(mCtx, &proto.AuthenticateRequest{
		Token: []byte(client.AuthToken),
	})
	if err != nil {
		_ = client.conn.Close()
		return
	}

	user := response.Result
	r.updateUserActivity(client)

	if _, ok := r.members[user.Id]; !ok {
		uwc := NewUserWithClients(user)
		uwc.Clients[client.Id] = client
		r.members[user.Id] = uwc
		err = r.updateClientToFriends(client, &protobuf.PersonalStateMsgEvent{
			User:  client.user,
			State: enums.EMSG_PERSONAL_STATE_ONLINE,
		})
		if err != nil {
			log.Println(err)
		}
	} else {
		r.members[user.Id].Clients[client.Id] = client
	}
	return
}

func (r *TheaterRoom) updateUserActivity(client *Client) {
	mCtx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
	_, err := grpc.TheaterServiceClient.AddMember(mCtx, &proto.AddOrRemoveMemberRequest{
		TheaterId: r.theater.Id,
		AuthRequest: &proto.AuthenticateRequest{
			Token: []byte(client.AuthToken),
		},
	})
	if err != nil {
		log.Println(err)
	}
	_, _ = grpc.UserServiceClient.UpdateActivity(mCtx, &proto.UpdateActivityRequest{
		Activity: &messages.Activity{
			Id:       r.theater.Id,
			Activity: r.theater.Title,
		},
		AuthRequest: &proto.AuthenticateRequest{
			Token: []byte(client.AuthToken),
		},
	})
	err = r.updateClientToFriends(client, &protobuf.PersonalStateMsgEvent{
		User:  client.user,
		State: enums.EMSG_PERSONAL_STATE_ONLINE,
		Activity: &messages.Activity{
			Id:       r.theater.Id,
			Activity: r.theater.Title,
		},
	})
	if err != nil {
		log.Println(err)
	}
}

func (r *TheaterRoom) removeUserActivity(client *Client) {
	mCtx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
	_, err := grpc.TheaterServiceClient.RemoveMember(mCtx, &proto.AddOrRemoveMemberRequest{
		TheaterId: r.theater.Id,
		AuthRequest: &proto.AuthenticateRequest{
			Token: []byte(client.AuthToken),
		},
	})
	if err != nil {
		log.Println(err)
	}
	_, _ = grpc.UserServiceClient.RemoveActivity(mCtx, &proto.AuthenticateRequest{
		Token: []byte(client.AuthToken),
	})
	err = r.updateClientToFriends(client, &protobuf.PersonalStateMsgEvent{
		User:  client.user,
		State: enums.EMSG_PERSONAL_STATE_ONLINE,
	})
	if err != nil {
		log.Println(err)
	}
}

/* Removes client from room */
func (r *TheaterRoom) Leave(client *Client) {
	// removing client from room
	delete(r.clients, client.Id)
	delete(r.members[client.user.Id].Clients, client.Id)

	r.removeUserActivity(client)

	if len(r.members[client.user.Id].Clients) == 0 {
		delete(r.members, client.user.Id)
		err := r.updateClientToFriends(client, &protobuf.PersonalStateMsgEvent{
			User:  client.user,
			State: enums.EMSG_PERSONAL_STATE_OFFLINE,
		})
		if err != nil {
			log.Println(err)
		}
	}

	if len(r.clients) == 0 {
		r.hub.RemoveRoom(r.name)
	}
}

/* Send to specific client */
func (r *TheaterRoom) SendTo(id uint32, msg []byte) (err error) {
	if client := r.clients[id]; client != nil {
		err = client.WriteMessage(msg)
	}
	return
}

/* Broadcast to every client */
func (r *TheaterRoom) BroadcastAll(msg []byte) (err error) {
	for _, client := range r.clients {
		err = client.WriteMessage(msg)
	}
	return
}

func (r *TheaterRoom) SendAll(msg []byte) (err error) {
	for _, client := range r.clients {
		err = client.WriteMessage(msg)
	}
	return
}

/* Broadcast to all except */
func (r *TheaterRoom) BroadcastEx(senderid uint32, msg []byte) (err error) {
	for _, client := range r.clients {
		if client.Id != senderid {
			err = client.WriteMessage(msg)
		}
	}
	return
}

func (r *TheaterRoom) ReadLoop(id uint32) {
	r.clients[id].Listen()
}

func (r *TheaterRoom) updateClientToFriends(client *Client, msg *protobuf.PersonalStateMsgEvent) error {
	buffer, err := protobuf.NewMsgProtobuf(enums.EMSG_THEATER_UPDATE_USER, msg)
	if err != nil {
		return err
	}
	return r.BroadcastEx(client.Id, buffer.Bytes())
}

/* Handle messages */
func (r *TheaterRoom) HandleEvents(client *Client) {

	for {

		if event := <-client.Event; event != nil {

			switch event.EMsg {
			case enums.EMSG_THEATER_LOGON:
				if !client.IsAuthenticated() {

					logOnEvent := new(protobuf.TheaterLogOnEvent)
					if err := event.ReadProtoMsg(logOnEvent); err != nil {
						log.Println(err)
						break
					}

					if err := client.Authentication(logOnEvent.Token, logOnEvent); err != nil {
						log.Println(err)
						break
					}

				}
			}
		}

	}
}

/* Constructor */
func NewTheaterRoom(name string, hub *TheaterHub) (*TheaterRoom, error) {

	mCtx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
	response, err := grpc.TheaterServiceClient.GetTheater(mCtx, &messages.Theater{
		Id: name,
	})

	if err != nil {
		return nil, err
	}

	return &TheaterRoom{
		name:     name,
		clients:  make(map[uint32] *Client, 0),
		members:  make(map[string] *UserWithClients, 0),
		theater:  response.Result,
		hub:      hub,
	}, nil
}