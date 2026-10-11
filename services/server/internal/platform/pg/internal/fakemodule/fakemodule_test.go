// SPDX-License-Identifier: Apache-2.0

package fakemodule

import (
	"context"
	"errors"
	"testing"
)

// TestWrappersCallTheTestFunction: Hook and Handler pass context, event and error through.
func TestWrappersCallTheTestFunction(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	if err := Hook(func(context.Context) error { return boom })(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("Hook = %v", err)
	}
	var got Event
	h := Handler(func(_ context.Context, ev Event) error { got = ev; return nil })
	if err := h(context.Background(), Event{N: 7}); err != nil || got.N != 7 {
		t.Fatalf("Handler = %v, event %+v", err, got)
	}
}
