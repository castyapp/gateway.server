package user

import (
	"fmt"
	"github.com/CastyLab/gateway.server/hub"
	"github.com/MrJoshLab/go-respond"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
)

func UpdatedUserEvent(ctx *gin.Context) {

	var (
		userId = ctx.PostForm("user_id")
		token = ctx.GetHeader("Authorization")
	)
	
	userRoom, err := hub.UsersHub.FindRoom(userId)
	if err == nil {
		if err := userRoom.(*hub.UserRoom).UpdateUserEvent(token); err != nil {
			sentry.CaptureException(fmt.Errorf("could not update user %v", err))
		}
	} else {
		sentry.CaptureException(err)
	}

	ctx.JSON(respond.Default.InsertSucceeded())
	return

}
