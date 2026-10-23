// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
)

var (
	alex = ids.MustParse("01928f3a-7b1c-7000-8a00-0000000000a1")
	sam  = ids.MustParse("01928f3a-7b1c-7000-8a00-0000000000a2")
)

// as returns a context of an authenticated person.
func as(principal ids.ID) context.Context {
	return actor.WithActor(context.Background(), actor.Actor{
		ID: principal, Kind: actor.KindUser,
		DeviceID: ids.MustParse("01928f3a-7b1c-7000-8a00-0000000000d1"), SessionID: ids.MustParse("01928f3a-7b1c-7000-8a00-0000000000e1"),
	})
}

// anonymousFrom returns the context of an anonymous request from ip.
func anonymousFrom(ip string) context.Context {
	return reqmeta.WithClientIP(actor.WithActor(context.Background(), actor.Anonymous()), netip.MustParseAddr(ip))
}

func deny(action string, reason auditing.Reason) auditing.Entry {
	return auditing.Entry{Action: action, Decision: auditing.DecisionDeny, Reason: reason}
}

// aggregate returns details.aggregate of r.
func aggregate(t *testing.T, r domain.Row) map[string]any {
	t.Helper()
	m, ok := r.Details[domain.DetailAggregate].(map[string]any)
	if !ok {
		t.Fatalf("row %d has no aggregate: %v", r.Seq, r.Details)
	}
	return m
}

