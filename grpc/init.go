package grpc

import (
	"fmt"
	"github.com/CastyLab/grpc.proto/proto"
	_ "github.com/joho/godotenv/autoload"
	"google.golang.org/grpc"
	"log"
	"os"
)

var (
	UserServiceClient     proto.UserServiceClient
	TheaterServiceClient  proto.TheaterServiceClient
	MessagesServiceClient proto.MessagesServiceClient
)

func init() {

	var (
		host = os.Getenv("GRPC_HOST")
		port = os.Getenv("GRPC_PORT")
	)

	conn, err := grpc.Dial(fmt.Sprintf("%s:%s", host, port), grpc.WithInsecure(), WithAuthInterceptor())
	if err != nil {
		log.Fatal(err)
	}

	UserServiceClient    = proto.NewUserServiceClient(conn)
	TheaterServiceClient = proto.NewTheaterServiceClient(conn)
	MessagesServiceClient = proto.NewMessagesServiceClient(conn)
}