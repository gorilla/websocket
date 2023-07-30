// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package websocket

import (
	"bufio"
	"bytes"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

var subprotocolTests = []struct {
	h         string
	protocols []string
}{
	{"", nil},
	{"foo", []string{"foo"}},
	{"foo,bar", []string{"foo", "bar"}},
	{"foo, bar", []string{"foo", "bar"}},
	{" foo, bar", []string{"foo", "bar"}},
	{" foo, bar ", []string{"foo", "bar"}},
}

func TestSubprotocols(t *testing.T) {
	for _, st := range subprotocolTests {
		r := http.Request{Header: http.Header{"Sec-Websocket-Protocol": {st.h}}}
		protocols := Subprotocols(&r)
		if !reflect.DeepEqual(st.protocols, protocols) {
			t.Errorf("SubProtocols(%q) returned %#v, want %#v", st.h, protocols, st.protocols)
		}
	}
}

var isWebSocketUpgradeTests = []struct {
	ok bool
	h  http.Header
}{
	{false, http.Header{"Upgrade": {"websocket"}}},
	{false, http.Header{"Connection": {"upgrade"}}},
	{true, http.Header{"Connection": {"upgRade"}, "Upgrade": {"WebSocket"}}},
}

func TestIsWebSocketUpgrade(t *testing.T) {
	for _, tt := range isWebSocketUpgradeTests {
		ok := IsWebSocketUpgrade(&http.Request{Header: tt.h})
		if tt.ok != ok {
			t.Errorf("IsWebSocketUpgrade(%v) returned %v, want %v", tt.h, ok, tt.ok)
		}
	}
}

var checkSameOriginTests = []struct {
	ok bool
	r  *http.Request
}{
	{false, &http.Request{Host: "example.org", Header: map[string][]string{"Origin": {"https://other.org"}}}},
	{true, &http.Request{Host: "example.org", Header: map[string][]string{"Origin": {"https://example.org"}}}},
	{true, &http.Request{Host: "Example.org", Header: map[string][]string{"Origin": {"https://example.org"}}}},
}

func TestCheckSameOrigin(t *testing.T) {
	for _, tt := range checkSameOriginTests {
		ok := checkSameOrigin(tt.r)
		if tt.ok != ok {
			t.Errorf("checkSameOrigin(%+v) returned %v, want %v", tt.r, ok, tt.ok)
		}
	}
}

type reuseTestResponseWriter struct {
	brw *bufio.ReadWriter
	http.ResponseWriter
}

func (resp *reuseTestResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return fakeNetConn{strings.NewReader(""), &bytes.Buffer{}}, resp.brw, nil
}

var bufioReuseTests = []struct {
	n     int
	reuse bool
}{
	{4096, true},
	{128, false},
}

func TestBufioReuse(t *testing.T) {
	for i, tt := range bufioReuseTests {
		br := bufio.NewReaderSize(strings.NewReader(""), tt.n)
		bw := bufio.NewWriterSize(&bytes.Buffer{}, tt.n)
		resp := &reuseTestResponseWriter{
			brw: bufio.NewReadWriter(br, bw),
		}
		upgrader := Upgrader{}
		c, err := upgrader.Upgrade(resp, &http.Request{
			Method: http.MethodGet,
			Header: http.Header{
				"Upgrade":               []string{"websocket"},
				"Connection":            []string{"upgrade"},
				"Sec-Websocket-Key":     []string{"dGhlIHNhbXBsZSBub25jZQ=="},
				"Sec-Websocket-Version": []string{"13"},
			}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if reuse := c.br == br; reuse != tt.reuse {
			t.Errorf("%d: buffered reader reuse=%v, want %v", i, reuse, tt.reuse)
		}
		writeBuf := bufioWriterBuffer(c.NetConn(), bw)
		if reuse := &c.writeBuf[0] == &writeBuf[0]; reuse != tt.reuse {
			t.Errorf("%d: write buffer reuse=%v, want %v", i, reuse, tt.reuse)
		}
	}
}

var negotiateSubprotocolTests = []struct {
	*Upgrader
	match     bool
	shouldErr bool
}{
	{
		&Upgrader{
			NegotiateSubprotocol: func(r *http.Request) (s string, err error) { return "json", nil },
		}, true, false,
	},
	{
		&Upgrader{
			Subprotocols: []string{"json"},
		}, true, false,
	},
	{
		&Upgrader{
			Subprotocols: []string{"not-match"},
		}, false, false,
	},
	{
		&Upgrader{
			NegotiateSubprotocol: func(r *http.Request) (s string, err error) { return "", errors.New("not-match") },
		}, false, true,
	},
}

func TestNegotiateSubprotocol(t *testing.T) {
	for i := range negotiateSubprotocolTests {
		upgrade := negotiateSubprotocolTests[i].Upgrader

		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			upgrade.Upgrade(w, r, nil)
		}))

		req, err := http.NewRequest("GET", s.URL, strings.NewReader(""))
		if err != nil {
			t.Fatalf("NewRequest retuened error %v", err)
		}

		req.Header.Set("Connection", "upgrade")
		req.Header.Set("Upgrade", "websocket")
		req.Header.Set("Sec-Websocket-Version", "13")
		req.Header.Set("Sec-Websocket-Protocol", "json")
		req.Header.Set("Sec-Websocket-key", "dGhlIHNhbXBsZSBub25jZQ==")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Do returned error %v", err)
		}

		if negotiateSubprotocolTests[i].shouldErr && resp.StatusCode != http.StatusBadRequest {
			t.Errorf("The expecred status code is %d,actual status code is %d", http.StatusBadRequest, resp.StatusCode)
		} else {
			if negotiateSubprotocolTests[i].match {
				protocol := resp.Header.Get("Sec-Websocket-Protocol")
				if protocol != "json" {
					t.Errorf("Negotiation protocol failed,request protocol is json,reponese protocol is %s", protocol)
				}
			} else {
				if _, ok := resp.Header["Sec-Websocket-Protocol"]; ok {
					t.Errorf("Negotiation protocol failed,Sec-Websocket-Protocol field should be empty")
				}
			}
		}
		s.Close()
		resp.Body.Close()
	}

}
