// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	instanceport "github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// These are the integration tests of the feature flags of M1-47 on PostgreSQL 18 with the real
// authz store (groups, memberships), identity (people, step-up, tokens), audit and event log, and
// the controllable clock of the session world. The flags are wired with wire.NewInstanceSettings
// exactly as the server builds them.

const flagsPath = "/api/v1/feature-flags"

// registeredFlagsOff reports whether v is the features array of getInstance with every
// registered flag off, in key order.
func registeredFlagsOff(v any) bool {
	list, ok := v.([]any)
	defs := instanceport.RegisteredFlags()
	if !ok || len(list) != len(defs) {
		return false
	}
	for i, d := range defs {
		f := list[i].(map[string]any)
		if f["key"] != d.Key || f["enabled"] != false {
			return false
		}
	}
	return true
}

// features returns the evaluated flags of getInstance as key to value.
func features(m map[string]any) map[string]bool {
	out := map[string]bool{}
	for _, f := range m["features"].([]any) {
		e := f.(map[string]any)
		out[e["key"].(string)] = e["enabled"].(bool)
	}
	return out
}

// group inserts a live local group named name, created by by.
func (w *sworld) group(name string, by ids.ID) ids.ID {
	w.t.Helper()
	id := ids.New()
	w.sql(`INSERT INTO groups (id, name, created_by) VALUES ($1, $2, $3)`, id.String(), name, by.String())
	return id
}

