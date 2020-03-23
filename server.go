package main

import (
	"flag"
	"fmt"
	"github.com/CastyLab/gateway.server/hub"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"
	"log"
	"os"
	"time"
)

func main() {

	log.SetFlags(log.Lshortfile | log.Ltime)

	if err := sentry.Init(sentry.ClientOptions{ Dsn: os.Getenv("SENTRY_DSN") }); err != nil {
		log.Fatal(err)
	}

	// Since sentry emits events in the background we need to make sure
	// they are sent before we shut down
	defer sentry.Flush(time.Second * 5)

	var (
		router     = gin.Default()
		userhub    = hub.NewUserHub()
		theaterhub = hub.NewTheaterHub(userhub)
		port       = flag.Int("port", 3000, "Server port")
	)

	flag.Parse()

	defer userhub.Close()

	router.GET("/user", userhub.Handler)
	router.GET("/theater", theaterhub.Handler)

	log.Printf("Server running and listeting on :%d", *port)

	if err := router.Run(fmt.Sprintf(":%d", *port)); err != nil {
		log.Printf("http_err: %v", err)
	}

}
