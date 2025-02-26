// Copyright 2025 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package websocket

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

// These test cases use a websocket client (Dialer)/proxy/websocket server (Upgrader)
// to validate the cases where a proxy is an intermediary between a websocket client
// and server. The test cases usually 1) create a websocket server which echoes any
// data received back to the client, 2) a basic duplex streaming proxy, and 3) a
// websocket client which sends random data to the server through the proxy,
// validating any subsequent data received is the same as the data sent. The various
// permutations include the proxy and backend schemes (HTTP or HTTPS), as well as
// the custom dial functions (e.g NetDialContext, NetDial) set on the Dialer.

const (
	subprotocolV1 = "subprotocol-version-1"
	subprotocolV2 = "subprotocol-version-2"
)

// Permutation 1
//
//	Backend: HTTP
//	Proxy:   HTTP
func TestHTTPProxyAndBackend(t *testing.T) {
	websocketTLS := false
	proxyTLS := false
	// Start the websocket server, which echoes data back to sender.
	websocketServer, websocketURL, err := newWebsocketServer(websocketTLS)
	defer websocketServer.Close()
	if err != nil {
		t.Fatalf("error starting websocket server: %v", err)
	}
	// Start the proxy server.
	proxyServer, proxyServerURL, err := newProxyServer(proxyTLS)
	defer proxyServer.Close()
	if err != nil {
		t.Fatalf("error starting proxy server: %v", err)
	}
	// Dial the websocket server through the proxy server.
	dialer := Dialer{
		Proxy:        http.ProxyURL(proxyServerURL),
		Subprotocols: []string{subprotocolV1},
	}
	wsClient, _, err := dialer.Dial(websocketURL.String(), nil)
	if err != nil {
		t.Fatalf("websocket dial error: %v", err)
	}
	// Send, receive, and validate random data over websocket connection.
	sendReceiveData(t, wsClient)
	// Validate the proxy server was called.
	if e, a := int64(1), proxyServer.numCalls(); e != a {
		t.Errorf("proxy not called")
	}
}

// Permutation 2
//
//	Backend: HTTP
//	Proxy:   HTTP
//	DialFn:  NetDial (dials proxy)
func TestHTTPProxyWithNetDial(t *testing.T) {
	websocketTLS := false
	proxyTLS := false
	// Start the websocket server, which echoes data back to sender.
	websocketServer, websocketURL, err := newWebsocketServer(websocketTLS)
	defer websocketServer.Close()
	if err != nil {
		t.Fatalf("error starting websocket server: %v", err)
	}
	// Start the proxy server.
	proxyServer, proxyServerURL, err := newProxyServer(proxyTLS)
	defer proxyServer.Close()
	if err != nil {
		t.Fatalf("error starting proxy server: %v", err)
	}
	// Dial the websocket server through the proxy server.
	var netDialCalled atomic.Int64
	dialer := Dialer{
		NetDial: func(network, addr string) (net.Conn, error) {
			netDialCalled.Add(1)
			return (&net.Dialer{}).DialContext(context.Background(), network, addr)
		},
		Proxy:        http.ProxyURL(proxyServerURL),
		Subprotocols: []string{subprotocolV1},
	}
	wsClient, _, err := dialer.Dial(websocketURL.String(), nil)
	if err != nil {
		t.Fatalf("websocket dial error: %v", err)
	}
	// Send, receive, and validate random data over websocket connection.
	sendReceiveData(t, wsClient)
	if e, a := int64(1), netDialCalled.Load(); e != a {
		t.Errorf("netDial not called")
	}
	// Validate the proxy server was called.
	if e, a := int64(1), proxyServer.numCalls(); e != a {
		t.Errorf("proxy not called")
	}
}

// Permutation 3
//
//	Backend: HTTP
//	Proxy:   HTTP
//	DialFn:  NetDialContext (dials proxy)
func TestHTTPProxyWithNetDialContext(t *testing.T) {
	websocketTLS := false
	proxyTLS := false
	// Start the websocket server, which echoes data back to sender.
	websocketServer, websocketURL, err := newWebsocketServer(websocketTLS)
	defer websocketServer.Close()
	if err != nil {
		t.Fatalf("error starting websocket server: %v", err)
	}
	// Start the proxy server.
	proxyServer, proxyServerURL, err := newProxyServer(proxyTLS)
	defer proxyServer.Close()
	if err != nil {
		t.Fatalf("error starting proxy server: %v", err)
	}
	// Dial the websocket server through the proxy server.
	var netDialCalled atomic.Int64
	dialer := Dialer{
		NetDialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			netDialCalled.Add(1)
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
		Proxy:        http.ProxyURL(proxyServerURL),
		Subprotocols: []string{subprotocolV1},
	}
	wsClient, _, err := dialer.Dial(websocketURL.String(), nil)
	if err != nil {
		t.Fatalf("websocket dial error: %v", err)
	}
	// Send, receive, and validate random data over websocket connection.
	sendReceiveData(t, wsClient)
	if e, a := int64(1), netDialCalled.Load(); e != a {
		t.Errorf("netDial not called")
	}
	// Validate the proxy server was called.
	if e, a := int64(1), proxyServer.numCalls(); e != a {
		t.Errorf("proxy not called")
	}
}

