package server

import (
	"errors"
	"io"
	"net"

	"github.com/uig27055/http1.1/internal/request"
	"github.com/uig27055/http1.1/internal/status"
)

var parseErrorStatus = []struct {
	err  error
	code status.Code
}{
	{request.ErrUnsupportedVersion, status.VersionNotSupported},
	{request.ErrUnsupportedTransferEncoding, status.NotImplemented},
	{request.ErrBodyTooLarge, status.PayloadTooLarge},
	{request.ErrLineTooLong, status.HeaderFieldsTooLarge},
	{request.ErrTooManyHeaders, status.HeaderFieldsTooLarge},
}

// statusForParseError maps a parse failure to a response status. It returns
// false when the peer is gone and no response can be delivered.
func statusForParseError(err error) (status.Code, bool) {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, net.ErrClosed) {
		return 0, false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return status.RequestTimeout, true
	}
	for _, m := range parseErrorStatus {
		if errors.Is(err, m.err) {
			return m.code, true
		}
	}
	return status.BadRequest, true
}
