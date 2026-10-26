// SPDX-License-Identifier: Apache-2.0

package archive_test

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/privacy/adapters/archive"
	"github.com/OWNER/bothy/services/server/internal/privacy/domain"
)

// TestArchive: every file of 20 5.2 is in the ZIP, the manifest comes last and names the SHA-256
// of every other file, sections become one object per file, and Discard removes the directory.
func TestArchive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a, err := archive.New(dir).New()
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Section(personaldata.FileDevices, "devices", []map[string]string{{"name": "Laptop"}}); err != nil {
		t.Fatal(err)
	}
	if err := a.Section(personaldata.FileDevices, "sessions", []string{}); err != nil {
		t.Fatal(err)
	}
	if a.Section(personaldata.FileDevices, "devices", 1) == nil {
		t.Fatal("duplicate section accepted")
	}
	for i := range 3 {
		if err := a.Line(personaldata.FileMessages, map[string]int{"n": i}); err != nil {
			t.Fatal(err)
		}
	}
	up := ids.New()
	if n, sum, err := a.Content(up, strings.NewReader("hello")); err != nil || n != 5 || len(sum) != 64 {
		t.Fatalf("Content = %d, %s, %v", n, sum, err)
	}
	built, err := a.Finish("readme", domain.Manifest{SchemaVersion: 1, InstanceID: ids.New(), SubjectUserID: ids.New(), RequestID: ids.New(), CreatedAt: time.Unix(0, 0).UTC()})
	if err != nil {
		t.Fatal(err)
	}
	rc, err := built.Open()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil || int64(len(raw)) != built.Size {
		t.Fatalf("archive size %d, built %d, %v", len(raw), built.Size, err)
	}
	if sum := sha256.Sum256(raw); hex.EncodeToString(sum[:]) != built.SHA256 {
		t.Fatal("archive checksum differs")
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	content := map[string][]byte{}
	var order []string
	for _, f := range zr.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(r)
		_ = r.Close()
		content[f.Name] = b
		order = append(order, f.Name)
	}
	if order[len(order)-1] != domain.FileManifest {
		t.Fatalf("manifest is not last: %v", order)
	}
	for _, f := range personaldata.Files() {
		if _, ok := content[string(f)]; !ok {
			t.Errorf("%s missing", f)
		}
	}
	var m domain.Manifest
	if err := json.Unmarshal(content[domain.FileManifest], &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Files) != len(content)-1 {
		t.Fatalf("manifest lists %d files, archive holds %d", len(m.Files), len(content))
	}
	for _, f := range m.Files {
		sum := sha256.Sum256(content[f.Path])
		if hex.EncodeToString(sum[:]) != f.SHA256 || int64(len(content[f.Path])) != f.Size {
			t.Errorf("checksum of %s differs", f.Path)
		}
	}
	var devices map[string]json.RawMessage
	if err := json.Unmarshal(content[string(personaldata.FileDevices)], &devices); err != nil || len(devices) != 2 {
		t.Fatalf("devices.json = %s", content[string(personaldata.FileDevices)])
	}
	if string(content[string(personaldata.FileProfile)]) != "{}" || len(content[string(personaldata.FileAudit)]) != 0 {
		t.Fatal("empty files are wrong")
	}
	if got := strings.Count(string(content[string(personaldata.FileMessages)]), "\n"); got != 3 {
		t.Fatalf("messages.jsonl has %d lines", got)
	}
	if string(content["files/"+up.String()]) != "hello" || string(content[domain.FileReadme]) != "readme" {
		t.Fatal("upload or README differs")
	}
	a.Discard()
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("Discard left %d entries", len(entries))
	}
}

// failingReader fails on the first read.
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

// TestArchiveRefusesBadInput: a missing parent directory, a section or line that does not fit
// its file or cannot be encoded, an upload whose reader fails or that comes twice, and every write
// after Finish are errors.
func TestArchiveRefusesBadInput(t *testing.T) {
	t.Parallel()
	if _, err := archive.New(filepath.Join(t.TempDir(), "missing")).New(); err == nil {
		t.Fatal("an archive in a missing directory was created")
	}
	a, err := archive.New(t.TempDir()).New()
	if err != nil {
		t.Fatal(err)
	}
	defer a.Discard()
	if a.Section(personaldata.FileMessages, "messages", 1) == nil || a.Section(personaldata.FileProfile, "Bad Key", 1) == nil {
		t.Fatal("a section of a JSON Lines file or with a bad key was accepted")
	}
	if a.Section(personaldata.FileProfile, "profile", make(chan int)) == nil || a.Line(personaldata.FileMessages, make(chan int)) == nil {
		t.Fatal("a value that cannot be encoded was accepted")
	}
	if a.Line(personaldata.FileProfile, 1) == nil {
		t.Fatal("a line of an object file was accepted")
	}
	if _, _, err := a.Content(ids.New(), failingReader{}); err == nil {
		t.Fatal("a failing upload reader was accepted")
	}
	up := ids.New()
	if _, _, err := a.Content(up, strings.NewReader("x")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Content(up, strings.NewReader("x")); err == nil {
		t.Fatal("an upload was written twice")
	}
	if err := a.Line(personaldata.FileMessages, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Finish("readme", domain.Manifest{}); err != nil {
		t.Fatal(err)
	}
	if a.Section(personaldata.FileProfile, "profile", 1) == nil || a.Line(personaldata.FileMessages, 1) == nil {
		t.Fatal("a write after Finish was accepted")
	}
	if _, _, err := a.Content(ids.New(), strings.NewReader("x")); err == nil {
		t.Fatal("an upload after Finish was accepted")
	}
	if _, err := a.Finish("readme", domain.Manifest{}); err == nil {
		t.Fatal("Finish ran twice")
	}
}
