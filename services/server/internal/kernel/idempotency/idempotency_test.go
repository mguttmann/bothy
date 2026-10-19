// SPDX-License-Identifier: Apache-2.0

package idempotency_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/idempotency"
)

// childKey is the key of a value that derives a child context.
type childKey struct{}

func TestOneTimeSecret_MarkedOnlyInsideAScope(t *testing.T) {
	ctx := context.Background()
	idempotency.MarkOneTimeSecret(ctx) // no scope: nothing to mark, no panic
	if idempotency.OneTimeSecretMarked(ctx) {
		t.Fatal("marked without a scope")
	}
	scoped := idempotency.WithSecretScope(ctx)
	if idempotency.OneTimeSecretMarked(scoped) {
		t.Fatal("a fresh scope is marked")
	}
	child := context.WithValue(scoped, childKey{}, 1)
	idempotency.MarkOneTimeSecret(child)
	if !idempotency.OneTimeSecretMarked(scoped) {
		t.Error("a mark in a derived context is not seen by the scope")
	}
	if idempotency.OneTimeSecretMarked(idempotency.WithSecretScope(scoped)) {
		t.Error("a new scope inherits the mark of the outer one")
	}
}

// TestKeepEffectOnFinal_MarkedOnlyInsideAScope (M2-41): the mark works like MarkOneTimeSecret and
// is independent of it.
func TestKeepEffectOnFinal_MarkedOnlyInsideAScope(t *testing.T) {
	ctx := context.Background()
	idempotency.KeepEffectOnFinal(ctx)
	if idempotency.EffectKept(ctx) {
		t.Fatal("kept without a scope")
	}
	scoped := idempotency.WithSecretScope(ctx)
	idempotency.KeepEffectOnFinal(context.WithValue(scoped, childKey{}, 1))
	if !idempotency.EffectKept(scoped) || idempotency.OneTimeSecretMarked(scoped) {
		t.Fatal("the mark is not seen by its scope, or it marked a secret")
	}
}
