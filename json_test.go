// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package websocket

import (
	"bytes"
	"reflect"
	"testing"
)

func TestJSON(t *testing.T) {
	var buf bytes.Buffer
	c := fakeNetConn{&buf, &buf}
	wc := newConn(c, true, 1024, 1024)
	rc := newConn(c, false, 1024, 1024)

	var actual, expect struct {
		A int
		B string
	}
	expect.A = 1
	expect.B = "hello"

	if err := wc.WriteJSON(&expect); err != nil {
		t.Fatal("write", err)
	}

	if err := rc.ReadJSON(&actual); err != nil {
		t.Fatal("read", err)
	}

	if !reflect.DeepEqual(&actual, &expect) {
		t.Fatal("equal", actual, expect)
	}
}

func TestDeprecatedJSON(t *testing.T) {
	var buf bytes.Buffer
	c := fakeNetConn{&buf, &buf}
	wc := newConn(c, true, 1024, 1024)
	rc := newConn(c, false, 1024, 1024)

	var actual, expect struct {
		A int
		B string
	}
	expect.A = 1
	expect.B = "hello"

	if err := WriteJSON(wc, &expect); err != nil {
		t.Fatal("write", err)
	}

	if err := ReadJSON(rc, &actual); err != nil {
		t.Fatal("read", err)
	}

	if !reflect.DeepEqual(&actual, &expect) {
		t.Fatal("equal", actual, expect)
	}
}
