// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"crypto/hmac"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/domain/apitoken"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/test/terminaltest"
	"github.com/OWNER/bothy/services/server/test/terminalworld"
)

// pairingCodePattern is the pattern of HostPairingCode.pairing_code.
var pairingCodePattern = regexp.MustCompile(`^bothy_hpc_[0-9A-Za-z]{43}_[0-9A-Za-z]{6}$`)

// TestCreateHost_PairingCode (M2-39, 24 section 3.2 point 1, T-TM-06): 201 with a pairing code of
// the pattern of HostPairingCode and a valid checksum, only its HMAC stored, valid
// terminal.pairing_code_ttl after the fake clock, state pairing, version 1, Cache-Control:
// no-store; one host.updated, one audit entry create, access.changed for Robin with an added host
// entry; the code appears in no other row, log line or terminalworld.event.
func TestCreateHost_PairingCode(t *testing.T) {
	t.Parallel()
	w := terminalworld.New(t)
	before, seq := w.PermissionVersion(), w.LastSeq()
	r := w.Do("robin", http.MethodPost, terminalworld.HostsPath, `{"name":"Test bench","reason":"New machine for the team"}`, terminalworld.Key()...)
	m := w.Want(r, http.StatusCreated, "")
	if cc := r.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
	code, _ := m["pairing_code"].(string)
	if !pairingCodePattern.MatchString(code) {
		t.Fatalf("pairing code of %d characters does not match the pattern", len(code))
	}
	if p, err := apitoken.Parse(code); err != nil || p.Kind != apitoken.KindHostPairingCode {
		t.Fatalf("checksum or kind of the code: %v", err)
	}
	host := ids.MustParse(m["host_id"].(string))
	if got, err := time.Parse(time.RFC3339Nano, m["expires_at"].(string)); err != nil || !got.Equal(terminalworld.Start.Add(terminalworld.HostConfig.PairingCodeTTL)) {
		t.Errorf("expires_at = %v, want the clock plus terminal.pairing_code_ttl", m["expires_at"])
	}
	var hash []byte
	var state string
	var version int64
	var expires time.Time
	if err := w.Owner.QueryRow(context.Background(), `SELECT pairing_code_hash, state, version, pairing_expires_at FROM hosts WHERE id = $1`,
		host.String()).Scan(&hash, &state, &version, &expires); err != nil {
		t.Fatal(err)
	}
	if !hmac.Equal(hash, apitoken.Hash(terminalworld.Pepper, code)) || state != "pairing" || version != 1 || !expires.Equal(terminalworld.Start.Add(terminalworld.HostConfig.PairingCodeTTL)) {
		t.Fatalf("stored host: hash ok %v, %s, %d, %v", hmac.Equal(hash, apitoken.Hash(terminalworld.Pepper, code)), state, version, expires)
	}
	if ev := w.Events("host.updated"); len(ev) != 1 || ev[0].ResourceID != host.String() || ev[0].Version != 1 {
		t.Fatalf("host.updated %+v", ev)
	}
	allows := w.Allows("host.manage")
	if len(allows) != 1 || allows[0].Details["operation"] != "create" || allows[0].Details["reason"] != "New machine for the team" {
		t.Fatalf("audit %+v", allows)
	}
	if w.PermissionVersion() != before+1 {
		t.Errorf("permission_version %d, want %d", w.PermissionVersion(), before+1)
	}
	changes := w.AccessChanges(seq)
	if got := changes[w.Robin]; len(got) != 1 || !terminalworld.HasHostEntry(t, got[0], "added", host, true) {
		t.Fatalf("access.changed of Robin %v", got)
	}
	if len(changes) != 1 {
		t.Fatalf("access.changed for %d principals, want only Robin", len(changes))
	}
	w.AssertNowhere(code, "pairing code")
}

// TestCreateHost_Validation (M2-39): a name of a live host in another case is 409 conflict; the
// name of a retired host may be reused; control or bidi characters and a name above
// host.name_max_chars are 400; an unknown, agent or deleted holder is 400 unknown_user; a missing
// or blank reason is 400 required; Alex gets 403 owner_only; nothing is written in any case.
func TestCreateHost_Validation(t *testing.T) {
	t.Parallel()
	w := terminalworld.New(t)
	agent := w.AgentToken("lotse")
	deleted := w.Person("gone", false)
	w.SQL(`UPDATE principals SET status = 'deleted' WHERE id = $1`, deleted.String())
	w.Host(terminaltest.Host{Name: "Old rack", State: "retired"})
	hosts := w.Count(`SELECT count(*) FROM hosts`)
	events := len(w.Events("host.updated"))
	create := func(body string) *httpservertest.Response {
		return w.Do("robin", http.MethodPost, terminalworld.HostsPath, body, terminalworld.Key()...)
	}
	w.Want(create(`{"name":"build SERVER","reason":"Again"}`), http.StatusConflict, "conflict")
	for name, c := range map[string]struct{ body, code, pointer string }{
		"control":        {`{"name":"a\u0007b","reason":"x"}`, "control_character", "/name"},
		"bidi":           {`{"name":"a\u202eb","reason":"x"}`, "bidi_control", "/name"},
		"too long":       {`{"name":"` + strings.Repeat("a", terminalworld.HostConfig.NameMaxChars+1) + `","reason":"x"}`, "too_long", "/name"},
		"unknown holder": {`{"name":"New","assigned_user_id":"` + ids.New().String() + `","reason":"x"}`, "unknown_user", "/assigned_user_id"},
		"agent holder":   {`{"name":"New","assigned_user_id":"` + agent.String() + `","reason":"x"}`, "unknown_user", "/assigned_user_id"},
		"deleted holder": {`{"name":"New","assigned_user_id":"` + deleted.String() + `","reason":"x"}`, "unknown_user", "/assigned_user_id"},
		"blank reason":   {`{"name":"New","reason":"   "}`, "required", "/reason"},
		"long reason":    {`{"name":"New","reason":"` + strings.Repeat("r", terminalworld.HostConfig.ReasonMaxChars+1) + `"}`, "too_long", "/reason"},
	} {
		p := w.Want(create(c.body), http.StatusBadRequest, "validation_failed")
		if code, ptr := terminalworld.Field(p); code != c.code || ptr != c.pointer {
			t.Errorf("%s: %s at %s, want %s at %s", name, code, ptr, c.code, c.pointer)
		}
	}
	if code, ptr := terminalworld.Field(w.Want(create(`{"name":"New"}`), http.StatusBadRequest, "validation_failed")); code != "required" || ptr != "/reason" {
		t.Errorf("missing reason: %s at %s", code, ptr)
	}
	p := w.Want(w.Do("alex", http.MethodPost, terminalworld.HostsPath, `{"name":"Mine","reason":"x"}`, terminalworld.Key()...), http.StatusForbidden, "forbidden")
	if p["reason"] != "owner_only" {
		t.Fatalf("Alex: %v", p)
	}
	if w.Count(`SELECT count(*) FROM hosts`) != hosts || len(w.Events("host.updated")) != events || len(w.Allows("host.manage")) != 0 {
		t.Fatal("a refused createHost wrote something")
	}
	w.Want(create(`{"name":"Old rack","reason":"Reuse the name"}`), http.StatusCreated, "")
}
