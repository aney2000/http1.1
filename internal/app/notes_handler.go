package app

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/uig27055/http1.1/internal/notes"
	"github.com/uig27055/http1.1/internal/request"
	"github.com/uig27055/http1.1/internal/response"
	"github.com/uig27055/http1.1/internal/status"
)

// notesHandler exposes a notes.Store as a small REST resource.
type notesHandler struct {
	store notes.Store
}

type createNoteRequest struct {
	Text string `json:"text"`
}

func (h *notesHandler) list(w response.Writer, _ *request.Request) {
	response.JSON(w, status.OK, h.store.List())
}

func (h *notesHandler) create(w response.Writer, r *request.Request) {
	var in createNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || strings.TrimSpace(in.Text) == "" {
		response.Error(w, status.BadRequest)
		return
	}
	note := h.store.Create(in.Text)
	w.Header().Set("Location", "/notes/"+strconv.Itoa(note.ID))
	response.JSON(w, status.Created, note)
}

func (h *notesHandler) get(w response.Writer, r *request.Request) {
	id, ok := noteID(w, r)
	if !ok {
		return
	}
	note, err := h.store.Get(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	response.JSON(w, status.OK, note)
}

func (h *notesHandler) delete(w response.Writer, r *request.Request) {
	id, ok := noteID(w, r)
	if !ok {
		return
	}
	if err := h.store.Delete(id); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(status.NoContent)
}

func noteID(w response.Writer, r *request.Request) (int, bool) {
	id, err := strconv.Atoi(r.Param("id"))
	if err != nil {
		response.Error(w, status.BadRequest)
		return 0, false
	}
	return id, true
}

func writeStoreError(w response.Writer, err error) {
	if errors.Is(err, notes.ErrNotFound) {
		response.Error(w, status.NotFound)
		return
	}
	response.Error(w, status.InternalServerError)
}
