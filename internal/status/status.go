// Package status defines HTTP response status codes (RFC 9110 §15).
package status

import "fmt"

// Code is an HTTP status code.
type Code int

// Status codes supported by the server.
const (
	Continue             Code = 100
	OK                   Code = 200
	Created              Code = 201
	NoContent            Code = 204
	NotModified          Code = 304
	BadRequest           Code = 400
	NotFound             Code = 404
	MethodNotAllowed     Code = 405
	RequestTimeout       Code = 408
	PayloadTooLarge      Code = 413
	HeaderFieldsTooLarge Code = 431
	InternalServerError  Code = 500
	NotImplemented       Code = 501
	VersionNotSupported  Code = 505
)

var reasonPhrases = map[Code]string{
	Continue:             "Continue",
	OK:                   "OK",
	Created:              "Created",
	NoContent:            "No Content",
	NotModified:          "Not Modified",
	BadRequest:           "Bad Request",
	NotFound:             "Not Found",
	MethodNotAllowed:     "Method Not Allowed",
	RequestTimeout:       "Request Timeout",
	PayloadTooLarge:      "Content Too Large",
	HeaderFieldsTooLarge: "Request Header Fields Too Large",
	InternalServerError:  "Internal Server Error",
	NotImplemented:       "Not Implemented",
	VersionNotSupported:  "HTTP Version Not Supported",
}

// Text returns the reason phrase, or "" for unknown codes.
func (c Code) Text() string {
	return reasonPhrases[c]
}

// StatusLine returns the HTTP/1.1 status line including the trailing CRLF.
func (c Code) StatusLine() string {
	return fmt.Sprintf("HTTP/1.1 %d %s\r\n", int(c), c.Text())
}

// AllowsBody reports whether a response with this code may carry content.
// RFC 9110 forbids content for 1xx, 204 and 304.
func (c Code) AllowsBody() bool {
	return c >= 200 && c != NoContent && c != NotModified
}
