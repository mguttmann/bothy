// SPDX-License-Identifier: Apache-2.0

package fs

// SetExpectedOwner replaces the user ID Open expects as owner of the root and returns a function
// that restores it. A test cannot create a directory owned by another user, so it changes the
// expectation instead. Tests that call it must not run in parallel.
func SetExpectedOwner(uid int) (restore func()) {
	old := expectedOwner
	expectedOwner = func() int { return uid }
	return func() { expectedOwner = old }
}

// SetHook installs fn as the seam that sees the steps of every write ("write", "sync-file",
// "before-rename", "rename", "sync-dir") with the key or temporary name involved. Call it before
// the store is used.
func (s *Store) SetHook(fn func(event, name string)) { s.hook = fn }

// ErrSymlink is the cause of the storage failure for a symbolic link in an object path.
var ErrSymlink = errSymlink
