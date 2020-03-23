package middlewares

import (
	"context"
	"github.com/CastyLab/gateway.server/grpc"
	proto "github.com/CastyLab/grpc.proto"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"time"
)

func Authentication(ctx *gin.Context)  {

	var (
		token, err = ctx.Cookie("token")
		authRequest = &proto.AuthenticateRequest{
			Token: []byte(token),
		}
		mCtx, _ = context.WithTimeout(ctx, 5 * time.Second)
	)

	if err != nil {
		log.Println("could not get token from cookie!")
		ctx.AbortWithStatus(http.StatusBadRequest)
	}

	response, err := grpc.UserServiceClient.GetUser(mCtx, authRequest)
	if err != nil || response.Code != http.StatusOK {
		log.Println(err)
		ctx.AbortWithStatus(http.StatusUnauthorized)
	}

	ctx.Set("user", response.Result)
	ctx.Set("token", token)
	ctx.Next()
}