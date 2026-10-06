// Package headers implements an HTTP/1.1 header collection (RFC 9110 §5).
//
// Field names are case-insensitive, so every name is stored in canonical
// form ("content-type" -> "Content-Type").
package headers

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

// ErrMalformedField is returned when a header line violates RFC 9112 §5.
var ErrMalformedField = errors.New("malformed header field")

// Headers is a case-insensitive multimap of HTTP header fields.
type Headers struct {
	fields map[string][]string
}

// New returns an empty header collection.
func New() *Headers {
	return &Headers{fields: make(map[string][]string)}
}

// Get returns the first value for name, or "" if absent.
func (h *Headers) Get(name string) string {
	values := h.fields[CanonicalName(name)]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// Values returns all values for name in insertion order.
func (h *Headers) Values(name string) []string {
	return h.fields[CanonicalName(name)]
}

// Set replaces any existing values for name.
func (h *Headers) Set(name, value string) {
	h.fields[CanonicalName(name)] = []string{value}
}

// Add appends a value for name, keeping existing ones.
func (h *Headers) Add(name, value string) {
	key := CanonicalName(name)
	h.fields[key] = append(h.fields[key], value)
}

// Has reports whether name is present.
func (h *Headers) Has(name string) bool {
	_, ok := h.fields[CanonicalName(name)]
	return ok
}

// Del removes all values for name.
func (h *Headers) Del(name string) {
	delete(h.fields, CanonicalName(name))
}

// Len returns the number of distinct field names.
func (h *Headers) Len() int {
	return len(h.fields)
}

// WriteTo serializes the headers in wire format. Names are sorted so the
// output is deterministic, which keeps responses reproducible and testable.
func (h *Headers) WriteTo(w io.Writer) (int64, error) {
	names := make([]string, 0, len(h.fields))
	for name := range h.fields {
		names = append(names, name)
	}
	sort.Strings(names)

	var total int64
	for _, name := range names {
		for _, value := range h.fields[name] {
			n, err := fmt.Fprintf(w, "%s: %s\r\n", name, value)
			total += int64(n)
			if err != nil {
				return total, err
			}
		}
	}
	return total, nil
}

// CanonicalName converts a field name to its canonical form.
func CanonicalName(name string) string {
	parts := strings.Split(strings.ToLower(name), "-")
	for i, part := range parts {
		if part != "" {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "-")
}

// ParseLine parses a single "Name: value" field line (without CRLF).
func ParseLine(line string) (name, value string, err error) {
	rawName, rawValue, found := strings.Cut(line, ":")
	if !found || !isToken(rawName) {
		return "", "", fmt.Errorf("%w: %q", ErrMalformedField, line)
	}

	value = strings.Trim(rawValue, " \t")
	if !isFieldValue(value) {
		return "", "", fmt.Errorf("%w: invalid value in %q", ErrMalformedField, line)
	}
	return CanonicalName(rawName), value, nil
}

// isToken validates a field name against the RFC 9110 "token" grammar.
// Whitespace before the colon is forbidden (RFC 9112 §5.1), so it fails here.
func isToken(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isTokenChar(s[i]) {
			return false
		}
	}
	return true
}

func isTokenChar(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	default:
		return strings.IndexByte("!#$%&'*+-.^_`|~", c) >= 0
	}
}

func isFieldValue(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < 0x20 && c != '\t') || c == 0x7f {
			return false
		}
	}
	return true
}
