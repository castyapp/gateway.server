package user

import (
	"context"
	"encoding/json"
	"github.com/CastyLab/gateway.server/grpc"
	"github.com/CastyLab/gateway.server/hub"
	"github.com/CastyLab/grpc.proto/proto"
	"github.com/CastyLab/grpc.proto/protocol"
	"github.com/MrJoshLab/go-respond"
	"github.com/gin-gonic/gin"
	"log"
	"time"
)

func NewNotificationEvent(ctx *gin.Context) {

	userId := ctx.PostForm("user_id")

	userRoom, err := hub.UsersHub.FindRoom(userId)
	if err == nil {
		userRoom.GetClients().IterCb(func(key string, v interface{}) {
			client := v.(*hub.Client)
			buffer, err := protocol.NewMsgProtobuf(proto.EMSG_NEW_NOTIFICATION, nil)
			if err == nil {
				_ = client.WriteMessage(buffer.Bytes())
			}
		})
	} else {
		log.Println(err)
	}

	ctx.JSON(respond.Default.InsertSucceeded())
	return
}

func FriendRequestAcceptedEvent(ctx *gin.Context) {

	var (
		user = new(proto.User)
		friendId = ctx.PostForm("friend_id")
		userJsonString = ctx.PostForm("user")
	)

	// Find friend's room
	friendRoom, err := hub.UsersHub.FindRoom(friendId)
	if err == nil {

		// decode user from request
		if err := json.Unmarshal([]byte(userJsonString), &user); err != nil {
			return
		}

		friendRoom.GetClients().IterCb(func(key string, v interface{}) {

			client := v.(*hub.Client)
			request := &proto.FriendRequestAcceptedMsgEvent{
				Friend: user,
			}

			// Create a new Friend Request Accepted Proto Message
			buffer, err := protocol.NewMsgProtobuf(proto.EMSG_FRIEND_REQUEST_ACCEPTED, request)
			if err == nil {
				_ = client.WriteMessage(buffer.Bytes())
			}

		})

	} else {
		log.Println(err)
	}

	// Adding friend to user room
	userRoom, err := hub.UsersHub.FindRoom(user.Id)
	if err == nil {
		mCtx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
		response, err := grpc.UserServiceClient.GetFriend(mCtx, &proto.FriendRequest{
			FriendId: friendId,
			AuthRequest: &proto.AuthenticateRequest{
				Token: []byte(ctx.GetHeader("Authorization")),
			},
		})
		if err == nil && response != nil {
			userRoom.(*hub.UserRoom).AddFriend(response.Result)
		}
	} else {
		log.Println(err)
	}

	ctx.JSON(respond.Default.InsertSucceeded())
	return
}