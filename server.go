// Copyright 2013 Gary Burd. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package websocket

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"
)

// HandshakeError describes an error with the handshake from the peer.
type HandshakeError struct {
	message string
}

func (e HandshakeError) Error() string { return e.message }

const (
	DEFAULT_READ_BUFFER_SIZE  = 4096
	DEFAULT_WRITE_BUFFER_SIZE = 4096
)

type Upgrader struct {
	// HandshakeTimeout specifies the duration for the handshake to complete.
	HandshakeTimeout time.Duration

	// Input and output buffer sizes. If the buffer size is zero, then
	// default values will be used.
	ReadBufferSize, WriteBufferSize int

	// Subprotocols specifies the server's supported protocols. If Subprotocols
	// is nil, then Upgrade does not negotiate a subprotocol.
	Subprotocols []string

	// Error specifies the function for generating HTTP error responses. If Error
	// is nil, then http.Error is used to generate the HTTP response.
	Error func(w http.ResponseWriter, r *http.Request, status int, reason error)

	// CheckOrigin returns true if the request Origin header is acceptable.
	// If CheckOrigin is nil, then no origin check is done.
	CheckOrigin func(r *http.Request) bool
}

// Return an error depending on settings on the Upgrader
func (u *Upgrader) returnError(w http.ResponseWriter, r *http.Request, status int, reason error) {
	if u.Error != nil {
		u.Error(w, r, status, reason)
	} else {
		http.Error(w, reason.Error(), status)
	}
}

// Check if the passed subprotocol is supported by the server
func (u *Upgrader) hasSubprotocol(subprotocol string) bool {
	if u.Subprotocols == nil {
		return false
	}

	for _, s := range u.Subprotocols {
		if s == subprotocol {
			return true
		}
	}

	return false
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
		err := HandshakeError{"websocket: version != 13"}
		u.returnError(w, r, http.StatusBadRequest, err)
		return nil, err
	}

	if !tokenListContainsValue(r.Header, "Connection", "upgrade") {
		err := HandshakeError{"websocket: connection header != upgrade"}
		u.returnError(w, r, http.StatusBadRequest, err)
		return nil, err
	}

	if !tokenListContainsValue(r.Header, "Upgrade", "websocket") {
		err := HandshakeError{"websocket: upgrade != websocket"}
		u.returnError(w, r, http.StatusBadRequest, err)
		return nil, err
	}

	if u.CheckOrigin != nil && !u.CheckOrigin(r) {
		err := HandshakeError{"websocket: origin not allowed"}
		u.returnError(w, r, http.StatusForbidden, err)
		return nil, err
	}

	var challengeKey string
	values := r.Header["Sec-Websocket-Key"]
	if len(values) == 0 || values[0] == "" {
		err := HandshakeError{"websocket: key missing or blank"}
		u.returnError(w, r, http.StatusBadRequest, err)
		return nil, err
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
		readBufSize = DEFAULT_READ_BUFFER_SIZE
	}
	writeBufSize := u.WriteBufferSize
	if writeBufSize == 0 {
		writeBufSize = DEFAULT_WRITE_BUFFER_SIZE
	}
	c := newConn(netConn, true, readBufSize, writeBufSize)

	if u.Subprotocols != nil {
		for _, proto := range Subprotocols(r) {
			if u.hasSubprotocol(proto) {
				c.subprotocol = proto
				break
			}
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

// This method is deprecated, use websocket.Upgrader instead.
//
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
func Upgrade(w http.ResponseWriter, r *http.Request, responseHeader http.Header, readBufSize, writeBufSize int) (*Conn, error) {
	u := Upgrader{ReadBufferSize: readBufSize, WriteBufferSize: writeBufSize}
	u.Error = func(w http.ResponseWriter, r *http.Request, status int, reason error) {
		// don't return errors to maintain backwards compatibility
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
