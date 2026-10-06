package response

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/uig27055/http1.1/internal/status"
)

// Text writes a plain-text response.
func Text(w Writer, code status.Code, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(code)
	_, _ = w.Write([]byte(body))
}

// JSON writes v encoded as JSON. Encoding happens before any header is
// committed so a failure can still be reported as a 500.
func JSON(w Writer, code status.Code, v any) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		Error(w, status.InternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(buf.Bytes())
}

// Error writes a plain-text body of the form "404 Not Found".
func Error(w Writer, code status.Code) {
	Text(w, code, fmt.Sprintf("%d %s\n", int(code), code.Text()))
}