func TestRecord_WritesInTheUnitWithActorAndCorrelation(t *testing.T) {
	t.Parallel()
	w := newWorld(t, time.Minute, 10)
	ctx := reqmeta.WithCorrelationID(as(alex), "4bf92f35-77b3-4da6-a3ce-929d0e0e4736")
	w.clock.Set(t0.Add(1234567 * time.Microsecond))
	res := ids.New()
	err := w.uow.Do(ctx, func(ctx context.Context) error {
		return w.rec.Record(ctx, auditing.Entry{
			Action: "grant.create", Decision: auditing.DecisionAllow, ResourceKind: "grant", ResourceID: res,
			Attrs: auditing.Attrs{"level": auditing.Code("read"), "reason": auditing.ReasonText("handover to Sam")},
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	rows := w.store.committed()
	if len(rows) != 1 {
		t.Fatalf("rows = %d", len(rows))
	}
	r := rows[0]
	if r.Seq != 1 || r.PrevHash != domain.ZeroHash || r.ActorPrincipalID != alex || r.ActorDeviceID.IsZero() ||
		r.ActorSessionID.IsZero() || r.CorrelationID.String() != "4bf92f35-77b3-4da6-a3ce-929d0e0e4736" ||
		r.ResourceID != res || r.ResourceKind != "grant" || r.DenyReason != "" || r.Decision != "allow" {
		t.Fatalf("row = %+v", r)
	}
	if !r.OccurredAt.Equal(t0.Add(1234 * time.Millisecond)) {
		t.Fatalf("occurred_at %v is not cut to milliseconds", r.OccurredAt)
	}
	if r.Details["level"] != "read" || r.Details["reason"] != "handover to Sam" {
		t.Fatalf("details = %v", r.Details)
	}
	c, err := domain.CanonicalRow(r)
	if err != nil || domain.ChainHash(r.PrevHash, c) != r.Hash {
		t.Fatalf("the hash does not recompute: %v", err)
	}
}

func TestRecord_ChainsAcrossUnits(t *testing.T) {
	t.Parallel()
	w := newWorld(t, time.Minute, 10)
	for range 3 {
		err := w.uow.Do(as(alex), func(ctx context.Context) error {
			if err := w.rec.Record(ctx, auditing.Entry{Action: "a.b", Decision: auditing.DecisionAllow}); err != nil {
				return err
			}
			return w.rec.Record(ctx, auditing.Entry{Action: "a.c", Decision: auditing.DecisionAllow})
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	rows := w.store.committed()
	prev := domain.ZeroHash
	for i, r := range rows {
		if r.Seq != int64(i+1) || r.PrevHash != prev {
			t.Fatalf("row %d: seq %d, prev_hash does not chain", i, r.Seq)
		}
		prev = r.Hash
	}
	if len(rows) != 6 {
		t.Fatalf("rows = %d", len(rows))
	}
}

func TestRecord_OutsideUnitFails(t *testing.T) {
	t.Parallel()
	w := newWorld(t, time.Minute, 10)
	err := w.rec.Record(as(alex), auditing.Entry{Action: "a.b", Decision: auditing.DecisionAllow})
	if errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("Record outside a unit = %v, want kind internal", err)
	}
}

func TestRecord_NoEntriesNoCounter(t *testing.T) {
	t.Parallel()
	w := newWorld(t, time.Minute, 10)
	if err := w.uow.Do(as(alex), func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if w.counters.calls != 0 {
		t.Fatal("a unit without entries advanced audit_seq")
	}
}

func TestRecord_ChainBrokenFailsTheUnit(t *testing.T) {
	t.Parallel()
	w := newWorld(t, time.Minute, 10)
	w.counters.value = 5 // rows 1 .. 5 are missing
	err := w.uow.Do(as(alex), func(ctx context.Context) error {
		return w.rec.Record(ctx, auditing.Entry{Action: "a.b", Decision: auditing.DecisionAllow})
	})
	if errs.KindOf(err) != errs.KindInternal || !strings.Contains(err.Error(), "audit chain broken") || !w.rep.has("chain_broken") {
		t.Fatalf("Do = %v, reports %v; want internal audit chain broken, reported", err, w.rep.events)
	}
	if len(w.store.committed()) != 0 {
		t.Fatal("a row was committed after the chain broke")
	}
}

func TestRecord_RejectsInvalidEntries(t *testing.T) {
	t.Parallel()
	w := newWorld(t, time.Minute, 10)
	cases := map[string]auditing.Entry{
		"empty action":          {Decision: auditing.DecisionAllow},
		"action with space":     {Action: "grant create", Decision: auditing.DecisionAllow},
		"unknown decision":      {Action: "a.b", Decision: "maybe"},
		"allow with reason":     {Action: "a.b", Decision: auditing.DecisionAllow, Reason: auditing.ReasonNoGrant},
		"deny without reason":   {Action: "a.b", Decision: auditing.DecisionDeny},
		"reason not a code":     {Action: "a.b", Decision: auditing.DecisionDeny, Reason: "No Grant"},
		"resource kind text":    {Action: "a.b", Decision: auditing.DecisionAllow, ResourceKind: "a grant"},
		"reserved ip_hmac":      {Action: "a.b", Decision: auditing.DecisionAllow, Attrs: auditing.Attrs{"ip_hmac": auditing.Code("ab")}},
		"reserved ip_key_id":    {Action: "a.b", Decision: auditing.DecisionAllow, Attrs: auditing.Attrs{"ip_key_id": auditing.ID(alex)}},
		"reserved aggregate":    {Action: "a.b", Decision: auditing.DecisionAllow, Attrs: auditing.Attrs{"aggregate": auditing.Int(1)}},
		"reason as code":        {Action: "a.b", Decision: auditing.DecisionAllow, Attrs: auditing.Attrs{"reason": auditing.Code("x")}},
		"reason text elsewhere": {Action: "a.b", Decision: auditing.DecisionAllow, Attrs: auditing.Attrs{"note": auditing.ReasonText("x")}},
		"violations elsewhere":  {Action: "a.b", Decision: auditing.DecisionAllow, Attrs: auditing.Attrs{"v": auditing.Violations()}},
		"violations as code":    {Action: "a.b", Decision: auditing.DecisionAllow, Attrs: auditing.Attrs{"violations": auditing.Code("x")}},
		"free text code":        {Action: "a.b", Decision: auditing.DecisionAllow, Attrs: auditing.Attrs{"note": auditing.Code("hello world")}},
		"e-mail code":           {Action: "a.b", Decision: auditing.DecisionAllow, Attrs: auditing.Attrs{"mail": auditing.Code("sam@example.com")}},
		"bad key":               {Action: "a.b", Decision: auditing.DecisionAllow, Attrs: auditing.Attrs{"Level": auditing.Code("x")}},
		"zero value":            {Action: "a.b", Decision: auditing.DecisionAllow, Attrs: auditing.Attrs{"x": {}}},
		"mixed list":            {Action: "a.b", Decision: auditing.DecisionAllow, Attrs: auditing.Attrs{"x": auditing.List(auditing.Int(1), auditing.Bool(true))}},
		"nested list":           {Action: "a.b", Decision: auditing.DecisionAllow, Attrs: auditing.Attrs{"x": auditing.List(auditing.List())}},
		"int beyond 2^53":       {Action: "a.b", Decision: auditing.DecisionAllow, Attrs: auditing.Attrs{"x": auditing.Int(1 << 53)}},
		"violation zero agent":  {Action: "a.b", Decision: auditing.DecisionDeny, Reason: auditing.ReasonI1Violation, Attrs: auditing.Attrs{"violations": auditing.Violations(auditing.Violation{Code: "i1_violation", CredentialHolderID: sam})}},
		"reason with newline":   {Action: "a.b", Decision: auditing.DecisionAllow, Attrs: auditing.Attrs{"reason": auditing.ReasonText("a\nb")}},
	}
	for name, e := range cases {
		err := w.uow.Do(as(alex), func(ctx context.Context) error { return w.rec.Record(ctx, e) })
		if errs.KindOf(err) != errs.KindInternal {
			t.Errorf("%s: Record = %v, want kind internal", name, err)
		}
		if err := w.rec.RecordDetached(as(alex), e); errs.KindOf(err) != errs.KindInternal {
			t.Errorf("%s: RecordDetached = %v, want kind internal", name, err)
		}
	}
	if n := len(w.store.committed()); n != 0 {
		t.Fatalf("%d invalid entries were written", n)
	}
	if err := w.rec.RecordDenial(as(alex), auditing.Entry{Action: "a.b", Decision: auditing.DecisionAllow}); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("RecordDenial of an allow = %v, want kind internal", err)
	}
}

func TestRecord_AcceptsViolationsAndEveryKind(t *testing.T) {
	t.Parallel()
	w := newWorld(t, time.Minute, 10)
	e := auditing.Entry{
		Action: "grant.create", Decision: auditing.DecisionDeny, Reason: auditing.ReasonI1Violation,
		Attrs: auditing.Attrs{
			"violations": auditing.Violations(auditing.Violation{Code: "i1_violation", AgentID: alex, CredentialHolderID: sam, PrincipalIDs: []ids.ID{sam}}),
			"subject":    auditing.ID(sam), "level": auditing.Code("trigger"), "n": auditing.Int(-2),
			"inherit": auditing.Bool(false), "until": auditing.Time(t0.Add(time.Microsecond)),
			"targets": auditing.List(auditing.ID(alex), auditing.ID(sam)), "none": auditing.List(),
		},
	}
	if err := w.rec.RecordDetached(as(alex), e); err != nil {
		t.Fatal(err)
	}
	d := w.store.committed()[0].Details
	v := d["violations"].([]any)[0].(map[string]any)
	if v["agent_id"] != alex.String() || v["principal_ids"].([]any)[0] != sam.String() || d["until"] != "2026-10-12T08:00:00.000Z" ||
		d["n"] != int64(-2) || d["inherit"] != false || len(d["none"].([]any)) != 0 {
		t.Fatalf("details = %v", d)
	}
}

func TestRecord_UnknownReasonWrittenAndReported(t *testing.T) {
	t.Parallel()
	w := newWorld(t, time.Minute, 10)
	if err := w.rec.RecordDetached(as(alex), deny("a.b", "brand_new_reason")); err != nil {
		t.Fatal(err)
	}
	if rows := w.store.committed(); len(rows) != 1 || rows[0].DenyReason != "brand_new_reason" {
		t.Fatalf("rows = %v", rows)
	}
	if !w.rep.has("unknown_reason brand_new_reason") {
		t.Fatalf("no report: %v", w.rep.events)
	}
}

func TestRecordDetached_DoesNotJoinTheAmbientUnit(t *testing.T) {
	t.Parallel()
	w := newWorld(t, time.Minute, 10)
	errBusiness := errors.New("business rollback")
	err := w.uow.Do(as(alex), func(ctx context.Context) error {
		if err := w.rec.RecordDetached(ctx, auditing.Entry{Action: "a.b", Decision: auditing.DecisionError, Reason: auditing.ReasonUnknownAction}); err != nil {
			return err
		}
		return errBusiness
	})
	if !errors.Is(err, errBusiness) {
		t.Fatal(err)
	}
	if rows := w.store.committed(); len(rows) != 1 || rows[0].Decision != "error" {
		t.Fatalf("rows = %v; the detached entry must survive the rollback of the ambient unit", rows)
	}
}

func TestAudit_IPPseudonymOnly(t *testing.T) {
	t.Parallel()
	w := newWorld(t, time.Minute, 10)
	if err := w.rec.RecordDetached(anonymousFrom("192.0.2.44"), deny("auth.login.begin", auditing.ReasonUnauthenticated)); err != nil {
		t.Fatal(err)
	}
	r := w.store.committed()[0]
	keyID, _ := r.Details[domain.DetailIPKeyID].(string)
	hmacHex, _ := r.Details[domain.DetailIPHMAC].(string)
	if len(hmacHex) != 64 || len(w.store.keys) != 1 || keyID != w.store.keys[0].ID.String() {
		t.Fatalf("details = %v, keys = %v", r.Details, w.store.keys)
	}
	_, key, err := w.keyring.Current(context.Background(), t0)
	if err != nil {
		t.Fatal(err)
	}
	if hmacHex != domain.IPHMAC(key, netip.MustParseAddr("192.0.2.44")) {
		t.Fatal("ip_hmac is not the HMAC of the client IP under the current key")
	}
	c, _ := domain.CanonicalRow(r)
	if strings.Contains(string(c), "192.0.2.44") || !r.ActorPrincipalID.IsZero() {
		t.Fatalf("raw IP or an actor in the row: %s", c)
	}
}

func TestIPKey_EncryptedAtRest(t *testing.T) {
	t.Parallel()
	w := newWorld(t, time.Minute, 10)
	id, key, err := w.keyring.Current(context.Background(), t0)
	if err != nil {
		t.Fatal(err)
	}
	stored := w.store.keys[0]
	if stored.ID != id || stored.KeyRef != kekRef || len(stored.Ciphertext) == 0 || equalBytes(stored.Ciphertext, key) ||
		strings.Contains(string(stored.Ciphertext), string(key)) {
		t.Fatalf("stored key = %+v", stored)
	}
	kek, err := domain.DecodeKEK(kekHex)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := domain.OpenIPKey(kek, stored.Ciphertext, stored.ID)
	if err != nil || !equalBytes(opened, key) {
		t.Fatalf("the configured KEK does not open the key: %v", err)
	}
	if _, err := domain.OpenIPKey(make([]byte, 32), stored.Ciphertext, stored.ID); err == nil {
		t.Fatal("another KEK opened the key")
	}
	from, until := domain.KeyPeriod(t0, 30*24*time.Hour)
	if !stored.ValidFrom.Equal(from) || !stored.ValidUntil.Equal(until) {
		t.Fatalf("period %v .. %v, want %v .. %v", stored.ValidFrom, stored.ValidUntil, from, until)
	}
}

func TestKeyring_CachesUntilValidUntilAndRotates(t *testing.T) {
	t.Parallel()
	w := newWorld(t, time.Minute, 10)
	id1, _, err := w.keyring.Current(context.Background(), t0)
	if err != nil {
		t.Fatal(err)
	}
	w.store.mu.Lock()
	w.store.keys[0].KeyRef = "file:/elsewhere" // a cache hit never reads the row again
	w.store.mu.Unlock()
	if id, _, err := w.keyring.Current(context.Background(), t0.Add(time.Hour)); err != nil || id != id1 {
		t.Fatalf("second call = %v, %v; want the cached key", id, err)
	}
	_, until := domain.KeyPeriod(t0, 30*24*time.Hour)
	id2, _, err := w.keyring.Current(context.Background(), until)
	if err != nil || id2 == id1 || len(w.store.keys) != 2 {
		t.Fatalf("new period = %v, %v, keys %d", id2, err, len(w.store.keys))
	}
}

func TestKeyring_DestroyedKeyNeverUsed(t *testing.T) {
	t.Parallel()
	w := newWorld(t, time.Minute, 10)
	from, until := domain.KeyPeriod(t0, 30*24*time.Hour)
	w.store.keys = append(w.store.keys, domain.IPKey{ID: ids.New(), ValidFrom: from, ValidUntil: until, KeyRef: kekRef})
	if _, _, err := w.keyring.Current(context.Background(), t0); err == nil {
		t.Fatal("a destroyed key was used")
	}
	err := w.rec.RecordDetached(anonymousFrom("192.0.2.44"), deny("a.b", auditing.ReasonUnauthenticated))
	if errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("RecordDetached with a destroyed key = %v, want kind internal", err)
	}
}

func TestRecord_ConcurrentInOneUnitKeepsEveryEntry(t *testing.T) {
	t.Parallel()
	w := newWorld(t, time.Minute, 10)
	const n = 64
	err := w.uow.Do(as(alex), func(ctx context.Context) error {
		var wg sync.WaitGroup
		errc := make(chan error, n)
		for i := range n {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errc <- w.rec.Record(ctx, auditing.Entry{Action: "a.b", Decision: auditing.DecisionAllow, Attrs: auditing.Attrs{"i": auditing.Int(int64(i))}})
			}()
		}
		wg.Wait()
		close(errc)
		for err := range errc {
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(w.store.committed()); got != n {
		t.Fatalf("%d of %d concurrent entries were written", got, n)
	}
}

// TestDetails_OSUserName (M2-39, 19 SEC-AUD-5): the name of a session user is accepted only under
// the reserved member os_user and only in the form of host_session_users.os_user; a value outside
// the pattern, the value under another key and another kind under os_user are refused at Record.
func TestDetails_OSUserName(t *testing.T) {
	t.Parallel()
	w := newWorld(t, time.Minute, 10)
	ok := auditing.Entry{Action: "host.manage", Decision: auditing.DecisionAllow,
		Attrs: auditing.Attrs{"os_user": auditing.OSUserName("BOTHY-S-SHARED"), "operation": auditing.Code("session_user")}}
	if err := w.uow.Do(as(alex), func(ctx context.Context) error { return w.rec.Record(ctx, ok) }); err != nil {
		t.Fatalf("Record with os_user: %v", err)
	}
	if got := w.store.committed()[0].Details["os_user"]; got != "BOTHY-S-SHARED" {
		t.Fatalf("os_user = %v", got)
	}
	bad := map[string]auditing.Attrs{
		"path":            {"os_user": auditing.OSUserName("../x")},
		"slash":           {"os_user": auditing.OSUserName("a/b")},
		"leading dot":     {"os_user": auditing.OSUserName(".hidden")},
		"empty":           {"os_user": auditing.OSUserName("")},
		"space":           {"os_user": auditing.OSUserName("two words")},
		"under other key": {"name": auditing.OSUserName("bothy-s-sam")},
		"code as os_user": {"os_user": auditing.Code("bothy-s-sam")},
	}
	for name, attrs := range bad {
		e := auditing.Entry{Action: "host.manage", Decision: auditing.DecisionAllow, Attrs: attrs}
		if err := w.uow.Do(as(alex), func(ctx context.Context) error { return w.rec.Record(ctx, e) }); errs.KindOf(err) != errs.KindInternal {
			t.Errorf("%s: Record = %v, want kind internal", name, err)
		}
	}
	if n := len(w.store.committed()); n != 1 {
		t.Fatalf("%d entries written, want the one valid entry", n)
	}
}

// TestDetails_OSUserNames (M2-43 Design note 8b, 19 SEC-AUD-5): lists of session user names are
// accepted only under the members of host.connect that name them (hd6_users, hd9_users,
// hd10_users, hd11_users, session_users_added, session_users_removed), each element in the form
// of host_session_users.os_user; another kind under those keys and the list under another key
// are refused at Record, and the stored row validates as audit_details.v1.
func TestDetails_OSUserNames(t *testing.T) {
	t.Parallel()
	w := newWorld(t, time.Minute, 10)
	ok := auditing.Entry{Action: "host.connect", Decision: auditing.DecisionAllow,
		Attrs: auditing.Attrs{"hd6_users": auditing.OSUserNames("bothy-s-alex", "BOTHY-S-SHARED"), "session_users_removed": auditing.OSUserNames()}}
	if err := w.uow.Do(as(alex), func(ctx context.Context) error { return w.rec.Record(ctx, ok) }); err != nil {
		t.Fatalf("Record with hd6_users: %v", err)
	}
	got := w.store.committed()[0].Details
	if l, _ := got["hd6_users"].([]any); len(l) != 2 || l[1] != "BOTHY-S-SHARED" {
		t.Fatalf("hd6_users = %v", got["hd6_users"])
	}
	if err := domain.DetailsV1(got).Validate(); err != nil {
		t.Fatalf("stored details: %v", err)
	}
	bad := map[string]auditing.Attrs{
		"path":            {"hd9_users": auditing.OSUserNames("bothy-s-sam", "../x")},
		"under other key": {"names": auditing.OSUserNames("bothy-s-sam")},
		"codes":           {"hd10_users": auditing.List(auditing.Code("bothy-s-sam"))},
	}
	for name, attrs := range bad {
		e := auditing.Entry{Action: "host.connect", Decision: auditing.DecisionAllow, Attrs: attrs}
		if err := w.uow.Do(as(alex), func(ctx context.Context) error { return w.rec.Record(ctx, e) }); errs.KindOf(err) != errs.KindInternal {
			t.Errorf("%s: Record = %v, want kind internal", name, err)
		}
	}
	if err := (domain.DetailsV1{"hd11_users": []any{"a/b"}}).Validate(); err == nil {
		t.Error("a path in hd11_users validated")
	}
}

// TestSourceURLValue (M2-22 Design note 14, 19 SEC-AUD-5): the source of a price row is accepted
// only under the member source_url and only as an absolute https URL with a host, without user
// info and without fragment; http, user info, a fragment, a relative URL, the value under another
// key and another kind under source_url are refused at Record, and the stored row validates as
// audit_details.v1.
func TestSourceURLValue(t *testing.T) {
	t.Parallel()
	w := newWorld(t, time.Minute, 10)
	ok := auditing.Entry{Action: "price.manage", Decision: auditing.DecisionAllow,
		Attrs: auditing.Attrs{"source_url": auditing.SourceURL("https://provider.example.com/pricing"), "op": auditing.Code("create")}}
	if err := w.uow.Do(as(alex), func(ctx context.Context) error { return w.rec.Record(ctx, ok) }); err != nil {
		t.Fatalf("Record with source_url: %v", err)
	}
	got := w.store.committed()[0].Details
	if got["source_url"] != "https://provider.example.com/pricing" {
		t.Fatalf("source_url = %v", got["source_url"])
	}
	if err := domain.DetailsV1(got).Validate(); err != nil {
		t.Fatalf("stored details: %v", err)
	}
	bad := map[string]auditing.Attrs{
		"http":               {"source_url": auditing.SourceURL("http://provider.example.com/pricing")},
		"user info":          {"source_url": auditing.SourceURL("https://sam@provider.example.com/pricing")},
		"fragment":           {"source_url": auditing.SourceURL("https://provider.example.com/pricing#models")},
		"relative":           {"source_url": auditing.SourceURL("/pricing")},
		"under other key":    {"link": auditing.SourceURL("https://provider.example.com/pricing")},
		"code as source_url": {"source_url": auditing.Code("pricing")},
		"in a list":          {"links": auditing.List(auditing.SourceURL("https://provider.example.com/pricing"))},
	}
	for name, attrs := range bad {
		e := auditing.Entry{Action: "price.manage", Decision: auditing.DecisionAllow, Attrs: attrs}
		if err := w.uow.Do(as(alex), func(ctx context.Context) error { return w.rec.Record(ctx, e) }); errs.KindOf(err) != errs.KindInternal {
			t.Errorf("%s: Record = %v, want kind internal", name, err)
		}
	}
	if n := len(w.store.committed()); n != 1 {
		t.Fatalf("%d entries written, want the one valid entry", n)
	}
}