// Permutation 4
//
//	Backend:    HTTPS
//	Proxy:      HTTP
//	DialFn:     NetDialTLSConfig (set but *ignored*)
//	TLS Config: set (used for backend TLS)
func TestHTTPProxyWithHTTPSBackend(t *testing.T) {
	websocketTLS := true
	proxyTLS := false
	// Start the websocket server, which echoes data back to sender.
	websocketServer, websocketURL, err := newWebsocketServer(websocketTLS)
	defer websocketServer.Close()
	if err != nil {
		t.Fatalf("error starting websocket server: %v", err)
	}
	// Start the proxy server.
	proxyServer, proxyServerURL, err := newProxyServer(proxyTLS)
	defer proxyServer.Close()
	if err != nil {
		t.Fatalf("error starting proxy server: %v", err)
	}
	var netDialTLSCalled atomic.Int64
	dialer := Dialer{
		Proxy: http.ProxyURL(proxyServerURL),
		// This function should be ignored, because an HTTP proxy exists
		// and the backend TLS handshake should use TLSClientConfig.
		NetDialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			netDialTLSCalled.Add(1)
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
		// Used for the backend server TLS handshake.
		TLSClientConfig: tlsConfig(websocketTLS, proxyTLS),
		Subprotocols:    []string{subprotocolV1},
	}
	wsClient, _, err := dialer.Dial(websocketURL.String(), nil)
	if err != nil {
		t.Fatalf("websocket dial error: %v", err)
	}
	// Send, receive, and validate random data over websocket connection.
	sendReceiveData(t, wsClient)
	if numTLSDials := netDialTLSCalled.Load(); numTLSDials > 0 {
		t.Errorf("NetDialTLS should have been ignored")
	}
	// Validate the proxy server was called.
	if e, a := int64(1), proxyServer.numCalls(); e != a {
		t.Errorf("proxy not called")
	}
}

// Permutation 5
//
//	Backend:    HTTPS
//	Proxy:      HTTPS
//	TLS Config: set (used for both proxy and backend TLS)
func TestHTTPSProxyAndBackend(t *testing.T) {
	websocketTLS := true
	proxyTLS := true
	// Start the websocket server, which echoes data back to sender.
	websocketServer, websocketURL, err := newWebsocketServer(websocketTLS)
	defer websocketServer.Close()
	if err != nil {
		t.Fatalf("error starting websocket server: %v", err)
	}
	// Start the proxy server.
	proxyServer, proxyServerURL, err := newProxyServer(proxyTLS)
	defer proxyServer.Close()
	if err != nil {
		t.Fatalf("error starting proxy server: %v", err)
	}
	dialer := Dialer{
		Proxy:           http.ProxyURL(proxyServerURL),
		TLSClientConfig: tlsConfig(websocketTLS, proxyTLS),
		Subprotocols:    []string{subprotocolV1},
	}
	wsClient, _, err := dialer.Dial(websocketURL.String(), nil)
	if err != nil {
		t.Fatalf("websocket dial error: %v", err)
	}
	// Send, receive, and validate random data over websocket connection.
	sendReceiveData(t, wsClient)
	// Validate the proxy server was called.
	if e, a := int64(1), proxyServer.numCalls(); e != a {
		t.Errorf("proxy not called")
	}
}

// Permutation 6
//
//	Backend:    HTTPS
//	Proxy:      HTTPS
//	DialFn:     NetDial (used to dial proxy)
//	TLS Config: set (used for both proxy and backend TLS)
func TestHTTPSProxyUsingNetDial(t *testing.T) {
	websocketTLS := true
	proxyTLS := true
	// Start the websocket server, which echoes data back to sender.
	websocketServer, websocketURL, err := newWebsocketServer(websocketTLS)
	defer websocketServer.Close()
	if err != nil {
		t.Fatalf("error starting websocket server: %v", err)
	}
	// Start the proxy server.
	proxyServer, proxyServerURL, err := newProxyServer(proxyTLS)
	defer proxyServer.Close()
	if err != nil {
		t.Fatalf("error starting proxy server: %v", err)
	}
	var netDialCalled atomic.Int64
	dialer := Dialer{
		NetDial: func(network, addr string) (net.Conn, error) {
			netDialCalled.Add(1)
			return (&net.Dialer{}).DialContext(context.Background(), network, addr)
		},
		Proxy:           http.ProxyURL(proxyServerURL),
		TLSClientConfig: tlsConfig(websocketTLS, proxyTLS),
		Subprotocols:    []string{subprotocolV1},
	}
	wsClient, _, err := dialer.Dial(websocketURL.String(), nil)
	if err != nil {
		t.Fatalf("websocket dial error: %v", err)
	}
	// Send, receive, and validate random data over websocket connection.
	sendReceiveData(t, wsClient)
	if e, a := int64(1), netDialCalled.Load(); e != a {
		t.Errorf("netDial not called")
	}
	// Validate the proxy server was called.
	if e, a := int64(1), proxyServer.numCalls(); e != a {
		t.Errorf("proxy not called")
	}
}

