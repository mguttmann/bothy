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

// TestWithStructureScope: each call gives a fresh structure memo, independent of the request memo.
func TestWithStructureScope(t *testing.T) {
	t.Parallel()
	if _, ok := authorize.StructureMemoFrom(context.Background()); ok {
		t.Fatal("a structure memo without WithStructureScope")
	}
	first := authorize.WithStructureScope(context.Background())
	a, ok := authorize.StructureMemoFrom(first)
	if !ok || a == nil {
		t.Fatal("no structure memo after WithStructureScope")
	}
	b, _ := authorize.StructureMemoFrom(authorize.WithStructureScope(first))
	if b == a {
		t.Fatal("a second scope reused the memo of the first")
	}
	if _, ok := authorize.RequestMemoFrom(first); ok {
		t.Fatal("WithStructureScope installed a request memo")
	}
}

// TestRecheck_ReadsCurrentVersion (M2-44, Design note 12): the recheck after the commit drops the
// request memo, so it reads the rights version a revocation raised in between instead of the one
// the request's first check memoised; it is a filter check and marked as a recheck.
func TestRecheck_ReadsCurrentVersion(t *testing.T) {
	t.Parallel()
	ctx := authorize.Recheck(authorize.WithRequestScope(context.Background()))
	if m, ok := authorize.RequestMemoFrom(ctx); ok || m != nil {
		t.Fatalf("memo under Recheck: %p %v", m, ok)
	}
	if !authorize.IsFilter(ctx) || !authorize.IsRecheck(ctx) {
		t.Fatal("Recheck is no filter recheck")
	}
}
