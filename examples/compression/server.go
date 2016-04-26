// +build ignore

package main

import (
	"io"
	"log"
	"net/http"
	"path/filepath"

	"github.com/euforia/websocket"
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(r *http.Request) bool { return true },
		Extensions:      []string{websocket.CompressPermessageDeflate},
	}
	webroot, _ = filepath.Abs("./")
	listenAddr = "0.0.0.0:9001"
)

func ServeWebSocket(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer ws.Close()

	log.Printf("Client connected: %s\n", r.RemoteAddr)

	//if err = ws.WriteMessage(websocket.TextMessage, []byte("Hello!")); err != nil {
	//	log.Println(err)
	//	err = nil
	//}

	for {
		/*
			if msgType, msgBytes, err = ws.ReadMessage(); err != nil {
				log.Printf("Client disconnected %s: %s\n", r.RemoteAddr, err)
				break
			}
			log.Printf("type: %d; payload: %d bytes;\n", msgType, len(msgBytes))
		*/

		msgType, rd, err := ws.NextReader()
		if err != nil {
			log.Printf("Client disconnected (%s): %s\n", r.RemoteAddr, err)
			break
		}

		wr, err := ws.NextWriter(msgType)
		if err != nil {
			log.Println(err)
			continue
		}

		if _, err = io.Copy(wr, rd); err != nil {
			log.Println(err)
		}

		if err = wr.Close(); err != nil {
			log.Println(err)
		}
	}
}

func main() {
	// Serve index.html
	http.Handle("/", http.FileServer(http.Dir(webroot)))
	// Websocket endpoint
	http.HandleFunc("/f", ServeWebSocket)

	log.Printf("Starting server on: %s\n", listenAddr)
	if err := http.ListenAndServe(listenAddr, nil); err != nil {
		log.Fatalln(err)
	}
}
