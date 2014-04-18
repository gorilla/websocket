// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package websocket

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HandshakeError describes an error with the handshake from the peer.
type HandshakeError struct {
	message string
}

func (e HandshakeError) Error() string { return e.message }

const (
	defaultReadBufferSize  = 4096
	defaultWriteBufferSize = 4096
)

type Upgrader struct {
	// HandshakeTimeout specifies the duration for the handshake to complete.
	HandshakeTimeout time.Duration

	// Input and output buffer sizes. If the buffer size is zero, then
	// default values will be used.
	ReadBufferSize, WriteBufferSize int

	// NegotiateSubprotocol specifies the function to negotiate a subprotocol
	// based on a request. If NegotiateSubprotocol is nil, then no subprotocol
	// will be used.
	NegotiateSubprotocol func(r *http.Request) (string, error)

	// Error specifies the function for generating HTTP error responses. If Error
	// is nil, then http.Error is used to generate the HTTP response.
	Error func(w http.ResponseWriter, r *http.Request, status int, reason error)

	// CheckOrigin returns true if the request Origin header is acceptable.
	// If CheckOrigin is nil, the host in the Origin header must match
	// the host of the request.
	CheckOrigin func(r *http.Request) bool
}

// Return an error depending on settings on the Upgrader
func (u *Upgrader) returnError(w http.ResponseWriter, r *http.Request, status int, reason string) (*Conn, error) {
	err := HandshakeError{reason}
	if u.Error != nil {
		u.Error(w, r, status, err)
	} else {
		http.Error(w, reason, status)
	}
	return nil, err
}

// Check if host in Origin header matches host of request
func (u *Upgrader) checkSameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	uri, err := url.ParseRequestURI(origin)
	if err != nil {
		return false
	}
	return uri.Host == r.Host
}

