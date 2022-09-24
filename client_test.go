// Copyright 2014 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package websocket

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"
)

var hostPortNoPortTests = []struct {
	u                    *url.URL
	hostPort, hostNoPort string
}{
	{&url.URL{Scheme: "ws", Host: "example.com"}, "example.com:80", "example.com"},
	{&url.URL{Scheme: "wss", Host: "example.com"}, "example.com:443", "example.com"},
	{&url.URL{Scheme: "ws", Host: "example.com:7777"}, "example.com:7777", "example.com"},
	{&url.URL{Scheme: "wss", Host: "example.com:7777"}, "example.com:7777", "example.com"},
}

func TestHostPortNoPort(t *testing.T) {
	for _, tt := range hostPortNoPortTests {
		hostPort, hostNoPort := hostPortNoPort(tt.u)
		if hostPort != tt.hostPort {
			t.Errorf("hostPortNoPort(%v) returned hostPort %q, want %q", tt.u, hostPort, tt.hostPort)
		}
		if hostNoPort != tt.hostNoPort {
			t.Errorf("hostPortNoPort(%v) returned hostNoPort %q, want %q", tt.u, hostNoPort, tt.hostNoPort)
		}
	}
}

func TestWsServer(t *testing.T) {
	u := &Upgrader{}
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		conn, err := u.Upgrade(writer, request, nil)
		if err != nil {
			writer.Write([]byte(err.Error()))
			fmt.Println("err upgrade", err)
			return
		}
		defer conn.Close()
		_, msg, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("err read message", err)
			return
		}
		conn.WriteMessage(TextMessage, []byte("hello world response："+string(msg)))
	})
	http.ListenAndServe(":8888", nil)
}

func TestAsyncDial(t *testing.T) {
	conn, err := DefaultDialer.DialContextAsync(context.Background(), "ws://127.0.0.1:8888", nil, nil)
	if err != nil {
		panic(err)
	}
	conn.WriteMessage(TextMessage, []byte("hello"))

	err = conn.AsyncWait()
	if err != nil {
		panic(err)
	}

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("error:", err)
			return
		}
		fmt.Println(string(msg))
	}
}
