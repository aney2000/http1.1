package request

import "errors"

// Parse errors. The server maps each to an appropriate status code.
var (
	ErrMalformedRequestLine = errors.New("malformed request line")
	ErrUnsupportedVersion   = errors.New("unsupported HTTP version")
	ErrMalformedHeader      = errors.New("malformed header")
	ErrMissingHost          = errors.New("missing Host header")
	ErrLineTooLong          = errors.New("line too long")
	ErrTooManyHeaders       = errors.New("too many headers")

	ErrInvalidContentLength        = errors.New("invalid Content-Length")
	ErrUnsupportedTransferEncoding = errors.New("unsupported Transfer-Encoding")
	ErrAmbiguousFraming            = errors.New("both Transfer-Encoding and Content-Length present")
	ErrBodyTooLarge                = errors.New("body too large")
	ErrMalformedChunk              = errors.New("malformed chunk")
)
