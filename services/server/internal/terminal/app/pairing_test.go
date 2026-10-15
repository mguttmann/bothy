// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/apitoken"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/terminal"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// countingStore is a Store whose every call counts; the embedded nil interface panics for a
// method this test does not expect.
type countingStore struct {
	app.Store
	calls int
}

func (s *countingStore) HostByPairingHash(context.Context, []byte) (domain.Host, bool, error) {
	s.calls++
	return domain.Host{}, false, nil
}

// countingUoW counts Do and never runs fn.
type countingUoW struct{ calls int }

func (u *countingUoW) Do(context.Context, func(context.Context) error, ...txn.Option) error {
	u.calls++
	return nil
}

// countingAudit counts the denials.
type countingAudit struct {
	auditing.Recorder
	denials []auditing.Entry
}

func (a *countingAudit) RecordDenial(_ context.Context, e auditing.Entry) error {
	a.denials = append(a.denials, e)
	return nil
}

// realCodes hashes like identity: apitoken.Parse first, ErrTokenMalformed for anything but hpc.
type realCodes struct{}

func (realCodes) Issue(context.Context) (identity.PairingCode, error) {
	return identity.PairingCode{}, nil
}

func (realCodes) Hash(_ context.Context, raw string) ([]byte, error) {
	p, err := apitoken.Parse(raw)
	if err != nil || p.Kind != apitoken.KindHostPairingCode {
		return nil, identity.ErrTokenMalformed
	}
	return apitoken.Hash([]byte("pepper"), raw), nil
}

type nopInstance struct{}

func (nopInstance) InstanceID(context.Context) (ids.ID, error) { return ids.New(), nil }

type nopPeople struct{ app.People }

type nopAuthorizer struct{ authorize.Authorizer }

type nopEvents struct{ eventlog.Appender }

type nopCursors struct{ app.Cursors }

// TestTMT20_WrongChecksumNoDatabase (TM-T20, "wrong checksum without database access"): a code with
// one changed checksum character and a code of kind agt give 401 unauthenticated with zero units of
// work and zero store calls, and exactly one denial with reason unauthenticated each.
func TestTMT20_WrongChecksumNoDatabase(t *testing.T) {
	t.Parallel()
	store, uow, audit := &countingStore{}, &countingUoW{}, &countingAudit{}
	cfg := app.Config{NameMaxChars: 10, OSUserMaxChars: 10, ReasonMaxChars: 10, PairingCodeTTL: time.Minute, PageSizeDefault: 1, PageSizeMax: 1,
		FingerprintConfirmChars: 8, FingerprintMaxAttempts: 3, ReportLabelMaxChars: 10, ReportMaxEntries: 4}
	svc, err := app.New(cfg, app.Deps{Store: store, PairingCodes: realCodes{}, Instance: nopInstance{}, People: nopPeople{}, UoW: uow,
		Authorizer: nopAuthorizer{}, Audit: audit, Events: nopEvents{}, Cursors: nopCursors{}})
	if err != nil {
		t.Fatal(err)
	}
	good, err := apitoken.Generate(nil, apitoken.KindHostPairingCode)
	if err != nil {
		t.Fatal(err)
	}
	other := byte('A')
	if good[len(good)-1] == other {
		other = 'B'
	}
	agent, err := apitoken.Generate(nil, apitoken.KindAgent)
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{good[:len(good)-1] + string(other), agent} {
		_, err := svc.Pair(context.Background(), terminal.PairHost{Code: terminal.NewSecret(code)})
		if e, ok := errs.As(err); !ok || e.Code != errs.CodeUnauthenticated {
			t.Fatalf("Pair: %v, want 401 unauthenticated", err)
		}
	}
	if uow.calls != 0 || store.calls != 0 {
		t.Fatalf("%d units of work and %d store calls, want none", uow.calls, store.calls)
	}
	if len(audit.denials) != 2 {
		t.Fatalf("%d denials, want 2", len(audit.denials))
	}
	for _, d := range audit.denials {
		if d.Action != "host.pair" || d.Reason != auditing.ReasonUnauthenticated || !d.ResourceID.IsZero() {
			t.Fatalf("denial %+v", d)
		}
	}
}
