# WebSocket 

This project is a [Go](http://golang.org/) implementation of the
[WebSocket](http://www.rfc-editor.org/rfc/rfc6455.txt) protocol.

The project passes the server tests in the [Autobahn WebSockets Test
Suite](http://autobahn.ws/testsuite) using the application in the [examples/autobahn
subdirectory](https://github.com/gorilla/websocket/tree/master/examples/autobahn).

## Documentation

* [Reference](http://godoc.org/github.com/gorilla/websocket)
* [Chat example](https://github.com/gorilla/websocket/tree/master/examples/chat)

## Features

- Send and receive ping, pong and close control messages.
- Limit size of received messages.
- Stream messages.
- Specify IO buffer sizes.
- Application has full control over origin checks and sub-protocol negotiation.

## Installation

    go get github.com/gorilla/websocket

