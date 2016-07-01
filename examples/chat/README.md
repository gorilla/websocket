# Chat Example

This application shows how to use use the
[websocket](https://github.com/gorilla/websocket) package and
[jQuery](http://jquery.com) to implement a simple web chat application.

## Running the example

The example requires a working Go development environment. The [Getting
Started](http://golang.org/doc/install) page describes how to install the
development environment.

Once you have Go up and running, you can download, build and run the example
using the following commands.

    $ go get github.com/gorilla/websocket
    $ cd `go list -f '{{.Dir}}' github.com/gorilla/websocket/examples/chat`
    $ go run *.go

To use the chat example, open http://localhost:8080/ in your browser.

## Server

The server application is implemented with Go's [http](https://golang.org/pkg/net/http/) and the Gorilla [websocket](https://godoc.org/github.com/gorilla/websocket) package.

The application defines two types, `Conn` and `Hub`. The server creates an instance of the `Conn` type for each webscocket connection. A `Conn` acts as an intermediary between the websocket and a single instance of the `Hub` type. The `Hub` maintains a set of registered connections and broadcasts messages to the connections.

The application runs one goroutine for the `Hub` and two goroutines for each `Conn`. The goroutines communicate with each other using channels. The `Hub` has channels for registering connections, unregistering connections and broadcasting messages. A `Conn` has a buffered channel of outbound messages. One of the connection's goroutines reads messages from this channel and writes the messages to the webscoket. The other connection goroutine reads messages from the websocket and sends them to the hub.

### Hub 

The code for the `Hub` type is in [hub.go](https://github.com/gorilla/websocket/blob/master/examples/chat/hub.go). 

The application's `main` function starts the hub's `run` method as a goroutine. Connections send requests to the hub using the `register`, `unregister` and `broadcast` channels.

The hub registers connections by adding the connection pointer as a key in the `connections` map. The map value is always true.

The unregister code is a little more complicated. In addition to deleting the connection pointer from the connections map, the hub closes the connection's `send` channel to signal the connection that no more messages will be sent to the connection.

The hub handles messages by looping over the registered connections and sending the message to the connection's `send` channel. If the connection's `send` buffer is full, then the hub assumes that the client is dead or stuck. In this case, the hub unregisters the connection and closes the websocket.

### Conn

The code for the `Conn` type is in [conn.go](https://github.com/gorilla/websocket/blob/master/examples/chat/conn.go).

An instance of the `wsHandler` type is registered by the application's `main` function as an HTTP handler. The handler upgrades the HTTP connection to the WebSocket protocol, creates a connection object, registers the connection with the hub and schedules the connection to be unregistered using a defer statement.

Next, the HTTP handler starts the connection's `writePump` method as a goroutine. This method transfers messages from the connection's send channel to the websocket. The writer method exits when the channel is closed by the hub or there's an error writing to the websocket.

Finally, the HTTP handler calls the connection's `readPump` method. This method transfers inbound messages from the websocket to the hub.

## Client

The code for the client is in [home.html](https://github.com/gorilla/websocket/blob/master/examples/chat/home.html).

The client uses [jQuery](http://jquery.com/) to manipulate objects in the browser.

On document load, the script checks for websocket functionality in the browser. If websocket functionality is available, then the script opens a connection to the server and registers a callback to handle messages from the server. The callback appends the message to the chat log using the appendLog function.

To allow the user to manually scroll through the chat log without interruption from new messages, the `appendLog` function checks the scroll position before adding new content. If the chat log is scrolled to the bottom, then the function scrolls new content into view after adding the content. Otherwise, the scroll position is not changed.

The form handler writes the user input to the websocket and clears the input field.
