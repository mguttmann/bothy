// SPDX-License-Identifier: Apache-2.0

// Package filesink is the file system implementation of app.CheckpointSink, the value
// file://<absolute path> of audit.checkpoint_sink (19 SEC-AUD-7, table "Format der Prüfpunkte"):
// one file <root>/<instance_id>/<seq with 20 digits, leading zeros>.jws per checkpoint holding the
// compact JWS, the directory with mode 0700, the file with mode 0600. A file is written once: into
// a temporary file of the same directory created with O_EXCL, synced, then linked to its name,
// which fails when the name exists (a rename would replace it), and the directory is synced. A
// stored file with other content is an error and stays. The root itself must exist; the sink
// never creates it, because a missing mount would put the checkpoints on another volume. The file
// system must support hard links. A link in place of the directory of the instance or of a
// checkpoint file is refused, and a wider mode of the directory is narrowed to 0700.
//
// Example:
//
//	sink, err := filesink.New("/var/lib/bothy-checkpoints", instanceID)
//	err = sink.Put(ctx, app.SignedCheckpoint{Seq: 5301, JWS: jws})
package filesink

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The modes of the sink (19 SEC-AUD-7).
const (
	dirMode  fs.FileMode = 0o700
	fileMode fs.FileMode = 0o600
)

// seqDigits is the width of the seq in a file name: 20 digits hold every int64.
const seqDigits = 20

// tmpPrefix starts the name of a temporary file; List skips such files.
const tmpPrefix = ".tmp-"

// fileName matches the name of a checkpoint file.
var fileName = regexp.MustCompile(`^[0-9]{20}\.jws$`)

// Sink stores checkpoints below one directory. It is safe for concurrent use, also by several
// processes on the same directory.
type Sink struct {
	root string
	dir  string
}

// Compile-time check: Sink is the outgoing port of app.
var _ app.CheckpointSink = (*Sink)(nil)

// New returns the sink below root for the checkpoints of instance. It does no I/O. Errors: a root
// that is not an absolute clean path.
func New(root string, instance ids.ID) (*Sink, error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return nil, errors.New("filesink: the root must be an absolute, clean path")
	}
	return &Sink{root: root, dir: filepath.Join(root, instance.String())}, nil
}

// Configured reports true: a file sink is a sink.
func (s *Sink) Configured() bool { return true }

// path returns the file of seq.
func (s *Sink) path(seq int64) string {
	return filepath.Join(s.dir, fmt.Sprintf("%0*d.jws", seqDigits, seq))
}

// Put stores c once (app.CheckpointSink). Errors: a seq below 1, a missing root, a stored file
// with other content, and the errors of the file system.
func (s *Sink) Put(_ context.Context, c app.SignedCheckpoint) error {
	if c.Seq < 1 {
		return errors.New("filesink: a checkpoint has a seq of at least 1")
	}
	if _, err := os.Stat(s.root); err != nil {
		return fmt.Errorf("filesink: the root of the sink: %w", err)
	}
	if err := s.ensureDir(); err != nil {
		return err
	}
	final := s.path(c.Seq)
	switch err := compare(final, c.JWS); {
	case err == nil:
		return nil
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}
	tmp, err := s.writeTemp(c.JWS)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp) }()
	if err := os.Link(tmp, final); err != nil {
		if errors.Is(err, fs.ErrExist) {
			// A concurrent writer linked first; its content decides.
			return compare(final, c.JWS)
		}
		return fmt.Errorf("filesink: link the checkpoint: %w", err)
	}
	return syncDir(s.dir)
}

// ensureDir creates the directory of the instance with mode 0700 (and syncs the root), or checks
// that the existing one is a directory and not a link, and narrows a wider mode to 0700.
func (s *Sink) ensureDir() error {
	err := os.Mkdir(s.dir, dirMode)
	if err == nil {
		return syncDir(s.root)
	}
	if !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("filesink: the directory of the instance: %w", err)
	}
	fi, err := os.Lstat(s.dir)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return errors.New("filesink: the directory of the instance is no directory (a link is refused)")
	}
	if fi.Mode().Perm() != dirMode {
		return os.Chmod(s.dir, dirMode)
	}
	return nil
}

// errOtherContent is the error of a Put whose seq holds other content.
var errOtherContent = fmt.Errorf("filesink: %w", app.ErrCheckpointConflict)

// compare returns nil when the file at name holds want, errOtherContent when it holds other
// content, and the read error otherwise (fs.ErrNotExist when it is missing).
func compare(name string, want []byte) error {
	fi, err := os.Lstat(name)
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() {
		return errors.New("filesink: a checkpoint name holds no regular file (a link is refused)")
	}
	got, err := os.ReadFile(name) //nolint:gosec // G304: the name is built from the root and a seq
	if err != nil {
		return err
	}
	if !bytes.Equal(got, want) {
		return errOtherContent
	}
	return nil
}

// writeTemp writes data into a new temporary file of the directory with O_EXCL and mode 0600,
// syncs and closes it, and returns its name.
func (s *Sink) writeTemp(data []byte) (string, error) {
	var suffix [16]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", err
	}
	name := filepath.Join(s.dir, tmpPrefix+hex.EncodeToString(suffix[:]))
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fileMode) //nolint:gosec // G304: the name is built from the root and random bytes
	if err != nil {
		return "", fmt.Errorf("filesink: create a temporary file: %w", err)
	}
	_, werr := f.Write(data)
	serr := f.Sync()
	cerr := f.Close()
	if err := errors.Join(werr, serr, cerr); err != nil {
		_ = os.Remove(name)
		return "", fmt.Errorf("filesink: write a temporary file: %w", err)
	}
	return name, nil
}

// syncDir syncs the directory, so the new name survives a crash.
func syncDir(dir string) error {
	d, err := os.Open(dir) //nolint:gosec // G304: the directory of the sink
	if err != nil {
		return err
	}
	return errors.Join(d.Sync(), d.Close())
}

// List returns every checkpoint file of the instance (app.CheckpointSink). Temporary files and
// names that are no checkpoint are skipped. Errors: a missing root, and the errors of the file
// system; a root without a directory of the instance holds no checkpoint yet.
func (s *Sink) List(_ context.Context) ([]app.SignedCheckpoint, error) {
	if _, err := os.Stat(s.root); err != nil {
		return nil, fmt.Errorf("filesink: the root of the sink: %w", err)
	}
	fi, err := os.Lstat(s.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, errors.New("filesink: the directory of the instance is no directory (a link is refused)")
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	var out []app.SignedCheckpoint
	for _, e := range entries {
		if !e.Type().IsRegular() || !fileName.MatchString(e.Name()) {
			continue
		}
		seq, err := strconv.ParseInt(e.Name()[:seqDigits], 10, 64)
		if err != nil {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			return nil, err
		}
		out = append(out, app.SignedCheckpoint{Seq: seq, JWS: data})
	}
	return out, nil
}