// Permutation 7
//
//	Backend:    HTTPS
//	Proxy:      HTTPS
//	DialFn:     NetDialContext (used to dial proxy)
//	TLS Config: set (used for both proxy and backend TLS)
func TestHTTPSProxyUsingNetDialContext(t *testing.T) {
	websocketTLS := true
	proxyTLS := true
	// Start the websocket server, which echoes data back to sender.
	websocketServer, websocketURL, err := newWebsocketServer(websocketTLS)
	defer websocketServer.Close()
	if err != nil {
		t.Fatalf("error starting websocket server: %v", err)
	}
	// Start the proxy server.
	proxyServer, proxyServerURL, err := newProxyServer(proxyTLS)
	defer proxyServer.Close()
	if err != nil {
		t.Fatalf("error starting proxy server: %v", err)
	}
	var netDialCalled atomic.Int64
	dialer := Dialer{
		NetDialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			netDialCalled.Add(1)
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
		Proxy:           http.ProxyURL(proxyServerURL),
		TLSClientConfig: tlsConfig(websocketTLS, proxyTLS),
		Subprotocols:    []string{subprotocolV1},
	}
	wsClient, _, err := dialer.Dial(websocketURL.String(), nil)
	if err != nil {
		t.Fatalf("websocket dial error: %v", err)
	}
	// Send, receive, and validate random data over websocket connection.
	sendReceiveData(t, wsClient)
	if e, a := int64(1), netDialCalled.Load(); e != a {
		t.Errorf("netDial not called")
	}
	// Validate the proxy server was called.
	if e, a := int64(1), proxyServer.numCalls(); e != a {
		t.Errorf("proxy not called")
	}
}

// Permutation 8
//
//	Backend:    HTTPS
//	Proxy:      HTTPS
//	DialFn:     NetDialTLSContext (used for proxy TLS)
//	TLS Config: set (used for backend TLS)
func TestHTTPSProxyUsingNetDialTLSContext(t *testing.T) {
	websocketTLS := true
	proxyTLS := true
	// Start the websocket server, which echoes data back to sender.
	websocketServer, websocketURL, err := newWebsocketServer(websocketTLS)
	defer websocketServer.Close()
	if err != nil {
		t.Fatalf("error starting websocket server: %v", err)
	}
	// Start the proxy server.
	proxyServer, proxyServerURL, err := newProxyServer(proxyTLS)
	defer proxyServer.Close()
	if err != nil {
		t.Fatalf("error starting proxy server: %v", err)
	}
	// Configure the proxy dialing function which dials the proxy and
	// performs the TLS handshake.
	var proxyDialCalled atomic.Int64
	proxyCerts := x509.NewCertPool()
	proxyCerts.AppendCertsFromPEM(proxyServerCert)
	proxyTLSConfig := &tls.Config{RootCAs: proxyCerts}
	proxyDial := func(ctx context.Context, network, addr string) (net.Conn, error) {
		proxyDialCalled.Add(1)
		return tls.Dial(network, addr, proxyTLSConfig)
	}
	// Configure the backend webscocket TLS configuration (handshake occurs
	// over the previously created proxy connection).
	websocketCerts := x509.NewCertPool()
	websocketCerts.AppendCertsFromPEM(websocketServerCert)
	websocketTLSConfig := &tls.Config{RootCAs: websocketCerts}
	dialer := Dialer{
		Proxy: http.ProxyURL(proxyServerURL),
		// Dial and TLS handshake function to proxy.
		NetDialTLSContext: proxyDial,
		// Used for second TLS handshake to backend server over previously
		// established proxy connection.
		TLSClientConfig: websocketTLSConfig,
		Subprotocols:    []string{subprotocolV1},
	}
	wsClient, _, err := dialer.Dial(websocketURL.String(), nil)
	if err != nil {
		t.Fatalf("websocket dial error: %v", err)
	}
	// Send, receive, and validate random data over websocket connection.
	sendReceiveData(t, wsClient)
	if e, a := int64(1), proxyDialCalled.Load(); e != a {
		t.Errorf("netDial not called")
	}
	// Validate the proxy server was called.
	if e, a := int64(1), proxyServer.numCalls(); e != a {
		t.Errorf("proxy not called")
	}
}

// Permutation 9
//
//	Backend:    HTTP
//	Proxy:      HTTPS
//	TLS Config: set (used for proxy TLS)
func TestHTTPSProxyHTTPBackend(t *testing.T) {
	websocketTLS := false
	proxyTLS := true
	// Start the websocket server, which echoes data back to sender.
	websocketServer, websocketURL, err := newWebsocketServer(websocketTLS)
	defer websocketServer.Close()
	if err != nil {
		t.Fatalf("error starting websocket server: %v", err)
	}
	// Start the proxy server.
	proxyServer, proxyServerURL, err := newProxyServer(proxyTLS)
	defer proxyServer.Close()
	if err != nil {
		t.Fatalf("error starting proxy server: %v", err)
	}
	dialer := Dialer{
		Proxy:           http.ProxyURL(proxyServerURL),
		TLSClientConfig: tlsConfig(websocketTLS, proxyTLS),
		Subprotocols:    []string{subprotocolV1},
	}
	wsClient, _, err := dialer.Dial(websocketURL.String(), nil)
	if err != nil {
		t.Fatalf("websocket dial error: %v", err)
	}
	// Send, receive, and validate random data over websocket connection.
	sendReceiveData(t, wsClient)
	// Validate the proxy server was called.
	if e, a := int64(1), proxyServer.numCalls(); e != a {
		t.Errorf("proxy not called")
	}
}

