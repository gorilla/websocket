# Compression example
This example covers enabling compression on the server.  It starts a websocket server with permessage-deflate enabled for compression.  You can then visit the page to send/recieve messages through the browser. 

Start the server by running the following in this directory:
	
	go run server.go

You can now navigate to the displayed address in you browser:

	http://localhost:12345/
