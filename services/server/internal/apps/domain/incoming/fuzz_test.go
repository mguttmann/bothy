// SPDX-License-Identifier: Apache-2.0

package incoming_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/apps/domain/incoming"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// seedFixtures adds every golden fixture to the seed corpus, which plain go test runs.
func seedFixtures(f *testing.F, add func(raw []byte)) {
	f.Helper()
	files, err := filepath.Glob(filepath.Join(goldenDir, "*.json"))
	if err != nil || len(files) == 0 {
		f.Fatalf("no fixtures: %v", err)
	}
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			f.Fatal(err)
		}
		add(raw)
	}
}

// unescapedBracket reports the index of a '[' in s that no odd run of backslashes escapes and
// that lies outside an autolink, or -1. Payload values escape every '<', so an unescaped '<'
// opens an autolink of a checked URL, which may hold '[' (an IPv6 host).
func unescapedBracket(s string) int {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++ // the next byte is escaped
		case '<':
			end := i + 1
			for end < len(s) && s[end] != '>' {
				end++
			}
			i = end
		case '[':
			return i
		}
	}
	return -1
}

func FuzzRenderAlertmanager(f *testing.F) {
	seedFixtures(f, func(raw []byte) {
		f.Add(raw, uint16(0xffff))
		f.Add(raw, uint16(200))
		f.Add(raw, uint16(17))
	})
	f.Fuzz(func(t *testing.T, raw []byte, limit uint16) {
		var p incoming.Payload
		if json.Unmarshal(raw, &p) != nil {
			return
		}
		n, err := incoming.ParseAlertmanager(p)
		if err != nil {
			if _, ok := errs.As(err); !ok {
				t.Fatalf("parse error is no *errs.Error: %v", err)
			}
			return
		}
		for _, lim := range []int{int(limit), largeLimit} {
			got := incoming.RenderAlertmanager(n, lim)
			if len(got) > lim {
				t.Fatalf("limit %d: %d bytes", lim, len(got))
			}
			if !utf8.ValidString(got) {
				t.Fatalf("limit %d: invalid UTF-8", lim)
			}
			if i := unescapedBracket(got); i >= 0 {
				t.Fatalf("limit %d: unescaped [ at byte %d", lim, i)
			}
		}
	})
}

func FuzzClassify(f *testing.F) {
	seedFixtures(f, func(raw []byte) { f.Add(raw) })
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"content_md":"Report","title":"Nightly"}`))
	f.Add([]byte(`{"content_md":"x","alerts":[]}`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		var body map[string]json.RawMessage
		if json.Unmarshal(raw, &body) != nil {
			return
		}
		var present []string
		var native, alert bool
		for _, m := range incoming.NativeMembers() {
			if _, ok := body[m]; ok {
				present, native = append(present, m), true
			}
		}
		for _, m := range incoming.AlertmanagerMembers() {
			if _, ok := body[m]; ok {
				present, alert = append(present, m), true
			}
		}
		got, err := incoming.Classify(present)
		switch {
		case alert && native:
			e, ok := errs.As(err)
			if !ok || len(e.Fields) != 1 || e.Fields[0].Code != incoming.CodeFormatMixed || e.Fields[0].Pointer != "/"+present[0] {
				t.Fatalf("mixed body %v: %v", present, err)
			}
		case alert:
			if err != nil || got != incoming.FormatAlertmanager {
				t.Fatalf("alertmanager body %v: %q, %v", present, got, err)
			}
		default:
			if err != nil || got != incoming.FormatNative {
				t.Fatalf("native body %v: %q, %v", present, got, err)
			}
		}
		if err == nil && !slices.Contains([]incoming.Format{incoming.FormatNative, incoming.FormatAlertmanager}, got) {
			t.Fatalf("unknown format %q", got)
		}
	})
}
