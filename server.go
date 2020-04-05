package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/CastyLab/gateway.server/hub"
	"github.com/getsentry/sentry-go"
	"github.com/gorilla/mux"
	_ "github.com/joho/godotenv/autoload"
)

func main() {

	go func() {

		ticker := time.NewTicker(3 * time.Second)

		for {
			select {
			case <-ticker.C:
				log.Printf("Number of goroutines: [%d]", runtime.NumGoroutine())
				break
			}
		}

	}()

	log.SetFlags(log.Lshortfile | log.Ltime)

	if err := sentry.Init(sentry.ClientOptions{Dsn: os.Getenv("SENTRY_DSN")}); err != nil {
		log.Fatal(err)
	}

	// Since sentry emits events in the background we need to make sure
	// they are sent before we shut down
	defer sentry.Flush(time.Second * 5)

	var (
		router     = mux.NewRouter()
		userhub    = hub.NewUserHub()
		theaterhub = hub.NewTheaterHub(userhub)
		port       = flag.Int("port", 3000, "Server port")
		env        = flag.String("env", "development", "Environment")
	)

	flag.Parse()

	iC := make(chan os.Signal, 2)
	signal.Notify(iC, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-iC
		userhub.Close()
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
				userhub.Handler(writer, request)
				return
			case "THEATER-ROOM":
				theaterhub.Handler(writer, request)
				return
			}
		}).Methods("GET")
	default:
		// routes for development
		router.HandleFunc("/user", userhub.Handler)
		router.HandleFunc("/theater", theaterhub.Handler)
	}

	http.Handle("/", router)

	defer unixListener.Close()

	log.Printf("%s server running and listeting on :%d", *env, *port)
	log.Printf("http_err: %v", http.Serve(unixListener, nil))
}
