package main

import (
	"strings"
	"testing"
)

// TestDecodeNotes verifies that decodeNotes decodes the note list the
// list endpoint returns, ignoring the server's extra fields.
func TestDecodeNotes(t *testing.T) {
	raw := `[{"ID":1,"Title":"a","Body":"b","CreatedAt":"2024-01-01T00:00:00Z"},{"ID":2,"Title":"c","Body":"d"}]`
	notes, err := decodeNotes(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("decodeNotes: %v", err)
	}
	if len(notes) != 2 {
		t.Fatalf("len(notes) = %d; want 2", len(notes))
	}
	if notes[0].ID != 1 || notes[0].Title != "a" || notes[0].Body != "b" {
		t.Errorf("notes[0] = %+v; want ID 1 Title a Body b", notes[0])
	}
	if notes[1].ID != 2 || notes[1].Title != "c" || notes[1].Body != "d" {
		t.Errorf("notes[1] = %+v; want ID 2 Title c Body d", notes[1])
	}
}

// TestDecodeNote verifies that decodeNote decodes the single note the
// create, update, and detail endpoints return.
func TestDecodeNote(t *testing.T) {
	note, err := decodeNote(strings.NewReader(`{"ID":7,"Title":"t","Body":"b"}`))
	if err != nil {
		t.Fatalf("decodeNote: %v", err)
	}
	if note.ID != 7 || note.Title != "t" || note.Body != "b" {
		t.Errorf("note = %+v; want ID 7 Title t Body b", note)
	}
}
