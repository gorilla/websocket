# Gorilla WebSocket

[![GoDoc](https://godoc.org/github.com/gorilla/websocket?status.svg)](https://godoc.org/github.com/gorilla/websocket)
[![CircleCI](https://circleci.com/gh/gorilla/websocket.svg?style=svg)](https://circleci.com/gh/gorilla/websocket)

Gorilla WebSocket is a [Go](http://golang.org/) implementation of the
[WebSocket](http://www.rfc-editor.org/rfc/rfc6455.txt) protocol.

### Documentation

* [API Reference](http://godoc.org/github.com/gorilla/websocket)
* [Chat example](https://github.com/gorilla/websocket/tree/master/examples/chat)
* [Command example](https://github.com/gorilla/websocket/tree/master/examples/command)
* [Client and server example](https://github.com/gorilla/websocket/tree/master/examples/echo)
* [File watch example](https://github.com/gorilla/websocket/tree/master/examples/filewatch)

### Status

The Gorilla WebSocket package provides a complete and tested implementation of
the [WebSocket](http://www.rfc-editor.org/rfc/rfc6455.txt) protocol. The
package API is stable.

### Installation

    go get github.com/gorilla/websocket

### Protocol Compliance

The Gorilla WebSocket package passes the server tests in the [Autobahn Test
Suite](https://github.com/crossbario/autobahn-testsuite) using the application in the [examples/autobahn
subdirectory](https://github.com/gorilla/websocket/tree/master/examples/autobahn).

### Gorilla WebSocket compared with other packages

This comparison used to include [golang.org/x/net/websocket](http://godoc.org/golang.org/x/net/websocket) but it has
been deprecated. Do not use [golang.org/x/net/websocket](http://godoc.org/golang.org/x/net/websocket) in future projects.

See [golang/go#18152](https://github.com/golang/go/issues/18152)

Another comparison of available Go WebSocket libraries is available at [nhooyr.io/websocket](https://github.com/nhooyr/websocket#comparison)

#### [nhooyr.io/websocket](https://nhooyr.io/websocket)

gorilla/websocket is 6 years old and thus very mature and battle tested. On the other hand, it has
accumulated cruft over the years.
[nhooyr.io/websocket](http://godoc.org/nhooyr.io/websocket) is much newer and attempts to
provide a more idiomatic, minimal and useful API for WebSockets.

|  | [github.com/gorilla](http://godoc.org/github.com/gorilla/websocket) | [nhooyr.io/websocket](http://godoc.org/nhooyr.io/websocket) |
| :--- | ---: | ---: |
| **[RFC 6455](http://tools.ietf.org/html/rfc6455) Features** | 
| Passes [Autobahn Test Suite](https://github.com/crossbario/autobahn-testsuite) | [Yes](https://github.com/gorilla/websocket/tree/master/examples/autobahn) | [Yes](https://github.com/nhooyr/websocket/blob/master/conn_test.go) |
| Receive [fragmented](https://tools.ietf.org/html/rfc6455#section-5.4) message | Yes | Yes |
| Send [close](https://tools.ietf.org/html/rfc6455#section-5.5.1) message | [Yes](http://godoc.org/github.com/gorilla/websocket#hdr-Control_Messages) | [Yes](https://godoc.org/nhooyr.io/websocket#Conn.Close) |
| Send [pings](https://tools.ietf.org/html/rfc6455#section-5.5.2) and receive [pongs](https://tools.ietf.org/html/rfc6455#section-5.5.3) | [Yes](http://godoc.org/github.com/gorilla/websocket#hdr-Control_Messages) | [Yes](https://godoc.org/nhooyr.io/websocket#Conn.Ping)
| Get [type](https://tools.ietf.org/html/rfc6455#section-5.6) of received message | Yes | Yes |
| **Other Features** |
| Well tested | Yes | [Yes](https://codecov.io/gh/nhooyr/websocket) | 
| Low level protocol control | [Yes](http://godoc.org/github.com/gorilla/websocket#hdr-Control_Messages) | No |
| [Compression Extensions](https://tools.ietf.org/html/rfc7692) | [Experimental](https://godoc.org/github.com/gorilla/websocket#hdr-Compression_EXPERIMENTAL) | [No](https://github.com/nhooyr/websocket#design-justifications) |
| Read with [io.Reader](https://golang.org/pkg/io/#Reader) | [Yes](http://godoc.org/github.com/gorilla/websocket#Conn.NextReader) | [Yes](https://godoc.org/nhooyr.io/websocket#Conn.Reader) |
| Write with [io.WriteCloser](https://golang.org/pkg/io/#WriteCloser) | [Yes](http://godoc.org/github.com/gorilla/websocket#Conn.NextWriter) | [Yes](https://godoc.org/nhooyr.io/websocket#Conn.Writer) |
| Compile for [Wasm](https://github.com/golang/go/wiki/WebAssembly) | [No](https://github.com/gorilla/websocket/issues/432) | [Yes](https://godoc.org/nhooyr.io/websocket#hdr-Wasm) |
| Use stdlib [*http.Client](https://golang.org/pkg/net/http/#Client) | No | [Yes](https://godoc.org/nhooyr.io/websocket#DialOptions) |
| Concurrent writers | [No](https://godoc.org/github.com/gorilla/websocket#hdr-Concurrency) | [Yes](https://godoc.org/nhooyr.io/websocket#Conn) |
| Configurable buffer sizes | [Yes](https://godoc.org/github.com/gorilla/websocket#hdr-Buffers) | No |
| Prepared messages | [Yes](https://godoc.org/github.com/gorilla/websocket#PreparedMessage) | No |
| Full [context.Context](https://blog.golang.org/context)  support | No | Yes
| [net.Conn](https://golang.org/pkg/net/#Conn) adapter | No | [Yes](https://godoc.org/nhooyr.io/websocket#NetConn) |
