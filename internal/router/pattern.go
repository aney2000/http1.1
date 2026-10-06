package router

import (
	"fmt"
	"strings"
)

// segment is one "/"-separated part of a route pattern.
type segment struct {
	value   string
	isParam bool
}

// pattern is a compiled route such as "/users/{id}".
type pattern struct {
	raw      string
	segments []segment
}

func compilePattern(raw string) (pattern, error) {
	if !strings.HasPrefix(raw, "/") {
		return pattern{}, fmt.Errorf("router: pattern %q must start with '/'", raw)
	}
	parts := strings.Split(raw[1:], "/")
	segments := make([]segment, len(parts))
	for i, part := range parts {
		seg, err := compileSegment(part)
		if err != nil {
			return pattern{}, fmt.Errorf("router: pattern %q: %w", raw, err)
		}
		segments[i] = seg
	}
	return pattern{raw: raw, segments: segments}, nil
}

func compileSegment(part string) (segment, error) {
	opens, closes := strings.HasPrefix(part, "{"), strings.HasSuffix(part, "}")
	switch {
	case opens && closes && len(part) > 2:
		return segment{value: part[1 : len(part)-1], isParam: true}, nil
	case opens || closes:
		return segment{}, fmt.Errorf("invalid parameter segment %q", part)
	default:
		return segment{value: part}, nil
	}
}

// match reports whether path fits the pattern and returns its parameters.
func (p pattern) match(path string) (map[string]string, bool) {
	if !strings.HasPrefix(path, "/") {
		return nil, false
	}
	parts := strings.Split(path[1:], "/")
	if len(parts) != len(p.segments) {
		return nil, false
	}

	var params map[string]string
	for i, seg := range p.segments {
		switch {
		case seg.isParam && parts[i] != "":
			if params == nil {
				params = make(map[string]string)
			}
			params[seg.value] = parts[i]
		case seg.isParam || seg.value != parts[i]:
			return nil, false
		}
	}
	return params, true
}

// staticCount ranks specificity: more literal segments win over parameters.
func (p pattern) staticCount() int {
	n := 0
	for _, seg := range p.segments {
		if !seg.isParam {
			n++
		}
	}
	return n
}
