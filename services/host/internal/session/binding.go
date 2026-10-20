// SPDX-License-Identifier: Apache-2.0

package session

import (
	"sync"
)

// BindingStore reads and writes bindings.json (internal/state: an atomic write).
type BindingStore interface {
	ReadBindings() (map[string]string, error)
	WriteBindings(map[string]string) error
}

// Bindings binds each session user to the person (sub) the host service first gave it (24 2.2
// point 3, Ruling T12). The binding is written before any later step runs and stays when a later
// step fails, because the server never reassigns a session user. It is safe for concurrent use:
// two first opens of one session user with two people never both bind.
type Bindings struct {
	mu    sync.Mutex
	store BindingStore
}

// NewBindings returns the bindings over store.
func NewBindings(store BindingStore) *Bindings { return &Bindings{store: store} }

// Bind binds osu to sub when it is unbound and reports whether osu is bound to sub afterwards.
// A store that cannot be read or written refuses (fail closed).
func (b *Bindings) Bind(osu, sub string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	m, err := b.store.ReadBindings()
	if err != nil {
		return false
	}
	if cur, ok := m[osu]; ok {
		return cur == sub
	}
	if m == nil {
		m = map[string]string{}
	}
	m[osu] = sub
	return b.store.WriteBindings(m) == nil
}
