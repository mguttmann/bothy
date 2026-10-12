// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// tempPrefix starts the name of every temp file init writes. A file with this name in the target
// is a leftover of an interrupted run and is removed; in the source it is an unknown file, so an
// adopted copy can never be mistaken for a leftover on the next run.
const tempPrefix = ".bothy-tmp-"

// isTemp reports whether the last element of a slash path names a temp file.
func isTemp(rel string) bool { return strings.HasPrefix(path.Base(rel), tempPrefix) }

// scan lists every entry below dir inside root as slash paths relative to dir (dir itself is not
// listed) with its Lstat result: links are reported, not followed, and root confines every access
// to its tree. A dir that does not exist gives an empty result.
func scan(root *os.Root, dir string) (map[string]fs.FileInfo, error) {
	out := map[string]fs.FileInfo{}
	if _, err := root.Lstat(dir); errors.Is(err, fs.ErrNotExist) {
		return out, nil
	}
	err := fs.WalkDir(root.FS(), dir, func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if name == dir {
			return nil
		}
		info, err := root.Lstat(name)
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(name, dir+"/")
		if dir == "." {
			rel = name
		}
		out[rel] = info
		return nil
	})
	return out, err
}

// requireDir checks a directory named in the options: a path that does not exist or is no
// directory is a usage error (exit code 2), any other failure a runtime error.
func requireDir(p, what string) error {
	info, err := os.Stat(p)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && !info.IsDir()) {
		return usagef("%s %s is not a directory", what, display(p))
	}
	return err
}

// readRegular reads a regular file inside root. The file is opened without blocking and checked
// with Stat on the open file against the entry found by scan, so a file swapped for a link, pipe
// or other file between scan and read is refused instead of read.
func readRegular(root *os.Root, name string, want fs.FileInfo) ([]byte, error) {
	f, err := root.OpenFile(name, os.O_RDONLY|readFlags, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	got, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !got.Mode().IsRegular() || !os.SameFile(got, want) {
		return nil, fmt.Errorf("%s changed while it was read", display(name))
	}
	return io.ReadAll(f)
}

// writeAtomic replaces dir/base inside root with data: temp file in the same directory, write,
// fsync, chmod, chown, rename, fsync of the directory. Readers see the old or the new content,
// never a part, and a crash leaves at most a temp file the next run removes.
func writeAtomic(root *os.Root, dir, base string, data []byte, mode fs.FileMode, o Owner) (err error) {
	tmp := path.Join(dir, tempPrefix+rand.Text())
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = f.Close()
			_ = root.Remove(tmp)
		}
	}()
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Chmod(mode); err != nil {
		return err
	}
	if err = f.Chown(o.UID, o.GID); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = root.Rename(tmp, path.Join(dir, base)); err != nil {
		return err
	}
	return syncDir(root, dir)
}

// setModeOwner gives an entry its mode and owner when either differs and touches nothing
// otherwise, so a second run changes no ctime.
func setModeOwner(root *os.Root, name string, info fs.FileInfo, mode fs.FileMode, o Owner) error {
	if info.Mode()&(fs.ModePerm|fs.ModeSetuid|fs.ModeSetgid|fs.ModeSticky) != mode {
		if err := root.Chmod(name, mode); err != nil {
			return err
		}
	}
	if got, ok := ownerOf(info); !ok || got != o {
		return root.Lchown(name, o.UID, o.GID)
	}
	return nil
}

// sameFileModeOwner reports whether an entry is a regular file with exactly the given permission
// bits and owner.
func sameFileModeOwner(info fs.FileInfo, mode fs.FileMode, o Owner) bool {
	if !info.Mode().IsRegular() {
		return false
	}
	if info.Mode()&(fs.ModePerm|fs.ModeSetuid|fs.ModeSetgid|fs.ModeSticky) != mode {
		return false
	}
	got, ok := ownerOf(info)
	return ok && got == o
}

// escapedError is what Init and Export return: names below offsite/ are the owner's and may hold
// any character, and they reach messages through operating system errors too. Escaping the whole
// message at this one place keeps a newline in a name from forging a line in a terminal or log;
// errors.Is and errors.As still see the wrapped error.
type escapedError struct{ err error }

func (e *escapedError) Error() string { return escape(e.err.Error()) }
func (e *escapedError) Unwrap() error { return e.err }

func escaped(err error) error {
	if err == nil {
		return nil
	}
	return &escapedError{err: err}
}

// escape writes every non-printable rune and every invalid byte as a Go escape sequence.
func escape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			fmt.Fprintf(&b, `\x%02x`, s[i])
		case unicode.IsPrint(r):
			b.WriteRune(r)
		default:
			q := strconv.QuoteRune(r)
			b.WriteString(q[1 : len(q)-1])
		}
		i += size
	}
	return b.String()
}

// display returns a relative path as it may appear in a message: as is when printable, Go-quoted
// otherwise, so a control character in an owner-supplied name cannot forge output lines.
func display(text string) string {
	if text == "" || !utf8.ValidString(text) || strings.TrimSpace(text) != text || strings.HasPrefix(text, `"`) ||
		strings.IndexFunc(text, func(r rune) bool { return !unicode.IsPrint(r) }) >= 0 {
		return strconv.Quote(text)
	}
	return text
}
