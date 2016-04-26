package websocket

import (
	"bytes"
	"compress/flate"
	"testing"
)

func Test_NewAdaptorWriter(t *testing.T) {
	backendBuff := new(bytes.Buffer)
	aw := NewAdaptorWriter(backendBuff)

	fw, err := flate.NewWriter(aw, -1)
	if err != nil {
		t.Fatal(err)
	}

	var n int
	n, err = fw.Write([]byte("test"))
	t.Log(n, err)

	if err = fw.Flush(); err != nil {
		t.Fatal(err)
	}

}
