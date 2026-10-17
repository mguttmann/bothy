// SPDX-License-Identifier: Apache-2.0

package doctor

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/hostos"
)

// stuckHelpers never answer until their context ends (a helper stopped with SIGSTOP).
type stuckHelpers struct {
	x        *tw
	scanOnly bool
}

func (s stuckHelpers) Check(ctx context.Context, user string, req Request) (HelperResult, error) {
	if user == sam && !s.scanOnly {
		<-ctx.Done()
		return HelperResult{}, ctx.Err()
	}
	return s.x.Check(ctx, user, req)
}

func (s stuckHelpers) Scan(ctx context.Context, user string, req Request) (string, error) {
	if user == sam {
		<-ctx.Done()
		return "", ctx.Err()
	}
	return s.x.Scan(ctx, user, req)
}

// TestStoppedHelperBlocksOnlyItself: a helper that never answers gives helper_unreachable for its
// session user after a bound, and the run ends (red team finding of M2-42).
func TestStoppedHelperBlocksOnlyItself(t *testing.T) {
	x := newTW(t)
	x.env.Helpers = stuckHelpers{x: x}
	x.env.HelperGrace = 50 * time.Millisecond
	x.cfg.AuthStatusTimeout = 10 * time.Millisecond
	x.cfg.EntryScanTimeout = 10 * time.Millisecond
	done := make(chan *Result)
	runner := NewRunner(x.env, nil)
	go func() { done <- runner.RunFull(context.Background()) }()
	select {
	case r := <-done:
		if !has(codes(r, HD10), CodeHelperUnreachable+"@"+sam) {
			t.Errorf("HD10 %v", codes(r, HD10))
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the run waits for a stopped helper")
	}
	runner.WaitScans()
}

// TestScanOfStoppedHelperEnds: a scan that never answers ends with entry_scan_timeout.
func TestScanOfStoppedHelperEnds(t *testing.T) {
	x := newTW(t)
	x.env.HelperGrace = 50 * time.Millisecond
	x.cfg.EntryScanTimeout = 10 * time.Millisecond
	x.cfg.AuthStatusTimeout = 10 * time.Millisecond
	runner := NewRunner(x.env, nil)
	runner.RunFull(context.Background())
	runner.WaitScans()
	runner.env.Helpers = stuckHelpers{x: x, scanOnly: true}
	done := make(chan struct{})
	go func() { runner.RunFull(context.Background()); runner.WaitScans(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("a stopped helper holds the scans")
	}
	if r := runner.Last(); hd11For(r, sam) != CodeEntryScanTimeout {
		t.Errorf("HD11 %v", codes(r, HD11))
	}
}

// TestChainFileLinkToAnotherPerson: a chain file that is a link of root into a file or folder of
// another session user fails HD11 (TM-T29 (j), red team finding of M2-42).
func TestChainFileLinkToAnotherPerson(t *testing.T) {
	for name, setup := range map[string]func(x *tw){
		"file": func(x *tw) {
			x.w.File("/home/"+alex+"/notes.md", "1001", 0o644, "EXAMPLE")
			x.w.Node("/home/" + alex).Sec.Mode = 0o711
			x.w.Link("/srv/work/CLAUDE.md", "0", "/home/"+alex+"/notes.md")
		},
		"folder": func(x *tw) {
			x.w.Dir("/home/"+alex+"/dotclaude", "1001", 0o755)
			x.w.File("/home/"+alex+"/dotclaude/settings.json", "1001", 0o644, "{}")
			x.w.Node("/home/" + alex).Sec.Mode = 0o711
			x.w.Link("/srv/work/.claude", "0", "/home/"+alex+"/dotclaude")
		},
		"dangling": func(x *tw) { x.w.Link("/srv/work/AGENTS.md", "0", "/srv/nowhere/AGENTS.md") },
	} {
		x := newTW(t)
		x.w.Dir("/srv/work", "0", 0o755)
		workdir(x, sam, "S", "/srv/work/s")
		setup(x)
		if r := x.run(); !has(codes(r, HD11), CodeSharedLocation+"@"+sam) {
			t.Errorf("%s: %v", name, codes(r, HD11))
		}
	}
}

// TestUntrustedClaudeBinIsNotRead: a host_service.claude_bin that fails the trust rule is not
// opened, so a FIFO there cannot block the run (red team finding of M2-42).
func TestUntrustedClaudeBinIsNotRead(t *testing.T) {
	x := newTW(t)
	x.w.File("/home/"+alex+"/claude", "1001", 0o755, "#!/usr/bin/env node\n")
	x.w.Node("/home/" + alex).Sec.Mode = 0o711
	x.cfg.ClaudeBin = "/home/" + alex + "/claude"
	reads := 0
	x.env.Files = countingFiles{Lister: x.env.Files, n: &reads, path: "/home/" + alex + "/claude"}
	r := x.run()
	if !has(codes(r, HD10), CodeClaudeBinUntrusted) || reads != 0 {
		t.Errorf("HD10 %v, reads %d", codes(r, HD10), reads)
	}
}

type countingFiles struct {
	hostos.Lister
	n    *int
	path string
}

func (c countingFiles) Head(p string, n int) ([]byte, error) {
	if p == c.path {
		*c.n++
	}
	return c.Lister.Head(p, n)
}
