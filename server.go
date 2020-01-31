package main

import (
	"fmt"
	"github.com/gorilla/mux"
	_ "github.com/joho/godotenv/autoload"
	"log"
	wsHttp "movie.night.ws.server/http"
	"movie.night.ws.server/hub"
	"net"
	"net/http"
)

func main() {

	log.SetFlags(log.Lshortfile | log.Ltime)

	var (
		router     = mux.NewRouter()
		userhub    = hub.NewUserHub()
	    theaterhub = hub.NewTheaterHub(userhub)
		port       = 3000
	)

	unixListener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		panic(err)
	}

	router.HandleFunc("/user", userhub.Handler).Methods("GET")

	router.HandleFunc("/user/events/{event}", func(w http.ResponseWriter, r *http.Request) {
		wsHttp.Handler(userhub, w, r)
	}).Methods("POST")

	router.HandleFunc("/theater", theaterhub.Handler).Methods("GET")

	http.Handle("/", router)

	defer unixListener.Close()

	log.Printf("Server running and listeting on :%d", port)
	log.Printf("http_err: %v", http.Serve(unixListener, nil))
}