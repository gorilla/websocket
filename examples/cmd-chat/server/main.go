package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var connections = make(map[string]*websocket.Conn)

func handler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		fmt.Println("Upgrade error:", err)
		return
	}

	connUserId := r.Header.Get("user-id")
	connections[connUserId] = conn

	alterCloseHandlerToRemoveConnections(conn, connUserId)

	go func() {
		defer conn.Close()
		for {
			_, p, err := conn.ReadMessage()
			if err != nil {
				switch {
				case websocket.IsCloseError(err, websocket.CloseNormalClosure):
					err = broadcastMessage(fmt.Sprintf("User %s left the chat\n", connUserId), connUserId)
					if err != nil {
						conn.Close()
					}
				default:
					fmt.Println(err)
					err = broadcastMessage(fmt.Sprintf("User %s disconnected unexpectedly\n", connUserId), connUserId)
					if err != nil {
						conn.Close()
					}
					conn.CloseHandler()(websocket.CloseNormalClosure, "")
				}
				return
			}
			err = broadcastMessage(fmt.Sprintf("%s: %s", connUserId, p), connUserId)
			if err != nil {
				conn.Close()
			}
		}
	}()
}

func main() {
	http.HandleFunc("/", handler)
	fmt.Println("Server started on :4000")
	err := http.ListenAndServe(":4000", nil)
	if err != nil {
		fmt.Println(err)
	}
}

func broadcastMessage(message, connUserId string) error {
	for cuid, con := range connections {
		if cuid != connUserId {
			err := con.WriteMessage(websocket.TextMessage, []byte(message))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func alterCloseHandlerToRemoveConnections(conn *websocket.Conn, connUserId string) {
	conn.SetCloseHandler(func(code int, text string) error {
		delete(connections, connUserId)
		message := websocket.FormatCloseMessage(code, "")
		if err := conn.WriteControl(websocket.CloseMessage, message, time.Now().Add(time.Second)); err != nil {
			return err
		}
		return nil
	})
}
