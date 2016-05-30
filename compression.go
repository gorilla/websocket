package websocket

import (
	//"bytes"
	"compress/flate"
	//"fmt"
	"io"
)

const (
	// Supported compression algorithm and parameters.
	CompressPermessageDeflate = "permessage-deflate; server_no_context_takeover; client_no_context_takeover"

	// Deflate compression level
	compressDeflateLevel int = 3
)

// Sits between a flate writer and the underlying writer i.e. messageWriter
// Truncates last bytes of flate compresses message
type FlateAdaptor struct {
	last5bytes []byte
	msgWriter  io.WriteCloser
}

func NewFlateAdaptor(w io.WriteCloser) *FlateAdaptor {
	return &FlateAdaptor{
		msgWriter:  w,
		last5bytes: []byte{},
	}
}

func (aw *FlateAdaptor) Write(p []byte) (n int, err error) {

	t := append(aw.last5bytes, p...)

	if len(t) > 4 {
		aw.last5bytes = make([]byte, 5)
		copy(aw.last5bytes, t[len(t)-5:])
		_, err = aw.msgWriter.Write(t[:len(t)-5])
	} else {
		aw.last5bytes = make([]byte, len(t))
		aw.last5bytes = t
	}

	n = len(p)
	return
}

func (aw *FlateAdaptor) writeEndBlock() (int, error) {
	var t []byte
	if aw.last5bytes[4] != 0x00 {
		t = append(aw.last5bytes, 0x00)
	}

	return aw.msgWriter.Write(t[:len(t)-5])
}

func (aw *FlateAdaptor) Close() (err error) {
	if _, err = aw.writeEndBlock(); err == nil {
		err = aw.msgWriter.Close()
	}
	return
}

// FlateAdaptorWriter --> FlateAdaptor --> messageWriter
type FlateAdaptorWriter struct {
	flWriter  *flate.Writer
	flAdaptor *FlateAdaptor
}

func NewFlateAdaptorWriter(msgWriter io.WriteCloser, level int) (faw *FlateAdaptorWriter, err error) {
	faw = &FlateAdaptorWriter{
		flAdaptor: NewFlateAdaptor(msgWriter),
	}
	faw.flWriter, err = flate.NewWriter(faw.flAdaptor, level)
	return
}

func (faw *FlateAdaptorWriter) Write(p []byte) (c int, err error) {
	if c, err = faw.flWriter.Write(p); err == nil {
		err = faw.flWriter.Flush()
	}
	return
}

func (faw *FlateAdaptorWriter) Close() (err error) {
	if err = faw.flWriter.Close(); err == nil {
		err = faw.flAdaptor.Close()
	}

	return
}
