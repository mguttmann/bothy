// SPDX-License-Identifier: Apache-2.0

package control

import (
	"encoding/json"
	gosync "sync"
)

// KeyStore holds the JWKS of purpose terminal the server sent in hello and in heartbeat_ack (24
// section 5.3); M2-45 checks the session tokens with it. It is safe for concurrent use.
type KeyStore struct {
	mu   gosync.Mutex
	keys JWKS
}

// Set replaces the key set.
func (k *KeyStore) Set(set JWKS) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.keys = set
}

// Keys returns a copy of the key set: the JSON objects of the keys.
func (k *KeyStore) Keys() []map[string]any {
	k.mu.Lock()
	defer k.mu.Unlock()
	b, _ := json.Marshal(k.keys.Keys) // JSON values the server sent: never fails
	var out []map[string]any
	_ = json.Unmarshal(b, &out)
	return out
}
