package theater

import (
	"github.com/CastyLab/gateway.server/hub"
	"github.com/MrJoshLab/go-respond"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
)

func UpdateMediaSource(ctx *gin.Context)  {

	var (
		theaterId     = ctx.PostForm("theater_id")
		mediaSourceId = ctx.PostForm("media_source_id")
		token         = ctx.GetHeader("Authorization")
	)

	room, err := hub.TheatersHub.FindRoom(theaterId)
	theaterRoom := room.(*hub.TheaterRoom)
	if err == nil {
		if err := theaterRoom.UpdateMediaSource(mediaSourceId, token); err != nil {
			sentry.CaptureException(err)
		}
	}

	ctx.JSON(respond.Default.InsertSucceeded())
	return
}