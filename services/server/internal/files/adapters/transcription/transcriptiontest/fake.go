// SPDX-License-Identifier: Apache-2.0

// Package transcriptiontest is the fake OpenAI-compatible transcription service of the tests of
// M3-09: an httptest server on 127.0.0.1 port 0 that records every request (method, path, form
// fields, the file part, the Authorization header) and answers with a programmable sequence of
// steps. Unit tests of adapters/transcription and the integration tests of the voice messages
// point voice.transcription_url at URL and add Rule to egress.allowed_hosts.
//
// Example:
//
//	fake := transcriptiontest.New(t)
//	fake.Respond(http.StatusServiceUnavailable, `{"error":"busy"}`)
//	fake.Respond(http.StatusOK, `{"text":"Hello Sam","language":"en"}`)
//	// voice.transcription_url = fake.URL(), egress.allowed_hosts = fake.Rule()
//	// the first call gets 503, every later call 200
package transcriptiontest

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// Path is the request path of URL, the path of the OpenAI audio transcription endpoint.
const Path = "/v1/audio/transcriptions"

// DefaultText is the transcript of the default answer, served while no step is programmed.
const DefaultText = "Hello Robin, this is the fake transcript."

// maxRecordedBytes bounds what the fake reads of one request body, so a broken client cannot
// fill the memory of a test.
const maxRecordedBytes = 64 << 20

// Request is one recorded request.
type Request struct {
	// Method and Path are the request line; RawQuery is the query, "" when absent.
	Method, Path, RawQuery string
	// ContentType is the Content-Type header of the request.
	ContentType string
	// Authorization is the Authorization header, "" when absent.
	Authorization string
	// Fields are the multipart form fields other than the file part, by name.
	Fields map[string]string
	// FieldOrder are the names of all parts in the order they arrived, the file part included.
	FieldOrder []string
	// FileName, FileType and FileBytes describe the part "file": its file name, its
	// Content-Type and its content. FileName is "" when the request had no file part.
	FileName, FileType string
	FileBytes          []byte
	// Body is the raw request body, for checks that a value appears nowhere in the request.
	Body []byte
	// ParseError is the error of reading the multipart body, "" when it parsed.
	ParseError string
}

// Server is the fake service. Its methods are safe for concurrent use.
type Server struct {
	srv     *httptest.Server
	release chan struct{}

	mu       sync.Mutex
	steps    []http.HandlerFunc
	served   int
	requests []Request
}

// New starts the fake on 127.0.0.1 port 0 and registers its shutdown with t.Cleanup: hanging
// requests (Hang) are released first, then the server closes.
func New(t testing.TB) *Server {
	t.Helper()
	s := &Server{release: make(chan struct{})}
	s.srv = httptest.NewServer(http.HandlerFunc(s.serve))
	// Cleanups run last in, first out: release the hanging handlers before Close waits for them.
	t.Cleanup(s.srv.Close)
	t.Cleanup(func() { close(s.release) })
	return s
}

// URL returns the endpoint of the fake: http://127.0.0.1:<port>/v1/audio/transcriptions.
func (s *Server) URL() string { return s.srv.URL + Path }

// Rule returns the entry of egress.allowed_hosts that allows the fake:
// "127.0.0.1:<port>;allow_private".
func (s *Server) Rule() string {
	return strings.TrimPrefix(s.srv.URL, "http://") + ";allow_private"
}

// Requests returns a copy of the recorded requests in arrival order.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Request(nil), s.requests...)
}

// Respond appends a step that answers with status and body (Content-Type application/json).
// Steps serve requests in order; the last step keeps serving once the sequence is used up.
func (s *Server) Respond(status int, body string) {
	s.RespondFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	})
}

// RespondFunc appends a step that answers with h, for example a redirect or an oversized body.
// The request body is already read and recorded when h runs.
func (s *Server) RespondFunc(h http.HandlerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.steps = append(s.steps, h)
}

// Hang appends a step that records the request and then answers nothing until the client gives
// up (its context ends) or the test ends (cleanup releases it).
func (s *Server) Hang() {
	s.RespondFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-s.release:
		}
	})
}

// serve records the request and runs the next step.
func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	rec := record(r)
	s.mu.Lock()
	s.requests = append(s.requests, rec)
	step := http.HandlerFunc(defaultAnswer)
	if n := len(s.steps); n > 0 {
		step = s.steps[min(s.served, n-1)]
	}
	s.served++
	s.mu.Unlock()
	step(w, r)
}

// defaultAnswer is the answer while no step is programmed: 200 with DefaultText.
func defaultAnswer(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w, `{"text":%q,"language":"en"}`, DefaultText)
}

// record reads the whole body and parses it as multipart/form-data when it is one.
func record(r *http.Request) Request {
	rec := Request{Method: r.Method, Path: r.URL.Path, RawQuery: r.URL.RawQuery,
		ContentType: r.Header.Get("Content-Type"), Authorization: r.Header.Get("Authorization"), Fields: map[string]string{}}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxRecordedBytes))
	rec.Body = body
	if err != nil {
		rec.ParseError = err.Error()
		return rec
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	mr, err := r.MultipartReader()
	if err != nil {
		rec.ParseError = err.Error()
		return rec
	}
	for {
		p, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			return rec
		}
		if err != nil {
			rec.ParseError = err.Error()
			return rec
		}
		b, err := io.ReadAll(p)
		if err != nil {
			rec.ParseError = err.Error()
			return rec
		}
		rec.FieldOrder = append(rec.FieldOrder, p.FormName())
		if p.FormName() == "file" {
			rec.FileName, rec.FileType, rec.FileBytes = p.FileName(), p.Header.Get("Content-Type"), b
			continue
		}
		rec.Fields[p.FormName()] = string(b)
	}
}
