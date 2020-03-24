package hub

import (
	"github.com/CastyLab/grpc.proto/proto"
)

type UserWithClients struct {
	Clients  map[uint32] *Client
	User     *proto.User
}

func NewUserWithClients(user *proto.User) *UserWithClients {
	return &UserWithClients{
		Clients: make(map[uint32] *Client, 0),
		User: user,
	}
}
