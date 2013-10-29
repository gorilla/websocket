# Gorilla WebSocket 

Gorilla WebSocket is a [Go](http://golang.org/) implementation of the
[WebSocket](http://www.rfc-editor.org/rfc/rfc6455.txt) protocol.

### Documentation

* [Reference](http://godoc.org/github.com/gorilla/websocket)
* [Chat example](https://github.com/gorilla/websocket/tree/master/examples/chat)

### Status

The Gorilla WebSocket package provides a complete and tested implementation of
the [WebSocket](http://www.rfc-editor.org/rfc/rfc6455.txt) protocol. The
package API is stable. 

### Installation

    go get github.com/gorilla/websocket

### Protocol Compliance

The Gorilla WebSocket package passes the server tests in the [Autobahn WebSockets Test
Suite](http://autobahn.ws/testsuite) using the application in the [examples/autobahn
subdirectory](https://github.com/gorilla/websocket/tree/master/examples/autobahn).

### Gorilla WebSocket compared with other packages

<table>
<tr>
<th></th>
<th><a href="http://godoc.org/github.com/gorilla/websocket">gorilla</a></th>
<th><a href="http://godoc.org/code.google.com/p/go.net/websocket">go.net</a></th>
</tr>
<tr>
<tr><td>Protocol support</td><td>RFC 6455</td><td>RFC 6455</td></tr>
<tr><td>Limit size of received message</td><td>Yes</td><td>No</td></tr>
<tr><td>Send pings and receive pongs</td><td>Yes</td><td>No</td></tr>
<tr><td>Send close message</td><td>Yes</td><td>No</td></tr>
<tr><td>Read message using io.Reader</td><td>Yes</td><td>No, see note</td></tr>
<tr><td>Write message using io.WriteCloser</td><td>Yes</td><td>No, see note</td></tr>
<tr><td>Encode, decode JSON message</td><td>Yes</td><td>Yes</td></tr>
</table>

Note: The go.net io.Reader and io.Writer operate across WebSocket message
boundaries. Read returns when the input buffer is full or a message boundary is
encountered, Each call to Write sends a message. The Gorilla io.Reader and
io.WriteCloser operate on a single WebSocket message.

