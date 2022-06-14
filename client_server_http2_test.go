// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.14
// +build go1.14

package websocket

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNextProtos(t *testing.T) {
	ts := httptest.NewUnstartedServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)
	ts.EnableHTTP2 = true
	ts.StartTLS()
	defer ts.Close()

	d := Dialer{
		TLSClientConfig: ts.Client().Transport.(*http.Transport).TLSClientConfig,
	}

	r, err := ts.Client().Get(ts.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	r.Body.Close()

	// Asserts that Dialer.TLSClientConfig.NextProtos contains "h2"
	// after the Client.Get call from net/http above.
	var containsHTTP2 bool = false
	for _, proto := range d.TLSClientConfig.NextProtos {
		if proto == "h2" {
			containsHTTP2 = true
		}
	}
	if !containsHTTP2 {
		t.Fatalf("Dialer.TLSClientConfig.NextProtos does not contain \"h2\"")
	}

	_, _, err = d.Dial(makeWsProto(ts.URL), nil)
	if err == nil {
		t.Fatalf("Dial succeeded, expect fail ")
	}
}
