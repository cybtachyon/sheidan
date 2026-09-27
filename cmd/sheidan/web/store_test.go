package main

import "testing"

// TestNoteStoreSetAll verifies that SetAll replaces the notes and
// notifies the subscribers once.
func TestNoteStoreSetAll(t *testing.T) {
	s := NewNoteStore()
	changes := 0
	s.Subscribe(func() { changes++ })

	s.SetAll([]Note{{ID: 1, Title: "a"}, {ID: 2, Title: "b"}})
	if changes != 1 {
		t.Fatalf("changes = %d; want 1", changes)
	}
	if got := len(s.Notes()); got != 2 {
		t.Fatalf("len(notes) = %d; want 2", got)
	}
}

// TestNoteStoreAdd verifies that Add appends a note and notifies.
func TestNoteStoreAdd(t *testing.T) {
	s := NewNoteStore()
	changes := 0
	s.Subscribe(func() { changes++ })

	s.Add(Note{ID: 1, Title: "a"})
	if changes != 1 {
		t.Fatalf("changes = %d; want 1", changes)
	}
	if got := s.Notes()[0]; got.ID != 1 || got.Title != "a" {
		t.Fatalf("note = %+v; want ID 1 Title %q", got, "a")
	}
}

// TestNoteStoreUpdate verifies that Update replaces the note with the
// matching ID, leaves the others, and notifies.
func TestNoteStoreUpdate(t *testing.T) {
	s := NewNoteStore()
	changes := 0
	s.Subscribe(func() { changes++ })
	s.SetAll([]Note{{ID: 1, Title: "a"}, {ID: 2, Title: "b"}})
	changes = 0

	s.Update(Note{ID: 2, Title: "B"})
	if changes != 1 {
		t.Fatalf("changes = %d; want 1", changes)
	}
	notes := s.Notes()
	if notes[0].Title != "a" || notes[1].Title != "B" {
		t.Fatalf("notes = %+v; want a and B", notes)
	}
}

// TestNoteStoreRemove verifies that Remove deletes the note with the
// matching ID and notifies.
func TestNoteStoreRemove(t *testing.T) {
	s := NewNoteStore()
	changes := 0
	s.Subscribe(func() { changes++ })
	s.SetAll([]Note{{ID: 1, Title: "a"}, {ID: 2, Title: "b"}, {ID: 3, Title: "c"}})
	changes = 0

	s.Remove(2)
	if changes != 1 {
		t.Fatalf("changes = %d; want 1", changes)
	}
	ids := make([]int, len(s.Notes()))
	for i, n := range s.Notes() {
		ids[i] = n.ID
	}
	if len(ids) != 2 || ids[0] != 1 || ids[1] != 3 {
		t.Fatalf("ids = %v; want [1 3]", ids)
	}
}

// TestNoteStoreUpsert verifies that Upsert inserts a new note, replaces
// an existing one in place, and notifies either way.
func TestNoteStoreUpsert(t *testing.T) {
	s := NewNoteStore()
	changes := 0
	s.Subscribe(func() { changes++ })

	s.Upsert(Note{ID: 1, Title: "a"})
	if changes != 1 {
		t.Fatalf("changes after insert = %d; want 1", changes)
	}
	if got := s.Notes()[0]; got.ID != 1 || got.Title != "a" {
		t.Fatalf("note = %+v; want ID 1 Title %q", got, "a")
	}

	s.Upsert(Note{ID: 1, Title: "A"})
	if changes != 2 {
		t.Fatalf("changes after replace = %d; want 2", changes)
	}
	if got := len(s.Notes()); got != 1 {
		t.Fatalf("len(notes) = %d; want 1", got)
	}
	if got := s.Notes()[0].Title; got != "A" {
		t.Fatalf("title = %q; want %q", got, "A")
	}
}

// TestNoteStoreMultipleSubscribers verifies that every subscriber is
// called on a mutation.
func TestNoteStoreMultipleSubscribers(t *testing.T) {
	s := NewNoteStore()
	a, b := 0, 0
	s.Subscribe(func() { a++ })
	s.Subscribe(func() { b++ })

	s.Add(Note{ID: 1})
	if a != 1 || b != 1 {
		t.Fatalf("subscribers = a:%d b:%d; want 1:1", a, b)
	}
}
