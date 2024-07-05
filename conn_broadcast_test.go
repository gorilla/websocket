// Copyright 2017 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package websocket

import (
	"io"
	"sync/atomic"
	"testing"
)

// broadcastBench allows to run broadcast benchmarks.
// In every broadcast benchmark we create many connections, then send the same
// message into every connection and wait for all writes complete. This emulates
// an application where many connections listen to the same data - i.e. PUB/SUB
// scenarios with many subscribers in one channel.
type broadcastBench struct {
	w           io.Writer
	closeCh     chan struct{}
	doneCh      chan struct{}
	count       int32
	conns       []*broadcastConn
	compression bool
	usePrepared bool
}

type broadcastMessage struct {
	payload  []byte
	prepared *PreparedMessage
}

type broadcastConn struct {
	conn  *Conn
	msgCh chan *broadcastMessage
}

func newBroadcastConn(c *Conn) *broadcastConn {
	return &broadcastConn{
		conn:  c,
		msgCh: make(chan *broadcastMessage, 1),
	}
}

func newBroadcastBench(usePrepared, compression bool) *broadcastBench {
	bench := &broadcastBench{
		w:           io.Discard,
		doneCh:      make(chan struct{}),
		closeCh:     make(chan struct{}),
		usePrepared: usePrepared,
		compression: compression,
	}
	bench.makeConns(10000)
	return bench
}

func (b *broadcastBench) makeConns(numConns int) {
	conns := make([]*broadcastConn, numConns)

	for i := 0; i < numConns; i++ {
		c := newTestConn(nil, b.w, true)
		if b.compression {
			c.enableWriteCompression = true
			c.newCompressionWriter = compressNoContextTakeover
		}
		conns[i] = newBroadcastConn(c)
		go func(c *broadcastConn) {
			for {
				select {
				case msg := <-c.msgCh:
					if msg.prepared != nil {
						_ = c.conn.WritePreparedMessage(msg.prepared)
					} else {
						_ = c.conn.WriteMessage(TextMessage, msg.payload)
					}
					val := atomic.AddInt32(&b.count, 1)
					if val%int32(numConns) == 0 {
						b.doneCh <- struct{}{}
					}
				case <-b.closeCh:
					return
				}
			}
		}(conns[i])
	}
	b.conns = conns
}

func (b *broadcastBench) close() {
	close(b.closeCh)
}

func (b *broadcastBench) broadcastOnce(msg *broadcastMessage) {
	for _, c := range b.conns {
		c.msgCh <- msg
	}
	<-b.doneCh
}

func BenchmarkBroadcast(b *testing.B) {
	benchmarks := []struct {
		name        string
		usePrepared bool
		compression bool
	}{
		{"NoCompression", false, false},
		{"Compression", false, true},
		{"NoCompressionPrepared", true, false},
		{"CompressionPrepared", true, true},
	}
	payload := textMessages(1)[0]
	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			bench := newBroadcastBench(bm.usePrepared, bm.compression)
			defer bench.close()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				message := &broadcastMessage{
					payload: payload,
				}
				if bench.usePrepared {
					pm, _ := NewPreparedMessage(TextMessage, message.payload)
					message.prepared = pm
				}
				bench.broadcastOnce(message)
			}
			b.ReportAllocs()
		})
	}
}
