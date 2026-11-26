// SPDX-License-Identifier: Apache-2.0

package localchan

import "testing"

func TestWithCreator(t *testing.T) {
	if got := WithCreator("D:P(A;;GA;;;SY)", "S-1-5-21-1-2-3-1001"); got != "D:P(A;;GA;;;SY)(A;;0x4;;;S-1-5-21-1-2-3-1001)" {
		t.Errorf("WithCreator: %s", got)
	}
	if got := WithCreator("D:P(A;;GA;;;SY)", ""); got != "D:P(A;;GA;;;SY)" {
		t.Errorf("without an account: %s", got)
	}
}