// Permutation 10
//
//	Backend:    HTTP
//	Proxy:      HTTPS
//	DialFn:     NetDialTLSContext (used for proxy TLS)
//	TLS Config: set (ignored)
func TestHTTPSProxyUsingNetDialTLSContextWithHTTPBackend(t *testing.T) {
	websocketTLS := false
	proxyTLS := true
	// Start the websocket server, which echoes data back to sender.
	websocketServer, websocketURL, err := newWebsocketServer(websocketTLS)
	defer websocketServer.Close()
	if err != nil {
		t.Fatalf("error starting websocket server: %v", err)
	}
	// Start the proxy server.
	proxyServer, proxyServerURL, err := newProxyServer(proxyTLS)
	defer proxyServer.Close()
	if err != nil {
		t.Fatalf("error starting proxy server: %v", err)
	}
	var proxyDialCalled atomic.Int64
	dialer := Dialer{
		NetDialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			proxyDialCalled.Add(1)
			return tls.Dial(network, addr, tlsConfig(websocketTLS, proxyTLS))
		},
		Proxy:           http.ProxyURL(proxyServerURL),
		TLSClientConfig: &tls.Config{}, // Misconfigured, but ignored.
		Subprotocols:    []string{subprotocolV1},
	}
	wsClient, _, err := dialer.Dial(websocketURL.String(), nil)
	if err != nil {
		t.Fatalf("websocket dial error: %v", err)
	}
	// Send, receive, and validate random data over websocket connection.
	sendReceiveData(t, wsClient)
	if e, a := int64(1), proxyDialCalled.Load(); e != a {
		t.Errorf("netDial not called")
	}
	// Validate the proxy server was called.
	if e, a := int64(1), proxyServer.numCalls(); e != a {
		t.Errorf("proxy not called")
	}
}

func TestTLSValidationErrors(t *testing.T) {
	// Both websocket and proxy servers are started with TLS.
	websocketTLS := true
	proxyTLS := true
	websocketServer, websocketURL, err := newWebsocketServer(websocketTLS)
	defer websocketServer.Close()
	if err != nil {
		t.Fatalf("error starting websocket server: %v", err)
	}
	proxyServer, proxyServerURL, err := newProxyServer(proxyTLS)
	defer proxyServer.Close()
	if err != nil {
		t.Fatalf("error starting proxy server: %v", err)
	}
	// Dialer without proxy CA cert fails TLS verification.
	tlsError := "tls: failed to verify certificate"
	dialer := Dialer{
		Proxy:           http.ProxyURL(proxyServerURL),
		TLSClientConfig: tlsConfig(true, false),
		Subprotocols:    []string{subprotocolV1},
	}
	_, _, err = dialer.Dial(websocketURL.String(), nil)
	if err == nil {
		t.Errorf("expected proxy TLS verification error did not arrive")
	} else if !strings.Contains(err.Error(), tlsError) {
		t.Errorf("expected proxy TLS error (%s), got (%s)", err.Error(), tlsError)
	}
	// Validate the proxy handler was *NOT* called (because proxy
	// server TLS validation failed).
	if e, a := int64(0), proxyServer.numCalls(); e != a {
		t.Errorf("proxy should not have been called")
	}
	// Dialer without websocket CA cert fails TLS verification.
	dialer = Dialer{
		Proxy:           http.ProxyURL(proxyServerURL),
		TLSClientConfig: tlsConfig(false, true),
		Subprotocols:    []string{subprotocolV1},
	}
	_, _, err = dialer.Dial(websocketURL.String(), nil)
	if err == nil {
		t.Errorf("expected websocket TLS verification error did not arrive")
	} else if !strings.Contains(err.Error(), tlsError) {
		t.Errorf("expected websocket TLS error (%s), got (%s)", err.Error(), tlsError)
	}
	// Validate the proxy server *was* called (but subsequent
	// websocket server failed TLS validation).
	if e, a := int64(1), proxyServer.numCalls(); e != a {
		t.Errorf("proxy have been called")
	}
}

func TestProxyFnErrorIsPropagated(t *testing.T) {
	websocketServer, websocketURL, err := newWebsocketServer(false)
	defer websocketServer.Close()
	if err != nil {
		t.Fatalf("error starting websocket server: %v", err)
	}
	// Create a Dialer where Proxy function always returns an error.
	proxyURLError := errors.New("proxy URL generation error")
	dialer := Dialer{
		Proxy: func(r *http.Request) (*url.URL, error) {
			return nil, proxyURLError
		},
		Subprotocols: []string{subprotocolV1},
	}
	// Proxy URL generation error should halt request and be propagated.
	_, _, err = dialer.Dial(websocketURL.String(), nil)
	if err == nil {
		t.Fatalf("expected websocket dial error, received none")
	} else if !errors.Is(proxyURLError, err) {
		t.Fatalf("expected error (%s), got (%s)", proxyURLError, err)
	}
}

