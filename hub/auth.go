package hub

import (
	"github.com/castyapp/libcasty-protocol-go/proto"
	pb "github.com/golang/protobuf/proto"
)

type Auth struct {
	err           error
	authenticated bool
	guest         bool
	token         []byte
	event         pb.Message
	user          *proto.User
}

func (a *Auth) User() *proto.User {
	return a.user
}

func (a *Auth) Err() error {
	return a.err
}

func (a *Auth) Event() pb.Message {
	return a.event
}

func (a *Auth) Token() []byte {
	return a.token
}
