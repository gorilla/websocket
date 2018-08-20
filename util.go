// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package websocket

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"
)

var keyGUID = []byte("258EAFA5-E914-47DA-95CA-C5AB0DC85B11")

func computeAcceptKey(challengeKey string) string {
	h := sha1.New()
	h.Write([]byte(challengeKey))
	h.Write(keyGUID)
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func generateChallengeKey() (string, error) {
	p := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, p); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(p), nil
}

// Octet types from RFC 2616.
//
// OCTET      = <any 8-bit sequence of data>
// CHAR       = <any US-ASCII character (octets 0 - 127)>
// CTL        = <any US-ASCII control character (octets 0 - 31) and DEL (127)>
// CR         = <US-ASCII CR, carriage return (13)>
// LF         = <US-ASCII LF, linefeed (10)>
// SP         = <US-ASCII SP, space (32)>
// HT         = <US-ASCII HT, horizontal-tab (9)>
// <">        = <US-ASCII double-quote mark (34)>
// CRLF       = CR LF
// LWS        = [CRLF] 1*( SP | HT )
// TEXT       = <any OCTET except CTLs, but including LWS>
// separators = "(" | ")" | "<" | ">" | "@" | "," | ";" | ":" | "\" | <">
//              | "/" | "[" | "]" | "?" | "=" | "{" | "}" | SP | HT
// token      = 1*<any CHAR except CTLs or separators>
// qdtext     = <any TEXT except <">>

func skipSpace(s string) string {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ', '\t', '\r', '\n':
		default:
			return s[i:]
		}
	}
	return ""
}

func nextToken(s string) (token, rest string) {
	i := 0
loop:
	for ; i < len(s); i++ {
		c := s[i]
		if c <= 31 || c >= 127 { // control characters & non-ascii are not token octets
			break
		}
		switch c { //separators are not token octets
		case ' ', '\t', '"', '(', ')', ',', '/', ':', ';', '<',
			'=', '>', '?', '@', '[', ']', '\\', '{', '}':
			break loop
		}
	}
	return s[:i], s[i:]
}

// nextTokenOrQuoted gets the next token, unescaping and unquoting quoted tokens
func nextTokenOrQuoted(s string) (value string, rest string) {
	// if it isnt quoted, then regular tokenization rules apply
	if !strings.HasPrefix(s, "\"") {
		return nextToken(s)
	}

	// trim off opening quote
	s = s[1:]

	// find closing quote while counting escapes
	escapes := 0     // count escapes
	escaped := false // whether the next char is escaped
	i := 0
scan:
	for ; i < len(s); i++ {
		// skip escaped characters
		if escaped {
			escaped = false
			continue
		}

		switch s[i] {
		case '"':
			// closing quote
			break scan
		case '\\':
			// escape sequence
			escaped = true
			escapes++
		}
	}

	// handle unterminated quoted token
	if i == len(s) {
		return "", ""
	}

	// split out token
	value, rest = s[:i], s[i+1:]

	// handle token without escapes
	if escapes == 0 {
		return value, rest
	}

	// unescape token
	buf := make([]byte, len(value)-escapes)
	j := 0
	escaped = false
	for i := 0; i < len(value); i++ {
		c := value[i]

		// handle escape sequence
		if c == '\\' && !escaped {
			escaped = true
			continue
		}
		escaped = false

		// copy character
		buf[j] = c
		j++
	}
	return string(buf), rest
}

// equalASCIIFold returns true if s is equal to t with ASCII case folding.
func equalASCIIFold(s, t string) bool {
	for s != "" && t != "" {
		// get first rune from both strings
		var sr, tr rune
		if s[0] < utf8.RuneSelf {
			sr, s = rune(s[0]), s[1:]
		} else {
			r, size := utf8.DecodeRuneInString(s)
			sr, s = r, s[size:]
		}
		if t[0] < utf8.RuneSelf {
			tr, t = rune(t[0]), t[1:]
		} else {
			r, size := utf8.DecodeRuneInString(t)
			tr, t = r, t[size:]
		}

		// compare runes
		switch {
		case sr == tr:
		case 'A' <= sr && sr <= 'Z':
			if sr+'a'-'A' != tr {
				return false
			}
		case 'A' <= tr && tr <= 'Z':
			if tr+'a'-'A' != sr {
				return false
			}
		default:
			return false
		}
	}

	return s == t
}

// tokenListContainsValue returns true if the 1#token header with the given
// name contains a token equal to value with ASCII case folding.
func tokenListContainsValue(header http.Header, name string, value string) bool {
headers:
	for _, s := range header[name] {
		for {
			var t string
			t, s = nextToken(skipSpace(s))
			if t == "" {
				continue headers
			}
			s = skipSpace(s)
			if s != "" && s[0] != ',' {
				continue headers
			}
			if equalASCIIFold(t, value) {
				return true
			}
			if s == "" {
				continue headers
			}
			s = s[1:]
		}
	}
	return false
}

// parseExtensions parses WebSocket extensions from a header.
func parseExtensions(header http.Header) []map[string]string {
	// From RFC 6455:
	//
	//  Sec-WebSocket-Extensions = extension-list
	//  extension-list = 1#extension
	//  extension = extension-token *( ";" extension-param )
	//  extension-token = registered-token
	//  registered-token = token
	//  extension-param = token [ "=" (token | quoted-string) ]
	//     ;When using the quoted-string syntax variant, the value
	//     ;after quoted-string unescaping MUST conform to the
	//     ;'token' ABNF.

	var result []map[string]string
headers:
	for _, s := range header["Sec-Websocket-Extensions"] {
		for {
			var t string
			t, s = nextToken(skipSpace(s))
			if t == "" {
				continue headers
			}
			ext := map[string]string{"": t}
			for {
				s = skipSpace(s)
				if !strings.HasPrefix(s, ";") {
					break
				}
				var k string
				k, s = nextToken(skipSpace(s[1:]))
				if k == "" {
					continue headers
				}
				s = skipSpace(s)
				var v string
				if strings.HasPrefix(s, "=") {
					v, s = nextTokenOrQuoted(skipSpace(s[1:]))
					s = skipSpace(s)
				}
				if s != "" && s[0] != ',' && s[0] != ';' {
					continue headers
				}
				ext[k] = v
			}
			if s != "" && s[0] != ',' {
				continue headers
			}
			result = append(result, ext)
			if s == "" {
				continue headers
			}
			s = s[1:]
		}
	}
	return result
}
