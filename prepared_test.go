// Copyright 2017 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package websocket

import (
	"bytes"
	"compress/flate"
	"fmt"
	"io"
	"math/rand"
	"sync"
	"testing"
)

var preparedMessageTests = []struct {
	messageType            int
	isServer               bool
	enableWriteCompression bool
	compressionLevel       int
}{
	// Server
	{TextMessage, true, false, flate.BestSpeed},
	{TextMessage, true, true, flate.BestSpeed},
	{TextMessage, true, true, flate.BestCompression},
	{PingMessage, true, false, flate.BestSpeed},
	{PingMessage, true, true, flate.BestSpeed},

	// Client
	{TextMessage, false, false, flate.BestSpeed},
	{TextMessage, false, true, flate.BestSpeed},
	{TextMessage, false, true, flate.BestCompression},
	{PingMessage, false, false, flate.BestSpeed},
	{PingMessage, false, true, flate.BestSpeed},
}

func TestPreparedMessage(t *testing.T) {
	testRand := rand.New(rand.NewSource(99))
	prevMaskRand := maskRand
	maskRand = testRand
	defer func() { maskRand = prevMaskRand }()

	for _, tt := range preparedMessageTests {
		var data = []byte("this is a test")
		var buf bytes.Buffer
		c := newTestConn(nil, &buf, tt.isServer)
		if tt.enableWriteCompression {
			c.newCompressionWriter = compressNoContextTakeover
		}
		if err := c.SetCompressionLevel(tt.compressionLevel); err != nil {
			t.Fatal(err)
		}

		// Seed random number generator for consistent frame mask.
		testRand.Seed(1234)

		if err := c.WriteMessage(tt.messageType, data); err != nil {
			t.Fatal(err)
		}
		want := buf.String()

		pm, err := NewPreparedMessage(tt.messageType, data)
		if err != nil {
			t.Fatal(err)
		}

		// Scribble on data to ensure that NewPreparedMessage takes a snapshot.
		copy(data, "hello world")

		// Seed random number generator for consistent frame mask.
		testRand.Seed(1234)

		buf.Reset()
		if err := c.WritePreparedMessage(pm); err != nil {
			t.Fatal(err)
		}
		got := buf.String()

		if got != want {
			t.Errorf("write message != prepared message for %+v", tt)
		}
	}
}

// The pool factory records every allocated scratch buffer, so overwriting all
// buffers tests frame ownership even when sync.Pool discards an entry. This test
// must not run in parallel because it temporarily replaces the package pool.
func TestPreparedMessageBufferOwnership(t *testing.T) {
	newBuffer := preparedWriteBufferPool.New
	defer func() { preparedWriteBufferPool = sync.Pool{New: newBuffer} }()

	keys := []prepareKey{
		{isServer: true},
		{isServer: true, compress: true, compressionLevel: flate.BestSpeed},
		{isServer: true, compress: true, compressionLevel: flate.BestCompression},
		{isServer: false},
		{isServer: false, compress: true, compressionLevel: flate.BestSpeed},
		{isServer: false, compress: true, compressionLevel: flate.BestCompression},
	}
	for _, size := range []int{0, 125, 126, 4096, 65536} {
		t.Run(fmt.Sprintf("size=%d", size), func(t *testing.T) {
			var scratch []*[defaultWriteBufferSize + maxFrameHeaderSize]byte
			preparedWriteBufferPool = sync.Pool{New: func() interface{} {
				buf := new([defaultWriteBufferSize + maxFrameHeaderSize]byte)
				scratch = append(scratch, buf)
				return buf
			}}
			payload := make([]byte, size)
			for i := range payload {
				payload[i] = byte(i*31 + i/127)
			}
			wantPayload := append([]byte(nil), payload...)
			pm, err := NewPreparedMessage(BinaryMessage, payload)
			if err != nil {
				t.Fatal(err)
			}
			if len(scratch) == 0 {
				t.Fatal("initial frame did not borrow scratch from the pool")
			}
			for i := range payload {
				payload[i] ^= 0xff
			}
			retained := make(map[prepareKey][]byte)
			for _, key := range keys {
				_, frame, err := pm.frame(key)
				if err != nil {
					t.Fatal(err)
				}
				retained[key] = append([]byte(nil), frame...)
				// Simulate reuse by unrelated messages after frame construction.
				for _, buf := range scratch {
					for i := range buf {
						buf[i] = 0xff
					}
				}
				if !bytes.Equal(pm.data, wantPayload) {
					t.Fatal("prepared payload aliases caller data or scratch")
				}
				// Creating another variant must not invalidate earlier variants.
				for previous, wantFrame := range retained {
					_, cached, err := pm.frame(previous)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(cached, wantFrame) {
						t.Fatalf("cached frame changed for %+v", previous)
					}
				}
				reader := newTestConn(bytes.NewReader(frame), nil, !key.isServer)
				if key.compress {
					reader.newDecompressionReader = decompressNoContextTakeover
				}
				typ, got, err := reader.ReadMessage()
				if err != nil {
					t.Fatalf("read %+v: %v", key, err)
				}
				if typ != BinaryMessage || !bytes.Equal(got, wantPayload) {
					t.Fatalf("decoded payload differs for %+v", key)
				}
				if key.isServer {
					var out bytes.Buffer
					writer := newTestConn(nil, &out, true)
					// Match buffer size so ordinary/prepared fragmentation agrees.
					writer.writeBuf = make([]byte, defaultWriteBufferSize+maxFrameHeaderSize)
					if key.compress {
						writer.newCompressionWriter = compressNoContextTakeover
					}
					if err := writer.SetCompressionLevel(key.compressionLevel); err != nil {
						t.Fatal(err)
					}
					if err := writer.WriteMessage(BinaryMessage, wantPayload); err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(out.Bytes(), frame) {
						t.Fatalf("wire bytes differ for %+v", key)
					}
				}
			}
		})
	}
}

