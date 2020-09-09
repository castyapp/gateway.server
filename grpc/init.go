package grpc

import (
	"fmt"
	"github.com/CastyLab/gateway.server/config"
	"github.com/CastyLab/grpc.proto/proto"
	"google.golang.org/grpc"
	"log"
)

var (
	UserServiceClient     proto.UserServiceClient
	TheaterServiceClient  proto.TheaterServiceClient
	MessagesServiceClient proto.MessagesServiceClient
)

func Configure() error {

	var (
		host = config.Map.Grpc.Host
		port = config.Map.Grpc.Port
	)

	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", host, port), grpc.WithInsecure(), WithAuthInterceptor())
	if err != nil {
		log.Fatal(err)
	}

	UserServiceClient    = proto.NewUserServiceClient(conn)
	TheaterServiceClient = proto.NewTheaterServiceClient(conn)
	MessagesServiceClient = proto.NewMessagesServiceClient(conn)
	return nil
}