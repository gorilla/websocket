// Copyright 2013 Gary Burd. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package websocket implements the WebSocket protocol defined in RFC 6455.
//
// Overview
//
// The Conn type represents a WebSocket connection.
//
// A server application calls the Upgrade function to get a pointer to a Conn:
//
//  func handler(w http.ResponseWriter, r *http.Request) {
//      conn, err := websocket.Upgrade(w, r.Header, nil, 1024, 1024)
//      if _, ok := err.(websocket.HandshakeError); ok {
//          http.Error(w, "Not a websocket handshake", 400)
//          return
//      } else if err != nil {
//          log.Println(err)
//          return
//      }
//      ... Use conn to send and receive messages.
//  }
//
// WebSocket messages are represented by the io.Reader interface when receiving
// a message and by the io.WriteCloser interface when sending a message. An
// application receives a message by calling the Conn.NextReader method and
// reading the returned io.Reader to EOF. An application sends a message by
// calling the Conn.NextWriter method and writing the message to the returned
// io.WriteCloser. The application terminates the message by closing the
// io.WriteCloser.
//
// The following example shows how to use the connection NextReader and
// NextWriter method to echo messages:
//
//  for {
//      mt, r, err := conn.NextReader()
//      if err != nil {
//          return
//      }
//      w, err := conn.NextWriter(mt)
//      if err != nil {
//          return err
//      }
//      if _, err := io.Copy(w, r); err != nil {
//          return err
//      }
//      if err := w.Close(); err != nil {
//          return err
//      }
//  }
//
// The connection ReadMessage and WriteMessage methods are helpers for reading
// or writing an entire message in one method call. The following example shows
// how to echo messages using these connection helper methods:
//
//  for {
//      mt, p, err := conn.ReadMessage()
//      if err != nil {
//          return
//      }
//      if _, err := conn.WriteMessaage(mt, p); err != nil {
//          return err
//      }
//  }
//
// Concurrency
//
// A Conn supports a single concurrent caller to the write methods (NextWriter,
// SetWriteDeadline, WriteMessage) and a single concurrent caller to the read
// methods (NextReader, SetReadDeadline, ReadMessage). The Close and
// WriteControl methods can be called concurrently with all other methods.
//
// Data Messages
//
// The WebSocket protocol distinguishes between text and binary data messages.
// Text messages are interpreted as UTF-8 encoded text. The interpretation of
// binary messages is left to the application.
//
// This package uses the same types and methods to work with both types of data
// messages. It is the application's reponsiblity to ensure that text messages
// are valid UTF-8 encoded text.
//
// Control Messages
//
// The WebSocket protocol defines three types of control messages: close, ping
// and pong. Call the connection WriteControl, WriteMessage or NextWriter
// methods to send a control message to the peer.
//
// Connections handle received ping and pong messages by invoking a callback
// function set with SetPingHandler and SetPongHandler methods. These callback
// functions can be invoked from the ReadMessage method, the NextReader method
// or from a call to the data message reader returned from NextReader.
//
// Connections handle received close messages by returning an error from the
// ReadMessage method, the NextReader method or from a call to the data message
// reader returned from NextReader.
package websocket
