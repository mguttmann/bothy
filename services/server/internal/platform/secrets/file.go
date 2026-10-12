// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"
)

// maxFileSize is the largest secret file the driver reads (1 MiB). No secret is that large; the
// limit keeps a reference to the wrong file (a log, a dump) from filling memory.
const maxFileSize = 1 << 20

// Allowed modes of a secret file (SEC-SEC-1): readable by the owner only, writable at most by
// the owner.
const (
	modeOwnerRead      = 0o400
	modeOwnerReadWrite = 0o600
)

// fileDriver reads "file:" references (docs/spec/16-operations.md 5.2, SEC-SEC-1).
type fileDriver struct {
	// uid is the owner a secret file must have: the effective uid of the process, or another
	// uid in tests that check the owner rule without root.
	uid int
}

// NewFileDriver returns the driver for "file:" references.
//
// Get follows symlinks (Kubernetes mounts secrets as links), then requires a regular file with
// mode 0400 or 0600 (permission bits plus setuid, setgid and sticky) owned by the effective uid
// of the process; anything else is ErrInsecureFile with the observed mode or owner. Non-regular
// files are refused before they are opened, because opening a FIFO would block. The opened file
// is checked again and must be the file that was checked (os.SameFile), so a swap between check
// and read is noticed. A missing file is ErrNotFound; content above 1 MiB is ErrReadFailed.
// Exactly one trailing "\n", and a "\r" directly before it, is removed (`echo value > file` adds
// it, and no password ends with it); a file that is empty after that is ErrNotFound. Errors name
// the reference and never the content. On platforms without unix ownership every file is
// ErrInsecureFile: the owner rule cannot be enforced there, so nothing is read.
func NewFileDriver() Driver { return newFileDriver(os.Geteuid()) }

func newFileDriver(uid int) *fileDriver { return &fileDriver{uid: uid} }

// Scheme returns "file".
func (*fileDriver) Scheme() string { return schemeFile }

// Get reads the file ref names; see NewFileDriver for the rules.
func (d *fileDriver) Get(ctx context.Context, ref Ref) (Value, Version, error) {
	if err := checkDriverRef(ctx, schemeFile, ref); err != nil {
		return Value{}, "", err
	}
	name := ref.String()
	fail := func(sentinel error, detail string) (Value, Version, error) {
		return Value{}, "", &RefError{Ref: name, Err: sentinel, Detail: detail}
	}
	if platformProblem != "" {
		return fail(ErrInsecureFile, platformProblem)
	}

	checked, err := os.Stat(ref.Path)
	if err != nil {
		return Value{}, "", osError(name, err)
	}
	if detail := d.insecure(checked); detail != "" {
		return fail(ErrInsecureFile, detail)
	}
	f, err := os.OpenFile(ref.Path, openFlags, 0) // #nosec G304 -- the path is a checked absolute secret reference from the configuration; reading it is the purpose of this driver.
	if err != nil {
		return Value{}, "", osError(name, err)
	}
	defer func() { _ = f.Close() }()
	opened, err := f.Stat()
	if err != nil {
		return Value{}, "", osError(name, err)
	}
	if !os.SameFile(checked, opened) {
		return fail(ErrReadFailed, "file was replaced while it was read")
	}
	if detail := d.insecure(opened); detail != "" {
		return fail(ErrInsecureFile, detail)
	}

	buf, err := io.ReadAll(io.LimitReader(f, maxFileSize+1))
	// The Value keeps its own copy. Wiping the read buffer keeps one more copy of the secret out
	// of memory the garbage collector frees later; hygiene only, Go guarantees no wiping.
	defer clear(buf)
	if err != nil {
		return Value{}, "", osError(name, err)
	}
	if len(buf) > maxFileSize {
		return fail(ErrReadFailed, "file is larger than "+strconv.Itoa(maxFileSize)+" bytes")
	}
	content := stripNewline(buf)
	if len(content) == 0 {
		return fail(ErrNotFound, "file is empty")
	}
	return NewValue(content), versionOf(content), nil
}

// insecure returns why fi may not hold a secret, or "" when it may.
func (d *fileDriver) insecure(fi fs.FileInfo) string {
	if !fi.Mode().IsRegular() {
		return "not a regular file"
	}
	if m := unixMode(fi.Mode()); m != modeOwnerRead && m != modeOwnerReadWrite {
		return fmt.Sprintf("mode 0%03o, want 0400 or 0600", m)
	}
	return ownerProblem(fi, d.uid)
}

// unixMode returns the permission bits of m plus setuid, setgid and sticky in their unix octal
// positions, so the mode reads as chmod and ls show it (04600, not the Go bit layout).
func unixMode(m fs.FileMode) uint32 {
	mode := uint32(m.Perm())
	if m&fs.ModeSetuid != 0 {
		mode |= 0o4000
	}
	if m&fs.ModeSetgid != 0 {
		mode |= 0o2000
	}
	if m&fs.ModeSticky != 0 {
		mode |= 0o1000
	}
	return mode
}

// osError maps an error of stat, open or read for the reference name to ErrNotFound or
// ErrReadFailed, with the cause as detail but without the path (the reference already names it).
func osError(name string, err error) *RefError {
	if errors.Is(err, fs.ErrNotExist) {
		return &RefError{Ref: name, Err: ErrNotFound, Detail: "file does not exist"}
	}
	detail := "read error"
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		detail = pathErr.Err.Error()
	}
	return &RefError{Ref: name, Err: ErrReadFailed, Detail: detail}
}

// stripNewline removes exactly one trailing "\n" and a "\r" directly before it (D1).
func stripNewline(b []byte) []byte {
	trimmed, ok := bytes.CutSuffix(b, []byte("\n"))
	if !ok {
		return b
	}
	trimmed, _ = bytes.CutSuffix(trimmed, []byte("\r"))
	return trimmed
}
