package grpc

import (
	"fmt"
	"google.golang.org/grpc"
	"log"
	"movie.night.ws.server/proto"
	"os"
)

var (
	UserServiceClient     proto.UserServiceClient
	AuthServiceClient     proto.AuthServiceClient
	TheaterServiceClient  proto.TheaterServiceClient
	MessagesServiceClient proto.MessagesServiceClient
)

func init() {

	var (
		host = os.Getenv("GRPC_HOST")
		port = os.Getenv("GRPC_PORT")
	)

	conn, err := grpc.Dial(fmt.Sprintf("%s:%s", host, port), grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}

	UserServiceClient    = proto.NewUserServiceClient(conn)
	AuthServiceClient    = proto.NewAuthServiceClient(conn)
	TheaterServiceClient = proto.NewTheaterServiceClient(conn)
	MessagesServiceClient = proto.NewMessagesServiceClient(conn)
}