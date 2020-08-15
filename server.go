package main

import (
	"flag"
	"fmt"
	"github.com/CastyLab/gateway.server/hub"
	"github.com/CastyLab/gateway.server/internal"
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

	if err := sentry.Init(sentry.ClientOptions{Dsn: os.Getenv("SENTRY_DSN")}); err != nil {
		log.Fatal(err)
	}

	var (
		router     = mux.NewRouter()
		port       = flag.Int("port", 3000, "Server port")
		env        = flag.String("env", "development", "Environment")
	)

	flag.Parse()

	unixListener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		sentry.CaptureException(err)
		log.Fatal(err)
		return
	}

	defer func() {

		// Close userhub
		if err := hub.UsersHub.Close(); err != nil {
			mErr := fmt.Errorf("could not close UserHub: %v", err)
			sentry.CaptureException(mErr)
			log.Println(mErr)
		}

		// Close unix listener
		if err := unixListener.Close(); err != nil {
			mErr := fmt.Errorf("could not close UnixListener: %v", err)
			sentry.CaptureException(mErr)
			log.Println(mErr)
		}

		// Since sentry emits events in the background we need to make sure
		// they are sent before we shut down
		if ok := sentry.Flush(time.Second * 5); !ok {
			sentry.CaptureMessage("could not Flush sentry")
			log.Println("could not Flush sentry")
		}

	}()

	switch *env {
	case "production":
		// Handle caddy proxy server
		router.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
			switch request.Header.Get("Websocket-Room") {
			case "USER-ROOM":
				hub.UsersHub.Handler(writer, request)
				return
			case "THEATER-ROOM":
				hub.TheatersHub.Handler(writer, request)
				return
			}
		}).Methods("GET")
	default:
		// routes for development
		router.HandleFunc("/user", hub.UsersHub.Handler)
		router.HandleFunc("/theater", hub.TheatersHub.Handler)
	}

	http.Handle("/", router)

	go internal.NewInternalRouter()

	log.Printf("%s server running and listeting on http://0.0.0.0:%d", *env, *port)
	log.Printf("http_err: %v", http.Serve(unixListener, nil))
}
