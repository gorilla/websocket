//go:build go1.15
// +build go1.15

package websocket

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"testing"
)

func TestHttpsProxy(t *testing.T) {

	sTLS := newTLSServer(t)
	defer sTLS.Close()
	s := newServer(t)
	defer s.Close()

	surlTLS, _ := url.Parse(sTLS.Server.URL)

	cstDialer := cstDialer // make local copy for modification on next line.
	cstDialer.Proxy = http.ProxyURL(surlTLS)

	connect := false
	origHandler := sTLS.Server.Config.Handler

	// Capture the request Host header.
	sTLS.Server.Config.Handler = http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "CONNECT" {
				connect = true
				w.WriteHeader(http.StatusOK)
				return
			}

			if !connect {
				t.Log("connect not received")
				http.Error(w, "connect not received", http.StatusMethodNotAllowed)
				return
			}
			origHandler.ServeHTTP(w, r)
		})

	cstDialer.TLSClientConfig = &tls.Config{RootCAs: rootCAs(t, sTLS.Server)}
	ws, _, err := cstDialer.Dial(s.URL, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer ws.Close()
	sendRecv(t, ws)
}
