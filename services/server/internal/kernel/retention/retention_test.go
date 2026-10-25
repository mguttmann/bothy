// SPDX-License-Identifier: Apache-2.0

package retention_test

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/retention"
)

// privacySpec is chapter 20, relative to this package.
var privacySpec = filepath.Join("..", "..", "..", "..", "..", "docs", "spec", "20-privacy.md")

// classRow matches a row of the table of 20 section 4.1: the class in the first cell.
var classRow = regexp.MustCompile("^\\| `([a-z_]+)` \\|")

// TestClassesEqual20 holds Classes equal to the table of 20 section 4.1, in its order.
func TestClassesEqual20(t *testing.T) {
	f, err := os.Open(privacySpec) // #nosec G304 -- a fixed path of the repository
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var got []retention.Class
	in := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "### ") {
			in = strings.HasPrefix(line, "### 4.1 ")
			continue
		}
		if m := classRow.FindStringSubmatch(line); in && m != nil {
			got = append(got, retention.Class(m[1]))
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, retention.Classes()) {
		t.Fatalf("20 4.1 lists %v, Classes returns %v", got, retention.Classes())
	}
	for _, c := range retention.JobClasses() {
		if !slices.Contains(got, c) {
			t.Errorf("job class %s is not a class of 20 4.1", c)
		}
	}
}

// fake is a participant that removes n rows per call from a pool of rows.
type fake struct {
	class   retention.Class
	tables  []string
	left    int
	cutoffs []time.Time
	err     error
	onPurge func()
}

func (f *fake) Class() retention.Class { return f.class }
func (f *fake) Tables() []string       { return f.tables }
func (f *fake) Purge(_ context.Context, cutoff time.Time, batch int) (int, bool, error) {
	f.cutoffs = append(f.cutoffs, cutoff)
	if f.onPurge != nil {
		f.onPurge()
	}
	if f.err != nil {
		return 0, false, f.err
	}
	n := min(batch, f.left)
	f.left -= n
	return n, f.left > 0, nil
}

func TestRegisterRejects(t *testing.T) {
	reg := retention.NewRegistry()
	if err := reg.Register(nil, time.Hour); err == nil {
		t.Error("nil participant accepted")
	}
	if err := reg.Register(&fake{class: retention.ClassStats}, time.Hour); err == nil {
		t.Error("class without a job accepted")
	}
	if err := reg.Register(&fake{class: retention.ClassCost}, -time.Hour); err == nil {
		t.Error("negative period accepted")
	}
	if err := reg.Register(&fake{class: retention.ClassCost, tables: []string{"cost_entries"}}, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(&fake{class: retention.ClassSecurity, tables: []string{"cost_entries"}}, time.Hour); err == nil {
		t.Error("a table with two participants accepted")
	}
	if got := len(reg.Of(retention.ClassCost)); got != 1 {
		t.Errorf("Of(cost) = %d registrations, want 1", got)
	}
}

func TestRunCutoffBatchesAndSkip(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	c := clock.NewFake(now)
	a := &fake{class: retention.ClassSecurity, left: 5}
	off := &fake{class: retention.ClassSecurity, left: 5}
	b := &fake{class: retention.ClassSecurity, left: 1}
	regs := []retention.Registration{{Participant: a, Period: time.Hour}, {Participant: off}, {Participant: b, Period: 2 * time.Hour}}
	res, err := retention.Run(context.Background(), regs, c, 2, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if res.Deleted != 6 || res.Exhausted {
		t.Fatalf("Run = %+v, want 6 deleted and not exhausted", res)
	}
	if len(a.cutoffs) != 3 || !a.cutoffs[0].Equal(now.Add(-time.Hour)) {
		t.Errorf("a: cutoffs %v", a.cutoffs)
	}
	if len(off.cutoffs) != 0 {
		t.Error("a registration without a period was run")
	}
	if len(b.cutoffs) != 1 || !b.cutoffs[0].Equal(now.Add(-2*time.Hour)) {
		t.Errorf("b: cutoffs %v", b.cutoffs)
	}
}

func TestRunBudgetAndError(t *testing.T) {
	c := clock.NewFake(time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC))
	a := &fake{class: retention.ClassCost, left: 100}
	a.onPurge = func() { c.Advance(time.Minute) }
	res, err := retention.Run(context.Background(), []retention.Registration{{Participant: a, Period: time.Hour}}, c, 10, 3*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Exhausted || res.Deleted != 30 {
		t.Fatalf("Run = %+v, want 30 deleted and exhausted", res)
	}
	boom := errors.New("boom")
	_, err = retention.Run(context.Background(), []retention.Registration{{Participant: &fake{class: retention.ClassCost, err: boom}, Period: time.Hour}}, c, 10, time.Minute)
	if !errors.Is(err, boom) {
		t.Fatalf("Run error = %v, want boom", err)
	}
	if _, err := retention.Run(context.Background(), nil, c, 0, time.Minute); err == nil {
		t.Error("batch 0 accepted")
	}
}

// TestRunContinuesAfterAFailure: a participant that fails does not keep the next one of its class
// from its run; the error is returned with the rows the others removed.
func TestRunContinuesAfterAFailure(t *testing.T) {
	c := clock.NewFake(time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC))
	boom := errors.New("boom")
	next := &fake{class: retention.ClassAudit, left: 3}
	res, err := retention.Run(context.Background(), []retention.Registration{
		{Participant: &fake{class: retention.ClassAudit, err: boom}, Period: time.Hour}, {Participant: next, Period: time.Hour},
	}, c, 10, time.Minute)
	if !errors.Is(err, boom) || res.Deleted != 3 || next.left != 0 {
		t.Fatalf("Run = %+v, %v; want the next participant run and the error returned", res, err)
	}
}
