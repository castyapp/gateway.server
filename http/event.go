package http

import (
	"github.com/CastyLab/grpc.proto/messages"
	"github.com/gorilla/mux"
	"net/http"
)

type EventType int64

const (
	InvalidEvent EventType = 0
	ChatEvent    EventType = 1
)

func NewEventType(event string) EventType {
	switch event {
	case "chat":
		return ChatEvent
	}
	return InvalidEvent
}

type EventMessage struct {
	Type    EventType       `json:"name"`
	Data    interface{}     `json:"data"`
	User    *messages.User  `json:"user"`
}

func NewEvent(r *http.Request, user *messages.User) (EMsg *EventMessage) {
	params := mux.Vars(r)
	EMsg = &EventMessage{
		Type: NewEventType(params["event"]),
		User: user,
	}
	switch EMsg.Type {
	case ChatEvent:
		EMsg.Data = &messages.Message{
			Sender:   user,
			Reciever: &messages.User{
				Id: r.PostFormValue("to"),
			},
			Content: r.PostFormValue("content"),
		}
	}
	return
}