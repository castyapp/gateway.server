package theater

import (
	"github.com/CastyLab/gateway.server/hub"
	"github.com/CastyLab/grpc.proto/proto"
	"github.com/CastyLab/grpc.proto/protocol"
	"github.com/MrJoshLab/go-respond"
	"github.com/gin-gonic/gin"
	"log"
)

func UpdatedTheater(ctx *gin.Context)  {

	theaterId := ctx.PostForm("theater_id")

	log.Println("Updating theater: ", theaterId)

	theaterRoom, err := hub.TheatersHub.FindRoom(theaterId)
	if err == nil {
		theaterRoom.GetClients().IterCb(func(_ string, uc interface{}) {
			client := uc.(*hub.Client)
			buffer, err := protocol.NewMsgProtobuf(proto.EMSG_THEATER_UPDATED, nil)
			if err == nil {
				_ = client.WriteMessage(buffer.Bytes())
			}
		})
	}

	ctx.JSON(respond.Default.InsertSucceeded())
	return

}