package theater

import (
	"github.com/CastyLab/gateway.server/hub"
	"github.com/CastyLab/grpc.proto/proto"
	"github.com/CastyLab/grpc.proto/protocol"
	"github.com/MrJoshLab/go-respond"
	"github.com/gin-gonic/gin"
)

func UpdateMediaSource(ctx *gin.Context)  {

	var (
		theaterId = ctx.PostForm("theater_id")
		mediaSourceId = ctx.PostForm("media_source_id")
	)

	theaterRoom, err := hub.TheatersHub.FindRoom(theaterId)
	if err == nil {
		event := &proto.MediaSourceChangedEvent{
			TheaterId: theaterId,
			MediaSourceId: mediaSourceId,
		}
		theaterRoom.GetClients().IterCb(func(_ string, uc interface{}) {
			client := uc.(*hub.Client)
			buffer, err := protocol.NewMsgProtobuf(proto.EMSG_THEATER_MEDIA_SOURCE_CHANGED, event)
			if err == nil {
				_ = client.WriteMessage(buffer.Bytes())
			}
		})
		ctx.JSON(respond.Default.InsertSucceeded())
		return
	}

	ctx.JSON(respond.Default.InsertFailed())
	return

}