func TestPreparedMessageConcurrentBufferReuse(t *testing.T) {
	payload := bytes.Repeat([]byte("concurrent prepared message"), 400)
	pm, err := NewPreparedMessage(TextMessage, payload)
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			key := prepareKey{isServer: i%2 == 0, compress: i%3 != 0, compressionLevel: flate.BestSpeed}
			for n := 0; n < 16; n++ {
				freshPayload := bytes.Repeat([]byte{byte(i), byte(n)}, 4096)
				fresh, err := NewPreparedMessage(BinaryMessage, freshPayload)
				if err != nil {
					t.Error(err)
					return
				}
				_, freshData, err := fresh.frame(key)
				if err != nil {
					t.Error(err)
					return
				}
				_, data, err := pm.frame(key)
				if err != nil {
					t.Error(err)
					return
				}
				for _, frame := range []struct {
					name        string
					messageType int
					data        []byte
					payload     []byte
				}{
					{"fresh", BinaryMessage, freshData, freshPayload},
					{"shared", TextMessage, data, payload},
				} {
					reader := newTestConn(bytes.NewReader(frame.data), nil, !key.isServer)
					if key.compress {
						reader.newDecompressionReader = decompressNoContextTakeover
					}
					typ, got, err := reader.ReadMessage()
					if err != nil {
						t.Errorf("%s frame, worker %d, iteration %d: %v", frame.name, i, n, err)
						return
					}
					if typ != frame.messageType || !bytes.Equal(got, frame.payload) {
						t.Errorf("%s frame corrupted, worker %d, iteration %d", frame.name, i, n)
						return
					}
				}
			}
		}(i)
	}
	close(start)
	wg.Wait()
}

func TestPreparedMessageInvalidInput(t *testing.T) {
	for _, tt := range []struct {
		name        string
		messageType int
		payload     []byte
		wantErr     error
	}{
		{"invalid opcode", 99, []byte("invalid"), errBadWriteOpCode},
		{"oversized control", PingMessage, make([]byte, 126), errInvalidControlFrame},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pm, err := NewPreparedMessage(tt.messageType, tt.payload)
			if pm != nil || err != tt.wantErr {
				t.Fatalf("got (%v, %v), want (nil, %v)", pm, err, tt.wantErr)
			}
		})
	}
}

func BenchmarkPreparedMessageConstruction(b *testing.B) {
	for _, size := range []int{256, 1024, 4096, 8192} {
		for _, random := range []bool{false, true} {
			messageType := TextMessage
			pattern := []byte("prepared-message-payload-")
			payload := bytes.Repeat(pattern, (size+len(pattern)-1)/len(pattern))[:size]
			if random {
				messageType = BinaryMessage
				_, _ = rand.New(rand.NewSource(99)).Read(payload)
			}
			for _, compressed := range []bool{false, true} {
				b.Run(fmt.Sprintf("bytes=%d/random=%t/compress=%t", size, random, compressed), func(b *testing.B) {
					c := newTestConn(nil, io.Discard, true)
					if compressed {
						c.newCompressionWriter = compressNoContextTakeover
					}
					b.ReportAllocs()
					b.SetBytes(int64(size))
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						pm, err := NewPreparedMessage(messageType, payload)
						if err != nil {
							b.Fatal(err)
						}
						if err := c.WritePreparedMessage(pm); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		}
	}
}

// Measure the eager frame separately from lazy preparation on the first write.
func BenchmarkNewPreparedMessage(b *testing.B) {
	for _, size := range []int{0, 125, 4096} {
		b.Run(fmt.Sprintf("bytes=%d", size), func(b *testing.B) {
			payload := bytes.Repeat([]byte("x"), size)
			b.ReportAllocs()
			b.SetBytes(int64(size))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := NewPreparedMessage(BinaryMessage, payload); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Repeated writes reuse an already prepared frame and do not borrow scratch.
func BenchmarkPreparedMessageCachedWrite(b *testing.B) {
	payload := bytes.Repeat([]byte("prepared-message-payload-"), 35)
	for _, compressed := range []bool{false, true} {
		b.Run(fmt.Sprintf("compress=%t", compressed), func(b *testing.B) {
			c := newTestConn(nil, io.Discard, true)
			if compressed {
				c.newCompressionWriter = compressNoContextTakeover
			}
			pm, err := NewPreparedMessage(TextMessage, payload)
			if err != nil {
				b.Fatal(err)
			}
			if err := c.WritePreparedMessage(pm); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(payload)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := c.WritePreparedMessage(pm); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkPreparedMessageConstructionParallel(b *testing.B) {
	payload := bytes.Repeat([]byte("prepared-message-payload-"), 35)
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		c := newTestConn(nil, io.Discard, true)
		c.newCompressionWriter = compressNoContextTakeover
		for pb.Next() {
			pm, err := NewPreparedMessage(TextMessage, payload)
			if err != nil {
				b.Error(err)
				return
			}
			if err := c.WritePreparedMessage(pm); err != nil {
				b.Error(err)
				return
			}
		}
	})
}
