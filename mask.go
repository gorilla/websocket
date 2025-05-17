// Copyright 2016 The Gorilla WebSocket Authors. All rights reserved.  Use of
// this source code is governed by a BSD-style license that can be found in the
// LICENSE file.

//go:build !appengine
// +build !appengine

package websocket

import "unsafe"

const wordSize = int(unsafe.Sizeof(uintptr(0)))

func maskBytes(key [4]byte, pos int, b []byte) int {
	// Mask one byte at a time for small buffers.
	if len(b) <= 2*wordSize {
		for i := range b {
			b[i] ^= key[pos&3]
			pos++
		}
		return pos & 3
	}

	if key == [4]byte{} {
		return (pos + len(b)) & 3
	}
	// Mask one byte at a time to word boundary.
	if n := int(uintptr(unsafe.Pointer(&b[0]))) % wordSize; n != 0 {
		n = wordSize - n
		for i := range b[:n] {
			b[i] ^= key[pos&3]
			pos++
		}
		b = b[n:]
	}

	// Create aligned word size key.
	var kw uintptr
	var k [wordSize]byte
	if wordSize == 8 {
		k[0] = key[(pos+0)&3]
		k[1] = key[(pos+1)&3]
		k[2] = key[(pos+2)&3]
		k[3] = key[(pos+3)&3]
		kw = *(*uintptr)(unsafe.Pointer(&k))
		kw = (kw << 32) | kw
	} else {
		for i := range k {
			k[i] = key[(pos+i)&3]
		}
		kw = *(*uintptr)(unsafe.Pointer(&k))
	}

	// Mask one word at a time.
	n := (len(b) / wordSize) * wordSize
	p0 := unsafe.Pointer(&b[0])
	for i := 0; i < n; i += wordSize {
		*(*uintptr)(unsafe.Pointer(uintptr(p0) + uintptr(i))) ^= kw
	}

	// Mask one byte at a time for remaining bytes.
	b = b[n:]
	for i := range b {
		b[i] ^= key[pos&3]
		pos++
	}

	return pos & 3
}
