package hub

import (
	"github.com/CastyLab/grpc.proto/messages"
)

type UserWithClients struct {
	Clients  map[uint32] *Client
	User     *messages.User
}

func NewUserWithClients(user *messages.User) *UserWithClients {
	return &UserWithClients{
		Clients: make(map[uint32] *Client, 0),
		User: user,
	}
}
