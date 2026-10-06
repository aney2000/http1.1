// Package notes is a tiny demo domain used to exercise the server end-to-end.
package notes

import (
	"errors"
	"sort"
	"sync"
)

// ErrNotFound is returned when a note does not exist.
var ErrNotFound = errors.New("note not found")

// Note is a single text note.
type Note struct {
	ID   int    `json:"id"`
	Text string `json:"text"`
}

// Store abstracts persistence so handlers do not depend on a concrete
// backend (Dependency Inversion); swapping in a database needs no handler change.
type Store interface {
	Create(text string) Note
	Get(id int) (Note, error)
	List() []Note
	Delete(id int) error
}

// MemoryStore is a concurrency-safe in-memory Store.
type MemoryStore struct {
	mu     sync.RWMutex
	nextID int
	notes  map[int]Note
}

// NewMemoryStore returns an empty store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{nextID: 1, notes: make(map[int]Note)}
}

// Create stores a new note and returns it with its assigned ID.
func (s *MemoryStore) Create(text string) Note {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := Note{ID: s.nextID, Text: text}
	s.notes[n.ID] = n
	s.nextID++
	return n
}

// Get returns the note with the given ID.
func (s *MemoryStore) Get(id int) (Note, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n, ok := s.notes[id]
	if !ok {
		return Note{}, ErrNotFound
	}
	return n, nil
}

// List returns all notes ordered by ID.
func (s *MemoryStore) List() []Note {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]Note, 0, len(s.notes))
	for _, n := range s.notes {
		list = append(list, n)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list
}

// Delete removes the note with the given ID.
func (s *MemoryStore) Delete(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.notes[id]; !ok {
		return ErrNotFound
	}
	delete(s.notes, id)
	return nil
}
