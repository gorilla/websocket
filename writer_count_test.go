// Copyright 2026 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package websocket

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type writeCountFailConn struct {
	net.Conn
	remaining int
	err       error
}

func (c *writeCountFailConn) Write(p []byte) (int, error) {
	if c.remaining == 0 {
		return 0, c.err
	}
	if c.remaining > 0 {
		c.remaining--
	}
	return c.Conn.Write(p)
}

func newWriteCountPair(t *testing.T, isServer bool) (*Conn, *Conn) {
	t.Helper()
	type result struct {
		conn *Conn
		err  error
	}
	accepted := make(chan result, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := Upgrader{WriteBufferSize: 8}
		conn, err := upgrader.Upgrade(w, r, nil)
		accepted <- result{conn, err}
	}))
	t.Cleanup(server.Close)
	dialer := Dialer{WriteBufferSize: 8, HandshakeTimeout: 5 * time.Second}
	client, _, err := dialer.Dial(makeWsProto(server.URL), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	var peer *Conn
	select {
	case got := <-accepted:
		if got.err != nil {
			t.Fatal(got.err)
		}
		peer = got.conn
	case <-time.After(5 * time.Second):
		t.Fatal("waiting for upgraded server connection")
	}
	t.Cleanup(func() { _ = peer.Close() })
	for _, conn := range []*Conn{client, peer} {
		if _, ok := conn.NetConn().(*net.TCPConn); !ok {
			t.Fatalf("underlying connection is %T, want *net.TCPConn", conn.NetConn())
		}
		if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Fatal(err)
		}
		if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	if isServer {
		return peer, client
	}
	return client, peer
}

func TestMessageWriterProgressOnFlushError(t *testing.T) {
	const payload = "0123456789abcdefghijklmnopqrst"
	tests := []struct {
		name              string
		prefix            string
		data              string
		writesBeforeError int
		wantCount         int64
		wantPayload       string
	}{
		{"partial", "", payload, 1, 16, payload[:8]},
		{"primed", "abc", payload, 1, 13, "abc" + payload[:5]},
		{"no-current-progress", "abcdefgh", payload, 0, 0, ""},
		{"success", "abc", payload, -1, int64(len(payload)), "abc" + payload},
		{"large-server-fast-path", "abc", strings.Repeat(payload, 3), 1, 0, "abc"},
	}
	for _, isServer := range []bool{false, true} {
		for _, method := range []string{"Write", "WriteString", "Copy"} {
			for _, tt := range tests {
				if tt.name == "large-server-fast-path" && (!isServer || method == "WriteString") {
					continue
				}
				t.Run(fmt.Sprintf("server=%t/%s/%s", isServer, method, tt.name), func(t *testing.T) {
					conn, peer := newWriteCountPair(t, isServer)
					writeErr := errors.New("test flush failure")
					conn.conn = &writeCountFailConn{Conn: conn.conn, remaining: tt.writesBeforeError, err: writeErr}
					writer, err := conn.NextWriter(BinaryMessage)
					if err != nil {
						t.Fatal(err)
					}
					if n, err := writer.Write([]byte(tt.prefix)); n != len(tt.prefix) || err != nil {
						t.Fatalf("priming Write = (%d, %v)", n, err)
					}
					var n int64
					switch method {
					case "Write":
						var count int
						count, err = writer.Write([]byte(tt.data))
						n = int64(count)
					case "WriteString":
						var count int
						count, err = io.WriteString(writer, tt.data)
						n = int64(count)
					case "Copy":
						n, err = io.Copy(writer, bytes.NewReader([]byte(tt.data)))
					}
					if n != tt.wantCount {
						t.Errorf("%s count = %d, want %d bytes from this call", method, n, tt.wantCount)
					}
					if tt.writesBeforeError >= 0 {
						if !errors.Is(err, writeErr) {
							t.Fatalf("%s error = %v, want original flush error", method, err)
						}
						if err := writer.Close(); !errors.Is(err, writeErr) {
							t.Errorf("Close error = %v, want original flush error", err)
						}
						if n, err := writer.Write([]byte("later")); n != 0 || !errors.Is(err, writeErr) {
							t.Errorf("Write after failure = (%d, %v), want (0, original error)", n, err)
						}
						if err := conn.WriteMessage(BinaryMessage, []byte("later")); !errors.Is(err, writeErr) {
							t.Errorf("new message error = %v, want original flush error", err)
						}
					} else {
						if err != nil {
							t.Fatal(err)
						}
						if err := writer.Close(); err != nil {
							t.Fatal(err)
						}
					}
					if err := conn.Close(); err != nil {
						t.Fatal(err)
					}
					messageType, got, err := peer.ReadMessage()
					if string(got) != tt.wantPayload {
						t.Errorf("payload received over TCP = %q, want %q", got, tt.wantPayload)
					}
					if tt.wantPayload != "" && messageType != BinaryMessage {
						t.Errorf("message type = %d, want BinaryMessage", messageType)
					}
					if tt.writesBeforeError >= 0 {
						if !IsCloseError(err, CloseAbnormalClosure) {
							t.Errorf("truncated message error = %v, want abnormal closure", err)
						}
					} else if err != nil {
						t.Errorf("successful message error = %v", err)
					}
				})
			}
		}
	}
}
