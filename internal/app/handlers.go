package app

import (
	"io"

	"github.com/uig27055/http1.1/internal/request"
	"github.com/uig27055/http1.1/internal/response"
	"github.com/uig27055/http1.1/internal/status"
)

const indexText = `http1.1 - an HTTP/1.1 server written from scratch in Go

Routes:
  GET    /              this page
  GET    /health        liveness probe
  GET    /hello/{name}  greeting
  POST   /echo          echoes the request body
  GET    /notes         list notes
  POST   /notes         create a note   {"text": "..."}
  GET    /notes/{id}    fetch a note
  DELETE /notes/{id}    delete a note
`

func index(w response.Writer, _ *request.Request) {
	response.Text(w, status.OK, indexText)
}

func health(w response.Writer, _ *request.Request) {
	response.JSON(w, status.OK, map[string]string{"status": "ok"})
}

func hello(w response.Writer, r *request.Request) {
	response.Text(w, status.OK, "Hello, "+r.Param("name")+"!")
}

func echo(w response.Writer, r *request.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		response.Error(w, status.BadRequest)
		return
	}
	contentType := r.Headers.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	_, _ = w.Write(body)
}
