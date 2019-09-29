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

### Alternative libraries

The following libraries may fit your use-case better but are not as mature or widely
used as gorilla/websocket.

- [nhooyr.io/websocket](https://nhooyr.io/websocket)
- [github.com/gobwas/ws](https://github.com/gobwas/ws)
- [golang.org/x/net/websocket](http://godoc.org/golang.org/x/net/websocket)
	- Do not use. It has been deprecated. See [golang/go#18152](https://github.com/golang/go/issues/18152)
