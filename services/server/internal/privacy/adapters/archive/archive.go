// SPDX-License-Identifier: Apache-2.0

// Package archive builds the export package of docs/spec/20-privacy.md section 5.2 (brief M7-06
// design note 2): the participants write their sections, lines and uploads into a temporary
// directory, Finish writes every JSON file, README.txt and manifest.json last (with the SHA-256
// of every other file) and zips the directory. Nothing stays in memory but the sections of the
// JSON object files, which hold metadata only; lines and uploads stream to disk.
//
// Example:
//
//	arcs := archive.New(os.TempDir())
//	a, err := arcs.New()
//	defer a.Discard()
//	_ = a.Line(personaldata.FileMessages, msg)
//	built, err := a.Finish(readme, manifest)
package archive

import (
	"archive/zip"
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/privacy/app"
	"github.com/OWNER/bothy/services/server/internal/privacy/domain"
)

// Archives creates archives in a directory.
type Archives struct {
	dir string
}

// Compile-time check.
var _ app.Archives = Archives{}

// New returns the archive factory over dir, the parent of the temporary directories ("" means
// os.TempDir()).
func New(dir string) Archives { return Archives{dir: dir} }

// New implements app.Archives: a fresh temporary directory.
func (a Archives) New() (app.Archive, error) {
	dir, err := os.MkdirTemp(a.dir, "bothy-export-*")
	if err != nil {
		return nil, fmt.Errorf("archive: temporary directory: %w", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "pkg"), 0o700); err != nil {
		return nil, errors.Join(err, os.RemoveAll(dir))
	}
	if err := os.Mkdir(filepath.Join(dir, "pkg", "files"), 0o700); err != nil {
		return nil, errors.Join(err, os.RemoveAll(dir))
	}
	return &Archive{dir: dir, sections: map[personaldata.File]map[string]json.RawMessage{}, lines: map[personaldata.File]*os.File{},
		contents: map[ids.ID]bool{}}, nil
}

// Archive is one package under construction; it implements app.Archive and is safe for
// concurrent use.
type Archive struct {
	mu       sync.Mutex
	dir      string
	sections map[personaldata.File]map[string]json.RawMessage
	lines    map[personaldata.File]*os.File
	contents map[ids.ID]bool
	done     bool
}

// pkg returns the path of name inside the package directory.
func (a *Archive) pkg(name string) string {
	return filepath.Join(a.dir, "pkg", filepath.FromSlash(name))
}

// Section implements personaldata.Writer.
func (a *Archive) Section(f personaldata.File, key string, v any) error {
	if err := personaldata.CheckSection(f, key); err != nil {
		return err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("archive: section %s of %s: %w", key, f, err)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.done {
		return errors.New("archive: the package is finished")
	}
	if _, dup := a.sections[f][key]; dup {
		return fmt.Errorf("archive: section %s of %s is written twice", key, f)
	}
	if a.sections[f] == nil {
		a.sections[f] = map[string]json.RawMessage{}
	}
	a.sections[f][key] = b
	return nil
}

// Line implements personaldata.Writer.
func (a *Archive) Line(f personaldata.File, v any) error {
	if err := personaldata.CheckLines(f); err != nil {
		return err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("archive: line of %s: %w", f, err)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.done {
		return errors.New("archive: the package is finished")
	}
	out := a.lines[f]
	if out == nil {
		if out, err = os.OpenFile(a.pkg(string(f)), os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600); err != nil { // #nosec G304 -- a fixed name in an own directory
			return err
		}
		a.lines[f] = out
	}
	_, err = out.Write(append(b, '\n'))
	return err
}

// Content implements personaldata.Writer: the bytes go to files/<id>.
func (a *Archive) Content(id ids.ID, r io.Reader) (int64, string, error) {
	a.mu.Lock()
	if a.done || a.contents[id] {
		a.mu.Unlock()
		return 0, "", fmt.Errorf("archive: content %s is written twice or after the end", id)
	}
	a.contents[id] = true
	a.mu.Unlock()
	f, err := os.OpenFile(a.pkg("files/"+id.String()), os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600) // #nosec G304 -- an ID in an own directory
	if err != nil {
		return 0, "", err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), r)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return 0, "", err
	}
	return n, hex.EncodeToString(h.Sum(nil)), nil
}

