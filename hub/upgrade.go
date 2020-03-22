package hub

import (
	"github.com/gorilla/websocket"
	"net/http"
)

func newUpgrader() websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		Subprotocols: []string{
			"cp0",
			"cp1",
		},
		EnableCompression: true,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
}