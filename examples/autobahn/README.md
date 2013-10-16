# Test

Clients and servers for the [Autobahn WebSockets Test Suite](http://autobahn.ws/testsuite).

To test different code paths in the package, the test server echoes messages two ways:

- Read the entire message using io.ReadAll and write the message in one chunk.
- Copy the message in parts using io.Copy

To test the server, run it

    go run server.go

and start the client test driver

    wstest -m fuzzingclient -s fuzzingclient.json

When the client completes, it writes a report to reports/servers/index.html.
