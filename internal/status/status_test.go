package status

import "testing"

func TestCode_Text(t *testing.T) {
	tests := map[Code]string{
		OK:                  "OK",
		Created:             "Created",
		NoContent:           "No Content",
		BadRequest:          "Bad Request",
		NotFound:            "Not Found",
		MethodNotAllowed:    "Method Not Allowed",
		RequestTimeout:      "Request Timeout",
		PayloadTooLarge:     "Content Too Large",
		InternalServerError: "Internal Server Error",
		NotImplemented:      "Not Implemented",
		VersionNotSupported: "HTTP Version Not Supported",
	}
	for code, want := range tests {
		if got := code.Text(); got != want {
			t.Errorf("Code(%d).Text() = %q, want %q", code, got, want)
		}
	}
}

func TestCode_TextUnknownIsEmpty(t *testing.T) {
	if got := Code(799).Text(); got != "" {
		t.Fatalf("Text() = %q, want empty", got)
	}
}

func TestCode_StatusLine(t *testing.T) {
	tests := map[Code]string{
		OK:        "HTTP/1.1 200 OK\r\n",
		NotFound:  "HTTP/1.1 404 Not Found\r\n",
		Code(799): "HTTP/1.1 799 \r\n",
	}
	for code, want := range tests {
		if got := code.StatusLine(); got != want {
			t.Errorf("StatusLine() = %q, want %q", got, want)
		}
	}
}

func TestCode_AllowsBody(t *testing.T) {
	tests := map[Code]bool{
		Continue:    false,
		OK:          true,
		NoContent:   false,
		NotModified: false,
		NotFound:    true,
	}
	for code, want := range tests {
		if got := code.AllowsBody(); got != want {
			t.Errorf("Code(%d).AllowsBody() = %v, want %v", code, got, want)
		}
	}
}
