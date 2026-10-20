// SPDX-License-Identifier: Apache-2.0

package clock

import (
	"testing"
	"time"
)

func TestReal(t *testing.T) {
	c := Real()
	if c.Now().IsZero() {
		t.Fatal("Now is zero")
	}
	tm := c.NewTimer(time.Millisecond)
	<-tm.C()
	if tm.Stop() {
		t.Error("Stop after firing reported a pending timer")
	}
}
