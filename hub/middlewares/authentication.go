package middlewares

import (
	"context"
	"github.com/CastyLab/gateway.server/grpc"
	proto "github.com/CastyLab/grpc.proto"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

func Authentication(ctx *gin.Context)  {

	mCtx, _ := context.WithTimeout(ctx, 5 * time.Second)

	token, err := ctx.Cookie("token")
	if err != nil {
		token = ctx.Query("token")
		if token == "" {
			ctx.AbortWithStatus(http.StatusBadRequest)
			return
		}
	}

	authRequest := &proto.AuthenticateRequest{
		Token: []byte(token),
	}

	response, err := grpc.UserServiceClient.GetUser(mCtx, authRequest)
	if err != nil || response.Code != http.StatusOK {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	ctx.Set("user", response.Result)
	ctx.Set("token", token)
	ctx.Next()
}