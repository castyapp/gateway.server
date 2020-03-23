package middlewares

import (
	"github.com/gin-gonic/gin"
)

func SubProtocols(ctx *gin.Context) {
	//subprotos := websocket.Subprotocols(ctx.Request)
	//if !reflect.DeepEqual(subprotos, hub.Subprotocols) {
	//	ctx.AbortWithStatus(http.StatusBadRequest)
	//	return
	//}
	ctx.Next()
}