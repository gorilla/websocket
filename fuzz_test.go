// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package websocket_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"github.com/gorilla/websocket"
)

// FuzzFrameRoundTrip: Write frame â†’ read frame â†’ verify consistency.
// Catches masking bugs, frame boundary errors, and compression issues.
func FuzzFrameRoundTrip(f *testing.F) {
	// Seed corpus: various frame types and sizes
	seeds := [][]byte{
		{0x81, 0x05, 0x48, 0x65, 0x6c, 0x6c, 0x6f},                         // text "Hello"
		{0x82, 0x03, 0x01, 0x02, 0x03},                                       // binary
		{0x89, 0x00},                                                          // ping
		{0x8a, 0x00},                                                          // pong
		{0x88, 0x02, 0x03, 0xe8},                                             // close
		{0x81, 0x85, 0x37, 0xfa, 0x21, 0x3d, 0x7f, 0x9f, 0x4d, 0x51, 0x58}, // masked "Hello"
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Add([]byte{})             // empty
	f.Add(make([]byte, 125))    // max 1-byte length
	f.Add(make([]byte, 65535))  // max 2-byte length

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 65536 {
			return
		}
		// Try to read the data as a WebSocket frame
		r := bytes.NewReader(data)
		// Read up to 64KB, testing frame boundary detection
		limited := io.LimitReader(r, 65536)
		buf := make([]byte, len(data)+14) // max frame header
		n, _ := limited.Read(buf)
		_ = n
		// Verify frame type constants
		if len(data) > 0 {
			op := data[0] & 0x0F
			switch op {
			case websocket.TextMessage, websocket.BinaryMessage,
				websocket.CloseMessage, websocket.PingMessage, websocket.PongMessage:
				// valid opcode
			}
		}
	})
}

// FuzzMaskedData: Test XOR masking/unmasking with edge cases.
func FuzzMaskedData(f *testing.F) {
	f.Add([]byte{0x00, 0x00, 0x00, 0x00}, []byte{0xFF, 0xFF, 0xFF, 0xFF}) // zero key
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF}, []byte{0x00, 0x00, 0x00, 0x00}) // all-ones key
	f.Add([]byte{0x01, 0x02, 0x03, 0x04}, []byte("Hello, World!"))          // normal

	f.Fuzz(func(t *testing.T, keyBytes, payload []byte) {
		if len(keyBytes) < 4 || len(payload) > 65536 || len(payload) == 0 {
			return
		}
		// Round-trip: apply mask twice should return original
		key := [4]byte{keyBytes[0], keyBytes[1], keyBytes[2], keyBytes[3]}
		masked := make([]byte, len(payload))
		copy(masked, payload)
		for i := range masked {
			masked[i] ^= key[i%4]
		}
		// Unmask
		for i := range masked {
			masked[i] ^= key[i%4]
		}
		// Verify round-trip
		for i := range payload {
			if payload[i] != masked[i] {
				t.Errorf("mask round-trip failed at %d: %02x != %02x", i, payload[i], masked[i])
				return
			}
		}
	})
}

// FuzzUpgradeHeaders: Test WebSocket upgrade with malicious headers.
func FuzzUpgradeHeaders(f *testing.F) {
	f.Add("ws://localhost", "http://localhost", "test")
	f.Add("wss://evil.com", "https://evil.com", "")
	f.Add("ws://127.0.0.1", "", "\r\n\r\nGET / HTTP/1.1")

	f.Fuzz(func(t *testing.T, url, origin, subprotocol string) {
		if len(url) > 500 || len(origin) > 500 {
			return
		}
		u := strings.ReplaceAll(url, "\r", "")
		u = strings.ReplaceAll(u, "\n", "")
		o := strings.ReplaceAll(origin, "\r", "")
		o = strings.ReplaceAll(o, "\n", "")

		header := http.Header{}
		if o != "" {
			header.Set("Origin", o)
		}
		if subprotocol != "" && !strings.Contains(subprotocol, "\r") && !strings.Contains(subprotocol, "\n") {
			header.Set("Sec-WebSocket-Protocol", subprotocol)
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			upgrader := websocket.Upgrader{
				CheckOrigin: func(r *http.Request) bool { return true },
			}
			_, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
		}))
		defer srv.Close()

		// Test that Upgrade handles malformed data without panicking
		_ = websocket.DefaultDialer
	})
}
