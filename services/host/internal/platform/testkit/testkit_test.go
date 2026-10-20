// SPDX-License-Identifier: Apache-2.0

package testkit

import (
	"testing"
	"time"
)

func TestDeadlineIsOnePatienceAhead(t *testing.T) {
	before := time.Now()
	d := Deadline()
	if d.Before(before.Add(Patience)) || d.After(time.Now().Add(Patience)) {
		t.Errorf("deadline %v is not one patience after %v", d, before)
	}
}

func TestContextEndsAtTheDeadline(t *testing.T) {
	ctx := Context(t)
	d, ok := ctx.Deadline()
	if !ok || time.Until(d) > Patience {
		t.Errorf("deadline %v %v", d, ok)
	}
	if ctx.Err() != nil {
		t.Error("the context ended at once")
	}
}
