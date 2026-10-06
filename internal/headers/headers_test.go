package headers

import (
	"bytes"
	"errors"
	"testing"
)

func TestHeaders_SetAndGetAreCaseInsensitive(t *testing.T) {
	h := New()
	h.Set("content-TYPE", "text/plain")

	if got := h.Get("Content-Type"); got != "text/plain" {
		t.Fatalf("Get() = %q, want %q", got, "text/plain")
	}
}

func TestHeaders_GetMissingReturnsEmpty(t *testing.T) {
	if got := New().Get("X-Missing"); got != "" {
		t.Fatalf("Get() = %q, want empty", got)
	}
}

func TestHeaders_AddAppendsValues(t *testing.T) {
	h := New()
	h.Add("Accept", "text/html")
	h.Add("accept", "application/json")

	got := h.Values("ACCEPT")
	if len(got) != 2 || got[0] != "text/html" || got[1] != "application/json" {
		t.Fatalf("Values() = %v", got)
	}
	if h.Get("Accept") != "text/html" {
		t.Fatalf("Get() should return first value, got %q", h.Get("Accept"))
	}
}

func TestHeaders_SetReplacesValues(t *testing.T) {
	h := New()
	h.Add("X-A", "1")
	h.Add("X-A", "2")
	h.Set("X-A", "3")

	if got := h.Values("X-A"); len(got) != 1 || got[0] != "3" {
		t.Fatalf("Values() = %v, want [3]", got)
	}
}

func TestHeaders_HasAndDel(t *testing.T) {
	h := New()
	h.Set("Host", "localhost")

	if !h.Has("host") {
		t.Fatal("Has() = false, want true")
	}
	h.Del("HOST")
	if h.Has("Host") {
		t.Fatal("Has() = true after Del, want false")
	}
}

func TestHeaders_LenCountsDistinctNames(t *testing.T) {
	h := New()
	h.Add("A", "1")
	h.Add("a", "2")
	h.Add("B", "3")

	if h.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", h.Len())
	}
}

func TestCanonicalName(t *testing.T) {
	tests := map[string]string{
		"content-type":     "Content-Type",
		"HOST":             "Host",
		"x-request-ID":     "X-Request-Id",
		"accept":           "Accept",
		"www-authenticate": "Www-Authenticate",
	}
	for in, want := range tests {
		if got := CanonicalName(in); got != want {
			t.Errorf("CanonicalName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseLine(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantName  string
		wantValue string
		wantErr   error
	}{
		{"simple", "Host: localhost", "Host", "localhost", nil},
		{"trims optional whitespace", "Host:   localhost:8080  \t", "Host", "localhost:8080", nil},
		{"no whitespace", "Accept:*/*", "Accept", "*/*", nil},
		{"empty value", "X-Empty:", "X-Empty", "", nil},
		{"value with colon", "Referer: http://a/b", "Referer", "http://a/b", nil},
		{"missing colon", "Host localhost", "", "", ErrMalformedField},
		{"space before colon", "Host : localhost", "", "", ErrMalformedField},
		{"empty name", ": value", "", "", ErrMalformedField},
		{"invalid name char", "Ho(st: x", "", "", ErrMalformedField},
		{"control char in value", "X-A: a\x00b", "", "", ErrMalformedField},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, value, err := ParseLine(tt.line)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if name != tt.wantName || value != tt.wantValue {
				t.Fatalf("got (%q, %q), want (%q, %q)", name, value, tt.wantName, tt.wantValue)
			}
		})
	}
}

func TestHeaders_WriteToIsSortedAndCRLFTerminated(t *testing.T) {
	h := New()
	h.Set("Content-Type", "text/plain")
	h.Add("Set-Cookie", "a=1")
	h.Add("Set-Cookie", "b=2")
	h.Set("Content-Length", "5")

	var buf bytes.Buffer
	n, err := h.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo() error = %v", err)
	}

	want := "Content-Length: 5\r\n" +
		"Content-Type: text/plain\r\n" +
		"Set-Cookie: a=1\r\n" +
		"Set-Cookie: b=2\r\n"
	if buf.String() != want {
		t.Fatalf("WriteTo() =\n%q\nwant\n%q", buf.String(), want)
	}
	if n != int64(len(want)) {
		t.Fatalf("WriteTo() n = %d, want %d", n, len(want))
	}
}
