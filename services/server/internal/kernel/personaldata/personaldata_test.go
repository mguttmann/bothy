// SPDX-License-Identifier: Apache-2.0

package personaldata_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
)

// fake is a participant of module mod with tables.
type fake struct {
	mod    string
	tables []string
}

func (f fake) Module() string   { return f.mod }
func (f fake) Tables() []string { return f.tables }
func (fake) Export(context.Context, personaldata.Export, personaldata.Writer) error {
	return nil
}
func (fake) Erase(context.Context, personaldata.Step, personaldata.Erasure) error { return nil }

// TestRegistry_Rules: one participant per module, one per table, table names only.
func TestRegistry_Rules(t *testing.T) {
	t.Parallel()
	reg := personaldata.NewRegistry()
	if err := reg.Register(fake{mod: "identity", tables: []string{"users", "principals"}}); err != nil {
		t.Fatal(err)
	}
	for name, p := range map[string]personaldata.Participant{
		"nil":          nil,
		"no module":    fake{tables: []string{"x"}},
		"no tables":    fake{mod: "chat"},
		"same module":  fake{mod: "identity", tables: []string{"owners"}},
		"same table":   fake{mod: "chat", tables: []string{"users"}},
		"invalid name": fake{mod: "chat", tables: []string{"Users"}},
	} {
		if err := reg.Register(p); err == nil {
			t.Errorf("%s: Register accepted it", name)
		}
	}
	if err := reg.Register(fake{mod: "chat", tables: []string{"user_space_states"}}); err != nil {
		t.Fatal(err)
	}
	if got := reg.Tables(); !slices.Equal(got, []string{"principals", "user_space_states", "users"}) {
		t.Fatalf("Tables = %v", got)
	}
	if n := len(reg.All()); n != 2 {
		t.Fatalf("All has %d participants", n)
	}
}

// TestFiles: the files of 20 5.2 in order, JSON Lines by suffix.
func TestFiles(t *testing.T) {
	t.Parallel()
	fs := personaldata.Files()
	if len(fs) != 12 || fs[0] != personaldata.FileProfile || fs[len(fs)-1] != personaldata.FileAudit {
		t.Fatalf("Files = %v", fs)
	}
	if !personaldata.FileMessages.Lines() || personaldata.FilePersonal.Lines() || personaldata.File("x.json").Known() {
		t.Fatal("Lines or Known is wrong")
	}
}

// TestSteps: the participant steps of 20 4.4 in order, named as in progress.
func TestSteps(t *testing.T) {
	t.Parallel()
	var names []string
	for _, s := range personaldata.ParticipantSteps() {
		names = append(names, s.String())
	}
	want := []string{"disable", "security", "personal", "access", "content", "pseudonymize"}
	if !slices.Equal(names, want) || personaldata.StepAudit.String() != "audit" {
		t.Fatalf("steps = %v", names)
	}
	if !personaldata.KeepPseudonymized.Valid() || !personaldata.DeleteContent.Valid() || personaldata.ContentMode("x").Valid() {
		t.Fatal("ContentMode.Valid is wrong")
	}
	var e personaldata.Export
	if ok, err := e.CanRead(context.Background(), [16]byte{}); ok || err != nil {
		t.Fatal("CanRead without Readable must be false")
	}
}

// TestMemoryWriter: sections once per object file, lines only in JSON Lines files, contents once
// with their checksum.
func TestMemoryWriter(t *testing.T) {
	t.Parallel()
	w := personaldata.NewMemoryWriter()
	if err := w.Section(personaldata.FileDevices, "devices", []int{1}); err != nil {
		t.Fatal(err)
	}
	if w.Section(personaldata.FileDevices, "devices", []int{2}) == nil || w.Section(personaldata.FileMessages, "x", 1) == nil ||
		w.Section(personaldata.FileDevices, "Bad", 1) == nil {
		t.Fatal("Section accepted a duplicate, a lines file or a bad key")
	}
	if err := w.Line(personaldata.FileMessages, map[string]int{"a": 1}); err != nil {
		t.Fatal(err)
	}
	if w.Line(personaldata.FileDevices, 1) == nil {
		t.Fatal("Line accepted an object file")
	}
	id := ids.New()
	n, sum, err := w.Content(id, strings.NewReader("abc"))
	if err != nil || n != 3 || sum != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("Content = %d, %s, %v", n, sum, err)
	}
	if _, _, err := w.Content(id, strings.NewReader("x")); err == nil {
		t.Fatal("Content accepted a duplicate")
	}
	if string(w.Sections[personaldata.FileDevices]["devices"]) != "[1]" || len(w.Lines[personaldata.FileMessages]) != 1 {
		t.Fatal("stored values are wrong")
	}
}
