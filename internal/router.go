package internal

import (
	"github.com/CastyLab/gateway.server/internal/controllers"
	"github.com/MrJoshLab/go-respond"
	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"
	"log"
)

func NewInternalRouter() {

	gin.SetMode(gin.ReleaseMode)

	var (
		// Internal websocket router
		listenPort = "7190"
		router = gin.New()
	)

	router.POST("/user/@notifications/new", controllers.NewNotificationEvent)
	router.POST("/user/@notifications/friend/accepted", controllers.FriendRequestAcceptedEvent)

	router.GET("/", func(ctx *gin.Context) {
		ctx.JSON(respond.Default.NotFound())
		return
	})

	log.Printf("Internal router running and listeting on http://0.0.0.0:%s", listenPort)
	log.Printf("http_err: %v", router.Run("0.0.0.0:" + listenPort))
}