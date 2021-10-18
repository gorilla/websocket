//go:build !go1.8
// +build !go1.8

package websocket

import (
	"crypto/tls"
	"net/http/httptrace"
)

func doHandshakeWithTrace(ctx context.Context, trace *httptrace.ClientTrace, tlsConn *tls.Conn, cfg *tls.Config) error {
	return doHandshake(ctx, tlsConn, cfg)
}
