package hub

import (
	"gitlab.com/movienight1/grpc.proto/messages"
	"movie.night.ws.server/hub/protocol/protobuf"
	"movie.night.ws.server/hub/protocol/protobuf/enums"
)

type ActivityState struct {
	State      enums.EMSG_PERSONAL_STATE
	Activity   *protobuf.PersonalStateActivityMsgEvent
	User       *messages.User
}
