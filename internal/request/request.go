// Package request parses HTTP/1.1 request messages (RFC 9112).
package request

import (
	"io"
	"net/url"

	"github.com/uig27055/http1.1/internal/headers"
)

// Request is a parsed HTTP request.
type Request struct {
	Method   string
	Target   string // raw request-target as received
	Path     string // Target without the query component
	RawQuery string
	Version  string
	Headers  *headers.Headers
	Body     io.Reader

	// Params holds path parameters filled in by the router (e.g. {id}).
	Params map[string]string
}

// Query returns the decoded query parameters. Malformed pairs are skipped.
func (r *Request) Query() url.Values {
	values, _ := url.ParseQuery(r.RawQuery)
	return values
}

// Param returns the path parameter with the given name, or "".
func (r *Request) Param(name string) string {
	return r.Params[name]
}

// Is10 reports whether the request uses HTTP/1.0.
func (r *Request) Is10() bool {
	return r.Version == "HTTP/1.0"
}
