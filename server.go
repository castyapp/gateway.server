package main

import (
	"flag"
	"fmt"
	"github.com/CastyLab/gateway.server/config"
	"github.com/CastyLab/gateway.server/hub"
	"github.com/getsentry/sentry-go"
	"github.com/gorilla/mux"
	"log"
	"net"
	"net/http"
	"time"
)

var (
	port *int
	host *string
	env *string
)

func init() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	port = flag.Int("port", 3000, "Gateway server port")
	host = flag.String("host", "0.0.0.0", "Gateway server host")
	env  = flag.String("env", "development", "Environment")
	configFileName := flag.String("config-file", "config.yml", "config.yaml file")

	flag.Parse()
	log.Printf("Loading ConfigMap from file: [%s]", *configFileName)

	if err := config.Load(*configFileName); err != nil {
		log.Fatal(fmt.Errorf("could not load config: %v", err))
	}

	if err := sentry.Init(sentry.ClientOptions{ Dsn: config.Map.Secrets.SentryDsn }); err != nil {
		log.Fatal(fmt.Errorf("could not initilize sentry: %v", err))
	}
}

func main() {

	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", *host, *port))
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

		// Close listener
		if err := listener.Close(); err != nil {
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

	router := mux.NewRouter()

	// routes for development
	router.HandleFunc("/user", hub.UsersHub.ServeHTTP)
	router.HandleFunc("/theater", hub.TheatersHub.ServeHTTP)

	log.Printf("%s server running and listeting on http://%s:%d", *env, *host, *port)
	log.Printf("http_err: %v", http.Serve(listener, router))
}
