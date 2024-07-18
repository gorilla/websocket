// Copyright 2017 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package websocket

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
)

func newNetDialerFunc(
	scheme string,
	netDial func(network, addr string) (net.Conn, error),
	netDialContext func(ctx context.Context, network, addr string) (net.Conn, error),
	netDialTLSContext func(ctx context.Context, network, addr string) (net.Conn, error),
) netDialerFunc {
	switch {
	case scheme == "https" && netDialTLSContext != nil:
		return netDialTLSContext
	case netDialContext != nil:
		return netDialContext
	case netDial != nil:
		return func(ctx context.Context, net, addr string) (net.Conn, error) {
			return netDial(net, addr)
		}
	default:
		return (&net.Dialer{}).DialContext
	}
}

type netDialerFunc func(ctx context.Context, network, addr string) (net.Conn, error)

func (fn netDialerFunc) Dial(network, addr string) (net.Conn, error) {
	return fn(context.Background(), network, addr)
}

func (fn netDialerFunc) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return fn(ctx, network, addr)
}

// newHTTPProxyDialerFunc returns a netDialerFunc that dials using the provided
// proxyURL. The forwardDial function is used to establish the connection to the
// proxy server. If tlsClientConfig is not nil, the connection to the proxy is
// upgraded to a TLS connection with tls.Client.
func newHTTPProxyDialerFunc(proxyURL *url.URL, forwardDial netDialerFunc, tlsClientConfig *tls.Config) netDialerFunc {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		hostPort, _ := hostPortNoPort(proxyURL)
		conn, err := forwardDial(ctx, network, hostPort)
		if err != nil {
			return nil, err
		}

		if tlsClientConfig != nil {
			tlsConn := tls.Client(conn, tlsClientConfig)
			if err = tlsConn.HandshakeContext(ctx); err != nil {
				return nil, err
			}
			conn = tlsConn
		}

		connectHeader := make(http.Header)
		if user := proxyURL.User; user != nil {
			proxyUser := user.Username()
			if proxyPassword, passwordSet := user.Password(); passwordSet {
				credential := base64.StdEncoding.EncodeToString([]byte(proxyUser + ":" + proxyPassword))
				connectHeader.Set("Proxy-Authorization", "Basic "+credential)
			}
		}

		connectReq := &http.Request{
			Method: http.MethodConnect,
			URL:    &url.URL{Opaque: addr},
			Host:   addr,
			Header: connectHeader,
		}

		if err := connectReq.Write(conn); err != nil {
			conn.Close()
			return nil, err
		}

		// Read response. It's OK to use and discard buffered reader here because
		// the remote server does not speak until spoken to.
		br := bufio.NewReader(conn)
		resp, err := http.ReadResponse(br, connectReq)
		if err != nil {
			conn.Close()
			return nil, err
		}

		// Close the response body to silence false positives from linters. Reset
		// the buffered reader first to ensure that Close() does not read from
		// conn.
		// Note: Applications must call resp.Body.Close() on a response returned
		// http.ReadResponse to inspect trailers or read another response from the
		// buffered reader. The call to resp.Body.Close() does not release
		// resources.
		br.Reset(bytes.NewReader(nil))
		_ = resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			_ = conn.Close()
			f := strings.SplitN(resp.Status, " ", 2)
			return nil, errors.New(f[1])
		}
		return conn, nil
	}
}
