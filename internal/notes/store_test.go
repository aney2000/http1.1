package notes

import (
	"errors"
	"sync"
	"testing"
)

func TestMemoryStore_CreateAssignsSequentialIDs(t *testing.T) {
	s := NewMemoryStore()

	a := s.Create("first")
	b := s.Create("second")

	if a.ID != 1 || b.ID != 2 {
		t.Fatalf("IDs = %d, %d; want 1, 2", a.ID, b.ID)
	}
	if a.Text != "first" {
		t.Fatalf("Text = %q", a.Text)
	}
}

func TestMemoryStore_Get(t *testing.T) {
	s := NewMemoryStore()
	created := s.Create("hello")

	got, err := s.Get(created.ID)
	if err != nil || got != created {
		t.Fatalf("Get() = %+v, %v; want %+v", got, err, created)
	}
}

func TestMemoryStore_GetMissing(t *testing.T) {
	if _, err := NewMemoryStore().Get(99); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestMemoryStore_ListIsOrderedByID(t *testing.T) {
	s := NewMemoryStore()
	s.Create("a")
	s.Create("b")
	s.Create("c")

	list := s.List()
	if len(list) != 3 || list[0].Text != "a" || list[2].Text != "c" {
		t.Fatalf("List() = %+v", list)
	}
}

func TestMemoryStore_ListEmptyIsNonNil(t *testing.T) {
	if list := NewMemoryStore().List(); list == nil {
		t.Fatal("List() = nil, want empty slice (encodes as [] not null)")
	}
}

func TestMemoryStore_Delete(t *testing.T) {
	s := NewMemoryStore()
	n := s.Create("x")

	if err := s.Delete(n.ID); err != nil {
		t.Fatalf("Delete() = %v", err)
	}
	if err := s.Delete(n.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second Delete() = %v, want ErrNotFound", err)
	}
}

func TestMemoryStore_IsSafeForConcurrentUse(t *testing.T) {
	s := NewMemoryStore()
	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Create("n")
			s.List()
		}()
	}
	wg.Wait()

	if len(s.List()) != 50 {
		t.Fatalf("len = %d, want 50", len(s.List()))
	}
}
