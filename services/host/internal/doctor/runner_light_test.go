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
