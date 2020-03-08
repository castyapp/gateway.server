package main

import (
	"github.com/CastyLab/gateway.server/hub/protocol/protobuf"
	"github.com/CastyLab/gateway.server/hub/protocol/protobuf/enums"
	"github.com/gorilla/websocket"
	"log"
)

func main() {

	clientConn, _, err := websocket.DefaultDialer.Dial("ws://localhost:3000/user", nil)
	if err != nil {
		log.Fatal(err)
	}

	buffer, err := protobuf.NewMsgProtobuf(enums.EMSG_LOGON, &protobuf.LogOnEvent{
		Token: []byte("ACCESS_TOKEN"),
	})
	if err != nil {
		log.Fatal(err)
	}

	if err := clientConn.WriteMessage(websocket.BinaryMessage, buffer.Bytes()); err != nil {
		log.Fatal(err)
	}

	log.Println("Logged on!")

	for {
		_, msg, err := clientConn.ReadMessage()
		if err != nil {
			log.Fatal(err)
		}

		log.Println(string(msg))
	}
}
