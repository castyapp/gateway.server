package main

import (
	"github.com/CastyLab/gateway.server/hub/protocol/protobuf"
	"github.com/CastyLab/gateway.server/hub/protocol/protobuf/enums"
	"github.com/gorilla/websocket"
	"log"
	"time"
)

func main() {

	clientConn, _, err := websocket.DefaultDialer.Dial("ws://localhost:3000/user", nil)
	if err != nil {
		log.Fatal(err)
	}

	buffer, err := protobuf.NewMsgProtobuf(enums.EMSG_LOGON, &protobuf.LogOnEvent{
		Token: []byte("eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE1ODQ5OTY2NDgsInN1YiI6IjVlNjQzNzJkYjM5MTM5OWI0N2M4YjA1NCJ9.ss_rYIr94GAV09A_TZ37S_pm2aJSsU5ZEn2Z0VKOTyKinnlvRwY-Aj_z4brjoa-8qL2DgCRK86ZaHyjTkxO1F4vj3PHhPX-eJuhskiHg9o8s6G4YFbhA8kSoMvd2wGautkt-Tu-YQ5gNcoWLWFypniFReOE1JcUXPEx6-yDV0oq0o30LNI-I9C-fIEMdKw7RcKTlgNA4eKxEr4357u0ZWpBltFXJ_i8O6pPkkeNms2ay_3hCqn7Fe1EJZ7uqpzgg53xXHi7EgWGPN5QnY_K1UdOYGyTXB-UqT13Edz9XpFD5xRXCSL3CpOGUA4htPf1oYY2MZnhKpOETxqsg405DYv8nW5mjehLryYBhJmmkTzFPhU_RZyzEiblJjYSRhpS6kIh21WuiWNI2SgmaJ1LHRmZcbRemKx0Tye3TWzAh7rS1g36MUOuqn2_ushddGwGQn9PMrbRS1mvsToUlklxm7Vkzc_50Xvw4AdJ3qNd_O4ClzmH0iQ2VSZVNZVOavu7vh6jVnwJXKAmNdlJDk39sPvD_w1T9J4QaczBlG_KZmNh_IClijqsdeI_t8Vy35o6hKHn9BC66FRYJ--1AfWyqqR-4UkKOk3n2UefMrI1EpPgsg-GoynULYsHMRjz4ci1V57AbyWRsqWWDPi7fKP06KRu758spm9MmmNAjxfrGJ74"),
	})
	if err != nil {
		log.Fatal(err)
	}

	err = clientConn.WriteMessage(websocket.BinaryMessage, buffer.Bytes())
	if err != nil {
		log.Fatal(err)
	}

	ticket := time.NewTicker(6 * time.Second)

	for {
		select {
		case <-ticket.C:
			log.Println("Sending ping message!")
			buffer, err := protobuf.NewMsgProtobuf(enums.EMSG_PING, nil)
			err = clientConn.WriteMessage(websocket.BinaryMessage, buffer.Bytes())
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}
