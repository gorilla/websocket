// Copyright 2013 Gary Burd. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package websocket_test

import (
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type wsHandler struct {
	*testing.T
}

func (t wsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		t.Logf("bad method: %s", r.Method)
		return
	}
	if r.Header.Get("Origin") != "http://"+r.Host {
		http.Error(w, "Origin not allowed", 403)
		t.Logf("bad origin: %s", r.Header.Get("Origin"))
		return
	}
	ws, err := websocket.Upgrade(w, r, http.Header{"Set-Cookie": {"sessionID=1234"}}, 1024, 1024)
	if _, ok := err.(websocket.HandshakeError); ok {
		t.Logf("bad handshake: %v", err)
		http.Error(w, "Not a websocket handshake", 400)
		return
	} else if err != nil {
		t.Logf("upgrade error: %v", err)
		return
	}
	defer ws.Close()
	for {
		op, r, err := ws.NextReader()
		if err != nil {
			if err != io.EOF {
				t.Logf("NextReader: %v", err)
			}
			return
		}
		if op == websocket.PongMessage {
			continue
		}
		w, err := ws.NextWriter(op)
		if err != nil {
			t.Logf("NextWriter: %v", err)
			return
		}
		if _, err = io.Copy(w, r); err != nil {
			t.Logf("Copy: %v", err)
			return
		}
		if err := w.Close(); err != nil {
			t.Logf("Close: %v", err)
			return
		}
	}
}

func TestClientServer(t *testing.T) {
	s := httptest.NewServer(wsHandler{t})
	defer s.Close()
	u, _ := url.Parse(s.URL)
	c, err := net.Dial("tcp", u.Host)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	ws, resp, err := websocket.NewClient(c, u, http.Header{"Origin": {s.URL}}, 1024, 1024)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer ws.Close()

	var sessionID string
	for _, c := range resp.Cookies() {
		if c.Name == "sessionID" {
			sessionID = c.Value
		}
	}
	if sessionID != "1234" {
		t.Error("Set-Cookie not received from the server.")
	}

	w, _ := ws.NextWriter(websocket.TextMessage)
	io.WriteString(w, "HELLO")
	w.Close()
	ws.SetReadDeadline(time.Now().Add(1 * time.Second))
	op, r, err := ws.NextReader()
	if err != nil {
		t.Fatalf("NextReader: %v", err)
	}
	if op != websocket.TextMessage {
		t.Fatalf("op=%d, want %d", op, websocket.TextMessage)
	}
	b, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(b) != "HELLO" {
		t.Fatalf("message=%s, want %s", b, "HELLO")
	}
}
