package user

import (
	"encoding/json"
	"github.com/CastyLab/gateway.server/hub"
	"github.com/CastyLab/grpc.proto/proto"
	"github.com/CastyLab/grpc.proto/protocol"
	"github.com/MrJoshLab/go-respond"
	"github.com/gin-gonic/gin"
	"log"
)

func NewFriendRequestEvent(ctx *gin.Context) {

	var (
		userId = ctx.PostForm("user_id")
		userRoom = hub.UsersHub.GetOrCreateRoom(userId)
	)

	for _, client := range userRoom.GetClients() {
		buffer, err := protocol.NewMsgProtobuf(proto.EMSG_NEW_NOTIFICATION, nil)
		if err != nil {
			log.Println(err)
			continue
		}
		if err := client.WriteMessage(buffer.Bytes()); err != nil {
			log.Println(err)
			continue
		}
	}

	ctx.JSON(respond.Default.InsertSucceeded())
	return
}

func FriendRequestAcceptedEvent(ctx *gin.Context) {

	var (
		user = new(proto.User)
		friendId = ctx.PostForm("friend_id")
		userJsonString = ctx.PostForm("user")
		friendRoom = hub.UsersHub.GetOrCreateRoom(friendId)
	)

	if err := json.Unmarshal([]byte(userJsonString), &user); err != nil {
		return
	}

	for _, client := range friendRoom.GetClients() {

		buffer, err := protocol.NewMsgProtobuf(proto.EMSG_FRIEND_REQUEST_ACCEPTED, &proto.FriendRequestAcceptedMsgEvent{
			Friend: user,
		})
		if err != nil {
			log.Println(err)
			continue
		}
		if err := client.WriteMessage(buffer.Bytes()); err != nil {
			log.Println(err)
			continue
		}
	}

	ctx.JSON(respond.Default.InsertSucceeded())
	return
}