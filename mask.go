// Copyright 2016 The Gorilla WebSocket Authors. All rights reserved.  Use of
// this source code is governed by a BSD-style license that can be found in the
// LICENSE file.

package websocket

import (
	"encoding/binary"
	"math/bits"
)

var order = binary.LittleEndian

// MaskBytes uses the bytes from key, starting at pos, to XOR bytes.
// The return is the final (key) pos.
func maskBytes(key [4]byte, pos int, bytes []byte) int {
	if len(bytes) < 8 {
		for i := range bytes {
			bytes[i] ^= key[pos&3]
			pos++
		}
		return pos & 3
	}

	var i int
	// process per 64-bit words first
	key64 := uint64(order.Uint32(key[:]))
	key64 |= key64 << 32
	key64 = bits.RotateLeft64(key64, -pos*8)
	for ; len(bytes) - i > 7; i += 8 {
		order.PutUint64(bytes[i:], order.Uint64(bytes[i:]) ^ key64)
	}

	// multipe of 8 did not change pos
	for ; i < len(bytes); i++ {
		bytes[i] ^= key[pos&3]
		pos++
	}
	return pos & 3
}
