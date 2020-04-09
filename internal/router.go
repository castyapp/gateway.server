package internal

import (
	"github.com/CastyLab/gateway.server/internal/controllers/user"
	"github.com/MrJoshLab/go-respond"
	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"
	"log"
	"os"
)

func CreateInternalRouter() {

	listenerFile := os.Getenv("INTERNAL_UNIX_FILE")

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	router.POST("/user/@notifications/new", user.NewNotificationEvent)
	router.POST("/user/@notifications/friend/accepted", user.FriendRequestAcceptedEvent)

	router.GET("/", func(ctx *gin.Context) {
		ctx.JSON(respond.Default.NotFound())
		return
	})

	log.Printf("Internal router running and listeting on %s", listenerFile)
	log.Printf("http_err: %v", router.RunUnix(listenerFile))
}