// member adds user to group; expires is the end of the membership, zero for none. It returns the
// membership id.
func (w *sworld) member(group, user, by ids.ID, expires time.Time) ids.ID {
	w.t.Helper()
	id := ids.New()
	var exp any
	if !expires.IsZero() {
		exp = expires
	}
	w.sql(`INSERT INTO group_memberships (id, group_id, user_id, added_by, added_at, expires_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		id.String(), group.String(), user.String(), by.String(), w.clk.Now(), exp)
	return id
}

// flagRow stores key with enabled and the rollout JSON, the way putFeatureFlag would.
func (w *sworld) flagRow(key string, enabled bool, rollout string) {
	w.t.Helper()
	w.sql(`INSERT INTO feature_flags (key, enabled, rollout) VALUES ($1, $2, $3::jsonb)
		ON CONFLICT (key) DO UPDATE SET enabled = excluded.enabled, rollout = excluded.rollout, version = feature_flags.version + 1`,
		key, enabled, rollout)
}

// enabled evaluates key for the person id through the port instanceport.Flags.
func (w *sworld) enabled(id ids.ID, kind, key string) bool {
	w.t.Helper()
	on, err := w.settings.Admin.Enabled(context.Background(), instanceport.FlagSubject{PrincipalID: id, Kind: kind}, key)
	if err != nil {
		w.t.Fatal(err)
	}
	return on
}

// putFlag sends putFeatureFlag of key with body and optional If-Match tag.
func (c *stepClient) putFlag(key, body, tag string) *httpservertest.Response {
	c.w.t.Helper()
	var hs []string
	if tag != "" {
		hs = append(hs, "If-Match", tag)
	}
	return c.callH(http.MethodPut, flagsPath+"/"+key, body, hs...)
}

// flagWorld is a world with the instance row, the owner Alex with a fresh step-up and the person
// Sam.
func flagWorld(t *testing.T) (*sworld, *stepClient, ids.ID) {
	t.Helper()
	w, alex := adminWorld(t)
	return w, alex, w.person("sam", false)
}

func TestFlags_GlobalOnOff(t *testing.T) {
	t.Parallel()
	w, _, sam := flagWorld(t)
	if w.enabled(sam, "user", instanceport.FlagDMEnabled) {
		t.Fatal("default must be off")
	}
	w.flagRow(instanceport.FlagDMEnabled, true, `{}`)
	if !w.enabled(sam, "user", instanceport.FlagDMEnabled) {
		t.Fatal("global on")
	}
	w.flagRow(instanceport.FlagDMEnabled, false, `{}`)
	if w.enabled(sam, "user", instanceport.FlagDMEnabled) {
		t.Fatal("global off again")
	}
}

func TestFlags_PersonInRolloutInverts(t *testing.T) {
	t.Parallel()
	w, alex, sam := flagWorld(t)
	w.flagRow(instanceport.FlagVoiceEnabled, false, `{"user_ids":["`+sam.String()+`"]}`)
	if !w.enabled(sam, "user", instanceport.FlagVoiceEnabled) || w.enabled(alex.user, "user", instanceport.FlagVoiceEnabled) {
		t.Fatal("Sam on, Alex off")
	}
	w.flagRow(instanceport.FlagVoiceEnabled, true, `{"user_ids":["`+sam.String()+`"]}`)
	if w.enabled(sam, "user", instanceport.FlagVoiceEnabled) || !w.enabled(alex.user, "user", instanceport.FlagVoiceEnabled) {
		t.Fatal("Sam off, Alex on")
	}
}

func TestFlags_GroupInRolloutInverts(t *testing.T) {
	t.Parallel()
	w, alex, sam := flagWorld(t)
	research := w.group("Research", alex.user)
	w.member(research, sam, alex.user, time.Time{})
	w.flagRow(instanceport.FlagDMEnabled, false, `{"group_ids":["`+research.String()+`"]}`)
	if !w.enabled(sam, "user", instanceport.FlagDMEnabled) || w.enabled(alex.user, "user", instanceport.FlagDMEnabled) {
		t.Fatal("Sam via Research on, Alex off")
	}
}

func TestFlags_ExpiredMembershipDoesNotInvert(t *testing.T) {
	t.Parallel()
	w, alex, sam := flagWorld(t)
	research := w.group("Research", alex.user)
	expires := w.clk.Now().Add(time.Hour)
	w.member(research, sam, alex.user, expires)
	w.flagRow(instanceport.FlagDMEnabled, false, `{"group_ids":["`+research.String()+`"]}`)
	w.clk.Advance(time.Hour - time.Second)
	if !w.enabled(sam, "user", instanceport.FlagDMEnabled) {
		t.Fatal("one second before expires_at the membership is valid")
	}
	w.clk.Advance(time.Second)
	if !w.clk.Now().Equal(expires) {
		t.Fatalf("clock %v, want %v", w.clk.Now(), expires)
	}
	if w.enabled(sam, "user", instanceport.FlagDMEnabled) {
		t.Fatal("at t = expires_at Sam gets the global value (07 2.5)")
	}
}

func TestFlags_RemovedMembershipDoesNotInvert(t *testing.T) {
	t.Parallel()
	w, alex, sam := flagWorld(t)
	research := w.group("Research", alex.user)
	m := w.member(research, sam, alex.user, time.Time{})
	w.flagRow(instanceport.FlagDMEnabled, false, `{"group_ids":["`+research.String()+`"]}`)
	w.sql(`UPDATE group_memberships SET removed_at = $2, removed_by = $3 WHERE id = $1`, m.String(), w.clk.Now(), alex.user.String())
	if w.enabled(sam, "user", instanceport.FlagDMEnabled) {
		t.Fatal("a removed membership must not invert")
	}
}

func TestFlags_DeletedGroupDoesNotInvert(t *testing.T) {
	t.Parallel()
	w, alex, sam := flagWorld(t)
	research := w.group("Research", alex.user)
	w.member(research, sam, alex.user, time.Time{})
	w.flagRow(instanceport.FlagDMEnabled, false, `{"group_ids":["`+research.String()+`"]}`)
	w.sql(`UPDATE groups SET deleted_at = $2 WHERE id = $1`, research.String(), w.clk.Now())
	if w.enabled(sam, "user", instanceport.FlagDMEnabled) {
		t.Fatal("a deleted group must not invert")
	}
}

func TestFlags_UnknownKeyOff(t *testing.T) {
	t.Parallel()
	w, _, sam := flagWorld(t)
	if w.enabled(sam, "user", "no.such_flag") {
		t.Fatal("unknown key on")
	}
}

func TestFlags_OrphanedRowOffAndListed(t *testing.T) {
	t.Parallel()
	w, alex, sam := flagWorld(t)
	w.flagRow("chat.old_box", true, `{}`)
	if w.enabled(sam, "user", "chat.old_box") {
		t.Fatal("an orphaned row is off")
	}
	items := w.want(alex.call(http.MethodGet, flagsPath, ""), http.StatusOK, "")["items"].([]any)
	last := items[len(items)-1].(map[string]any)
	if last["key"] != "chat.old_box" || last["orphaned"] != true || last["kind"] != nil || last["description"] != nil || last["version"] != float64(1) {
		t.Fatalf("orphaned item %v", last)
	}
	sam2 := w.login(sam, "active")
	if f := features(w.want(sam2.call(http.MethodGet, instancePath, ""), http.StatusOK, "")); len(f) != len(instanceport.RegisteredFlags()) {
		t.Errorf("getInstance features %v, the orphaned row must not appear", f)
	}
	w.want(alex.putFlag("chat.old_box", `{"enabled":false,"reason":"cleanup"}`, ""), http.StatusNotFound, "not_found")
}

func TestFlags_AgentGetsGlobalValue(t *testing.T) {
	t.Parallel()
	w, alex, _ := flagWorld(t)
	agent := w.agentPrincipal("lotse")
	w.flagRow(instanceport.FlagVoiceEnabled, true, `{"user_ids":["`+agent.String()+`"]}`)
	if !w.enabled(agent, "agent", instanceport.FlagVoiceEnabled) {
		t.Fatal("an agent gets the global value even with its ID in user_ids")
	}
	tk := alex.agentToken(agent)
	if f := features(w.want(w.withToken(tk, http.MethodGet, instancePath, ""), http.StatusOK, "")); !f[instanceport.FlagVoiceEnabled] {
		t.Errorf("agent features %v", f)
	}
}

func TestListFeatureFlags_OwnerOnly(t *testing.T) {
	t.Parallel()
	w, alex, samID := flagWorld(t)
	sam := w.phone(samID)
	sam.stepUp()
	if p := w.want(sam.call(http.MethodGet, flagsPath, ""), http.StatusForbidden, "forbidden"); p["reason"] != "owner_only" {
		t.Errorf("reason %v", p["reason"])
	}
	items := w.want(alex.call(http.MethodGet, flagsPath, ""), http.StatusOK, "")["items"].([]any)
	defs := instanceport.RegisteredFlags()
	if len(items) != len(defs) {
		t.Fatalf("items %v", items)
	}
	for i, d := range defs {
		f := items[i].(map[string]any)
		if f["key"] != d.Key || f["kind"] != "option" || f["default"] != false || f["enabled"] != false || f["version"] != float64(0) ||
			f["module"] != d.Module || f["description"] != d.Description || f["orphaned"] != false || f["updated_at"] != nil ||
			f["updated_by"] != nil || len(f["rollout"].(map[string]any)) != 0 {
			t.Errorf("item %d %v", i, f)
		}
	}
	// Listing is no step-up action (05 4.2).
	plain := w.phone(w.person("robin", true))
	w.want(plain.call(http.MethodGet, flagsPath, ""), http.StatusOK, "")
}

func TestPutFeatureFlag_StepUpReasonAudit(t *testing.T) {
	t.Parallel()
	w, alex, sam := flagWorld(t)
	plain := w.phone(w.person("robin", true))
	w.want(plain.putFlag(instanceport.FlagDMEnabled, `{"enabled":true,"reason":"pilot"}`, ""), http.StatusUnauthorized, "step_up_required")
	w.want(alex.putFlag(instanceport.FlagDMEnabled, `{"enabled":true}`, ""), http.StatusBadRequest, "validation_failed")
	p := w.want(alex.putFlag(instanceport.FlagDMEnabled, `{"enabled":true,"reason":"   "}`, ""), http.StatusBadRequest, "validation_failed")
	if ptr, code := field(t, p); ptr != "/reason" || code != "required" {
		t.Errorf("blank reason: %s %s", ptr, code)
	}
	r := alex.putFlag(instanceport.FlagDMEnabled, `{"enabled":true,"rollout":{"user_ids":["`+sam.String()+`"]},"reason":"pilot with Sam"}`, "")
	m := w.want(r, http.StatusOK, "")
	if m["version"] != float64(1) || m["enabled"] != true || m["updated_by"] != alex.user.String() || m["updated_at"] == nil ||
		r.Header.Get("ETag") != `"1"` || m["description"] != "Direct messages between people" {
		t.Fatalf("flag %v %s", m, r.Header.Get("ETag"))
	}
	// The deny entry is aggregated and written at the flush, so the order is not the call order.
	audits := w.audits("feature_flag.update")
	slices.SortFunc(audits, func(a, b sauditRow) int { return strings.Compare(a.Decision, b.Decision) })
	if len(audits) != 2 || audits[0].Decision != "allow" || audits[1].Decision != "deny" || audits[1].Reason != "step_up_required" {
		t.Fatalf("audits %+v", audits)
	}
	d := audits[0].Details
	if d["key"] != instanceport.FlagDMEnabled || d["enabled"] != true || d["reason"] != "pilot with Sam" ||
		!slices.Equal(d["rollout_user_ids"].([]any), []any{sam.String()}) || strings.Contains(audits[0].Raw, "sam") {
		t.Errorf("allow details %v", audits[0].Raw)
	}
	var stored string
	w.scalar(&stored, `SELECT rollout::text FROM feature_flags WHERE key = 'dm.enabled'`)
	if stored != `{"user_ids": ["`+sam.String()+`"]}` {
		t.Errorf("stored rollout %s", stored)
	}
}

func TestPutFeatureFlag_UnknownKey404(t *testing.T) {
	t.Parallel()
	w, alex, _ := flagWorld(t)
	w.want(alex.putFlag("no.such_flag", `{"enabled":true,"reason":"x"}`, ""), http.StatusNotFound, "not_found")
	if n := w.count(`SELECT count(*) FROM feature_flags`); n != 0 {
		t.Errorf("%d rows", n)
	}
}

func TestPutFeatureFlag_OrphanedKey404(t *testing.T) {
	t.Parallel()
	w, alex, _ := flagWorld(t)
	w.flagRow("chat.old_box", true, `{}`)
	w.want(alex.putFlag("chat.old_box", `{"enabled":false,"reason":"x"}`, ""), http.StatusNotFound, "not_found")
}

func TestPutFeatureFlag_RolloutValidation(t *testing.T) {
	t.Parallel()
	w, alex, sam := flagWorld(t)
	agent := w.agentPrincipal("lotse")
	research := w.group("Research", alex.user)
	gone := w.group("Marketing", alex.user)
	w.sql(`UPDATE groups SET deleted_at = now() WHERE id = $1`, gone.String())
	body, _ := json.Marshal(map[string]any{"enabled": true, "reason": "pilot",
		"rollout": map[string]any{"user_ids": []string{sam.String(), agent.String()}, "group_ids": []string{ids.New().String(), research.String(), gone.String()}}})
	p := w.want(alex.putFlag(instanceport.FlagDMEnabled, string(body), ""), http.StatusBadRequest, "validation_failed")
	var got []string
	for _, f := range p["errors"].([]any) {
		e := f.(map[string]any)
		got = append(got, e["pointer"].(string)+" "+e["code"].(string))
	}
	want := []string{"/rollout/user_ids/1 unknown_user", "/rollout/group_ids/0 unknown_group", "/rollout/group_ids/2 unknown_group"}
	if !slices.Equal(got, want) {
		t.Fatalf("errors %v, want %v", got, want)
	}
	if n := w.count(`SELECT count(*) FROM feature_flags`); n != 0 {
		t.Errorf("%d rows", n)
	}
}

func TestPutFeatureFlag_IfMatch412(t *testing.T) {
	t.Parallel()
	w, alex, _ := flagWorld(t)
	p := w.want(alex.putFlag(instanceport.FlagDMEnabled, `{"enabled":true,"reason":"x"}`, `"3"`), http.StatusPreconditionFailed, "version_conflict")
	if cur := p["current"].(map[string]any); cur["key"] != instanceport.FlagDMEnabled || cur["version"] != float64(0) || cur["enabled"] != false {
		t.Fatalf("current %v", cur)
	}
	w.want(alex.putFlag(instanceport.FlagDMEnabled, `{"enabled":true,"reason":"x"}`, `"0"`), http.StatusOK, "")
	p = w.want(alex.putFlag(instanceport.FlagDMEnabled, `{"enabled":false,"reason":"x"}`, `"0"`), http.StatusPreconditionFailed, "version_conflict")
	if cur := p["current"].(map[string]any); cur["version"] != float64(1) || cur["enabled"] != true {
		t.Fatalf("current %v", cur)
	}
	w.want(alex.putFlag(instanceport.FlagDMEnabled, `{"enabled":false,"reason":"x"}`, `"1"`), http.StatusOK, "")
}

func TestPutFeatureFlag_EventNamesOnlyKey(t *testing.T) {
	t.Parallel()
	w, alex, _ := flagWorld(t)
	w.want(alex.putFlag(instanceport.FlagVoiceEnabled, `{"enabled":true,"reason":"go live"}`, ""), http.StatusOK, "")
	evs := w.updatedEvents()
	if len(evs) != 1 || !slices.Equal(evs[0].ChangedKeys, []string{instanceport.FlagVoiceEnabled}) || evs[0].Actor != alex.user.String() ||
		evs[0].Receive != "instance.read" {
		t.Fatalf("events %+v", evs)
	}
	var payload string
	w.scalar(&payload, `SELECT payload::text FROM event_log WHERE type = 'instance.updated'`)
	if payload != `{"changed_keys": ["voice.enabled"]}` {
		t.Errorf("payload %s names more than the key", payload)
	}
}

func TestPutFeatureFlag_NoChangeNoEvent(t *testing.T) {
	t.Parallel()
	w, alex, sam := flagWorld(t)
	if m := w.want(alex.putFlag(instanceport.FlagDMEnabled, `{"enabled":false,"reason":"x"}`, ""), http.StatusOK, ""); m["version"] != float64(0) {
		t.Fatalf("default written: %v", m)
	}
	body := `{"enabled":true,"rollout":{"user_ids":["` + sam.String() + `","` + sam.String() + `"],"group_ids":[]},"reason":"x"}`
	w.want(alex.putFlag(instanceport.FlagDMEnabled, body, ""), http.StatusOK, "")
	r := alex.putFlag(instanceport.FlagDMEnabled, `{"enabled":true,"rollout":{"user_ids":["`+sam.String()+`"]},"reason":"again"}`, "")
	if m := w.want(r, http.StatusOK, ""); m["version"] != float64(1) || r.Header.Get("ETag") != `"1"` {
		t.Errorf("same value: %v", m)
	}
	if n := len(w.updatedEvents()); n != 1 {
		t.Errorf("%d events, want 1", n)
	}
	// An absent rollout means empty: a change.
	if m := w.want(alex.putFlag(instanceport.FlagDMEnabled, `{"enabled":true,"reason":"all"}`, ""), http.StatusOK, ""); m["version"] != float64(2) ||
		len(m["rollout"].(map[string]any)) != 0 {
		t.Errorf("absent rollout: %v", m)
	}
}

func TestGetInstance_FeaturesEvaluatedForCaller(t *testing.T) {
	t.Parallel()
	w, alex, samID := flagWorld(t)
	w.want(alex.putFlag(instanceport.FlagVoiceEnabled, `{"enabled":false,"rollout":{"user_ids":["`+samID.String()+`"]},"reason":"pilot"}`, ""), http.StatusOK, "")
	sam := w.login(samID, "active")
	ra := alex.call(http.MethodGet, instancePath, "")
	rs := sam.call(http.MethodGet, instancePath, "")
	fa, fs := features(w.want(ra, http.StatusOK, "")), features(w.want(rs, http.StatusOK, ""))
	if fa[instanceport.FlagVoiceEnabled] || !fs[instanceport.FlagVoiceEnabled] {
		t.Fatalf("Alex %v, Sam %v", fa, fs)
	}
	if ra.Header.Get("ETag") == rs.Header.Get("ETag") {
		t.Error("the ETag must differ with the evaluation")
	}
}

// personalWith creates a personal token for c and returns the response.
func (c *stepClient) personalWith() *httpservertest.Response {
	c.w.t.Helper()
	return c.create(c.user, tokenBody("Laptop", c.w.clk.Now().Add(time.Hour), ""))
}

func TestPersonalToken_FlagOffForbidden(t *testing.T) {
	t.Parallel()
	w, _, samID := flagWorld(t)
	sam := w.phone(samID)
	sam.stepUp()
	w.want(sam.personalWith(), http.StatusForbidden, "feature_disabled")
}

func TestPersonalToken_FlagOnAllowed(t *testing.T) {
	t.Parallel()
	w, alex, samID := flagWorld(t)
	w.want(alex.putFlag(instanceport.FlagTokensPersonalEnabled, `{"enabled":true,"reason":"all people"}`, ""), http.StatusOK, "")
	sam := w.phone(samID)
	sam.stepUp()
	w.want(sam.personalWith(), http.StatusCreated, "")
}

func TestPersonalToken_RolloutForSamOnly(t *testing.T) {
	t.Parallel()
	w, alex, samID := flagWorld(t)
	w.want(alex.putFlag(instanceport.FlagTokensPersonalEnabled, `{"enabled":false,"rollout":{"user_ids":["`+samID.String()+`"]},"reason":"Sam only"}`, ""), http.StatusOK, "")
	sam := w.phone(samID)
	sam.stepUp()
	w.want(sam.personalWith(), http.StatusCreated, "")
	robin := w.phone(w.person("robin", false))
	robin.stepUp()
	w.want(robin.personalWith(), http.StatusForbidden, "feature_disabled")
	w.want(alex.personalWith(), http.StatusForbidden, "feature_disabled")
}
