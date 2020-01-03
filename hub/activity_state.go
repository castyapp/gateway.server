package hub

import (
	"movie.night.ws.server/hub/protocol/protobuf"
	"movie.night.ws.server/hub/protocol/protobuf/enums"
)

type ActivityState struct {
	State      enums.EMSG_PERSONAL_STATE
	Activity   *protobuf.PersonalStateActivityMsgEvent
	UserId     string
}
