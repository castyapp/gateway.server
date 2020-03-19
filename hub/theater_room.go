package hub

import (
	"context"
	"github.com/CastyLab/gateway.server/grpc"
	"github.com/CastyLab/gateway.server/hub/protocol/protobuf"
	"github.com/CastyLab/gateway.server/hub/protocol/protobuf/enums"
	"github.com/CastyLab/grpc.proto"
	"github.com/CastyLab/grpc.proto/messages"
	"log"
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

/* Add a conn to clients map so that it can be managed */
func (r *TheaterRoom) Join(client *Client) {

	r.clients[client.Id] = client

	mCtx, _ := context.WithTimeout(r.hub.ctx, 10 * time.Second)
	response, err := grpc.UserServiceClient.GetUser(mCtx, &proto.AuthenticateRequest{
		Token: client.auth.token,
	})

	if err != nil {
		_ = client.conn.Close()
		return
	}

	user := response.Result
	// r.updateUserActivity(client)

	if _, ok := r.members[user.Id]; !ok {
		uwc := NewUserWithClients(user)
		uwc.Clients[client.Id] = client
		r.members[user.Id] = uwc
		_ = r.updateClientToFriends(client, &protobuf.PersonalStateMsgEvent{
			User:  client.auth.user,
			State: enums.EMSG_PERSONAL_STATE_ONLINE,
		})
	} else {
		r.members[user.Id].Clients[client.Id] = client
	}

	// sending members through socket
	members := &protobuf.TheaterMembers{Members: r.getMembers()}
	if err := protobuf.BrodcastMsgProtobuf(client.conn, enums.EMSG_THEATER_MEMBERS, members); err != nil {
		log.Println(err)
	}

	// sending authentication succeed
	if err := protobuf.BrodcastMsgProtobuf(client.conn, enums.EMSG_AUTHORIZED, nil); err != nil {
		log.Println(err)
	}

	return
}

func (r *TheaterRoom) getMembers() (members []*messages.User) {
	for _, member := range r.members {
		members = append(members, &member.User)
	}
	return
}

func (r *TheaterRoom) updateUserActivity(client *Client) {
	mCtx, _ := context.WithTimeout(r.hub.ctx, 10 * time.Second)
	_, _ = grpc.TheaterServiceClient.AddMember(mCtx, &proto.AddOrRemoveMemberRequest{
		TheaterId: r.theater.Id,
		AuthRequest: &proto.AuthenticateRequest{
			Token: client.auth.token,
		},
	})
	_, _ = grpc.UserServiceClient.UpdateActivity(mCtx, &proto.UpdateActivityRequest{
		Activity: &messages.Activity{
			Id:       r.theater.Id,
			Activity: r.theater.Title,
		},
		AuthRequest: &proto.AuthenticateRequest{
			Token: client.auth.token,
		},
	})
}

func (r *TheaterRoom) removeUserActivity(client *Client) {
	mCtx, _ := context.WithTimeout(r.hub.ctx, 10 * time.Second)
	_, _ = grpc.TheaterServiceClient.RemoveMember(mCtx, &proto.AddOrRemoveMemberRequest{
		TheaterId: r.theater.Id,
		AuthRequest: &proto.AuthenticateRequest{
			Token: client.auth.token,
		},
	})
	_, _ = grpc.UserServiceClient.RemoveActivity(mCtx, &proto.AuthenticateRequest{
		Token: client.auth.token,
	})
}

/* Removes client from room */
func (r *TheaterRoom) Leave(client *Client) {

	// removing client from room
	delete(r.clients, client.Id)
	delete(r.members[client.auth.user.Id].Clients, client.Id)

	r.removeUserActivity(client)

	if len(r.members[client.auth.user.Id].Clients) == 0 {
		delete(r.members, client.auth.user.Id)
		err := r.updateClientToFriends(client, &protobuf.PersonalStateMsgEvent{
			User:  client.auth.user,
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

func (r *TheaterRoom) updateClientToFriends(client *Client, msg *protobuf.PersonalStateMsgEvent) error {
	buffer, err := protobuf.NewMsgProtobuf(enums.EMSG_MEMBER_STATE_CHANGED, msg)
	if err != nil {
		return err
	}
	pmae := &protobuf.PersonalActivityMsgEvent{
		User: client.auth.user,
	}
	if msg.State == enums.EMSG_PERSONAL_STATE_ONLINE {
		pmae.Activity = &messages.Activity{
			Id: r.theater.Id,
			Activity: r.theater.Title,
		}
	}
	//_ = r.updateMyActivity(client, pmae)
	return r.BroadcastEx(client.Id, buffer.Bytes())
}

func (r *TheaterRoom) sendMessageToMemebers(client *Client, event *protobuf.ChatMsgEvent) error {
	buffer, err := protobuf.NewMsgProtobuf(enums.EMSG_CHAT_MESSAGES, event)
	if err != nil {
		return err
	}
	return r.BroadcastEx(client.Id, buffer.Bytes())
}

func (r *TheaterRoom) updateMyActivity(client *Client, msg *protobuf.PersonalActivityMsgEvent) error {
	uroom, err := r.hub.userHub.FindRoom(client.auth.user.Id)
	if err != nil {
		return err
	}
	uroom.updateMyActivityOnFriendsList(msg)
	return nil
}

/* Handle messages */
func (r *TheaterRoom) HandleEvents(client *Client) {
	for {
		select {
		case <-r.hub.ctx.Done():
			log.Println("TheaterRoom HandleEvents Err: ", r.hub.ctx.Err())
			return
		case event := <-client.Event:
			if event != nil {
				switch event.EMsg {
				case enums.EMSG_NEW_CHAT_MESSAGE:
					if client.IsAuthenticated() {
						chatMessage := new(protobuf.ChatMsgEvent)
						if err := event.ReadProtoMsg(chatMessage); err != nil {
							log.Println(err)
							break
						}
						chatMessage.User = client.GetUser()
						_ = r.sendMessageToMemebers(client, chatMessage)
					}
				}

			}
		}
	}
}

/* Constructor */
func NewTheaterRoom(name string, hub *TheaterHub) (room *TheaterRoom, err error) {
	mCtx, _ := context.WithTimeout(hub.ctx, 10 * time.Second)
	response, err := grpc.TheaterServiceClient.GetTheater(mCtx, &messages.Theater{
		Id: name,
	})
	if err != nil {
		return nil, err
	}
	room = &TheaterRoom{
		name:     name,
		clients:  make(map[uint32] *Client, 0),
		members:  make(map[string] *UserWithClients, 0),
		theater:  response.Result,
		hub:      hub,
	}
	hub.cmap.Set(name, room)
	return
}