// Finish implements app.Archive: every object file of the package (an empty object when no
// participant wrote a section), every JSON Lines file (empty when no line was written), README.txt,
// then manifest.json with the size and SHA-256 of every other file, all zipped in the order of
// the manifest with manifest.json last.
func (a *Archive) Finish(readme string, m domain.Manifest) (app.Built, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.done {
		return app.Built{}, errors.New("archive: the package is finished")
	}
	a.done = true
	for _, f := range a.lines {
		if err := f.Close(); err != nil {
			return app.Built{}, err
		}
	}
	paths, err := a.writeFiles(readme)
	if err != nil {
		return app.Built{}, err
	}
	m.Files = make([]domain.ManifestFile, 0, len(paths))
	for _, p := range paths {
		size, sum, err := digest(a.pkg(p))
		if err != nil {
			return app.Built{}, err
		}
		m.Files = append(m.Files, domain.ManifestFile{Path: p, Size: size, SHA256: sum})
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return app.Built{}, err
	}
	if err := os.WriteFile(a.pkg(domain.FileManifest), b, 0o600); err != nil {
		return app.Built{}, err
	}
	zipPath := filepath.Join(a.dir, "package.zip")
	if err := writeZip(zipPath, a.pkg, append(paths, domain.FileManifest)); err != nil {
		return app.Built{}, err
	}
	size, sum, err := digest(zipPath)
	if err != nil {
		return app.Built{}, err
	}
	return app.Built{Size: size, SHA256: sum, Open: func() (io.ReadCloser, error) { return os.Open(zipPath) }}, nil // #nosec G304 -- own file
}

// writeFiles writes the object files, creates the missing JSON Lines files and README.txt, and
// returns the paths of every file but the manifest in archive order: the files of 20 5.2, then
// the uploads, then README.txt.
func (a *Archive) writeFiles(readme string) ([]string, error) {
	var paths []string
	for _, f := range personaldata.Files() {
		p := string(f)
		if f.Lines() {
			if _, ok := a.lines[f]; !ok {
				if err := os.WriteFile(a.pkg(p), nil, 0o600); err != nil {
					return nil, err
				}
			}
		} else if err := writeObject(a.pkg(p), a.sections[f]); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	var uploads []string
	for id := range a.contents {
		uploads = append(uploads, "files/"+id.String())
	}
	slices.Sort(uploads)
	paths = append(paths, uploads...)
	if err := os.WriteFile(a.pkg(domain.FileReadme), []byte(readme), 0o600); err != nil {
		return nil, err
	}
	return append(paths, domain.FileReadme), nil
}

// writeObject writes the sections as one JSON object with the keys in ascending order.
func writeObject(path string, sections map[string]json.RawMessage) error {
	if sections == nil {
		sections = map[string]json.RawMessage{}
	}
	b, err := json.MarshalIndent(sections, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// digest returns the size and lower-case hex SHA-256 of the file at path.
func digest(path string) (int64, string, error) {
	f, err := os.Open(path) // #nosec G304 -- own file
	if err != nil {
		return 0, "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return 0, "", err
	}
	return n, hex.EncodeToString(h.Sum(nil)), nil
}

// writeZip zips the files of paths (archive paths, resolved with local) into dst in that order.
func writeZip(dst string, local func(string) string, paths []string) (err error) {
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600) // #nosec G304 -- own file
	if err != nil {
		return err
	}
	bw := bufio.NewWriter(out)
	zw := zip.NewWriter(bw)
	defer func() {
		err = errors.Join(err, zw.Close(), bw.Flush(), out.Close())
	}()
	for _, p := range paths {
		if err := addFile(zw, p, local(p)); err != nil {
			return err
		}
	}
	return nil
}

// addFile copies the file at src into the archive as name.
func addFile(zw *zip.Writer, name, src string) error {
	in, err := os.Open(src) // #nosec G304 -- own file
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
	if err != nil {
		return err
	}
	_, err = io.Copy(w, in)
	return err
}

// Discard implements app.Archive.
func (a *Archive) Discard() {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, f := range a.lines {
		_ = f.Close()
	}
	a.done = true
	_ = os.RemoveAll(a.dir)
}
