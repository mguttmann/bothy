// SPDX-License-Identifier: Apache-2.0

package app

// GrantNotes returns how many working directory indexes the grant notes of s keep, for the tests.
func (s *Service) GrantNotes() int {
	s.notes.mu.Lock()
	defer s.notes.mu.Unlock()
	return len(s.notes.wdi)
}
