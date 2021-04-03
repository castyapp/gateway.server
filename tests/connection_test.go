package tests

//func TestUserGatewayConnection(t *testing.T) {
//t.Run("connection-test", func(t *testing.T) {

//server := httptest.NewServer(hub.NewUserHub())
//t.Logf("Gateway Server created: %s", server.URL)
//defer server.Close()

//wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/user"
//t.Logf("Dialing gateway server: %s", wsURL)

//ws, response, err := websocket.DefaultDialer.Dial(wsURL, nil)
//if err != nil {
//t.Fatalf("could not open a ws connection on %s %v", wsURL, err)
//}

//defer ws.Close()

//if response.StatusCode != http.StatusSwitchingProtocols {
//t.Errorf("Dialing response status code expected %d got %d", http.StatusSwitchingProtocols, response.StatusCode)
//}

//t.Log("Waiting for response from gateway server..")
//_, data, err := ws.ReadMessage()
//if err != nil {
//t.Errorf("got error while reading messages from ws :%v", err)
//}

//packet, err := protocol.NewPacket(data)
//if err != nil {
//t.Errorf("could not decode packet :%v", err)
//}

//t.Logf("Got a new packet %d", packet.EMsg)
//})
//}
