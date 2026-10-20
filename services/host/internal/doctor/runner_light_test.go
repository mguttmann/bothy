// SPDX-License-Identifier: Apache-2.0

package doctor

import (
	"context"
	"slices"
	"testing"
)

// countingHelpers counts check.run per session user and records the mode.
type countingHelpers struct {
	*tw
	calls map[string]int
	modes []string
}

func (c *countingHelpers) Check(ctx context.Context, user string, req Request) (HelperResult, error) {
	c.mu.Lock()
	c.calls[user]++
	c.modes = append(c.modes, req.Mode)
	c.mu.Unlock()
	return c.tw.Check(ctx, user, req)
}

func TestRunLightUser(t *testing.T) {
	x := newTW(t)
	c := &countingHelpers{tw: x, calls: map[string]int{}}
	x.env.Helpers = c
	r := NewRunner(x.env, nil)
	r.RunLight(context.Background())
	if c.calls[alex] != 1 || c.calls[sam] != 1 || c.calls[robin] != 1 {
		t.Fatalf("RunLight asks every helper once: %v", c.calls)
	}
	// A later subscription login of Sam is seen only by a run that asks Sam's helper.
	x.auth[sam] = map[string]string{"": "claude.ai"}
	res := r.RunLightUser(context.Background(), alex)
	if c.calls[alex] != 2 || c.calls[sam] != 1 || c.calls[robin] != 1 {
		t.Fatalf("RunLightUser asks only the helper of the session user: %v", c.calls)
	}
	if res.Check(HD6).Failed() {
		t.Error("the cached answer of Sam holds until Sam's helper is asked")
	}
	res = r.RunLightUser(context.Background(), sam)
	if !res.Check(HD6).Failed() {
		t.Error("Sam's own Light run sees the login")
	}
	if slices.ContainsFunc(c.modes, func(m string) bool { return m != ModeLight }) {
		t.Errorf("Light runs send mode light: %v", c.modes)
	}
	if res.ClaudeBin != "/opt/claude/bin/claude" || len(res.SearchList) == 0 || res.SearchList[0] != "/opt/claude/bin" {
		t.Errorf("claude_bin %q search %v", res.ClaudeBin, res.SearchList)
	}
}

// TestHD11SkipsDroppedWorkdir (TM-T29 (c)): a folder that Alex and Sam both list drops from both
// lists in HD7 and is then no place of either, so HD11 blocks neither; a kept folder that another
// session user can write still fails HD11 for its owner.
func TestHD11SkipsDroppedWorkdir(t *testing.T) {
	x := newTW(t)
	x.w.Dir("/srv", "0", 0o755)
	x.w.Dir("/srv/both", x.w.UID(alex), 0o755).ID = "dev:77"
	workdir(x, alex, "Both", "/srv/both")
	workdir(x, sam, "Both", "/srv/both")
	workdir(x, sam, "Own", "/home/"+sam+"/own")
	r := x.run()
	if len(r.Workdirs[sam]) != 1 || r.Workdirs[sam][0].DisplayName != "Own" || len(r.Workdirs[alex]) != 0 {
		t.Fatalf("lists after HD7 %+v", r.Workdirs)
	}
	for _, e := range r.Check(HD11).Entries {
		t.Errorf("a dropped folder blocks: %+v", e)
	}
	// The kept folder of Sam made writable by Alex fails HD11 for Sam only.
	x.w.Dir("/home/"+sam+"/own", x.w.UID(sam), 0o777)
	r = x.run()
	if got := r.Check(HD11).Entries; len(got) != 1 || got[0].SessionUser != sam || got[0].Code != CodeSharedLocation {
		t.Errorf("kept folder: %+v", got)
	}
}
