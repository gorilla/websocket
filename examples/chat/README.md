# Chat Example

This application shows how to use the
[websocket](https://github.com/gorilla/websocket) package to implement a simple
web chat application.

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

The server application defines two types, `Client` and `Hub`. The server
creates an instance of the `Client` type for each websocket connection. A
`Client` acts as an intermediary between the websocket connection and a single
instance of the `Hub` type. The `Hub` maintains a set of registered clients and
broadcasts messages to the clients.

The application runs one goroutine for the `Hub` and two goroutines for each
`Client`. The application ensures that there is at most one reader and writer on
a connection by executing all reads and writes for a client on a separate goroutine. 
This is because Gorilla websocket connections support at most one concurrent reader
and one concurrent writer. The goroutines communicate with each other using channels. 
The `Hub` has channels for registering clients, unregistering clients, and broadcasting
messages. 

A `Client` has a buffered channel of outbound messages which accepts messages of type 
`Message`. The `Message` type is a struct that represents a JSON message sent by a client. 
It contains one field representing the `Body` of the message. The Gorilla websocket library
does not enforce the datatype used to communicate between the client and the server. 
However, JSON is one of the most commonly used data-interchange formats and was chosen
for the purposes of this example.

### Hub 

The code for the `Hub` type is in
[hub.go](https://github.com/gorilla/websocket/blob/master/examples/chat/hub.go). 
The application's `main` function starts the hub's `run` method as a goroutine.
Clients send requests to the hub using the `register`, `unregister`, and
`broadcast` channels.

The hub registers clients by adding the client pointer as a key in the
`clients` map. The map value is always true.

The unregister code is a little more complicated. In addition to deleting the
client pointer from the `clients` map, the hub closes the client's `send`
channel to signal the client that no more messages will be sent to the client.

When a client's `readPump` method sends a message to the hub's broadcast channel, 
the hub must broadcast this message to all clients on the websocket connection. 
The hub handles this by looping over the registered clients, marshalling the `Message` 
into JSON, and sending it to each client's `send` channel. Writing the message to
the websocket is then handled by each client's `writePump` goroutine. If the
client's `send` buffer is full, then the hub assumes that the client is dead or
stuck. In this case, the hub unregisters the client and closes the websocket.

### Client

The code for the `Client` type is in [client.go](https://github.com/gorilla/websocket/blob/master/examples/chat/client.go).

The `serveWs` function is registered by the application's `main` function as
an HTTP handler. The handler upgrades the HTTP connection to the WebSocket
protocol, creates a client, registers the client with the hub, and schedules the
client to be unregistered using a defer statement.

Next, the HTTP handler starts the client's `writePump` method as a goroutine.
This method transfers messages from the client's send channel to the websocket
connection. The writer method exits when the channel is closed by the hub or
there's an error writing to the websocket connection.

Finally, the HTTP handler calls the client's `readPump` method. This method
decodes client messages into the `Message` type, and sends them to the hub's 
broadcast channel.

WebSocket connections [support one concurrent reader and one concurrent
writer](https://godoc.org/github.com/gorilla/websocket#hdr-Concurrency). The
application ensures that these concurrency requirements are met by executing
all reads from the `readPump` goroutine and all writes from the `writePump`
goroutine.

To improve efficiency under high load, the `writePump` function coalesces
pending chat messages in the `send` channel to a single WebSocket message. This
reduces the number of system calls and the amount of data sent over the
network.

## Frontend

The frontend code is in [home.html](https://github.com/gorilla/websocket/blob/master/examples/chat/home.html).

On document load, the script checks for websocket functionality in the browser.
If websocket functionality is available, then the script opens a connection to
the server and registers a callback to handle messages from the server. The
callback appends the message to the chat log using the appendLog function.

To allow the user to manually scroll through the chat log without interruption
from new messages, the `appendLog` function checks the scroll position before
adding new content. If the chat log is scrolled to the bottom, then the
function scrolls new content into view after adding the content. Otherwise, the
scroll position is not changed.

The form handler stringifies user input into the JSON corresponding to the server's 
`Message` type: `{ body: msg }`. It sends this message to the websocket and clears
the input field.
