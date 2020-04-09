package main

import (
	"flag"
	"fmt"
	"github.com/CastyLab/gateway.server/internal"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/CastyLab/gateway.server/hub"
	"github.com/getsentry/sentry-go"
	"github.com/gorilla/mux"
	_ "github.com/joho/godotenv/autoload"
)

func main() {

	log.SetFlags(log.Lshortfile | log.Ltime)

	if err := sentry.Init(sentry.ClientOptions{Dsn: os.Getenv("SENTRY_DSN")}); err != nil {
		log.Fatal(err)
	}

	// Since sentry emits events in the background we need to make sure
	// they are sent before we shut down
	defer sentry.Flush(time.Second * 5)

	var (
		router     = mux.NewRouter()
		port       = flag.Int("port", 3000, "Server port")
		env        = flag.String("env", "development", "Environment")
	)

	flag.Parse()

	iC := make(chan os.Signal, 2)
	signal.Notify(iC, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-iC
		_ = hub.UsersHub.Close()
		os.Exit(0)
	}()

	unixListener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		sentry.CaptureException(err)
		log.Fatal(err)
		return
	}

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

	go internal.CreateInternalRouter()

	defer unixListener.Close()

	log.Printf("%s server running and listeting on :%d", *env, *port)
	log.Printf("http_err: %v", http.Serve(unixListener, nil))
}
