// SPDX-License-Identifier: Apache-2.0

package templates_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents/adapters/templates"
)

// source is templates/agents at the repository root, the source of the generated copy.
const source = "../../../../../../templates/agents"

// TestCopyEqualsTheShippedTemplates: the embedded copy holds exactly the files of every template
// folder of templates/agents, byte for byte (26 7.2; make codegen-check holds the same through go
// generate).
func TestCopyEqualsTheShippedTemplates(t *testing.T) {
	t.Parallel()
	want := map[string][]byte{}
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		err := filepath.WalkDir(filepath.Join(source, e.Name()), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(source, p)
			data, err := os.ReadFile(p) //nolint:gosec // a path of the walk below the source folder
			want[filepath.ToSlash(rel)] = data
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	got := map[string][]byte{}
	err = fs.WalkDir(templates.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(templates.FS(), p)
		got[p] = data
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(want) == 0 {
		t.Fatal("no shipped templates")
	}
	for name, data := range want {
		if !bytes.Equal(got[name], data) {
			t.Errorf("%s differs from the source; run go generate ./internal/agents/adapters/templates/", name)
		}
	}
	for name := range got {
		if _, ok := want[name]; !ok {
			t.Errorf("%s is not in the source; run go generate ./internal/agents/adapters/templates/", name)
		}
	}
}

// TestFSListsTheTemplateFolders: the root of FS holds one folder per template and nothing else.
func TestFSListsTheTemplateFolders(t *testing.T) {
	t.Parallel()
	entries, err := fs.ReadDir(templates.FS(), ".")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			t.Errorf("%s at the root of the copy", e.Name())
		}
		names = append(names, e.Name())
	}
	if want := []string{"code-reviewer", "meeting-notes", "research-summarizer", "writing-helper"}; !slices.Equal(names, want) {
		t.Fatalf("templates %v, want %v", names, want)
	}
}
