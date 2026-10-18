// SPDX-License-Identifier: Apache-2.0

package authorize_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// TestWithoutRequestScope: a context derived from a request drops the request's memo, and a later
// WithRequestScope installs a fresh one instead of keeping the request's (M1-65).
func TestWithoutRequestScope(t *testing.T) {
	t.Parallel()
	req := authorize.WithRequestScope(context.Background())
	reqMemo, _ := authorize.RequestMemoFrom(req)
	conn := authorize.WithoutRequestScope(req)
	if m, ok := authorize.RequestMemoFrom(conn); ok || m != nil {
		t.Fatalf("memo after WithoutRequestScope: %p %v", m, ok)
	}
	page, ok := authorize.RequestMemoFrom(authorize.WithRequestScope(conn))
	if !ok || page == nil || page == reqMemo {
		t.Fatalf("WithRequestScope after WithoutRequestScope = %p %v, request memo %p", page, ok, reqMemo)
	}
}
