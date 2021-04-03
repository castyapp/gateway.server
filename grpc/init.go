package grpc

import (
	"fmt"
	"log"

	"github.com/castyapp/gateway.server/config"
	"github.com/castyapp/libcasty-protocol-go/proto"
	"google.golang.org/grpc"
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

	UserServiceClient = proto.NewUserServiceClient(conn)
	TheaterServiceClient = proto.NewTheaterServiceClient(conn)
	MessagesServiceClient = proto.NewMessagesServiceClient(conn)
	return nil
}
