package hub

import (
	"net/http"

	"github.com/gorilla/websocket"
)

var Subprotocols = []string{"cp0", "cp1"}

func newUpgrader() websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:    4096,
		WriteBufferSize:   4096,
		Subprotocols:      Subprotocols,
		EnableCompression: true,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
}
