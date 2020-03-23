package main

import (
	"github.com/CastyLab/gateway.server/hub/protocol"
	"github.com/CastyLab/gateway.server/hub/protocol/protobuf"
	"github.com/CastyLab/gateway.server/hub/protocol/protobuf/enums"
	"github.com/gorilla/websocket"
	"log"
	"time"
)

func main() {

	accessToken := "1eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE1ODUwODU1MzIsInN1YiI6IjVlNjQzNzJkYjM5MTM5OWI0N2M4YjA1NCJ9.V8qgH9HSngT8PrrNMk8Z_oxrxKsHsg8hXHnaf-w4OMeU-h9Rh64gYY-657Pzd1Gb3clpQHQe5kTqghRpeGu80Xth5XLFwpFs1ql1OQjFf28Iyhz04vN8-JwgVGaVPG70L7Eo_W-mxxPqxy-33b2OwmrYJmzmB2OnhgTRzUNt4TeIWtBwmFRVpSveeQwKKnJ462j8Q3p1ZsKgDdw1neXK3I3npc34-qX0IIMakoBhWcYM1ID-PNrRHwk41wDQrRMo8d_xqtiZXGD7gE3UEVhTjItLoVM_ZuXqUOYVivQZwvzqz5_E75RuoGfJDBM5cqlTbTqSyM0j-xLOSKschBg3ltEelCKAvaAtjvmac-e24raoqH-pH9rVGKMR5GZcTy2TiVF51KgzJ6XGThwUROa7XdY5HE1P8CFpcQ29lUcm75MtIZ25FagK2RhBZIF9v3Db9WnWQTzgXt6BZBFGjPRJtJ0QjadNXovXGJ5lhzsQTz15VENba6i4_Pky8rZyZtZo8DE4g9TEWNYh-H7ylo_gZrZDjZ_6-o8RpC21Co2YwPXHKuNWTuCin0h-ix2EHHYTAYLDVWM0Buv7H9AyoNdaFNIjxp3kCNNpPN7k7_PNXQPWiKxY87JKpM4zrJmfyRFs2Vn9H-9Cwr5PfWmTWvZmRU2ivg6bbFdUwQPkf_XZYG8"

	websocket.DefaultDialer.Subprotocols = []string{"cp0", "cp1"}
	clientConn, _, err := websocket.DefaultDialer.Dial("ws://localhost:3000/user", nil)
	if err != nil {
		log.Fatal(err)
	}

	buffer, err := protobuf.NewMsgProtobuf(enums.EMSG_LOGON, &protobuf.LogOnEvent{
		Token: []byte(accessToken),
	})
	if err != nil {
		log.Fatal(err)
	}

	err = clientConn.WriteMessage(websocket.BinaryMessage, buffer.Bytes())
	if err != nil {
		log.Fatal(err)
	}

	ticket := time.NewTicker(25 * time.Second)

	go func() {

		for {
			_, data, err := clientConn.ReadMessage()
			if err != nil {
				log.Println(err)
				return
			}
			packet, err := protocol.NewPacket(data)
			if err != nil {
				log.Println("Error while creating new packet: ", err)
				continue
			}
			log.Println(packet)
		}

	}()

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
