// Copyright 2016 The Gorilla WebSocket Authors. All rights reserved.  Use of
// this source code is governed by a BSD-style license that can be found in the
// LICENSE file.

// !appengine

package websocket

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
	"unsafe"
)

func maskBytesByByte(key [4]byte, pos int, b []byte) int {
	for i := range b {
		b[i] ^= key[pos&3]
		pos++
	}
	return pos & 3
}

func notzero(b []byte) int {
	for i := range b {
		if b[i] != 0 {
			return i
		}
	}
	return -1
}

func maskBytesV1(key [4]byte, pos int, b []byte) int {
	// Mask one byte at a time for small buffers.
	if len(b) < 2*wordSize {
		for i := range b {
			b[i] ^= key[pos&3]
			pos++
		}
		return pos & 3
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
	var k [wordSize]byte
	for i := range k {
		k[i] = key[(pos+i)&3]
	}
	kw := *(*uintptr)(unsafe.Pointer(&k))

	// Mask one word at a time.
	n := (len(b) / wordSize) * wordSize
	for i := 0; i < n; i += wordSize {
		*(*uintptr)(unsafe.Pointer(uintptr(unsafe.Pointer(&b[0])) + uintptr(i))) ^= kw
	}

	// Mask one byte at a time for remaining bytes.
	b = b[n:]
	for i := range b {
		b[i] ^= key[pos&3]
		pos++
	}

	return pos & 3
}

func TestMaskBytes(t *testing.T) {
	key := [4]byte{1, 2, 3, 4}
	for size := 1; size <= 1024; size++ {
		for align := 0; align < wordSize; align++ {
			for pos := 0; pos < 4; pos++ {
				b := make([]byte, size+align)[align:]
				maskBytes(key, pos, b)
				maskBytesByByte(key, pos, b)
				if i := notzero(b); i >= 0 {
					t.Errorf("size:%d, align:%d, pos:%d, offset:%d", size, align, pos, i)
				}
			}
		}
	}
}

func TestMaskBytesWithRandomMessage(t *testing.T) {
	keys := [][4]byte{
		{1, 2, 3, 4},
		{0, 0, 0, 0},
	}
	for _, key := range keys {
		for size := 1; size <= 1024; size++ {
			for align := 0; align < wordSize; align++ {
				for pos := 0; pos < 4; pos++ {
					byteMessage := make([]byte, size+align)[align:]
					for i := 0; i < len(byteMessage); i++ {
						byteMessage[i] = uint8(rand.Uint32())
					}
					byteMessageCopy := make([]byte, len(byteMessage))
					copy(byteMessageCopy, byteMessage)
					posBytes := maskBytes(key, pos, byteMessage)
					posBytesByByte := maskBytesByByte(key, pos, byteMessageCopy)
					if posBytes != posBytesByByte {
						t.Errorf("keys:%v, size:%d, align:%d, pos:%d", key, size, align, pos)
						return
					}
					if !bytes.Equal(byteMessage, byteMessageCopy) {
						t.Errorf("keys:%v, size:%d, align:%d, pos:%d", key, size, align, pos)
						return
					}
				}
			}
		}
	}
}

func BenchmarkMaskBytes(b *testing.B) {
	for _, size := range []int{2, 4, 8, 16, 32, 512, 1024, 1048576} {
		b.Run(fmt.Sprintf("size-%d", size), func(b *testing.B) {
			for _, align := range []int{wordSize / 2} {
				b.Run(fmt.Sprintf("align-%d", align), func(b *testing.B) {
					for _, fn := range []struct {
						name string
						fn   func(key [4]byte, pos int, b []byte) int
					}{
						{"byte", maskBytesByByte},
						{"wordV1", maskBytesV1},
						{"word", maskBytes},
					} {
						b.Run(fn.name, func(b *testing.B) {
							key := newMaskKey()
							data := make([]byte, size+align)[align:]
							b.ResetTimer()
							for i := 0; i < b.N; i++ {
								fn.fn(key, 0, data)
							}
							b.SetBytes(int64(len(data)))
						})
					}
				})
			}
		})
	}
}
