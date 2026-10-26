// SPDX-License-Identifier: Apache-2.0

package personaldata

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sync"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// sectionKey is the form of a section name: lower snake case.
var sectionKey = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// CheckSection reports why key cannot be a section of f: f is no object file of Files or key is
// not lower snake case. Writers call it before they store a section.
func CheckSection(f File, key string) error {
	if !f.Known() || f.Lines() {
		return fmt.Errorf("personaldata: %s is no JSON object file of the package", f)
	}
	if !sectionKey.MatchString(key) {
		return fmt.Errorf("personaldata: section %q is not lower snake case", key)
	}
	return nil
}

// CheckLines reports why f cannot take lines: it is no JSON Lines file of Files.
func CheckLines(f File) error {
	if !f.Known() || !f.Lines() {
		return fmt.Errorf("personaldata: %s is no JSON Lines file of the package", f)
	}
	return nil
}

// MemoryWriter is a Writer that keeps the export in memory, encoded as the package would hold
// it. Tests of participants use it; it is safe for concurrent use.
//
// Example:
//
//	w := personaldata.NewMemoryWriter()
//	if err := p.Export(ctx, personaldata.Export{Subject: alex}, w); err != nil {
//		t.Fatal(err)
//	}
//	raw := w.Sections[personaldata.FileDevices]["devices"]
type MemoryWriter struct {
	mu sync.Mutex
	// Sections holds the encoded value of every section per object file.
	Sections map[File]map[string]json.RawMessage
	// Lines holds the encoded lines per JSON Lines file, in the order written.
	Lines map[File][]json.RawMessage
	// Contents holds the bytes of every own upload by its ID.
	Contents map[ids.ID][]byte
}

// Compile-time check.
var _ Writer = (*MemoryWriter)(nil)

// NewMemoryWriter returns an empty MemoryWriter.
func NewMemoryWriter() *MemoryWriter {
	return &MemoryWriter{
		Sections: map[File]map[string]json.RawMessage{}, Lines: map[File][]json.RawMessage{}, Contents: map[ids.ID][]byte{},
	}
}

// Section implements Writer.
func (w *MemoryWriter) Section(f File, key string, v any) error {
	if err := CheckSection(f, key); err != nil {
		return err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("personaldata: section %s of %s: %w", key, f, err)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, dup := w.Sections[f][key]; dup {
		return fmt.Errorf("personaldata: section %s of %s is written twice", key, f)
	}
	if w.Sections[f] == nil {
		w.Sections[f] = map[string]json.RawMessage{}
	}
	w.Sections[f][key] = b
	return nil
}

// Line implements Writer.
func (w *MemoryWriter) Line(f File, v any) error {
	if err := CheckLines(f); err != nil {
		return err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("personaldata: line of %s: %w", f, err)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Lines[f] = append(w.Lines[f], b)
	return nil
}

// Content implements Writer.
func (w *MemoryWriter) Content(id ids.ID, r io.Reader) (int64, string, error) {
	var buf bytes.Buffer
	n, err := io.Copy(&buf, r)
	if err != nil {
		return 0, "", err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, dup := w.Contents[id]; dup {
		return 0, "", fmt.Errorf("personaldata: content %s is written twice", id)
	}
	w.Contents[id] = buf.Bytes()
	sum := sha256.Sum256(buf.Bytes())
	return n, hex.EncodeToString(sum[:]), nil
}
