// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (

	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512

	// Name of the broadcast channel
	BcPrefix = "#all"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// connection is an middleman between the websocket connection and the hub.
type connection struct {
	// The websocket connection.
	ws *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte
}

type msgIn struct {
	Chan string
	Msg  string
}

func (c *connection) startPumps() {

	c.ws.SetReadLimit(maxMessageSize)
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error { c.ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	//likely slow-start message miss problem here
	go c.readPump()
	go c.writePump()

}

// readPump pumps messages from the websocket connection to the hub.
func (c *connection) readPump() {

	for {

		var msg msgIn

		err := c.ws.ReadJSON(&msg)
		if err != nil {
			log.Printf("error reading message:shutting down connection:err:%s:", err)
			c.write(websocket.CloseMessage, []byte{})
			h.unregister <- c
			return
		}

		log.Printf("message:%s:\n", msg)

		if msg.Chan == BcPrefix {
			h.broadcast <- []byte(msg.Msg)
		} else {
			log.Printf("No target channel found in message:msg:%s:", msg)
		}

	}
}

// write writes a message with the given message type and payload.
func (c *connection) write(mt int, payload []byte) error {
	c.ws.SetWriteDeadline(time.Now().Add(writeWait))
	return c.ws.WriteMessage(mt, payload)
}

// writePump pumps messages from the hub to the websocket connection.
func (c *connection) writePump() {

	ticker := time.NewTicker(pingPeriod)

	for {
		select {
		case message, ok := <-c.send:

			if !ok {
				log.Printf("error reading from send channel:shutting it down:")
				c.write(websocket.CloseMessage, []byte{})
				h.unregister <- c
				return
			}

			if err := c.write(websocket.TextMessage, message); err != nil {
				log.Printf("error writing to channel:shutting it down:")
				h.unregister <- c
				return
			} else {
				log.Printf("writing to channel:")
			}

		case <-ticker.C:
			if err := c.write(websocket.PingMessage, []byte{}); err != nil {
				log.Printf("error pinging to channel:shutting it down:")
				h.unregister <- c
				return
			}
		}
	}

}

// serveWs handles websocket requests from the peer.
func serveWs(w http.ResponseWriter, r *http.Request) {

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	c := &connection{send: make(chan []byte, 256), ws: ws}
	h.register <- c

}
