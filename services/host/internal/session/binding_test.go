// SPDX-License-Identifier: Apache-2.0

package session

import (
	"errors"
	"testing"
)

// bindingStoreStub is a BindingStore whose read returns m and readErr; written keeps the last write.
type bindingStoreStub struct {
	m       map[string]string
	readErr error
	written map[string]string
	writes  int
}

func (s *bindingStoreStub) ReadBindings() (map[string]string, error) { return s.m, s.readErr }

func (s *bindingStoreStub) WriteBindings(m map[string]string) error {
	s.writes++
	s.written = m
	return nil
}

// TestBindRefusesUnreadableStore (24 2.2 point 3): a bindings.json that cannot be read refuses
// every binding and writes nothing (fail closed), so an unreadable file never rebinds a user.
func TestBindRefusesUnreadableStore(t *testing.T) {
	st := &bindingStoreStub{readErr: errors.New("unreadable")}
	if NewBindings(st).Bind(alexOSU, alexSub) {
		t.Error("bound over an unreadable store")
	}
	if st.writes != 0 {
		t.Errorf("%d writes", st.writes)
	}
}

// TestBindFirstOnEmptyStore: a store without bindings.json yet (a nil map) takes the first binding.
func TestBindFirstOnEmptyStore(t *testing.T) {
	st := &bindingStoreStub{}
	if !NewBindings(st).Bind(alexOSU, alexSub) {
		t.Fatal("first binding refused")
	}
	if st.writes != 1 || len(st.written) != 1 || st.written[alexOSU] != alexSub {
		t.Errorf("written %v after %d writes", st.written, st.writes)
	}
}
