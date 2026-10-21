// SPDX-License-Identifier: Apache-2.0

package title_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat/domain/title"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// wantLists is the content of the word lists (brief M2-18 note 2b): together exactly the non-empty
// entries of the default of thread.placeholder_titles in docs/spec/09-agenten.md section 21.
var wantLists = map[string][]string{
	"de": {"Neuer Chat", "Chat", "Ohne Titel", "Test"},
	"en": {"New chat", "Chat", "Untitled", "Test"},
}

// TestWordListFiles (brief note 2a and 2b): the languages are de and en; each file starts with
// the SPDX line and holds only normalised display texts, once each, with the content of note 2b.
func TestWordListFiles(t *testing.T) {
	t.Parallel()
	if got := title.Languages(); !slices.Equal(got, []string{"de", "en"}) {
		t.Fatalf("languages %v", got)
	}
	for _, lang := range title.Languages() {
		f, err := os.Open(filepath.Join("wordlists", lang+".txt"))
		if err != nil {
			t.Fatal(err)
		}
		sc := bufio.NewScanner(f)
		if !sc.Scan() || sc.Text() != "# SPDX-License-Identifier: Apache-2.0" {
			t.Errorf("%s: the first line is not the SPDX line", lang)
		}
		_ = f.Close()
		got, ok := title.WordList(lang)
		if !ok || !slices.Equal(got, wantLists[lang]) {
			t.Errorf("%s: %v, want %v", lang, got, wantLists[lang])
		}
		for i, e := range got {
			if title.Normalize(e) != e || e == "" {
				t.Errorf("%s: %q is not normalised", lang, e)
			}
			if err := textcheck.DisplayText(e); err != nil {
				t.Errorf("%s: %q: %v", lang, e, err)
			}
			for _, o := range got[:i] {
				if strings.EqualFold(o, e) {
					t.Errorf("%s: %q twice", lang, e)
				}
			}
		}
	}
	if _, ok := title.WordList("fr"); ok {
		t.Error("a word list for fr")
	}
}

// TestDefaultEntriesAndConfigText (brief note 2d and 2e): the union in language and file order
// without repeats, rendered exactly as the cell of 09 section 21.
func TestDefaultEntriesAndConfigText(t *testing.T) {
	t.Parallel()
	want := []string{"Neuer Chat", "Chat", "Ohne Titel", "Test", "New chat", "Untitled"}
	if got := title.DefaultEntries(); !slices.Equal(got, want) {
		t.Fatalf("default entries %v", got)
	}
	const cell = `["Neuer Chat", "Chat", "Ohne Titel", "Test", "New chat", "Untitled"]`
	if got := title.DefaultConfigText(); got != cell {
		t.Fatalf("config text %s", got)
	}
	var parsed []string
	if err := json.Unmarshal([]byte(title.DefaultConfigText()), &parsed); err != nil || !slices.Equal(parsed, want) {
		t.Fatalf("parsed %v, %v", parsed, err)
	}
	// The returned slices are copies.
	d := title.DefaultEntries()
	d[0] = "x"
	if title.DefaultEntries()[0] != "Neuer Chat" {
		t.Fatal("DefaultEntries shares its slice")
	}
}
