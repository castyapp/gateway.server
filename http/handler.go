package http

import (
	"context"
	"encoding/json"
	"github.com/MrJoshLab/go-respond"
	_ "github.com/MrJoshLab/go-respond"
	"gitlab.com/movienight1/grpc.proto"
	"gitlab.com/movienight1/grpc.proto/messages"
	"movie.night.ws.server/grpc"
	"movie.night.ws.server/hub"
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
