// SPDX-License-Identifier: Apache-2.0

// Package hostos is the port of the host service to the operating system (docs/DECISIONS.md
// D-M2-42-5): users and groups, file security, path resolution with identity, the command runner
// and the probes. Every rule of doctor, install and helper is written against the small interfaces
// of this package, so hostostest can test it on any build host; the files per platform implement
// them without cgo.
//
// The per-platform terms of docs/spec/24-terminal.md 2.2 point 6 ("trusted owner", "writable",
// "path resolution") live here as pure functions (Writable, Readable, TrustedOwner, the rights
// mappings and Resolve). An ACL entry that cannot be read, parsed or mapped to a user or group
// counts as granting every right to every session user (fail closed): one right too many only
// blocks, one too few opens a place.
package hostos

import (
	"context"
	"errors"
	"io"
	"os"
	"time"
)

// Account is a local user of the host.
type Account struct {
	// Name is the account name; ID is the uid (Linux, macOS) or the SID (Windows).
	Name, ID string
	// Home is the home folder the operating system records; Shell the login shell (Linux, macOS).
	Home, Shell string
	// Person tells that the account is a person account of the platform (HD9, D-M2-42-6).
	Person bool
}

// Group is a group of an account: name and gid or SID.
type Group struct {
	Name, ID string
}

// ErrUnknownUser is returned by Users.Lookup for a name the operating system does not know.
var ErrUnknownUser = errors.New("hostos: unknown user")

// Users reads the user database of the operating system.
type Users interface {
	// Lookup returns the account name or an error wrapping ErrUnknownUser.
	Lookup(name string) (Account, error)
	// Groups returns every group of the account name, the primary group included; on Linux and
	// macOS from "id -G" and "id -Gn", because the pure Go user database misses directory services.
	Groups(name string) ([]Group, error)
	// Accounts returns the local accounts with their homes and the Person flag of HD9.
	Accounts() ([]Account, error)
}

// Subject is an account as an evaluator of file rights sees it: its ID and the IDs of its groups
// (on Windows the SIDs of its token, including Everyone, Authenticated Users and Users).
type Subject struct {
	Name, ID string
	GroupIDs []string
}

// Runner starts programs by absolute path with an argument vector, never through a shell or PATH.
type Runner interface {
	// Run executes c and returns its output. An error means the program could not be started or
	// waited for; a non-zero exit code and a timeout are reported in Output.
	Run(ctx context.Context, c Cmd) (Output, error)
}

// Cmd is one program start.
type Cmd struct {
	// Path is the absolute path of the program; Args are its arguments without the program name.
	Path string
	Args []string
	// Env is the complete environment; nil means an empty one, never the environment of the caller.
	Env []string
	// Dir is the working directory; empty means the caller's.
	Dir string
	// Stdin is written to the standard input.
	Stdin []byte
	// Timeout bounds the run; zero means no bound beyond ctx.
	Timeout time.Duration
}

// Output is the result of a run.
type Output struct {
	Stdout, Stderr []byte
	ExitCode       int
	TimedOut       bool
}

// Probe asks the platform questions of HD2, HD3 and HD10 that are no file rights.
type Probe interface {
	// SudoWithoutPassword reports whether "sudo -n true" succeeds for the calling account (HD3).
	SudoWithoutPassword(ctx context.Context) (bool, error)
	// PtraceScope returns kernel.yama.ptrace_scope on Linux; other platforms return -1 and nil.
	PtraceScope() (int, error)
}

// Resolver resolves paths fully and tells their identity (D-M2-42-7).
type Resolver interface {
	// Resolve returns the resolution of path or an error wrapping ErrUnresolvable.
	Resolve(path string) (Resolved, error)
	// Identity returns the file system identity of path: "dev:ino" on Linux and macOS, the volume
	// serial number and the 128-bit file ID on Windows.
	Identity(path string) (string, error)
}

// Lister lists folders and reads the start of files with the rights of the calling process (HD8,
// HD9, HD10 and the entry scan of HD11).
type Lister interface {
	// ReadDir returns the names of the entries of path; an error means the folder cannot be listed.
	ReadDir(path string) ([]string, error)
	// Head returns at most n bytes from the start of the file path; an error means it cannot be read.
	Head(path string, n int) ([]byte, error)
}

// OSLister is the Lister of the operating system.
type OSLister struct{}

// ErrNotRegular is returned by Head for anything but a regular file (a FIFO would block).
var ErrNotRegular = errors.New("hostos: not a regular file")

// Head implements Lister; it reads only regular files.
func (OSLister) Head(path string, n int) ([]byte, error) {
	if fi, err := os.Stat(path); err != nil || !fi.Mode().IsRegular() {
		return nil, ErrNotRegular
	}
	f, err := os.Open(path) //nolint:gosec // reading is the check itself
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	b := make([]byte, n)
	k, err := io.ReadFull(f, b)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return b[:k], nil
}

// ReadDir implements Lister.
func (OSLister) ReadDir(path string) ([]string, error) {
	f, err := os.Open(path) //nolint:gosec // listing is the check itself
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return f.Readdirnames(-1)
}

// GOOS values of the three platforms of the host service.
const (
	Linux   = "linux"
	Darwin  = "darwin"
	Windows = "windows"
)
