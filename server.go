package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/castyapp/gateway.server/config"
	"github.com/castyapp/gateway.server/grpc"
	"github.com/castyapp/gateway.server/hub"
	"github.com/castyapp/gateway.server/redis"
	"github.com/getsentry/sentry-go"
	"github.com/gorilla/mux"
)

var (
	userGatewayPort,
	theaterGatewayPort *int
	userGatewayHost,
	theaterGatewayHost *string
	env *string
)

func init() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	userGatewayPort = flag.Int("ug-port", 3001, "UserGateway server port")
	userGatewayHost = flag.String("ug-host", "0.0.0.0", "UserGateway server host")

	theaterGatewayPort = flag.Int("tg-port", 3002, "TheaterGateway server port")
	theaterGatewayHost = flag.String("tg-host", "0.0.0.0", "TheaterGateway server host")

	env = flag.String("env", "development", "Environment")
	configFileName := flag.String("config-file", "config.hcl", "config.hcl file")

	flag.Parse()
	log.Printf("Loading ConfigMap from file: [%s]", *configFileName)

	if err := config.LoadFile(*configFileName); err != nil {
		log.Fatal(fmt.Errorf("could not load config: %v", err))
	}

	if err := grpc.Configure(); err != nil {
		log.Fatal(fmt.Errorf("could not configure grpc.server: %v", err))
	}

	if err := redis.Configure(); err != nil {
		log.Fatal(fmt.Errorf("could not configure redis: %v", err))
	}

	if config.Map.Sentry.Enabled {
		if err := sentry.Init(sentry.ClientOptions{Dsn: config.Map.Sentry.Dsn}); err != nil {
			log.Fatal(fmt.Errorf("could not initilize sentry: %v", err))
		}
	}
}

func main() {

	userGatewayListener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", *userGatewayHost, *userGatewayPort))
	if err != nil {
		sentry.CaptureException(err)
		log.Fatal(err)
		return
	}

	theaterGatewayListener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", *theaterGatewayHost, *theaterGatewayPort))
	if err != nil {
		sentry.CaptureException(err)
		log.Fatal(err)
		return
	}

	var (
		usersHub    = hub.NewUserHub()
		theatersHub = hub.NewTheaterHub()
	)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		fmt.Printf("Got interrupt Signal. Cleaning up...\n")
		// Close usersHub
		if err := usersHub.Close(); err != nil {
			mErr := fmt.Errorf("could not close UserHub: %v", err)
			sentry.CaptureException(mErr)
			log.Println(mErr)
		}
		// Close theatersHub
		if err := theatersHub.Close(); err != nil {
			mErr := fmt.Errorf("could not close TheatersHub: %v", err)
			sentry.CaptureException(mErr)
			log.Println(mErr)
		}
		os.Exit(1)
	}()

	defer func() {

		// Close listener
		if err := userGatewayListener.Close(); err != nil {
			mErr := fmt.Errorf("could not close TCPListener: %v", err)
			sentry.CaptureException(mErr)
			log.Println(mErr)
		}

		if err := theaterGatewayListener.Close(); err != nil {
			mErr := fmt.Errorf("could not close TCPListener: %v", err)
			sentry.CaptureException(mErr)
			log.Println(mErr)
		}

		// Since sentry emits events in the background we need to make sure
		// they are sent before we shut down
		if ok := sentry.Flush(time.Second * 5); !ok {
			sentry.CaptureMessage("could not Flush sentry")
			log.Println("could not Flush sentry")
		}

		// Close redis
		if err := redis.Close(); err != nil {
			mErr := fmt.Errorf("could not close Redis: %v", err)
			sentry.CaptureException(mErr)
			log.Println(mErr)
		}

	}()

	userGatewayRouter := mux.NewRouter()
	userGatewayRouter.HandleFunc("/", usersHub.ServeHTTP)
	log.Printf("[UserGateway] %s server running and listeting on http://%s:%d", *env, *userGatewayHost, *userGatewayPort)
	go func() {
		log.Printf("http_err: %v", http.Serve(userGatewayListener, userGatewayRouter))
	}()

	theaterGatewayRouter := mux.NewRouter()
	theaterGatewayRouter.HandleFunc("/", theatersHub.ServeHTTP)
	log.Printf("[TheaterGateway] %s server running and listeting on http://%s:%d", *env, *theaterGatewayHost, *theaterGatewayPort)
	log.Printf("http_err: %v", http.Serve(theaterGatewayListener, theaterGatewayRouter))
}
