// Copyright 2013 Gary Burd. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package websocket

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

type handshakeHandler struct {
	*testing.T
}

func (t handshakeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	ws, err := Upgrade(w, r, http.Header{"Set-Cookie": {"sessionID=1234"}}, 1024, 1024)
	if _, ok := err.(HandshakeError); ok {
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

func TestHandshake(t *testing.T) {
	s := httptest.NewServer(handshakeHandler{t})
	defer s.Close()
	u, _ := url.Parse(s.URL)
	c, err := net.Dial("tcp", u.Host)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	ws, resp, err := NewClient(c, u, http.Header{"Origin": {s.URL}}, 1024, 1024)
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

	w, _ := ws.NextWriter(TextMessage)
	io.WriteString(w, "HELLO")
	w.Close()
	ws.SetReadDeadline(time.Now().Add(1 * time.Second))
	op, r, err := ws.NextReader()
	if err != nil {
		t.Fatalf("NextReader: %v", err)
	}
	if op != TextMessage {
		t.Fatalf("op=%d, want %d", op, TextMessage)
	}
	b, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(b) != "HELLO" {
		t.Fatalf("message=%s, want %s", b, "HELLO")
	}
}

type dialHandler struct {
	*testing.T
}

func (t dialHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ws, err := Upgrade(w, r, nil, 1024, 1024)
	if _, ok := err.(HandshakeError); ok {
		t.Logf("bad handshake: %v", err)
		http.Error(w, "Not a websocket handshake", 400)
		return
	} else if err != nil {
		t.Logf("upgrade error: %v", err)
		return
	}
	defer ws.Close()
	for {
		mt, p, err := ws.ReadMessage()
		if err != nil {
			if err != io.EOF {
				t.Logf("ReadMessage: %v", err)
			}
			return
		}
		if err := ws.WriteMessage(mt, p); err != nil {
			t.Logf("WriteMessage: %v", err)
			return
		}
	}
}

func sendRecv(t *testing.T, ws *Conn) {
	const message = "Hello World!"
	if err := ws.WriteMessage(TextMessage, []byte(message)); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
	_, p, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if string(p) != message {
		t.Fatalf("message=%s, want %s", p, message)
	}
}

func httpToWs(u string) string {
	return "ws" + u[len("http"):]
}

func TestDial(t *testing.T) {
	s := httptest.NewServer(dialHandler{t})
	defer s.Close()
	ws, _, err := DefaultDialer.Dial(httpToWs(s.URL), nil)
	if err != nil {
		t.Fatalf("Dial() returned error %v", err)
	}
	defer ws.Close()
	sendRecv(t, ws)
}

func TestDialTLS(t *testing.T) {
	s := httptest.NewTLSServer(dialHandler{t})
	defer s.Close()

	certs := x509.NewCertPool()
	for _, c := range s.TLS.Certificates {
		roots, err := x509.ParseCertificates(c.Certificate[len(c.Certificate)-1])
		if err != nil {
			t.Fatalf("error parsing server's root cert: %v", err)
		}
		for _, root := range roots {
			certs.AddCert(root)
		}
	}

	u, _ := url.Parse(s.URL)
	d := &Dialer{
		NetDial:         func(network, addr string) (net.Conn, error) { return net.Dial(network, u.Host) },
		TLSClientConfig: &tls.Config{RootCAs: certs},
	}
	ws, _, err := d.Dial("wss://example.com/", nil)
	if err != nil {
		t.Fatalf("Dial() returned error %v", err)
	}
	defer ws.Close()
	sendRecv(t, ws)
}

func TestDialTLSBadCert(t *testing.T) {
	s := httptest.NewTLSServer(dialHandler{t})
	defer s.Close()
	_, _, err := DefaultDialer.Dial(httpToWs(s.URL), nil)
	if err == nil {
		t.Fatalf("Dial() did not return error")
	}
}

func TestDialTLSNoVerify(t *testing.T) {
	s := httptest.NewTLSServer(dialHandler{t})
	defer s.Close()
	d := &Dialer{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	ws, _, err := d.Dial(httpToWs(s.URL), nil)
	if err != nil {
		t.Fatalf("Dial() returned error %v", err)
	}
	defer ws.Close()
	sendRecv(t, ws)
}

func TestDialTimeout(t *testing.T) {
	s := httptest.NewServer(dialHandler{t})
	defer s.Close()
	d := &Dialer{
		HandshakeTimeout: -1,
	}
	_, _, err := d.Dial(httpToWs(s.URL), nil)
	if err == nil {
		t.Fatalf("Dial() did not return error")
	}
}

func TestDialBadScheme(t *testing.T) {
	s := httptest.NewServer(dialHandler{t})
	defer s.Close()
	_, _, err := DefaultDialer.Dial(s.URL, nil)
	if err == nil {
		t.Fatalf("Dial() did not return error")
	}
}
