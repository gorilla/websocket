// +build go1.4

package websocket

import (
	"bytes"
	"net"

	"github.com/valyala/fasthttp"
)

func checkSameOriginFastHTTP(ctx *fasthttp.RequestCtx) bool {
	return checkSameOriginFromHeaderAndHost(string(ctx.Request.Header.Peek(originHeader)), string(ctx.Host()))
}

// FastHTTPUpgrader is used to upgrade a fasthttp request into a websocket
// connection. A Handler function must be provided to receive that connection.
type FastHTTPUpgrader struct {
	// Handler receives a websocket connection after the handshake has been
	// completed. This must be provided.
	Handler func(*Conn)

	// ReadBufferSize and WriteBufferSize specify I/O buffer sizes. If a buffer
	// size is zero, then a default value of 4096 is used. The I/O buffer sizes
	// do not limit the size of the messages that can be sent or received.
	ReadBufferSize, WriteBufferSize int

	// Subprotocols specifies the server's supported protocols in order of
	// preference. If this field is set, then the Upgrade method negotiates a
	// subprotocol by selecting the first match in this list with a protocol
	// requested by the client.
	Subprotocols []string

	// CheckOrigin returns true if the request Origin header is acceptable. If
	// CheckOrigin is nil, the host in the Origin header must not be set or
	// must match the host of the request.
	CheckOrigin func(ctx *fasthttp.RequestCtx) bool
}

var websocketVersionByte = []byte(websocketVersion)

// UpgradeHandler handles a request for a websocket connection and does all the
// checks necessary to ensure the request is valid. If a CheckOrigin function
// was provided, it will be called, otherwise the Origin will be checked against
// the request host value. If a subprotocol has not already been set, the best
// choice will be made from the values provided to the upgrader and from the
// client.
//
// Once the request has been verified and the response sent, the connection will
// be hijacked and the provided Handler will be called.
func (f *FastHTTPUpgrader) UpgradeHandler(ctx *fasthttp.RequestCtx) {
	if f.Handler == nil {
		panic("FastHTTPUpgrader does not have a Handler set")
	}

	if !ctx.IsGet() {
		ctx.Error("websocket: method not GET", fasthttp.StatusMethodNotAllowed)
		return
	}

	if !bytes.Equal(ctx.Request.Header.Peek("Sec-Websocket-Version"), websocketVersionByte) {
		ctx.Error("websocket: version != 13", fasthttp.StatusBadRequest)
		return
	}

	if !ctx.Request.Header.ConnectionUpgrade() {
		ctx.Error("websocket: could not find connection header with token 'upgrade'", fasthttp.StatusBadRequest)
		return
	}

	if !tokenListContainsValue(string(ctx.Request.Header.Peek("Upgrade")), "websocket") {
		ctx.Error("websocket: could not find upgrade header with token 'websocket'", fasthttp.StatusBadRequest)
		return
	}

	checkOrigin := f.CheckOrigin
	if checkOrigin == nil {
		checkOrigin = checkSameOriginFastHTTP
	}
	if !checkOrigin(ctx) {
		ctx.Error("websocket: origin not allowed", fasthttp.StatusForbidden)
		return
	}

	challengeKey := ctx.Request.Header.Peek("Sec-Websocket-Key")
	if len(challengeKey) == 0 {
		ctx.Error("websocket: key missing or blank", fasthttp.StatusBadRequest)
		return
	}

	ctx.SetStatusCode(fasthttp.StatusSwitchingProtocols)
	ctx.Response.Header.Set("Upgrade", "websocket")
	ctx.Response.Header.Set("Connection", "Upgrade")
	ctx.Response.Header.Set("Sec-WebSocket-Accept", computeAcceptKeyByte(challengeKey))

	// The subprotocol may have already been set in the response
	subprotocol := string(ctx.Response.Header.Peek(protocolHeader))
	if subprotocol == "" {
		// Find the best protocol, if any
		clientProtocols := subprotocolsFromHeader(string(ctx.Request.Header.Peek(protocolHeader)))
		if len(clientProtocols) != 0 {
			subprotocol = matchSubprotocol(clientProtocols, f.Subprotocols)
			if subprotocol != "" {
				ctx.Response.Header.Set(protocolHeader, subprotocol)
			}
		}
	}

	ctx.Hijack(func(conn net.Conn) {
		c := newConn(conn, true, f.ReadBufferSize, f.WriteBufferSize)
		if subprotocol != "" {
			c.subprotocol = subprotocol
		}
		f.Handler(c)
	})
}
