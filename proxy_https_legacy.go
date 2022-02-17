//go:build !go1.15
// +build !go1.15

package websocket

import (
	"context"
	"net/url"
)

func registerDialerHttps() {
}

func modifyProxyDialer(ctx context.Context, d *Dialer, proxyURL *url.URL, proxyDialer *netDialerFunc) {
}
