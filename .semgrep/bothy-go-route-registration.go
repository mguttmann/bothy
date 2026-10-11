// SPDX-License-Identifier: Apache-2.0

// Test cases for the rule bothy-go-route-registration (run by `make semgrep`).
package fixtures

import "net/http"

type router struct{}

func (router) Route(action, pattern string, h http.HandlerFunc) {}

func handler(w http.ResponseWriter, r *http.Request) {}

func register(mux *http.ServeMux, r router) {
	// ruleid: bothy-go-route-registration
	mux.HandleFunc("GET /api/v1/threads/{id}", handler)
	// ruleid: bothy-go-route-registration
	mux.Handle("/api/v1/health", http.HandlerFunc(handler))
	// ruleid: bothy-go-route-registration
	http.HandleFunc("/debug", handler)
	// ok: bothy-go-route-registration
	r.Route("thread.read", "GET /api/v1/threads/{id}", handler)
	// ok: bothy-go-route-registration
	mux.Handle(prefix(), http.HandlerFunc(handler))
}

func prefix() string { return "" }
