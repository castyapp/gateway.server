package user

import (
	"github.com/CastyLab/gateway.server/hub"
	"github.com/CastyLab/grpc.proto/proto"
	"github.com/CastyLab/grpc.proto/protocol"
	"github.com/MrJoshLab/go-respond"
	"github.com/gin-gonic/gin"
	"log"
)

func CreateFriendRequest(ctx *gin.Context) {

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