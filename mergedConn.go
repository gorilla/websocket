package websocket

import (
	"net"
)

type mergedConnReader struct {
	net.Conn
	unread []byte
}

func newMergedNetConnReader(conn net.Conn, unread []byte) net.Conn {
	return &mergedConnReader{
		Conn:   conn,
		unread: unread,
	}
}

func (m *mergedConnReader) Read(b []byte) (n int, err error) {
	if len(m.unread) > 0 {
		n = copy(b, m.unread)
		m.unread = m.unread[n:]
		return n, nil
	}
	return m.Conn.Read(b)
}
