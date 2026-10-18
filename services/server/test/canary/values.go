// SPDX-License-Identifier: Apache-2.0

// Package canary is the canary test of SEC-T10 and DP-T09 (PLAN M1-69, threat T-LG-01): a
// scenario drives every enabled operation and every client gateway op of the server core with
// run-unique canary values, captures every output of the server (logs, traces, metrics, audit
// rows, error responses and the events of observers without read permissions) and Scan proves
// that no value appears where the allow matrix of its class forbids it.
//
// values.go holds the canary registry, sinks.go the capture of the outputs, scan.go the matcher
// and the allow matrix, scenario.go the scenario. The package belongs to the architecture
// component testsupport (M1-05) and is never imported by production code.
package canary

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
)

// Class is the class of a canary value; it decides in which sinks the value may appear (the
// allow matrix of brief M1-69, see README.md).
type Class string

// The classes of the allow matrix.
const (
	// ClassContent is content and direct personal data: message text, names, titles, e-mail
	// addresses, display names. Allowed only in events to principals that may read its scope.
	ClassContent Class = "content"
	// ClassAdminReason is the reason of an administrative change; like content, and additionally
	// in audit_events.details as the reserved member reason (19 SEC-AUD-5).
	ClassAdminReason Class = "admin_reason"
	// ClassSecret is a token, code, proof, password or the content of a secret file; never in
	// any sink.
	ClassSecret Class = "secret"
	// ClassNetwork is a client IP address; never in any sink (the audit log holds only ip_hmac).
	ClassNetwork Class = "network"
	// ClassIdentity is a principal or device ID created in the run; never in logs, traces or
	// metrics, where people appear only as bothy.principal.pseudo_id.
	ClassIdentity Class = "identity"
)

// canaryPrefix starts every generated canary word; canaryHexLen is the number of random
// lowercase hex characters after it.
const (
	canaryPrefix = "cnry"
	canaryHexLen = 20
)

// Canary is one registered value.
type Canary struct {
	// Value is the text the scenario sent or the server issued.
	Value string
	// Class decides the allowed sinks.
	Class Class
	// Scope is the channel or space ID the value belongs to, or empty for an instance-wide
	// value; only the foreign sink reads it.
	Scope string
	// Label names the value in a finding (for example "message text Support/Eskalation").
	Label string
}

// Registry holds the canary values of one run. It is safe for concurrent use.
type Registry struct {
	mu    sync.Mutex
	items []Canary
	seen  map[string]bool
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry { return &Registry{seen: map[string]bool{}} }

// New registers and returns a fresh canary word: "cnry" plus 20 lowercase hex characters from
// crypto/rand, alphanumeric so JSON and URL escaping keep it intact.
//
// Example:
//
//	text := reg.New(canary.ClassContent, channelID, "message text")
func (r *Registry) New(class Class, scope, label string) string {
	b := make([]byte, canaryHexLen/2)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	v := canaryPrefix + hex.EncodeToString(b)
	r.Register(v, class, scope, label)
	return v
}

// Register adds a value the scenario did not generate: a token, code or ID the server issued, or
// the content of a secret file. An empty value or one already registered is ignored.
func (r *Registry) Register(value string, class Class, scope, label string) {
	if value == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.seen[value] {
		return
	}
	r.seen[value] = true
	r.items = append(r.items, Canary{Value: value, Class: class, Scope: scope, Label: label})
}

// Canaries returns a copy of the registered values in registration order.
func (r *Registry) Canaries() []Canary {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Canary(nil), r.items...)
}

// Count returns how many values of class are registered.
func (r *Registry) Count(class Class) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, c := range r.items {
		if c.Class == class {
			n++
		}
	}
	return n
}

// labelOf returns the label of value, or "" (test messages).
func (r *Registry) labelOf(value string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.items {
		if c.Value == value {
			return c.Label
		}
	}
	return ""
}
