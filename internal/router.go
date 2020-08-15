package internal

import (
	"flag"
	"github.com/CastyLab/gateway.server/internal/controllers/theater"
	"github.com/CastyLab/gateway.server/internal/controllers/user"
	"github.com/MrJoshLab/go-respond"
	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"
	"log"
	"os"
)

func NewInternalRouter() {

	gin.SetMode(gin.ReleaseMode)

	var (
		// Internal websocket router
		unixFile = *flag.String("internal-ws-unixfile", os.Getenv("INTERNAL_UNIX_FILE"), "Internal websocket unixfile")
		router = gin.New()
	)

	flag.Parse()

	router.POST("/user/@notifications/new", user.NewNotificationEvent)
	router.POST("/user/@notifications/friend/accepted", user.FriendRequestAcceptedEvent)
	router.POST("/theater/@updated", theater.UpdatedTheater)

	router.GET("/", func(ctx *gin.Context) {
		ctx.JSON(respond.Default.NotFound())
		return
	})

	log.Printf("Internal router running and listeting on UnixFile:%s", unixFile)
	log.Printf("http_err: %v", router.RunUnix(unixFile))
}