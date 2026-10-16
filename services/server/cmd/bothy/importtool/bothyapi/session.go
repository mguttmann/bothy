// SPDX-License-Identifier: Apache-2.0

package bothyapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// sessionVersion is the version of the session file format.
const sessionVersion = 1

// Session is the content of the session file (docs/DECISIONS.md D-M8-10-2): the server, the
// device, its DPoP key and the token pair. String, GoString and every fmt verb print only
// server_url and device_id, so a session in a log line or an error never shows a secret.
type Session struct {
	// Version is the format version, 1.
	Version int `json:"version"`
	// ServerURL is the origin of the server (requirement 15).
	ServerURL string `json:"server_url"`
	// DeviceID is the UUIDv7 of the cli device.
	DeviceID string `json:"device_id"`
	// UserID is the person the session belongs to.
	UserID string `json:"user_id"`
	// DPoPPrivateJWK is the Ed25519 DPoP key as a private JWK (RFC 8037).
	DPoPPrivateJWK json.RawMessage `json:"dpop_private_jwk"`
	// AccessToken is the current access token.
	AccessToken string `json:"access_token"`
	// AccessExpiresAt is when the access token expires: the client's clock at the answer plus
	// expires_in. The tool knows no server key and trusts only this and a 401.
	AccessExpiresAt time.Time `json:"access_expires_at"`
	// RefreshToken is the current refresh token.
	RefreshToken string `json:"refresh_token"`
	// RefreshExpiresAt is refresh_expires_at of the server.
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
}

// String returns only server_url and device_id.
func (s Session) String() string {
	return "bothyapi.Session{server_url: " + s.ServerURL + ", device_id: " + s.DeviceID + "}"
}

// GoString returns the same as String, so %#v shows no secret either.
func (s Session) GoString() string { return s.String() }

// Format prints String for every verb, so %+v and %x cannot reach the fields.
func (s Session) Format(f fmt.State, _ rune) { _, _ = io.WriteString(f, s.String()) }

// valid reports whether s has every member a client needs.
func (s *Session) valid() bool {
	return s.Version == sessionVersion && s.ServerURL != "" && s.DeviceID != "" && len(s.DPoPPrivateJWK) > 0 &&
		s.AccessToken != "" && s.RefreshToken != ""
}

// DefaultSessionPath returns <os.UserConfigDir()>/bothy/import-session.json. Errors: the
// configuration directory of the user is unknown.
func DefaultSessionPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("bothyapi: no configuration directory: %w", err)
	}
	return filepath.Join(dir, "bothy", "import-session.json"), nil
}

// SessionFile is the session file at Path, with the lock file Path + ".lock" next to it. The
// file has mode 0600 in a directory with mode 0700; a file or directory that group or others can
// read or write is refused (on Windows the file lives in the user profile and the mode check is
// skipped, docs/DECISIONS.md D-M8-10-2).
type SessionFile struct {
	// Path is the path of the session file.
	Path string
	// rename replaces the file; tests simulate a crash before the rename. nil means os.Rename.
	rename func(oldpath, newpath string) error
}

// checkModes reports ErrInsecureSessionFile when info is not of the wanted kind or group or
// others have any permission bit.
func checkModes(info fs.FileInfo, dir bool) error {
	if info.IsDir() != dir || (!dir && !info.Mode().IsRegular()) {
		return ErrInsecureSessionFile
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return ErrInsecureSessionFile
	}
	return nil
}

// checkDir checks the directory of the file.
func (f SessionFile) checkDir() error {
	info, err := os.Stat(filepath.Dir(f.Path))
	if err != nil {
		return err
	}
	return checkModes(info, true)
}

// Load reads the session. Errors: ErrNotLoggedIn when the file does not exist,
// ErrInsecureSessionFile, ErrInvalidSessionFile, or the error of the file system.
func (f SessionFile) Load() (*Session, error) {
	info, err := os.Lstat(f.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotLoggedIn
	}
	if err != nil {
		return nil, err
	}
	if err := checkModes(info, false); err != nil {
		return nil, err
	}
	if err := f.checkDir(); err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Clean(f.Path))
	if err != nil {
		return nil, err
	}
	var s Session
	// The decode error is dropped: its text could quote a part of the file.
	if json.Unmarshal(raw, &s) != nil || !s.valid() {
		return nil, ErrInvalidSessionFile
	}
	return &s, nil
}

// Store writes s atomically: a temporary file with mode 0600 in the same directory, fsync,
// rename over the file, fsync of the directory. A crash before the rename leaves the old file as
// it was. The directory is created with mode 0700 when missing. Errors: ErrInsecureSessionFile
// when the directory is accessible to group or others, or the error of the file system.
func (f SessionFile) Store(s *Session) (err error) {
	dir := filepath.Dir(f.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := f.checkDir(); err != nil {
		return err
	}
	raw, err := json.Marshal(s) // #nosec G117 -- the session file is the one place the tokens are kept (D-M8-10-2)
	if err != nil {
		return errors.New("bothyapi: encoding the session failed")
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(f.Path)+".tmp-")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(tmp.Name())
		}
	}()
	if err := tmp.Chmod(0o600); err != nil && runtime.GOOS != "windows" {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	rename := f.rename
	if rename == nil {
		rename = os.Rename
	}
	if err := rename(tmp.Name(), f.Path); err != nil {
		return err
	}
	return syncDir(dir)
}

// Remove deletes the session file; a missing file is no error. The lock file stays, so a process
// that holds or waits for the lock never ends up in a second critical section on a new inode.
func (f SessionFile) Remove() error {
	err := os.Remove(f.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return syncDir(filepath.Dir(f.Path))
}

// Lock takes the exclusive advisory lock of Path + ".lock" (flock on Unix, LockFileEx on
// Windows), creating the directory with mode 0700 and the lock file with mode 0600 when missing,
// and blocks until it holds it. The returned function releases it. Errors:
// ErrInsecureSessionFile when the directory is accessible to group or others, or the error of the
// file system.
func (f SessionFile) Lock() (unlock func() error, err error) {
	if err := os.MkdirAll(filepath.Dir(f.Path), 0o700); err != nil {
		return nil, err
	}
	if err := f.checkDir(); err != nil {
		return nil, err
	}
	lf, err := os.OpenFile(filepath.Clean(f.Path+".lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err := lockFile(lf); err != nil {
		_ = lf.Close()
		return nil, err
	}
	return func() error {
		return errors.Join(unlockFile(lf), lf.Close())
	}, nil
}
