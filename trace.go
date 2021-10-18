//go:build go1.8
// +build go1.8

package websocket

import (
	"context"
	"crypto/tls"
	"net/http/httptrace"
)

func doHandshakeWithTrace(ctx context.Context, trace *httptrace.ClientTrace, tlsConn *tls.Conn, cfg *tls.Config) error {
	if trace.TLSHandshakeStart != nil {
		trace.TLSHandshakeStart()
	}
	err := doHandshake(ctx, tlsConn, cfg)
	if trace.TLSHandshakeDone != nil {
		trace.TLSHandshakeDone(tlsConn.ConnectionState(), err)
	}
	return err
}
