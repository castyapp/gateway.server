package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/CastyLab/grpc.proto/proto"
	"github.com/CastyLab/grpc.proto/protocol"
	"github.com/gorilla/websocket"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

func main() {

	client := flag.String("client", "1", "")
	flag.Parse()

	var accessToken string
	var friendID string

	switch *client {
	case "1":
		friendID = "test"
		accessToken = "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE1ODYzMTYxNjEsInN1YiI6IjVlODdjNWU1MWI2YmFiOTYyYzg5ZmM5MiJ9.jFwkMoxs4JW1deL96khANhO-m1EFQlB6syCskV9jkbhacUKgy2_4qpoJ9J4z3Fi1XkOaDd1y8zx4TAddc0F4xMUyonO3W4nhjSROblP16ak3Mh5i-_g4WSZoTN7H32v2H2bwR_GiHTZcM4i5Pxdk34JGg_zgKlT5Qm72vzEUKlLfgdMP7sjZsQJkxfztWRtwotahimz3W6U4AFT-0Qfy6CY6M14erqgc6cDA_bhlVvt0s9ZSYvAFd5sI3clxSpiQ35EHDlzlHwB6qVV8BxVtkc7dw8oGZC6mLSO8WV8v4cIm6mwKfhFRahiiiREL0FE51cEH0J6S2p6iANK32RrzKGRZ_7HaS5W_BrDNxXiabCtfkduhryu4mezmRIpgNslMd8I3MFLZqYGkYSRQTETp2wJKtlMgxxs1kSvNXESPDTFvC3r5RA355keBU_fhMi8EMOnn6lFzYvXosu2cDy8lnBB5jejX1rKyBccjEBy3QXS1Xid9-Sxz7OEDO0Opd3XLLsokTMzeDHlbOIJGgmb4ljKZ2zmB22nNtZPLMOvpU16I9ICyelRtqP89pwvIRVROFyzOzDbNDrDZ1Q9gP1slObUfHJJ4g4pd0wyH3jCoc1yX5fcjRZ71t-peD_A-KwaAaxwyjv_6p4q5hk6b6DBSKrbkkI6j0fkka3m1l_ch1R8"
	case "2":
		friendID = "josh"
		accessToken = "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE1ODYzMzYxNzAsInN1YiI6IjVlODdjYWNkMWI2YmFiOTYyYzg5ZmM5OSJ9.bWA-sHY8exJqR8_KvEm2-ZkxBwvx-52Ztt31rRH8lMvaDlCZ_ExCksOyVwkNNSBK9vrutjDkfN09ZJRZWoCeRoOBP6mDLBXt1jU5I86-1XqwdgSPPegg703fSo__G8k9nnP8vMnWYrPio4bEopBDyPc8UIV8-VHFa5R8lAN4625Gddl0UJhYtMv9Ve9Dg2aqTqNMjwvs5YIqMBUXIGaEZMB1NrIrJR9_q7__RGl04XK5XJJss3TEW-7dxUrj1jg4bwaLmDJaITkKTidIYdflxe_xa8jQTsEZ0XPJkNvy_5je837LRjV-97DUlMDMthyEZaEoUJteZ4l3xvL4D6YxxXFQRL1-pfXCDVwZEmRUgEsV6fIkF45xzmrEdvOUODBMieNjgkOLHe4um3X_mRrlbAcvayfDyUtk4Cw6pMgsJltlYtKFOOkAkrCPUjK0JidFceJ_HDEl3yBzzAqJnryNKrPVkHKm4VSVqE3xKXMOmsvVGHywVNuK18nlTbURnp3ojrjkjOR7OdkT2ZomhsJtDp-94tfshj5fD5s6DNtrSnLMCSdoPU4niFTXxTYlrd1M2sEFvmbY-GkgEiRzDuSlDiw9rureFMX8pf_K-iW0Z6JwefqWeaodBHdjX7YKWxu_B1KgJk2Ngy_aKeqI21ScOr1TTwSYUnHWA27Ji703dsE"
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)

	websocket.DefaultDialer.Subprotocols = []string{"cp0", "cp1"}
	clientConn, _, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:3000/user", nil)
	if err != nil {
		log.Fatal(err)
	}

	buffer, err := protocol.NewMsgProtobuf(proto.EMSG_LOGON, &proto.LogOnEvent{
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

			if packet.EMsg == proto.EMSG_AUTHORIZED {
				wg.Done()
			}

		}

	}()

	go func() {

		for {
			select {
			case <-ticket.C:
				log.Println("Sending ping message!")
				buffer, err := protocol.NewMsgProtobuf(proto.EMSG_PING, nil)
				err = clientConn.WriteMessage(websocket.BinaryMessage, buffer.Bytes())
				if err != nil {
					log.Fatal(err)
				}
			}
		}

	}()

	wg.Wait()

	go func() {

		for {
			reader := bufio.NewReader(os.Stdin)
			fmt.Println()
			fmt.Print("Enter text: ")
			text, _ := reader.ReadString('\n')

			msg := &proto.ChatMsgEvent{
				Message: []byte(text),
				To:      friendID,
			}

			buffer, err := protocol.NewMsgProtobuf(proto.EMSG_NEW_CHAT_MESSAGE, msg)
			err = clientConn.WriteMessage(websocket.BinaryMessage, buffer.Bytes())
			if err != nil {
				log.Fatal(err)
			}

			fmt.Printf("Sending message to [%s]: %s", friendID, strings.TrimSpace(text))
		}

	}()

	select {}

}
