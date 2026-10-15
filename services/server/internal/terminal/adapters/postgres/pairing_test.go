// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
	"github.com/OWNER/bothy/services/server/test/terminaltest"
)

// TestRepository_Pairing (M2-41): a host is found by its pairing code hash, stores a pairing in
// confirming, counts failures without a new version, is confirmed, falls back into pairing with
// its keys dropped and its fingerprint kept, and a login key of another host is 409
// host_key_in_use. Writes outside a unit of work fail.
func TestRepository_Pairing(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	robin := e.person(t, "robin")
	id, err := terminaltest.InsertHost(ctx, e.owner, terminaltest.Host{Name: "Build server", State: "pairing", CreatedBy: robin})
	if err != nil {
		t.Fatal(err)
	}
	other, err := terminaltest.InsertHost(ctx, e.owner, terminaltest.Host{Name: "Other", State: "pairing", CreatedBy: robin})
	if err != nil {
		t.Fatal(err)
	}
	h, _, _ := e.repo.Host(ctx, id)
	if got, found, err := e.repo.HostByPairingHash(ctx, h.PairingCodeHash); err != nil || !found || got.ID != id {
		t.Fatalf("by hash: %v %v", found, err)
	}
	if _, found, _ := e.repo.HostByPairingHash(ctx, []byte("unknown")); found {
		t.Fatal("an unknown hash found a host")
	}
	noise := make([]byte, 32)
	noise[0] = 9
	keys := domain.HostKeys{Noise: noise, AuthRaw: make([]byte, 32), JWK: []byte(`{"crv":"Ed25519","kty":"OKP","x":"AAAA"}`), JKT: "jkt-pairing"}
	w := app.PairingWrite{Keys: keys, Fingerprint: "F0CC-6QS6", PairedAt: time.Now(), Report: domain.PairReport{
		Platform: "linux", HostVersion: "1.0.0", SessionUsers: []domain.ReportedSessionUser{{OSUser: "bothy-s-robin"}},
	}}
	if err := e.repo.StorePairing(ctx, id, w, 2); err == nil {
		t.Fatal("a write outside a unit of work")
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(e.write(func(ctx context.Context) error { return e.repo.StorePairing(ctx, id, w, 2) }))
	h, _, _ = e.repo.Host(ctx, id)
	if h.State != domain.StateConfirming || h.KeyGeneration != 1 || h.AuthJKT != "jkt-pairing" || len(h.PairingCodeHash) != 0 ||
		len(h.ReportedUsers) != 1 || h.ReportedUsers[0].Workdirs == nil || len(h.Profiles) != 0 {
		t.Fatalf("stored pairing %+v", h)
	}
	err = e.write(func(ctx context.Context) error { return e.repo.StorePairing(ctx, other, w, 2) })
	if ee, ok := errs.As(err); !ok || ee.Reason != domain.ReasonHostKeyInUse {
		t.Fatalf("key of another host: %v", err)
	}
	must(e.write(func(ctx context.Context) error { return e.repo.SetFingerprintFailures(ctx, id, 2) }))
	if h, _, _ = e.repo.Host(ctx, id); h.FingerprintFailures != 2 || h.Version != 2 {
		t.Fatalf("failures %d, version %d", h.FingerprintFailures, h.Version)
	}
	must(e.write(func(ctx context.Context) error {
		return e.repo.SetPairingCode(ctx, id, []byte("fresh hash"), time.Now(), true, 3)
	}))
	if h, _, _ = e.repo.Host(ctx, id); h.State != domain.StatePairing || h.HasKeys() || h.AuthJKT != "" || h.KeyFingerprint != "F0CC-6QS6" || h.FingerprintFailures != 0 {
		t.Fatalf("fallback %+v", h)
	}
	if err := e.write(func(ctx context.Context) error {
		return e.repo.ConfirmPairing(ctx, ids.New(), true, robin, time.Now(), 4)
	}); err == nil {
		t.Fatal("confirming an unknown host changed a row")
	}
	must(e.write(func(ctx context.Context) error { return e.repo.StorePairing(ctx, id, w, 4) }))
	must(e.write(func(ctx context.Context) error { return e.repo.ConfirmPairing(ctx, id, true, robin, time.Now(), 5) }))
	if h, _, _ = e.repo.Host(ctx, id); h.State != domain.StateActive || !h.DeclaredServerHost || h.Version != 5 || h.KeyGeneration != 2 {
		t.Fatalf("confirmed %+v", h)
	}
}
