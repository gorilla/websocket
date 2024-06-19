// Copyright 2017 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package websocket

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
)

type netDialer struct {
	proxyHeader http.Header
	f           func(network, addr string) (net.Conn, error)
}

func (n netDialer) Dial(network, addr string) (net.Conn, error) {
	return n.f(network, addr)
}

func init() {
	proxy_RegisterDialerType("http", func(proxyURL *url.URL, forwardDialer proxy_Dialer) (proxy_Dialer, error) {
		p, ok := forwardDialer.(*netDialer)
		if !ok {
			return nil, errors.New("type assertion failed when ini proxy info")
		}
		return &httpProxyDialer{proxyURL: proxyURL, forwardDial: forwardDialer.Dial, proxyHeader: p.proxyHeader}, nil
	})
}

type httpProxyDialer struct {
	proxyURL    *url.URL
	forwardDial func(network, addr string) (net.Conn, error)
	proxyHeader http.Header
}

func (hpd *httpProxyDialer) Dial(network string, addr string) (net.Conn, error) {
	hostPort, _ := hostPortNoPort(hpd.proxyURL)
	conn, err := hpd.forwardDial(network, hostPort)
	if err != nil {
		return nil, err
	}

	connectHeader := make(http.Header)
	if user := hpd.proxyURL.User; user != nil {
		proxyUser := user.Username()
		if proxyPassword, passwordSet := user.Password(); passwordSet {
			credential := base64.StdEncoding.EncodeToString([]byte(proxyUser + ":" + proxyPassword))
			connectHeader.Set("Proxy-Authorization", "Basic "+credential)
		}
	}

	for k, v := range hpd.proxyHeader {
		connectHeader[k] = v
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

	// Read response. It's OK to use and discard buffered reader here becaue
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
