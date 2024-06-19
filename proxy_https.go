//go:build go1.15
// +build go1.15

package websocket

import (
	"context"
	"crypto/tls"
	"net"
	"net/url"
)

func registerDialerHttps() {
	proxy_RegisterDialerType("https", func(proxyURL *url.URL, forwardDialer proxy_Dialer) (proxy_Dialer, error) {
		fwd := forwardDialer.Dial
		if dialerEx, ok := forwardDialer.(proxyDialerEx); !ok || !dialerEx.UsesTLS() {
			tlsDialer := &tls.Dialer{
				Config:    &tls.Config{},
				NetDialer: &net.Dialer{},
			}
			fwd = tlsDialer.Dial
		}
		return &httpProxyDialer{proxyURL: proxyURL, forwardDial: fwd, usesTLS: true}, nil
	})
}

func modifyProxyDialer(ctx context.Context, d *Dialer, proxyURL *url.URL, proxyDialer *netDialerFunc) {
	if proxyURL.Scheme == "https" {
		proxyDialer.usesTLS = true
		proxyDialer.fn = func(network, addr string) (net.Conn, error) {
			t := tls.Dialer{}
			t.Config = d.TLSClientConfig
			t.NetDialer = &net.Dialer{}
			return t.DialContext(ctx, network, addr)
		}
	}
}
