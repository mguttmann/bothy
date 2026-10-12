// SPDX-License-Identifier: Apache-2.0

package auditing_test

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// chapter07 returns the path of docs/spec/07-permissions.md.
func chapter07(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "docs", "spec", "07-permissions.md")
}

// reasonCell is the first cell of a row of the table in 07 "4.1".
var reasonCell = regexp.MustCompile("^\\| `([^`]+)` \\|")

// TestReasonsMatchChapter07 holds R-DM-28: the closed list of reasons is the first column of the
// table in 07 "4.1 Decisions and reasons", both ways.
func TestReasonsMatchChapter07(t *testing.T) {
	t.Parallel()
	f, err := os.Open(chapter07(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var chapter []string
	in := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "### 4.1 "):
			in = true
		case in && strings.HasPrefix(line, "### "):
			in = false
		case in:
			if m := reasonCell.FindStringSubmatch(line); m != nil {
				chapter = append(chapter, m[1])
			}
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if len(chapter) < 40 {
		t.Fatalf("read only %d reasons from 07 4.1; the table moved?", len(chapter))
	}
	var code []string
	for _, r := range auditing.KnownReasons() {
		code = append(code, string(r))
	}
	for _, r := range chapter {
		if !slices.Contains(code, r) {
			t.Errorf("07 4.1 lists %q, KnownReasons does not", r)
		}
	}
	for _, r := range code {
		if !slices.Contains(chapter, r) {
			t.Errorf("KnownReasons has %q, 07 4.1 does not", r)
		}
	}
	if !slices.Equal(code, chapter) {
		t.Errorf("KnownReasons is not in the order of 07 4.1")
	}
	if !auditing.ReasonNoGrant.Known() || auditing.Reason("made_up").Known() {
		t.Error("Known does not follow the list")
	}
}

func TestKnownReasons_ReturnsACopy(t *testing.T) {
	t.Parallel()
	a := auditing.KnownReasons()
	a[0] = "changed"
	if auditing.KnownReasons()[0] != auditing.ReasonUnauthenticated {
		t.Fatal("KnownReasons returns the internal slice")
	}
}

func TestValue_Kinds(t *testing.T) {
	t.Parallel()
	id := ids.New()
	at := time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC)
	v := auditing.Violation{Code: "i1_violation", AgentID: id}
	cases := []struct {
		v    auditing.Value
		kind auditing.Kind
		data any
	}{
		{auditing.ID(id), auditing.KindID, id.String()},
		{auditing.Code("read"), auditing.KindCode, "read"},
		{auditing.Int(-3), auditing.KindInt, int64(-3)},
		{auditing.Bool(true), auditing.KindBool, true},
		{auditing.Time(at), auditing.KindTime, at},
		{auditing.ReasonText("Prüfung"), auditing.KindReasonText, "Prüfung"},
	}
	for _, c := range cases {
		if c.v.Kind() != c.kind || c.v.Data() != c.data {
			t.Errorf("%v: kind %v data %v, want %v %v", c.v, c.v.Kind(), c.v.Data(), c.kind, c.data)
		}
	}
	list := auditing.List(auditing.Code("a"), auditing.Code("b"))
	if list.Kind() != auditing.KindList || len(list.Data().([]auditing.Value)) != 2 {
		t.Errorf("List = %v", list)
	}
	viol := auditing.Violations(v)
	if viol.Kind() != auditing.KindViolations || viol.Data().([]auditing.Violation)[0].AgentID != id {
		t.Errorf("Violations = %v", viol)
	}
	if (auditing.Value{}).Kind() != auditing.KindInvalid {
		t.Error("the zero Value is not invalid")
	}
}