func TestProxyFnNilMeansNoProxy(t *testing.T) {
	// Both websocket and proxy servers are started.
	websocketTLS := false
	proxyTLS := false
	websocketServer, websocketURL, err := newWebsocketServer(websocketTLS)
	defer websocketServer.Close()
	if err != nil {
		t.Fatalf("error starting websocket server: %v", err)
	}
	proxyServer, _, err := newProxyServer(proxyTLS)
	defer proxyServer.Close()
	if err != nil {
		t.Fatalf("error starting proxy server: %v", err)
	}
	// Dialer created with Proxy URL generation function returning nil
	// proxy URL, which continues with backend server connection without
	// proxying.
	dialer := Dialer{
		Proxy: func(r *http.Request) (*url.URL, error) {
			return nil, nil
		},
		Subprotocols: []string{subprotocolV1},
	}
	wsClient, _, err := dialer.Dial(websocketURL.String(), nil)
	if err != nil {
		t.Fatalf("websocket dial error: %v", err)
	}
	sendReceiveData(t, wsClient)
	// Validate the proxy handler was *NOT* called (because proxy
	// URL generation returned nil).
	if e, a := int64(0), proxyServer.numCalls(); e != a {
		t.Errorf("proxy should not have been called")
	}
}

// "counter" interface can be implemented by a server to keep track
// of the number of times a handler was called, as well as "Close".
type counter interface {
	increment()
	numCalls() int64
	closer
}

type closer interface {
	Close()
}

// testServer implements "counter" interface.
type testServer struct {
	server     *httptest.Server
	numHandled atomic.Int64
}

func (ts *testServer) numCalls() int64 {
	return ts.numHandled.Load()
}

func (ts *testServer) increment() {
	ts.numHandled.Add(1)
}

func (ts *testServer) Close() {
	if ts.server != nil {
		ts.server.Close()
	}
}

// websocketEchoHandler upgrades the connection associated with the request, and
// echoes binary messages read off the websocket connection back to the client.
var websocketEchoHandler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
	upgrader := Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Accepting all requests
		},
		Subprotocols: []string{
			subprotocolV1,
			subprotocolV2,
		},
	}
	wsConn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	defer wsConn.Close()
	for {
		writer, err := wsConn.NextWriter(BinaryMessage)
		if err != nil {
			break
		}
		messageType, reader, err := wsConn.NextReader()
		if err != nil {
			break
		}
		if messageType != BinaryMessage {
			http.Error(w, "websocket reader not binary message type",
				http.StatusInternalServerError)
		}
		_, err = io.Copy(writer, reader)
		if err != nil {
			http.Error(w, "websocket server io copy error",
				http.StatusInternalServerError)
		}
	}
})

// Returns a test backend websocket server as well as the URL pointing
// to the server, or an error if one occurred. Sets up a TLS endpoint
// on the server if the passed "tlsServer" is true.
// func newWebsocketServer(tlsServer bool) (*httptest.Server, *url.URL, error) {
func newWebsocketServer(tlsServer bool) (closer, *url.URL, error) {
	// Start the websocket server, which echoes data back to sender.
	websocketServer := httptest.NewUnstartedServer(websocketEchoHandler)
	if tlsServer {
		websocketKeyPair, err := tls.X509KeyPair(websocketServerCert, websocketServerKey)
		if err != nil {
			return nil, nil, err
		}
		websocketServer.TLS = &tls.Config{
			Certificates: []tls.Certificate{websocketKeyPair},
		}
		websocketServer.StartTLS()
	} else {
		websocketServer.Start()
	}
	websocketURL, err := url.Parse(websocketServer.URL)
	if err != nil {
		return nil, nil, err
	}
	if tlsServer {
		websocketURL.Scheme = "wss"
	} else {
		websocketURL.Scheme = "ws"
	}
	return websocketServer, websocketURL, nil
}

// proxyHandler creates a full duplex streaming connection between the client
// (hijacking the http request connection), and an "upstream" dialed connection
// to the "Host". Creates two goroutines to copy between connections in each direction.
var proxyHandler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
	// Validate the CONNECT method.
	if req.Method != http.MethodConnect {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Dial upstream server.
	upstream, err := (&net.Dialer{}).DialContext(req.Context(), "tcp", req.URL.Host)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer upstream.Close()
	// Return 200 OK to client.
	w.WriteHeader(http.StatusOK)
	// Hijack client connection.
	client, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer client.Close()
	// Create duplex streaming between client and upstream connections.
	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(upstream, client)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(client, upstream)
		done <- struct{}{}
	}()
	<-done
})

// Returns a new test HTTP server, as well as the URL to that server, or
// an error if one occurred. numProxyCalls keeps track of the number of
// times the proxy handler was called with this server.
func newProxyServer(tlsServer bool) (counter, *url.URL, error) {
	// Start the proxy server, keeping track of how many times the handler is called.
	ts := &testServer{}
	proxyServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ts.increment()
		proxyHandler.ServeHTTP(w, req)
	}))
	if tlsServer {
		proxyKeyPair, err := tls.X509KeyPair(proxyServerCert, proxyServerKey)
		if err != nil {
			return nil, nil, err
		}
		proxyServer.TLS = &tls.Config{
			Certificates: []tls.Certificate{proxyKeyPair},
		}
		proxyServer.StartTLS()
	} else {
		proxyServer.Start()
	}
	proxyURL, err := url.Parse(proxyServer.URL)
	if err != nil {
		return nil, nil, err
	}
	return ts, proxyURL, nil
}