// Upgrade upgrades the HTTP server connection to the WebSocket protocol.
//
// The responseHeader is included in the response to the client's upgrade
// request. Use the responseHeader to specify cookies (Set-Cookie).
//
// The connection buffers IO to the underlying network connection.
// Messages can be larger than the buffers.
//
// If the request is not a valid WebSocket handshake, then Upgrade returns an
// error of type HandshakeError. Depending on settings on the Upgrader,
// an error message already has been returned to the caller.
func (u *Upgrader) Upgrade(w http.ResponseWriter, r *http.Request, responseHeader http.Header) (*Conn, error) {
	if values := r.Header["Sec-Websocket-Version"]; len(values) == 0 || values[0] != "13" {
		return u.returnError(w, r, http.StatusBadRequest, "websocket: version != 13")
	}

	if !tokenListContainsValue(r.Header, "Connection", "upgrade") {
		return u.returnError(w, r, http.StatusBadRequest, "websocket: connection header != upgrade")
	}

	if !tokenListContainsValue(r.Header, "Upgrade", "websocket") {
		return u.returnError(w, r, http.StatusBadRequest, "websocket: upgrade != websocket")
	}

	checkOrigin := u.CheckOrigin
	if checkOrigin == nil {
		checkOrigin = u.checkSameOrigin
	}
	if !checkOrigin(r) {
		return u.returnError(w, r, http.StatusForbidden, "websocket: origin not allowed")
	}

	var challengeKey string
	values := r.Header["Sec-Websocket-Key"]
	if len(values) == 0 || values[0] == "" {
		return u.returnError(w, r, http.StatusBadRequest, "websocket: key missing or blank")
	}
	challengeKey = values[0]

	var (
		netConn net.Conn
		br      *bufio.Reader
		err     error
	)

	h, ok := w.(http.Hijacker)
	if !ok {
		return nil, errors.New("websocket: response does not implement http.Hijacker")
	}
	var rw *bufio.ReadWriter
	netConn, rw, err = h.Hijack()
	br = rw.Reader

	if br.Buffered() > 0 {
		netConn.Close()
		return nil, errors.New("websocket: client sent data before handshake is complete")
	}

	readBufSize := u.ReadBufferSize
	if readBufSize == 0 {
		readBufSize = defaultReadBufferSize
	}
	writeBufSize := u.WriteBufferSize
	if writeBufSize == 0 {
		writeBufSize = defaultWriteBufferSize
	}
	c := newConn(netConn, true, readBufSize, writeBufSize)

	if u.NegotiateSubprotocol != nil {
		c.subprotocol, err = u.NegotiateSubprotocol(r)
		if err != nil {
			netConn.Close()
			return nil, err
		}
	} else if responseHeader != nil {
		c.subprotocol = responseHeader.Get("Sec-Websocket-Protocol")
	}

	p := c.writeBuf[:0]
	p = append(p, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: "...)
	p = append(p, computeAcceptKey(challengeKey)...)
	p = append(p, "\r\n"...)
	if c.subprotocol != "" {
		p = append(p, "Sec-Websocket-Protocol: "...)
		p = append(p, c.subprotocol...)
		p = append(p, "\r\n"...)
	}
	for k, vs := range responseHeader {
		if k == "Sec-Websocket-Protocol" {
			continue
		}
		for _, v := range vs {
			p = append(p, k...)
			p = append(p, ": "...)
			for i := 0; i < len(v); i++ {
				b := v[i]
				if b <= 31 {
					// prevent response splitting.
					b = ' '
				}
				p = append(p, b)
			}
			p = append(p, "\r\n"...)
		}
	}
	p = append(p, "\r\n"...)

	if u.HandshakeTimeout > 0 {
		netConn.SetWriteDeadline(time.Now().Add(u.HandshakeTimeout))
	}
	if _, err = netConn.Write(p); err != nil {
		netConn.Close()
		return nil, err
	}

	return c, nil
}

// Upgrade upgrades the HTTP server connection to the WebSocket protocol.
//
// The application is responsible for checking the request origin before
// calling Upgrade. An example implementation of the same origin policy is:
//
//	if req.Header.Get("Origin") != "http://"+req.Host {
//		http.Error(w, "Origin not allowed", 403)
//		return
//	}
//
// If the endpoint supports subprotocols, then the application is responsible
// for negotiating the protocol used on the connection. Use the Subprotocols()
// function to get the subprotocols requested by the client. Use the
// Sec-Websocket-Protocol response header to specify the subprotocol selected
// by the application.
//
// The responseHeader is included in the response to the client's upgrade
// request. Use the responseHeader to specify cookies (Set-Cookie) and the
// negotiated subprotocol (Sec-Websocket-Protocol).
//
// The connection buffers IO to the underlying network connection. The
// readBufSize and writeBufSize parameters specify the size of the buffers to
// use. Messages can be larger than the buffers.
//
// If the request is not a valid WebSocket handshake, then Upgrade returns an
// error of type HandshakeError. Applications should handle this error by
// replying to the client with an HTTP error response.
//
// This method is deprecated, use websocket.Upgrader instead.
func Upgrade(w http.ResponseWriter, r *http.Request, responseHeader http.Header, readBufSize, writeBufSize int) (*Conn, error) {
	u := Upgrader{ReadBufferSize: readBufSize, WriteBufferSize: writeBufSize}
	u.Error = func(w http.ResponseWriter, r *http.Request, status int, reason error) {
		// don't return errors to maintain backwards compatibility
	}
	u.CheckOrigin = func(r *http.Request) bool {
		// allow all connections by default
		return true
	}
	return u.Upgrade(w, r, responseHeader)
}

// Subprotocols returns the subprotocols requested by the client in the
// Sec-Websocket-Protocol header.
func Subprotocols(r *http.Request) []string {
	h := strings.TrimSpace(r.Header.Get("Sec-Websocket-Protocol"))
	if h == "" {
		return nil
	}
	protocols := strings.Split(h, ",")
	for i := range protocols {
		protocols[i] = strings.TrimSpace(protocols[i])
	}
	return protocols
}
