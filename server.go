package main

import (
	"fmt"
	"github.com/CastyLab/gateway.server/hub"
	"github.com/getsentry/sentry-go"
	"github.com/gorilla/mux"
	_ "github.com/joho/godotenv/autoload"
	"log"
	"net"
	"net/http"
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
		router     = mux.NewRouter()
		userhub    = hub.NewUserHub()
		theaterhub = hub.NewTheaterHub(userhub)
		port       = 3000
	)

	defer userhub.Close()

	unixListener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		sentry.CaptureException(err)
		log.Fatal(err)
	}

	router.HandleFunc("/user", userhub.Handler).Methods("GET")
	router.HandleFunc("/theater", theaterhub.Handler).Methods("GET")

	http.Handle("/", router)

	defer unixListener.Close()

	log.Printf("Server running and listeting on :%d", port)
	log.Printf("http_err: %v", http.Serve(unixListener, nil))
}
