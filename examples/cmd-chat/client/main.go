package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"

	"github.com/gorilla/websocket"
)

func main() {
	connUserId := rand.Intn(100)
	header := http.Header{}
	header.Add("user-id", fmt.Sprint(connUserId))

	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial("ws://localhost:4000", header)
	if err != nil {
		err := conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, err.Error()))
		if err != nil {
			conn.Close()
			os.Exit(1)
		}
		os.Exit(0)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for range c {
			err := conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure,
					fmt.Sprintf("User %d closing connection", connUserId)))
			if err != nil {
				conn.Close()
				os.Exit(1)
			}
			os.Exit(0)
		}
	}()

	reader := bufio.NewReader(os.Stdin)
	go func() {
		for {
			fmt.Print(fmt.Sprintf("%d: ", connUserId))
			text, _ := reader.ReadString('\n')

			err = conn.WriteMessage(websocket.TextMessage, []byte(text))
			if err != nil {
				err := conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseInternalServerErr, err.Error()))
				if err != nil {
					conn.Close()
					os.Exit(1)
				}
				os.Exit(0)
			}
		}
	}()

	for {
		_, p, err := conn.ReadMessage()
		if err != nil {
			err := conn.WriteMessage(websocket.CloseInternalServerErr,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, err.Error()))
			if err != nil {
				conn.Close()
				os.Exit(1)
			}
			os.Exit(0)
		}
		fmt.Printf("\n%s", p)
		fmt.Print(fmt.Sprintf("%d: ", connUserId))

	}

}
