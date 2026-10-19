// SPDX-License-Identifier: Apache-2.0

// Package state is the state directory host_service.state_dir of the host service
// (docs/spec/24-terminal.md 4.2, docs/DECISIONS.md D-M2-42-4). It belongs to the service user,
// mode 0700 (on Windows a protected DACL), and holds:
//
//   - keys: both private keys (X25519 for Noise, Ed25519 for the login); on Windows the content is
//     a DPAPI blob of the service user's context;
//   - pairing.json: server URL, host_id, instance_id and fingerprint, no private member;
//   - bindings.json: session user name to person ID (written by M2-45 at the first session);
//   - declaration.json: the last llm_credentials learned from a ready message;
//   - admin.sock (Linux and macOS): the admin endpoint of the running service.
//
// The name claude_api_key is reserved for the API key of the organization declaration, which
// M2-45 reads. Every file is written atomically (temporary file in the directory, fsync, rename,
// fsync of the directory) with mode 0600.
package state

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// File names inside the state directory.
const (
	KeysFile        = "keys"
	PairingFile     = "pairing.json"
	BindingsFile    = "bindings.json"
	DeclarationFile = "declaration.json"
	// ServerCAFile is the CA of `pair --ca-file`, trusted by the control connection besides the
	// system roots (M2-43); absent without --ca-file.
	ServerCAFile = "server_ca.pem"
	AdminSocket  = "admin.sock"
	// ClaudeAPIKeyFile is reserved for the API key of the declaration organization (M2-45).
	ClaudeAPIKeyFile = "claude_api_key"
)

// Declarations of hosts.llm_credentials, plus Unknown for a host that never received one.
const (
	DeclNone         = "none"
	DeclPersonal     = "personal"
	DeclOrganization = "organization"
	DeclUnknown      = "unknown"
)

// Protector seals the content of the key file. On Linux and macOS it is the identity (the file mode
// protects the keys); on Windows it is DPAPI in the context of the service user.
type Protector interface {
	// Protect returns the sealed form of plain.
	Protect(plain []byte) ([]byte, error)
	// Unprotect returns the plain form of sealed, or an error if it was not sealed by Protect.
	Unprotect(sealed []byte) ([]byte, error)
}

// Plain is the Protector of Linux and macOS: the content is written as is.
type Plain struct{}

// Protect implements Protector.
func (Plain) Protect(b []byte) ([]byte, error) { return b, nil }

// Unprotect implements Protector.
func (Plain) Unprotect(b []byte) ([]byte, error) { return b, nil }

// Dir is an opened state directory.
type Dir struct {
	path    string
	protect Protector
	rename  func(oldpath, newpath string) error
}

// Open returns the state directory at path with the key protector p. It does not touch the
// file system; Ensure creates the directory.
func Open(path string, p Protector) *Dir {
	return &Dir{path: path, protect: p, rename: os.Rename}
}

// Path returns the directory path.
func (d *Dir) Path() string { return d.path }

// File returns the path of the file name inside the directory.
func (d *Dir) File(name string) string { return filepath.Join(d.path, name) }

// Ensure creates the directory with mode 0700 if it is missing.
func (d *Dir) Ensure() error {
	if err := os.MkdirAll(d.path, 0o700); err != nil {
		return fmt.Errorf("state directory: %w", err)
	}
	return nil
}

// ErrNotPaired is returned when the host has no pairing record or keys.
var ErrNotPaired = errors.New("state: not paired")

// WriteFile replaces the file name with b atomically, mode 0600: temporary file in the directory,
// fsync, rename, fsync of the directory. A failure before the rename leaves the old file.
func (d *Dir) WriteFile(name string, b []byte) error {
	f, err := os.CreateTemp(d.path, "."+name+"-*.tmp")
	if err != nil {
		return fmt.Errorf("state %s: %w", name, err)
	}
	tmp := f.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmp)
		}
	}()
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return fmt.Errorf("state %s: %w", name, err)
	}
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		return fmt.Errorf("state %s: %w", name, err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("state %s: %w", name, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("state %s: %w", name, err)
	}
	if err := d.rename(tmp, d.File(name)); err != nil {
		return fmt.Errorf("state %s: %w", name, err)
	}
	ok = true
	syncDir(d.path)
	return nil
}

// syncDir flushes the directory entry of a rename. Windows cannot open a directory for a sync;
// there the rename itself is the commit point, so an error is ignored.
func syncDir(path string) {
	if f, err := os.Open(path); err == nil { //nolint:gosec // the state directory
		_ = f.Sync()
		_ = f.Close()
	}
}

func (d *Dir) readJSON(name string, v any) (bool, error) {
	b, err := os.ReadFile(d.File(name))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("state %s: %w", name, err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		return false, fmt.Errorf("state %s: not valid JSON", name)
	}
	return true, nil
}

func (d *Dir) writeJSON(name string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("state %s: %w", name, err)
	}
	return d.WriteFile(name, b)
}

// Keys are the two private keys of the host (24 4.1).
type Keys struct {
	// NoisePrivate is the static X25519 private key of the Noise pattern NK.
	NoisePrivate [32]byte
	// AuthPrivate is the Ed25519 private key of the login.
	AuthPrivate ed25519.PrivateKey
}

