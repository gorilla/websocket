package websocket

import (
	"bytes"
	"fmt"
	"io"
	"testing"
)

type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }

func TestTruncWriter(t *testing.T) {
	const data = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijlkmnopqrstuvwxyz987654321"
	for n := 1; n <= 10; n++ {
		var b bytes.Buffer
		w := &truncWriter{w: nopCloser{&b}}
		p := []byte(data)
		for len(p) > 0 {
			m := len(p)
			if m > n {
				m = n
			}
			_, _ = w.Write(p[:m])
			p = p[m:]
		}
		if b.String() != data[:len(data)-len(w.p)] {
			t.Errorf("%d: %q", n, b.String())
		}
	}
}

func textMessages(num int) [][]byte {
	messages := make([][]byte, num)
	for i := 0; i < num; i++ {
		msg := fmt.Sprintf("planet: %d, country: %d, city: %d, street: %d", i, i, i, i)
		messages[i] = []byte(msg)
	}
	return messages
}

func BenchmarkWriteNoCompression(b *testing.B) {
	w := io.Discard
	c := newTestConn(nil, w, false)
	messages := textMessages(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.WriteMessage(TextMessage, messages[i%len(messages)])
	}
	b.ReportAllocs()
}

func BenchmarkWriteWithCompression(b *testing.B) {
	w := io.Discard
	c := newTestConn(nil, w, false)
	messages := textMessages(100)
	c.enableWriteCompression = true
	c.newCompressionWriter = compressNoContextTakeover
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.WriteMessage(TextMessage, messages[i%len(messages)])
	}
	b.ReportAllocs()
}

func TestValidCompressionLevel(t *testing.T) {
	c := newTestConn(nil, nil, false)
	for _, level := range []int{minCompressionLevel - 1, maxCompressionLevel + 1} {
		if err := c.SetCompressionLevel(level); err == nil {
			t.Errorf("no error for level %d", level)
		}
	}
	for _, level := range []int{minCompressionLevel, maxCompressionLevel} {
		if err := c.SetCompressionLevel(level); err != nil {
			t.Errorf("error for level %d", level)
		}
	}
}

func TestFlateReadWrapperDoubleClose(t *testing.T) {
	var buf bytes.Buffer
	c := newTestConn(nil, &buf, true)
	c.newCompressionWriter = compressNoContextTakeover
	c.enableWriteCompression = true

	// Write a compressed message to get valid compressed data.
	if err := c.WriteMessage(TextMessage, []byte("hello")); err != nil {
		t.Fatal(err)
	}

	// Set up a reader connection with decompression.
	rc := newTestConn(&buf, io.Discard, false)
	rc.newDecompressionReader = decompressNoContextTakeover

	_, r, err := rc.NextReader()
	if err != nil {
		t.Fatal(err)
	}

	// Read all data, which triggers preemptive close on EOF.
	if _, err := io.ReadAll(r); err != nil {
		t.Fatal(err)
	}

	// NextReader calls Close() again on the reader. This must not return an
	// error because Close() should be idempotent per the fix for #859.
	_, _, err = rc.NextReader()
	// NextReader blocks waiting for the next frame. Since there's no more data,
	// we expect an EOF-related error, not a close error.
	if err != nil && err.Error() == "io: read/write on closed pipe" {
		t.Fatal("Close() returned error on second call, should be idempotent")
	}
}

func TestFlateWriteWrapperDoubleClose(t *testing.T) {
	ww := &flateWriteWrapper{}
	// Close on an already-nil writer should return nil, not an error.
	if err := ww.Close(); err != nil {
		t.Fatalf("expected nil error on double close of flateWriteWrapper, got: %v", err)
	}
}

func TestFlateReadWrapperCloseIdempotent(t *testing.T) {
	rw := &flateReadWrapper{}
	// Close on an already-nil reader should return nil, not an error.
	if err := rw.Close(); err != nil {
		t.Fatalf("expected nil error on double close of flateReadWrapper, got: %v", err)
	}
}
