// +build ignore

package main

import (
	"log"
	"net/http"
	"time"

	"github.com/euforia/websocket"
)

func main() {
	dialer := websocket.Dialer{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		Extensions:      []string{"permessage-deflate"},
		Proxy:           http.ProxyFromEnvironment,
	}

	c, respHdr, err := dialer.Dial("ws://localhost:9001/f", nil)

	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	log.Printf("Extensions: %s\n", respHdr.Header.Get("Sec-Websocket-Extensions"))

	compressEnabled := true

	go func() {
		defer c.Close()
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				break
			}
			log.Printf("Received: %s", message)
		}
	}()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for t := range ticker.C {
		err := c.WriteMessage(websocket.TextMessage, []byte(t.String()))
		if err != nil {
			log.Println("write:", err)
			break
		}

		log.Printf("Wrote: compressed=%v; value=%s\n", compressEnabled, t.String())

		compressEnabled = !compressEnabled
		c.EnableWriteCompression(compressEnabled)
	}
}
