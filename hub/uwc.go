package hub

import (
	"github.com/CastyLab/grpc.proto/messages"
)

type UserWithClients struct {
	Clients map[uint32]*Client
	messages.User
}

func NewUserWithClients(user *messages.User) *UserWithClients {
	return &UserWithClients{
		Clients: map[uint32]*Client{},
		User:    *user,
	}
}
