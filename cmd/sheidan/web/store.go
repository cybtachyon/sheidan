package main

// Note is the client-side copy of a note, as the reactive components
// render it.
type Note struct {
	ID    int
	Title string
	Body  string
}

// NoteStore is a reactive store of notes. A mutation updates the notes
// and calls every subscriber, so the subscribed components re-render,
// batched by the scheduler.
type NoteStore struct {
	notes []Note
	subs  []func()
}

// NewNoteStore creates an empty note store.
func NewNoteStore() *NoteStore {
	return &NoteStore{}
}

// Notes returns the store's notes.
func (s *NoteStore) Notes() []Note {
	return s.notes
}

// SetAll replaces the store's notes and notifies the subscribers.
func (s *NoteStore) SetAll(notes []Note) {
	s.notes = notes
	s.notify()
}

// Add appends note and notifies the subscribers.
func (s *NoteStore) Add(note Note) {
	s.notes = append(s.notes, note)
	s.notify()
}

// Upsert inserts note, or replaces the note with the same ID, and
// notifies the subscribers.
func (s *NoteStore) Upsert(note Note) {
	for i := range s.notes {
		if s.notes[i].ID == note.ID {
			s.notes[i] = note
			s.notify()
			return
		}
	}
	s.notes = append(s.notes, note)
	s.notify()
}

// Update replaces the note with the given ID and notifies the
// subscribers.
func (s *NoteStore) Update(note Note) {
	for i := range s.notes {
		if s.notes[i].ID == note.ID {
			s.notes[i] = note
			break
		}
	}
	s.notify()
}

// Remove deletes the note with the given ID and notifies the
// subscribers.
func (s *NoteStore) Remove(id int) {
	kept := make([]Note, 0, len(s.notes))
	for _, n := range s.notes {
		if n.ID != id {
			kept = append(kept, n)
		}
	}
	s.notes = kept
	s.notify()
}

// Subscribe registers fn to be called on every mutation.
func (s *NoteStore) Subscribe(fn func()) {
	s.subs = append(s.subs, fn)
}

// notify calls every subscriber.
func (s *NoteStore) notify() {
	for _, fn := range s.subs {
		fn()
	}
}
