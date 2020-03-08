package http

import (
	"context"
	"encoding/json"
	"github.com/CastyLab/gateway.server/grpc"
	"github.com/CastyLab/gateway.server/hub"
	"github.com/CastyLab/grpc.proto"
	"github.com/CastyLab/grpc.proto/messages"
	"github.com/MrJoshLab/go-respond"
	_ "github.com/MrJoshLab/go-respond"
	"net/http"
	"time"
)

func toResponse(w http.ResponseWriter, status int, data interface{})  {
	w.Header().Set("Content-Type", "application/json")
	response, _ := json.Marshal(data)
	w.WriteHeader(status)
	_, _ = w.Write(response)
}

func Handler(userhub *hub.UserHub, w http.ResponseWriter, r *http.Request)  {

	mCtx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
	response, err := grpc.UserServiceClient.GetUser(mCtx, &proto.AuthenticateRequest{
		Token: []byte(r.Header.Get("Authorization")),
	})

	if err != nil {
		status, response := respond.Default.NotFound()
		toResponse(w, status, response)
		return
	}

	event := NewEvent(r, response.Result)
	switch event.Type {
	case ChatEvent:

		message := event.Data.(*messages.Message)
		userRoom, err := userhub.FindRoom(message.Reciever.Id)
		if err != nil {
			status, response := respond.Default.NotFound()
			toResponse(w, status, response)
			return
		}

		if err := userRoom.SendMessage(message); err != nil {
			status, response := respond.Default.InsertFailed()
			toResponse(w, status, response)
			return
		}

		status, response := respond.Default.InsertSucceeded()
		toResponse(w, status, response)
		return
	}
}
