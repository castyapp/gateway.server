package hub

import (
	"context"

	"github.com/castyapp/libcasty-protocol-go/proto"
	"github.com/google/uuid"
)

type Session struct {
	Id                  uint32
	c                   *Client
	State               *proto.PERSONAL_STATE
	StaticPersonalState bool
	ctx                 context.Context
	ctxCancel           context.CancelFunc
}

func NewSession(client *Client) *Session {
	mCtx, cancel := context.WithCancel(client.ctx)
	return &Session{
		Id:                  uuid.New().ID(),
		c:                   client,
		StaticPersonalState: false,
		ctx:                 mCtx,
		ctxCancel:           cancel,
	}
}

func (s *Session) Destroy() {
	// TODO: Destroy the session
}

// Get client authenticated user
func (s *Session) Token() []byte {
	return s.c.auth.Token()
}
