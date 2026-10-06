package request

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// chunkedReader decodes the chunked transfer coding (RFC 9112 §7.1):
//
//	chunk-size [;ext] CRLF  chunk-data CRLF  ...  0 CRLF  [trailers] CRLF
//
// Chunk extensions and trailer fields are read and discarded.
type chunkedReader struct {
	r        *bufio.Reader
	maxLine  int
	pending  int64 // bytes left in the current chunk
	finished bool
	err      error
}

func newChunkedReader(r *bufio.Reader, maxLine int) *chunkedReader {
	return &chunkedReader{r: r, maxLine: maxLine}
}

func (c *chunkedReader) Read(p []byte) (int, error) {
	if c.err != nil {
		return 0, c.err
	}
	if c.finished {
		return 0, io.EOF
	}
	if c.pending == 0 {
		if c.err = c.nextChunk(); c.err != nil {
			return 0, c.err
		}
		if c.finished {
			return 0, io.EOF
		}
	}

	if int64(len(p)) > c.pending {
		p = p[:c.pending]
	}
	n, err := c.r.Read(p)
	c.pending -= int64(n)
	if err != nil {
		c.err = unexpectedEOF(err)
		return n, c.err
	}
	if c.pending == 0 {
		c.err = c.expectCRLF()
	}
	return n, c.err
}

func (c *chunkedReader) nextChunk() error {
	line, err := readLine(c.r, c.maxLine)
	if err != nil {
		return unexpectedEOF(err)
	}
	sizeText, _, _ := strings.Cut(line, ";")
	size, err := strconv.ParseInt(strings.TrimSpace(sizeText), 16, 64)
	if err != nil || size < 0 {
		return fmt.Errorf("%w: invalid size %q", ErrMalformedChunk, line)
	}
	if size == 0 {
		c.finished = true
		return c.skipTrailers()
	}
	c.pending = size
	return nil
}

func (c *chunkedReader) skipTrailers() error {
	for {
		line, err := readLine(c.r, c.maxLine)
		if err != nil {
			return unexpectedEOF(err)
		}
		if line == "" {
			return nil
		}
	}
}

func (c *chunkedReader) expectCRLF() error {
	line, err := readLine(c.r, c.maxLine)
	if err != nil {
		return unexpectedEOF(err)
	}
	if line != "" {
		return fmt.Errorf("%w: missing CRLF after chunk data", ErrMalformedChunk)
	}
	return nil
}
