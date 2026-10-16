// SPDX-License-Identifier: Apache-2.0

// Package contract is the orchestrator of G7 (docs/spec/06-api.md "Contract tests and gate"):
// TestG7_SchemathesisAllEnabledOperations (build tag contract_live, run by make contract-test)
// starts the harness server with the matrix world and a credential broker and runs Schemathesis
// from tools/contract against it. The broker is an HTTP server of the test, never part of the
// product; it hands fresh DPoP-bound sessions to the Python side.
package contract

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/test/harness"
)

// The broker kinds of POST /principals.
const (
	KindOwner = "owner"
	KindUser  = "user"
)

// maxBrokerBody bounds a broker request body.
const maxBrokerBody = 4096

// Credentials are the answer of POST /principals: a fresh session with its DPoP key.
type Credentials struct {
	PrincipalID    string          `json:"principal_id"`
	DeviceID       string          `json:"device_id"`
	SessionID      string          `json:"session_id"`
	AccessToken    string          `json:"access_token"`
	RefreshToken   string          `json:"refresh_token"`
	DPoPPrivateJWK json.RawMessage `json:"dpop_private_jwk"`
}

// Broker hands out credentials of the harness server s: POST /principals with {"kind":"owner"}
// gives a new device of the first owner, {"kind":"user"} a new device of the person userID;
// POST /step-up with {"session_id"} performs a fresh step-up for a session it handed out. It
// listens on 127.0.0.1 only and refuses requests from any other address.
type Broker struct {
	s      *harness.Server
	userID string

	mu       sync.Mutex
	sessions map[string]*harness.Client

	srv *http.Server
	url string
}

// StartBroker starts a broker for s on 127.0.0.1:0; Close stops it.
func StartBroker(s *harness.Server, userID string) (*Broker, error) {
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	b := &Broker{s: s, userID: userID, sessions: map[string]*harness.Client{}, url: "http://" + ln.Addr().String()}
	b.srv = &http.Server{Handler: b, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = b.srv.Serve(ln) }()
	return b, nil
}

// URL returns the origin of the broker.
func (b *Broker) URL() string { return b.url }

// Close stops the broker.
func (b *Broker) Close() error { return b.srv.Close() }

// ServeHTTP implements http.Handler.
func (b *Broker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if ip := net.ParseIP(host); err != nil || ip == nil || !ip.IsLoopback() {
		http.Error(w, "loopback only", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var in struct {
		Kind      string `json:"kind"`
		SessionID string `json:"session_id"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBrokerBody)).Decode(&in); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	switch r.URL.Path {
	case "/principals":
		c, err := b.session(r.Context(), in.Kind)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		access, refresh := c.Tokens()
		writeJSON(w, Credentials{PrincipalID: c.PrincipalID, DeviceID: c.DeviceID, SessionID: c.SessionID,
			AccessToken: access, RefreshToken: refresh, DPoPPrivateJWK: c.Key()})
	case "/step-up":
		b.mu.Lock()
		c := b.sessions[in.SessionID]
		b.mu.Unlock()
		if c == nil {
			http.Error(w, "unknown session", http.StatusNotFound)
			return
		}
		if err := c.StepUp(r.Context()); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, map[string]string{"session_id": in.SessionID})
	default:
		http.NotFound(w, r)
	}
}

// session creates a fresh session of kind and remembers it for /step-up.
func (b *Broker) session(ctx context.Context, kind string) (*harness.Client, error) {
	var (
		c   *harness.Client
		err error
	)
	switch kind {
	case KindOwner:
		c, err = b.s.OwnerSession(ctx)
	case KindUser:
		c, err = b.s.UserSession(ctx, b.userID)
	default:
		return nil, errors.New("kind must be owner or user")
	}
	if err != nil {
		return nil, err
	}
	b.mu.Lock()
	b.sessions[c.SessionID] = c
	b.mu.Unlock()
	return c, nil
}

// writeJSON writes v as a JSON 200 answer.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
