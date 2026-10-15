// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// rotateWebhookOut announces a next webhook_out key and promotes it after agent.jwks_max_age; it
// returns the rotation (previous key retiring, new key active).
func rotateWebhookOut(t *testing.T, e *keysEnv, k interface {
	Rotate(context.Context, actor.Actor, instance.RotateCommand) (instance.Rotation, error)
	RunRotation(context.Context) (int, error)
}) instance.Rotation {
	t.Helper()
	ctx := context.Background()
	r, err := k.Rotate(ctx, actor.Actor{ID: ids.New(), Kind: actor.KindUser},
		instance.RotateCommand{Purpose: instance.PurposeWebhookOut, Mode: instance.ModeScheduled, Reason: "Test"})
	if err != nil {
		t.Fatal(err)
	}
	e.clk.Advance(keyCfg.JWKSMaxAge)
	if _, err := k.RunRotation(ctx); err != nil {
		t.Fatal(err)
	}
	return r
}

// kids returns the kids of signers.
func kids(ss []instance.Signer) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		out = append(out, s.KeyID())
	}
	return out
}

// TestSignersOrder: the signing set is the active key first, then retiring keys whose not_after
// lies ahead; a retiring key past not_after, a next key and a retired key never sign (webhooks.md
// section 2.1 point 2).
func TestSignersOrder(t *testing.T) {
	t.Parallel()
	e := newKeysEnv(t)
	k := e.keys(t)
	ctx := context.Background()
	if err := k.EnsureActive(ctx); err != nil {
		t.Fatal(err)
	}
	first, err := k.Signer(ctx, instance.PurposeWebhookOut)
	if err != nil {
		t.Fatal(err)
	}
	ss, err := k.Signers(ctx, instance.PurposeWebhookOut)
	if err != nil || !slices.Equal(kids(ss), []string{first.KeyID()}) {
		t.Fatalf("signers of a fresh purpose %v %v", kids(ss), err)
	}
	r := rotateWebhookOut(t, e, k)
	// The promotion sets not_after to its own time: the retiring key no longer signs.
	ss, err = k.Signers(ctx, instance.PurposeWebhookOut)
	if err != nil || !slices.Equal(kids(ss), []string{r.NewKID}) {
		t.Fatalf("signers after the promotion %v %v, want only %s", kids(ss), err, r.NewKID)
	}
	// A retiring key with not_after ahead signs after the active key.
	if _, err := e.super.Exec(ctx, `UPDATE server_signing_keys SET not_after = $1 WHERE kid = $2`, e.clk.Now().Add(time.Hour), r.PreviousKID); err != nil {
		t.Fatal(err)
	}
	// A next key is published but never signs.
	if _, err := k.Rotate(ctx, actor.Actor{ID: ids.New(), Kind: actor.KindUser},
		instance.RotateCommand{Purpose: instance.PurposeWebhookOut, Mode: instance.ModeScheduled, Reason: "Test"}); err != nil {
		t.Fatal(err)
	}
	ss, err = k.Signers(ctx, instance.PurposeWebhookOut)
	if err != nil || !slices.Equal(kids(ss), []string{r.NewKID, r.PreviousKID}) {
		t.Fatalf("signers with a retiring key ahead %v %v", kids(ss), err)
	}
	e.clk.Advance(time.Hour)
	ss, err = k.Signers(ctx, instance.PurposeWebhookOut)
	if err != nil || !slices.Equal(kids(ss), []string{r.NewKID}) {
		t.Fatalf("signers after not_after %v %v", kids(ss), err)
	}
	// Each signer signs with its own key.
	for _, s := range ss {
		if _, err := s.Sign(ctx, []byte("content")); err != nil {
			t.Fatal(err)
		}
	}
}

// TestPublicKeysWebhookOut: the published webhook_out keys are next, active and retiring, ordered
// by state and kid; retired keys and keys of other purposes are absent.
func TestPublicKeysWebhookOut(t *testing.T) {
	t.Parallel()
	e := newKeysEnv(t)
	k := e.keys(t)
	ctx := context.Background()
	if err := k.EnsureActive(ctx); err != nil {
		t.Fatal(err)
	}
	r := rotateWebhookOut(t, e, k)
	next, err := k.Rotate(ctx, actor.Actor{ID: ids.New(), Kind: actor.KindUser},
		instance.RotateCommand{Purpose: instance.PurposeWebhookOut, Mode: instance.ModeScheduled, Reason: "Test"})
	if err != nil {
		t.Fatal(err)
	}
	keys, err := k.PublicKeys(ctx, instance.PurposeWebhookOut)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, pk := range keys {
		if pk.Purpose != instance.PurposeWebhookOut || len(pk.Key) == 0 {
			t.Fatalf("key %+v", pk)
		}
		got = append(got, string(pk.State)+":"+pk.KID)
	}
	want := []string{"active:" + r.NewKID, "next:" + next.NewKID, "retiring:" + r.PreviousKID}
	if !slices.Equal(got, want) {
		t.Fatalf("published keys %v, want %v", got, want)
	}
	if keys[2].NotAfter.IsZero() || !keys[0].NotAfter.IsZero() {
		t.Fatalf("not_after only on the retiring key: %+v", keys)
	}
	// The emergency rotation retires every key; retired keys are not published.
	if _, err := k.Rotate(ctx, actor.Actor{ID: ids.New(), Kind: actor.KindUser},
		instance.RotateCommand{Purpose: instance.PurposeWebhookOut, Mode: instance.ModeEmergency, Reason: "Test"}); err != nil {
		t.Fatal(err)
	}
	keys, err = k.PublicKeys(ctx, instance.PurposeWebhookOut)
	if err != nil || len(keys) != 1 || keys[0].State != instance.KeyActive {
		t.Fatalf("after the emergency rotation %+v %v", keys, err)
	}
	if _, err := k.PublicKeys(ctx, "nope"); err == nil {
		t.Fatal("unknown purpose accepted")
	}
	set, err := k.JWKS(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, j := range set.Keys {
		if j.Kid == keys[0].KID {
			t.Fatal("the JWKS lists a webhook_out key")
		}
	}
}