// Returns the TLS config with the RootCAs cert pool set. If
// neither websocket nor proxy server uses TLS, returns nil.
func tlsConfig(websocketTLS bool, proxyTLS bool) *tls.Config {
	if !websocketTLS && !proxyTLS {
		return nil
	}
	certPool := x509.NewCertPool()
	tlsConfig := &tls.Config{
		RootCAs: certPool,
	}
	if websocketTLS {
		tlsConfig.RootCAs.AppendCertsFromPEM(websocketServerCert)
	}
	if proxyTLS {
		tlsConfig.RootCAs.AppendCertsFromPEM(proxyServerCert)
	}
	return tlsConfig
}

// Sends, receives, and validates random data sent and received
// over the passed websocket connection.
const randomDataSize = 128 * 1024

func sendReceiveData(t *testing.T, wsConn *Conn) {
	// Create the random data.
	randomData := make([]byte, randomDataSize)
	if _, err := rand.Read(randomData); err != nil {
		t.Errorf("unexpected error reading random data: %v", err)
	}
	// Send the random data.
	err := wsConn.WriteMessage(BinaryMessage, randomData)
	if err != nil {
		t.Errorf("websocket write error: %v", err)
	}
	// Read from the websocket connection, and validate the
	// read data is the same as the previously sent data.
	_, received, err := wsConn.ReadMessage()
	if !bytes.Equal(randomData, received) {
		t.Errorf("unexpected data received: %d bytes sent, %d bytes received",
			len(received), len(randomData))
	}
}

// proxyServerCert was generated from crypto/tls/generate_cert.go with the following command:
//
//	go run generate_cert.go  --rsa-bits 2048 --host 127.0.0.1,::1,example.com --ca --start-date "Jan 1 00:00:00 1970" --duration=1000000h
//
// proxyServerCert is a self-signed.
var proxyServerCert = []byte(`-----BEGIN CERTIFICATE-----
MIIDGTCCAgGgAwIBAgIRALL5AZcefF4kkYV1SEG6YrMwDQYJKoZIhvcNAQELBQAw
EjEQMA4GA1UEChMHQWNtZSBDbzAgFw03MDAxMDEwMDAwMDBaGA8yMDg0MDEyOTE2
MDAwMFowEjEQMA4GA1UEChMHQWNtZSBDbzCCASIwDQYJKoZIhvcNAQEBBQADggEP
ADCCAQoCggEBALQ/FHcyVwdFHxARbbD2KBtDUT7Eni+8ioNdjtGcmtXqBv45EC1C
JOqqGJTroFGJ6Q9kQIZ9FqH5IJR2fOOJD9kOTueG4Vt1JY1rj1Kbpjefu8XleZ5L
SBwIWVnN/lEsEbuKmj7N2gLt5AH3zMZiBI1mg1u9Z5ZZHYbCiTpBrwsq6cTlvR9g
dyo1YkM5hRESCzsrL0aUByoo0qRMD8ZsgANJwgsiO0/M6idbxDwv1BnGwGmRYvOE
Hxpy3v0Jg7GJYrvnpnifJTs4nw91N5X9pXxR7FFzi/6HTYDWRljvTb0w6XciKYAz
bWZ0+cJr5F7wB7ovlbm7HrQIR7z7EIIu2d8CAwEAAaNoMGYwDgYDVR0PAQH/BAQD
AgKkMBMGA1UdJQQMMAoGCCsGAQUFBwMBMA8GA1UdEwEB/wQFMAMBAf8wLgYDVR0R
BCcwJYILZXhhbXBsZS5jb22HBH8AAAGHEAAAAAAAAAAAAAAAAAAAAAEwDQYJKoZI
hvcNAQELBQADggEBAFPPWopNEJtIA2VFAQcqN6uJK+JVFOnjGRoCrM6Xgzdm0wxY
XCGjsxY5dl+V7KzdGqu858rCaq5osEBqypBpYAnS9C38VyCDA1vPS1PsN8SYv48z
DyBwj+7R2qar0ADBhnhWxvYO9M72lN/wuCqFKYMeFSnJdQLv3AsrrHe9lYqOa36s
8wxSwVTFTYXBzljPEnSaaJMPqFD8JXaZK1ryJPkO5OsCNQNGtatNiWAf3DcmwHAT
MGYMzP0u4nw47aRz9shB8w+taPKHx2BVwE1m/yp3nHVioOjXqA1fwRQVGclCJSH1
D2iq3hWVHRENgjTjANBPICLo9AZ4JfN6PH19mnU=
-----END CERTIFICATE-----`)

