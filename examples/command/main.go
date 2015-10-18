// Copyright 2015 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"text/template"
	"time"

	"github.com/gorilla/websocket"
)

var (
	addr      = flag.String("addr", "127.0.0.1:8080", "http service address")
	homeTempl = template.Must(template.ParseFiles("home.html"))
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Maximum message size allowed from peer.
	maxMessageSize = 8192
)

// connection is an middleman between the websocket connection and the command.
type connection struct {
	ws     *websocket.Conn
	stdout io.ReadCloser
	stdin  io.WriteCloser
	cmd    *exec.Cmd
}

func (c *connection) pumpStdin() {
	defer c.ws.Close()
	c.ws.SetReadLimit(maxMessageSize)
	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			break
		}
		message = append(message, '\n')
		if _, err := c.stdin.Write(message); err != nil {
			break
		}
	}
	c.stdin.Close()
	log.Println("exit stdin pump")
}

func (c *connection) pumpStdout() {
	defer c.ws.Close()
	s := bufio.NewScanner(c.stdout)
	for s.Scan() {
		c.ws.SetWriteDeadline(time.Now().Add(writeWait))
		if err := c.ws.WriteMessage(websocket.TextMessage, s.Bytes()); err != nil {
			break
		}
	}
	if s.Err() != nil {
		log.Println("scan:", s.Err())
	}
	c.stdout.Close()
	log.Println("exit stdout pump")
}

func internalError(ws *websocket.Conn, fmt string, err error) {
	log.Println(fmt, err)
	ws.WriteMessage(websocket.TextMessage, []byte("Internal server error."))
}

var upgrader = websocket.Upgrader{}

func serveWs(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	c := &connection{
		cmd: exec.Command(flag.Args()[0], flag.Args()[1:]...),
		ws:  ws,
	}

	c.stdout, err = c.cmd.StdoutPipe()
	if err != nil {
		internalError(ws, "stdout: %v", err)
		ws.Close()
		return
	}

	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		internalError(ws, "stdin: %v", err)
		c.stdout.Close()
		if closer, ok := c.cmd.Stdout.(io.Closer); ok {
			closer.Close()
		}
		ws.Close()
		return
	}

	if err := c.cmd.Start(); err != nil {
		internalError(ws, "start: %v", err)
		c.stdout.Close()
		c.stdin.Close()
		ws.Close()
		return
	}

	go c.pumpStdout()
	c.pumpStdin()

	c.cmd.Process.Signal(os.Interrupt)
	if err := c.cmd.Wait(); err != nil {
		log.Println("wait:", err)
	}
	log.Println("exit serveWs")
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Not found", 404)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	homeTempl.Execute(w, r.Host)
}

func main() {
	flag.Parse()
	if len(flag.Args()) < 1 {
		log.Fatal("must specify at least one argument")
	}
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", serveWs)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