type keysFile struct {
	Noise string `json:"noise_private"`
	Auth  string `json:"auth_seed"`
}

var b64 = base64.RawURLEncoding

// WriteKeys stores k sealed by the protector of d.
func (d *Dir) WriteKeys(k Keys) error {
	if len(k.AuthPrivate) != ed25519.PrivateKeySize {
		return errors.New("state keys: invalid login key")
	}
	plain, err := json.Marshal(keysFile{Noise: b64.EncodeToString(k.NoisePrivate[:]), Auth: b64.EncodeToString(k.AuthPrivate.Seed())})
	if err != nil {
		return fmt.Errorf("state keys: %w", err)
	}
	sealed, err := d.protect.Protect(plain)
	if err != nil {
		return fmt.Errorf("state keys: %w", err)
	}
	return d.WriteFile(KeysFile, sealed)
}

// ReadKeys returns the stored keys; ErrNotPaired when the file is missing. Errors never contain
// key material.
func (d *Dir) ReadKeys() (Keys, error) {
	sealed, err := os.ReadFile(d.File(KeysFile))
	if errors.Is(err, fs.ErrNotExist) {
		return Keys{}, ErrNotPaired
	}
	if err != nil {
		return Keys{}, fmt.Errorf("state keys: %w", err)
	}
	plain, err := d.protect.Unprotect(sealed)
	if err != nil {
		return Keys{}, fmt.Errorf("state keys: %w", err)
	}
	var f keysFile
	if json.Unmarshal(plain, &f) != nil {
		return Keys{}, errors.New("state keys: unreadable")
	}
	n, err1 := b64.DecodeString(f.Noise)
	s, err2 := b64.DecodeString(f.Auth)
	if err1 != nil || err2 != nil || len(n) != 32 || len(s) != ed25519.SeedSize {
		return Keys{}, errors.New("state keys: unreadable")
	}
	var k Keys
	copy(k.NoisePrivate[:], n)
	k.AuthPrivate = ed25519.NewKeyFromSeed(s)
	return k, nil
}

// Pairing is the pairing record: public data of the last accepted pairing only.
type Pairing struct {
	ServerURL   string `json:"server_url"`
	HostID      string `json:"host_id"`
	InstanceID  string `json:"instance_id"`
	Fingerprint string `json:"key_fingerprint"`
}

// ReadPairing returns the pairing record; ErrNotPaired when it is missing.
func (d *Dir) ReadPairing() (Pairing, error) {
	var p Pairing
	ok, err := d.readJSON(PairingFile, &p)
	if err != nil {
		return Pairing{}, err
	}
	if !ok {
		return Pairing{}, ErrNotPaired
	}
	return p, nil
}

// WritePairing stores p.
func (d *Dir) WritePairing(p Pairing) error { return d.writeJSON(PairingFile, p) }

// WriteServerCA stores the CA certificates of `pair --ca-file` (PEM, public data); nil or empty
// removes a stored one, so a pairing without --ca-file trusts the system roots only.
func (d *Dir) WriteServerCA(pem []byte) error {
	if len(pem) == 0 {
		if err := os.Remove(d.File(ServerCAFile)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("state %s: %w", ServerCAFile, err)
		}
		return nil
	}
	return d.WriteFile(ServerCAFile, pem)
}

// ReadServerCA returns the stored CA of the pairing, nil when there is none.
func (d *Dir) ReadServerCA() ([]byte, error) {
	b, err := os.ReadFile(d.File(ServerCAFile))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("state %s: %w", ServerCAFile, err)
	}
	return b, nil
}

// ReadBindings returns the bindings of session user names to person IDs (24 2.2 point 3); an
// empty map when the file is missing.
func (d *Dir) ReadBindings() (map[string]string, error) {
	m := map[string]string{}
	if _, err := d.readJSON(BindingsFile, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// WriteBindings stores the bindings (M2-45 and the platform suite write them).
func (d *Dir) WriteBindings(m map[string]string) error { return d.writeJSON(BindingsFile, m) }

type declarationFile struct {
	LLMCredentials string `json:"llm_credentials"`
}

// ReadDeclaration returns the stored llm_credentials, DeclUnknown when no ready was ever stored
// (D-M2-42-15). A stored value outside the three declarations is an error.
func (d *Dir) ReadDeclaration() (string, error) {
	var f declarationFile
	ok, err := d.readJSON(DeclarationFile, &f)
	if err != nil {
		return DeclUnknown, err
	}
	if !ok {
		return DeclUnknown, nil
	}
	if !ValidDeclaration(f.LLMCredentials) {
		return DeclUnknown, errors.New("state declaration: unknown value")
	}
	return f.LLMCredentials, nil
}

// WriteDeclaration stores the declaration of the latest ready.
func (d *Dir) WriteDeclaration(decl string) error {
	if !ValidDeclaration(decl) {
		return errors.New("state declaration: unknown value")
	}
	return d.writeJSON(DeclarationFile, declarationFile{LLMCredentials: decl})
}

// ValidDeclaration reports whether s is one of none, personal or organization.
func ValidDeclaration(s string) bool {
	return s == DeclNone || s == DeclPersonal || s == DeclOrganization
}