// proxyServerKey is the private key for proxyServerCert.
var proxyServerKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEAtD8UdzJXB0UfEBFtsPYoG0NRPsSeL7yKg12O0Zya1eoG/jkQ
LUIk6qoYlOugUYnpD2RAhn0WofkglHZ844kP2Q5O54bhW3UljWuPUpumN5+7xeV5
nktIHAhZWc3+USwRu4qaPs3aAu3kAffMxmIEjWaDW71nllkdhsKJOkGvCyrpxOW9
H2B3KjViQzmFERILOysvRpQHKijSpEwPxmyAA0nCCyI7T8zqJ1vEPC/UGcbAaZFi
84QfGnLe/QmDsYliu+emeJ8lOzifD3U3lf2lfFHsUXOL/odNgNZGWO9NvTDpdyIp
gDNtZnT5wmvkXvAHui+VubsetAhHvPsQgi7Z3wIDAQABAoIBAGmw93IxjYCQ0ncc
kSKMJNZfsdtJdaxuNRZ0nNNirhQzR2h403iGaZlEpmdkhzxozsWcto1l+gh+SdFk
bTUK4MUZM8FlgO2dEqkLYh5BcMT7ICMZvSfJ4v21E5eqR68XVUqQKoQbNvQyxFk3
EddeEGdNrkb0GDK8DKlBlzAW5ep4gjG85wSTjR+J+muUv3R0BgLBFSuQnIDM/IMB
LWqsja/QbtB7yppe7jL5u8UCFdZG8BBKT9fcvFIu5PRLO3MO0uOI7LTc8+W1Xm23
uv+j3SY0+v+6POjK0UlJFFi/wkSPTFIfrQO1qFBkTDQHhQ6q/7GnILYYOiGbIRg2
NNuP52ECgYEAzXEoy50wSYh8xfFaBuxbm3ruuG2W49jgop7ZfoFrPWwOQKAZS441
VIwV4+e5IcA6KkuYbtGSdTYqK1SMkgnUyD/VevwAqH5TJoEIGu0pDuKGwVuwqioZ
frCIAV5GllKyUJ55VZNbRr2vY2fCsWbaCSCHETn6C16DNuTCe5C0JBECgYEA4JqY
5GpNbMG8fOt4H7hU0Fbm2yd6SHJcQ3/9iimef7xG6ajxsYrIhg1ft+3IPHMjVI0+
9brwHDnWg4bOOx/VO4VJBt6Dm/F33bndnZRkuIjfSNpLM51P+EnRdaFVHOJHwKqx
uF69kihifCAG7YATgCveeXImzBUSyZUz9UrETu8CgYARNBimdFNG1RcdvEg9rC0/
p9u1tfecvNySwZqU7WF9kz7eSonTueTdX521qAHowaAdSpdJMGODTTXaywm6cPhQ
jIfj9JZZhbqQzt1O4+08Qdvm9TamCUB5S28YLjza+bHU7nBaqixKkDfPqzCyilpX
yVGGL8SwjwmN3zop/sQXAQKBgC0JMsESQ6YcDsRpnrOVjYQc+LtW5iEitTdfsaID
iGGKihmOI7B66IxgoCHMTws39wycKdSyADVYr5e97xpR3rrJlgQHmBIrz+Iow7Q2
LiAGaec8xjl6QK/DdXmFuQBKqyKJ14rljFODP4QuE9WJid94bGqjpf3j99ltznZP
4J8HAoGAJb4eb4lu4UGwifDzqfAPzLGCoi0fE1/hSx34lfuLcc1G+LEu9YDKoOVJ
9suOh0b5K/bfEy9KrVMBBriduvdaERSD8S3pkIQaitIz0B029AbE4FLFf9lKQpP2
KR8NJEkK99Vh/tew6jAMll70xFrE7aF8VLXJVE7w4sQzuvHxl9Q=
-----END RSA PRIVATE KEY-----
`)

// websocketServerCert is self-signed.
var websocketServerCert = []byte(`-----BEGIN CERTIFICATE-----
MIIDOTCCAiGgAwIBAgIQYSN1VY/favsLUo+B7gJ5tTANBgkqhkiG9w0BAQsFADAS
MRAwDgYDVQQKEwdBY21lIENvMCAXDTcwMDEwMTAwMDAwMFoYDzIwODQwMTI5MTYw
MDAwWjASMRAwDgYDVQQKEwdBY21lIENvMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A
MIIBCgKCAQEApBlintjkL1fO1Sk2pzNvl862CtTwU7/Jy6EZqWzI17wEbPn4sbSD
bHhfDlPl2nmw3hVkc6LNK+eqzm2GX/ai4tgMiaH7kyyNit1K3g7y7GISMf9poWIa
POJhid2wmhKHbEtHECSdQ5c/jEN1UVzB4go5LO7MEEVo9kyQ+yBqS6gISyFmfaT4
qOsPJBir33bBpptSend1JSXaRTXqRa1p+oudw2ILa4U7KfuKK3emp21m5/HYAuSf
CV4WqqDoDiBPMpsQ0kPEPugWZKFeF3qanmqFFvptYx+zJbOznWYY2D3idWsvcg6q
VLPEB19oXaVBV0HXPFtObm5m1jCpl8FI1wIDAQABo4GIMIGFMA4GA1UdDwEB/wQE
AwICpDATBgNVHSUEDDAKBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MB0GA1Ud
DgQWBBQcSkjqA9rgos1daegNj49BpRCA0jAuBgNVHREEJzAlggtleGFtcGxlLmNv
bYcEfwAAAYcQAAAAAAAAAAAAAAAAAAAAATANBgkqhkiG9w0BAQsFAAOCAQEAnk9i
9rogNTi9B1pn+Fbk3WALKdEjv/uyePsTnwdyvswVbeYbQweU9TrhYT2+eXbMA5kY
7TaQm46idRqxCKMgc3Ip3DADJdm8cJX9p2ExU4fKdkPc1KD/J+4QHHx1W2Ml5S2o
foOo6j1F0UdZP/rBj0UumEZp32qW+4DhVV/QQjUB8J0gaDC7yZBMdyMIeClR0RqE
YfZdCJbQHqtTwBXN+imQUHPGmksYkRDpFRvw/4crpcMIE04mVVd99nOpFCQnK61t
9US1y17VW1lYpkqlCS+rkcAtor4Z5naSf9/oLGCxEAwyW0pwHGO6MXtMxvB/JD20
hJdlz1I7wlSfF4MiRQ==
-----END CERTIFICATE-----`)

// websocketServerKey is the private key for websocketServerCert.
var websocketServerKey = []byte(`-----BEGIN PRIVATE KEY-----
MIIEvAIBADANBgkqhkiG9w0BAQEFAASCBKYwggSiAgEAAoIBAQCkGWKe2OQvV87V
KTanM2+XzrYK1PBTv8nLoRmpbMjXvARs+fixtINseF8OU+XaebDeFWRzos0r56rO
bYZf9qLi2AyJofuTLI2K3UreDvLsYhIx/2mhYho84mGJ3bCaEodsS0cQJJ1Dlz+M
Q3VRXMHiCjks7swQRWj2TJD7IGpLqAhLIWZ9pPio6w8kGKvfdsGmm1J6d3UlJdpF
NepFrWn6i53DYgtrhTsp+4ord6anbWbn8dgC5J8JXhaqoOgOIE8ymxDSQ8Q+6BZk
oV4XepqeaoUW+m1jH7Mls7OdZhjYPeJ1ay9yDqpUs8QHX2hdpUFXQdc8W05ubmbW
MKmXwUjXAgMBAAECggEAE6BkTDgH//rnkP/Ej/Y17Zkv6qxnMLe/4evwZB7PsrBu
cxOUAWUOpvA1UO215bh87+2XvcDbUISnyC1kpKDyAGGeC5llER2DXE11VokWgtvZ
Q0OXavw5w83A+WVGFFdiUmXP0l10CxEm7OwQjFz6D21GQ1qC65tG9NZZghTxbFTe
iZKqgWqyHsaAWLOuDQbj1FTEBMFrY8f9RbclSh0luPZnzGc4BVI/t34jKPZBpH2N
NCkr8aB7MMHGhrNZFHAu/KAvq8UBrDTX+O8ERMwcwQWB4nne2+GOTN0MdcAUc72i
GryzIa8TgO+TpQOYoZ4NPnzFrsa+m3G2Tug3vbt62QKBgQDOPfM4/5/x/h/ggxQn
aRvEOC+8ldeqEOS1VTGiuDKJMWXrNkG+d+AsxfNP4k0QVNrpEAZSYcf0gnS9Odcl
luEsi/yPZDDnPg/cS+Z3336VKsggly7BWFs1Ct/9I+ZfSCl88TkVpIfeCBC34XEb
0mFUq/RdLqXj/mVLbBfr+H8cEwKBgQDLsJUm8lkWFAPJ8UMto8xeUMGk44VukYwx
+oI6KhplFntiI0C1Dd9wrxyCjySlJcc0NFt6IPN84d7pI9LQSbiKXQ1jMvsBzd4G
EMtG8SHpIY/mMU+KzWLHYVFS0FA4PvXXvPRNLOXas7hbALZdLshVKd7aDlkQAb5C
KWFHeIFwrQKBgA8r5Xl67HQrwoKMge4IQF+l1nUj/LJo/boNI1KaBDWtaZbs7dcq
EFaa1TQ6LHsYEuZ0JFLpGIF3G0lUOOxt9fCF97VApIxON3J4LuMAkNo+RGyJUoos
isETJLkFbAv0TgD/6bga21fM9hXgwqZOSpSk9ZvpM5DbBO6QbA4SwJ77AoGAX7h1
/z14XAW/2hDE7xfAnLn6plA9jj5b0cjVlhvfF44/IVlLuUnxrPS9wyUdpXZhbMkG
DBicFB3ZMVqiYTuju3ILLojwqGJkahlOTeJXe0VIaHbX2HS4bNXw76fxat07jsy/
Sd1Fj0dR5YIqMRQhFNR+Y57Gf90x2cm0a2/X9GkCgYANawYx9bNfcX0HMVG7vktK
6/80omnoBM0JUxA+V7DxS8kr9Cj2Y/kcS+VHb4yyoSkDgnsSdnCr1ZTctcj828MJ
8AUwskAtEjPkHRXEgRRnEl2oJGD1TT5iwBNnuPAQDXwzkGCRYBnlfZNbILbOoSUz
m+VDcqT5XzcRADa/TLlEXA==
-----END PRIVATE KEY-----
`)
