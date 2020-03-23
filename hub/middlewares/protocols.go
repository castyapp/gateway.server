package middlewares

import (
	"github.com/CastyLab/gateway.server/hub"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"net/http"
	"reflect"
)

func SubProtocols(ctx *gin.Context) {
	subprotos := websocket.Subprotocols(ctx.Request)
	if !reflect.DeepEqual(subprotos, hub.Subprotocols) {
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}
	ctx.Next()
}