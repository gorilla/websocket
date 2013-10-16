// Copyright 2013 Gary Burd. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package websocket

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// ErrBadHandshake is returned when the server response to opening handshake is
// invalid.
var ErrBadHandshake = errors.New("websocket: bad handshake")

// NewClient creates a new client connection using the given net connection.
// The URL u specifies the host and request URI. Use requestHeader to specify
// the origin (Origin), subprotocols (Set-WebSocket-Protocol) and cookies
// (Cookie). Use the response.Header to get the selected subprotocol
// (Sec-WebSocket-Protocol) and cookies (Set-Cookie).
//
// If the WebSocket handshake fails, ErrBadHandshake is returned along with a
// non-nil *http.Response so that callers can handle redirects, authentication,
// etc.
func NewClient(netConn net.Conn, u *url.URL, requestHeader http.Header, readBufSize, writeBufSize int) (c *Conn, response *http.Response, err error) {
	challengeKey, err := generateChallengeKey()
	if err != nil {
		return nil, nil, err
	}
	acceptKey := computeAcceptKey(challengeKey)

	c = newConn(netConn, false, readBufSize, writeBufSize)
	p := c.writeBuf[:0]
	p = append(p, "GET "...)
	p = append(p, u.RequestURI()...)
	p = append(p, " HTTP/1.1\r\nHost: "...)
	p = append(p, u.Host...)
	p = append(p, "\r\nUpgrade: websocket\r\nConnection: upgrade\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: "...)
	p = append(p, challengeKey...)
	p = append(p, "\r\n"...)
	for k, vs := range requestHeader {
		for _, v := range vs {
			p = append(p, k...)
			p = append(p, ": "...)
			p = append(p, v...)
			p = append(p, "\r\n"...)
		}
	}
	p = append(p, "\r\n"...)

	if _, err := netConn.Write(p); err != nil {
		return nil, nil, err
	}

	resp, err := http.ReadResponse(c.br, &http.Request{Method: "GET", URL: u})
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode != 101 ||
		!strings.EqualFold(resp.Header.Get("Upgrade"), "websocket") ||
		!strings.EqualFold(resp.Header.Get("Connection"), "upgrade") ||
		resp.Header.Get("Sec-Websocket-Accept") != acceptKey {
		return nil, resp, ErrBadHandshake
	}
	return c, resp, nil
}
