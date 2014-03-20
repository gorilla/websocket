# File Watch example.

This example displays the contents of a file in the browser. If the file
changes, the server sends the update file to the browser.

    $ go get github.com/gorilla/websocket
    $ cd `go list -f '{{.Dir}}' github.com/gorilla/websocket/examples/tail`
    $ go run tail.go <name of file>
    # open http://localhost:8080/
    # modify the file to see it update in the browser.
