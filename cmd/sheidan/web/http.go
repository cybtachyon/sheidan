package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gopherjs/gopherjs/js"
)

// pageOrigin returns the page's origin, the scheme and host part of the
// page's URL. The http client rejects relative URLs, so the client
// resolves every request path against it.
func pageOrigin() string {
	return js.Global.Get("location").Get("origin").String()
}

// logError reports an error to the browser console.
func logError(err error) {
	js.Global.Get("console").Call("error", err.Error())
}

// request sends an HTTP request to the demo app and returns the
// response. The path is resolved against the page's origin, and the
// request asks for JSON, so the demo's content-negotiated endpoints
// return their JSON representation.
func request(method, path string, body []byte) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, pageOrigin()+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return http.DefaultClient.Do(req)
}

// decodeNotes decodes the note list the list endpoint returns. The
// server's notes carry extra fields (timestamps), which the decoder
// ignores.
func decodeNotes(body io.Reader) ([]Note, error) {
	var notes []Note
	err := json.NewDecoder(body).Decode(&notes)
	return notes, err
}

// decodeNote decodes the single note the create, update, and detail
// endpoints return.
func decodeNote(body io.Reader) (Note, error) {
	var note Note
	err := json.NewDecoder(body).Decode(&note)
	return note, err
}
