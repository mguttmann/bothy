// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// invNow is the issuing time of these tests; invTTL stands for agent.invocation_token_ttl.
var (
	invNow = time.Date(2026, 10, 20, 9, 0, 0, 500_000_000, time.UTC)
	invTTL = 90 * time.Second
)

// backticked finds the names in backticks of a table cell.
var backticked = regexp.MustCompile("`([^`]+)`")

// claimTable reads the backticked names of the first column of the claim table (the table whose
// header cell is "Claim") under the heading of chapter glob that starts with prefix. The chapter
// is matched by its number, so the test reads it in either language of the repository.
func claimTable(t *testing.T, glob, prefix string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(repoRoot(t), "docs", "spec", glob))
	if err != nil || len(files) != 1 {
		t.Fatalf("want exactly one chapter docs/spec/%s, got %v (%v)", glob, files, err)
	}
	f, err := os.Open(files[0])
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var names []string
	in, table := false, false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ") {
			in = strings.HasPrefix(line, prefix)
			table = false
			continue
		}
		if !in {
			continue
		}
		if !strings.HasPrefix(line, "|") {
			table = false
			continue
		}
		first := strings.TrimSpace(strings.Split(strings.Trim(line, "|"), "|")[0])
		if first == "Claim" {
			table = true
			continue
		}
		if !table {
			continue
		}
		for _, m := range backticked.FindAllStringSubmatch(first, -1) {
			names = append(names, m[1])
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if len(names) == 0 {
		t.Fatalf("no claim table under %q of docs/spec/%s", prefix, glob)
	}
	sort.Strings(names)
	return names
}

// jsonMembers returns the JSON member names of the fields of v.
func jsonMembers(v any) []string {
	var out []string
	rt := reflect.TypeOf(v)
	for i := range rt.NumField() {
		name, _, _ := strings.Cut(rt.Field(i).Tag.Get("json"), ",")
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// TestInvocationClaims_MatchSpec is the schema test of ADR 0023 "Confirmation": the claim tables
// of 05 section 5 and 09 4.5 name the same set, which is the set of members of InvocationClaims,
// and the header constants are the values both chapters state.
func TestInvocationClaims_MatchSpec(t *testing.T) {
	t.Parallel()
	actions := claimTable(t, "05-*.md", "## 5.")
	agents := claimTable(t, "09-*.md", "### 4.5 ")
	if !slices.Equal(actions, agents) {
		t.Fatalf("05 section 5 claims %v differ from 09 4.5 claims %v", actions, agents)
	}
	if got := jsonMembers(domain.InvocationClaims{}); !slices.Equal(got, actions) {
		t.Fatalf("InvocationClaims members %v, want the claims of 05 section 5 %v", got, actions)
	}
	for _, glob := range []string{"05-*.md", "09-*.md"} {
		files, _ := filepath.Glob(filepath.Join(repoRoot(t), "docs", "spec", glob))
		b, err := os.ReadFile(files[0])
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"`alg = EdDSA`", "`typ = agent-invocation+jwt`"} {
			if glob == "09-*.md" {
				want = strings.ReplaceAll(strings.ReplaceAll(want, "= ", `= "`), "`", "")
				want = "`" + want + "\"`"
			}
			if !bytes.Contains(b, []byte(want)) {
				t.Errorf("%s does not state %s", glob, want)
			}
		}
	}
	if domain.InvocationAlg != "EdDSA" || domain.InvocationTyp != "agent-invocation+jwt" {
		t.Errorf("header constants %q %q", domain.InvocationAlg, domain.InvocationTyp)
	}
}

// openapiEnum reads the enum of the schema name from docs/api/openapi.yaml.
func openapiEnum(t *testing.T, name string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(b), "\n")
	for i, l := range lines {
		if l != "    "+name+":" {
			continue
		}
		for _, m := range lines[i+1:] {
			if strings.HasPrefix(m, "    ") && !strings.HasPrefix(m, "     ") {
				break
			}
			if v, ok := strings.CutPrefix(strings.TrimSpace(m), "enum: ["); ok {
				var out []string
				for _, e := range strings.Split(strings.TrimSuffix(v, "]"), ",") {
					out = append(out, strings.TrimSpace(e))
				}
				return out
			}
		}
	}
	t.Fatalf("no enum of schema %s in openapi.yaml", name)
	return nil
}

// grantOf returns a complete grant of kind with every ID set that the kind needs.
func grantOf(kind domain.TriggerKind) domain.InvocationGrant {
	g := domain.InvocationGrant{
		AgentID: ids.New(), TriggeredBy: ids.New(), TriggerKind: kind, RunID: ids.New(),
		ThreadID: ids.New(), ChannelID: ids.New(), SpaceID: ids.New(), Level: "trigger",
		BillingMode: "organization", ToolClass: "scoped", PermissionVersion: 7,
	}
	switch kind {
	case domain.TriggerMessage, domain.TriggerFollowup:
		g.MessageID = ids.New()
	case domain.TriggerInteraction:
		g.InteractionID = ids.New()
	case domain.TriggerCommand:
		g.CommandID = ids.New()
	case domain.TriggerAutocomplete:
		g.CommandID, g.RunID = ids.New(), ids.ID{}
	}
	return g
}

// claimsOf builds the claims of g with a fresh jti and fails the test on an error.
func claimsOf(t *testing.T, g domain.InvocationGrant) domain.InvocationClaims {
	t.Helper()
	c, err := domain.NewInvocationClaims(g, ids.New(), invNow, invTTL)
	if err != nil {
		t.Fatalf("NewInvocationClaims(%+v): %v", g, err)
	}
	return c.WithIssuer(ids.New())
}

// canonicalID matches a canonical lower-case UUID.
var canonicalID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func TestInvocationClaims_ShapePerTriggerKind(t *testing.T) {
	t.Parallel()
	always := []string{"iss", "aud", "sub", "jti", "iat", "exp", "trigger_kind", "thread_id", "channel_id", "space_id",
		"level", "billing_mode", "tool_class", "permission_version"}
	extra := map[domain.TriggerKind][]string{
		domain.TriggerMessage:      {"run_id", "message_id"},
		domain.TriggerFollowup:     {"run_id", "message_id"},
		domain.TriggerInteraction:  {"run_id", "interaction_id"},
		domain.TriggerCommand:      {"run_id", "command_id"},
		domain.TriggerAutocomplete: {"command_id"},
	}
	enums := map[string][]string{
		"level": openapiEnum(t, "Level"), "billing_mode": openapiEnum(t, "BillingMode"), "tool_class": openapiEnum(t, "ToolClass"),
	}
	if len(domain.TriggerKinds()) != len(extra) {
		t.Fatalf("TriggerKinds %v", domain.TriggerKinds())
	}
	for _, kind := range domain.TriggerKinds() {
		c := claimsOf(t, grantOf(kind))
		payload, err := c.Payload()
		if err != nil {
			t.Fatal(err)
		}
		dec := json.NewDecoder(bytes.NewReader(payload))
		dec.UseNumber()
		var m map[string]any
		if err := dec.Decode(&m); err != nil {
			t.Fatal(err)
		}
		want := append(slices.Clone(always), extra[kind]...)
		var got []string
		for k := range m {
			got = append(got, k)
		}
		sort.Strings(want)
		sort.Strings(got)
		if !slices.Equal(got, want) {
			t.Errorf("%s: members %v, want %v", kind, got, want)
		}
		for k, v := range m {
			if v == nil {
				t.Errorf("%s: member %s is null", kind, k)
			}
			if k == "jti" || strings.HasSuffix(k, "_id") || k == "iss" || k == "aud" || k == "sub" {
				if s, ok := v.(string); !ok || !canonicalID.MatchString(s) {
					t.Errorf("%s: %s = %v is not a canonical lower-case UUID", kind, k, v)
				}
			}
		}
		iat, err1 := m["iat"].(json.Number).Int64()
		exp, err2 := m["exp"].(json.Number).Int64()
		if err1 != nil || err2 != nil || exp-iat != int64(invTTL/time.Second) || iat != invNow.Unix() {
			t.Errorf("%s: iat %v exp %v, want integers %d apart from %d", kind, m["iat"], m["exp"], int64(invTTL/time.Second), invNow.Unix())
		}
		if _, err := m["permission_version"].(json.Number).Int64(); err != nil {
			t.Errorf("%s: permission_version %v is not an integer", kind, m["permission_version"])
		}
		for k, values := range enums {
			if !slices.Contains(values, m[k].(string)) {
				t.Errorf("%s: %s = %v outside %v", kind, k, m[k], values)
			}
		}
		if _, ok := m["dev_environment_id"]; ok {
			t.Errorf("%s: dev_environment_id is set", kind)
		}
		if m["trigger_kind"] != string(kind) {
			t.Errorf("%s: trigger_kind %v", kind, m["trigger_kind"])
		}
	}
	// Every value of the enums is accepted.
	for k, values := range enums {
		for _, v := range values {
			g := grantOf(domain.TriggerMessage)
			switch k {
			case "level":
				g.Level = v
			case "billing_mode":
				g.BillingMode = v
			case "tool_class":
				g.ToolClass = v
			}
			claimsOf(t, g)
		}
	}
}

// brokenGrants are the grants that break the table of Design note 2 of M2-06.
func brokenGrants() map[string]domain.InvocationGrant {
	out := map[string]domain.InvocationGrant{}
	mod := func(name string, kind domain.TriggerKind, f func(*domain.InvocationGrant)) {
		g := grantOf(kind)
		f(&g)
		out[name] = g
	}
	for _, kind := range []domain.TriggerKind{domain.TriggerMessage, domain.TriggerFollowup, domain.TriggerInteraction, domain.TriggerCommand} {
		mod("no run_id for "+string(kind), kind, func(g *domain.InvocationGrant) { g.RunID = ids.ID{} })
	}
	mod("run_id with autocomplete", domain.TriggerAutocomplete, func(g *domain.InvocationGrant) { g.RunID = ids.New() })
	mod("no triggering object", domain.TriggerMessage, func(g *domain.InvocationGrant) { g.MessageID = ids.ID{} })
	mod("no command for autocomplete", domain.TriggerAutocomplete, func(g *domain.InvocationGrant) { g.CommandID = ids.ID{} })
	mod("two triggering objects", domain.TriggerMessage, func(g *domain.InvocationGrant) { g.CommandID = ids.New() })
	mod("message and interaction", domain.TriggerInteraction, func(g *domain.InvocationGrant) { g.MessageID = ids.New() })
	mod("wrong object for message", domain.TriggerMessage, func(g *domain.InvocationGrant) { g.MessageID, g.InteractionID = ids.ID{}, ids.New() })
	mod("wrong object for followup", domain.TriggerFollowup, func(g *domain.InvocationGrant) { g.MessageID, g.CommandID = ids.ID{}, ids.New() })
	mod("wrong object for command", domain.TriggerCommand, func(g *domain.InvocationGrant) { g.CommandID, g.MessageID = ids.ID{}, ids.New() })
	mod("wrong object for interaction", domain.TriggerInteraction, func(g *domain.InvocationGrant) { g.InteractionID, g.CommandID = ids.ID{}, ids.New() })
	mod("wrong object for autocomplete", domain.TriggerAutocomplete, func(g *domain.InvocationGrant) { g.CommandID, g.MessageID = ids.ID{}, ids.New() })
	mod("zero thread_id", domain.TriggerMessage, func(g *domain.InvocationGrant) { g.ThreadID = ids.ID{} })
	mod("zero thread_id for autocomplete", domain.TriggerAutocomplete, func(g *domain.InvocationGrant) { g.ThreadID = ids.ID{} })
	mod("zero channel_id", domain.TriggerMessage, func(g *domain.InvocationGrant) { g.ChannelID = ids.ID{} })
	mod("zero space_id", domain.TriggerMessage, func(g *domain.InvocationGrant) { g.SpaceID = ids.ID{} })
	mod("zero agent", domain.TriggerMessage, func(g *domain.InvocationGrant) { g.AgentID = ids.ID{} })
	mod("zero triggering person", domain.TriggerMessage, func(g *domain.InvocationGrant) { g.TriggeredBy = ids.ID{} })
	mod("unknown trigger kind", domain.TriggerMessage, func(g *domain.InvocationGrant) { g.TriggerKind = "mention" })
	mod("unknown level", domain.TriggerMessage, func(g *domain.InvocationGrant) { g.Level = "admin" })
	mod("unknown billing mode", domain.TriggerMessage, func(g *domain.InvocationGrant) { g.BillingMode = "unknown" })
	mod("unknown tool class", domain.TriggerMessage, func(g *domain.InvocationGrant) { g.ToolClass = "Scoped" })
	mod("empty level", domain.TriggerMessage, func(g *domain.InvocationGrant) { g.Level = "" })
	mod("negative permission_version", domain.TriggerMessage, func(g *domain.InvocationGrant) { g.PermissionVersion = -1 })
	return out
}

func TestNewInvocationClaims_PresenceRules(t *testing.T) {
	t.Parallel()
	for name, g := range brokenGrants() {
		if _, err := domain.NewInvocationClaims(g, ids.New(), invNow, invTTL); err == nil {
			t.Errorf("%s: NewInvocationClaims accepted %+v", name, g)
		}
	}
	g := grantOf(domain.TriggerMessage)
	if _, err := domain.NewInvocationClaims(g, ids.ID{}, invNow, invTTL); err == nil {
		t.Error("a zero jti was accepted")
	}
	if _, err := domain.NewInvocationClaims(g, ids.New(), invNow, 0); err == nil {
		t.Error("a ttl below one second was accepted")
	}
	g.PermissionVersion = 0
	claimsOf(t, g)
	// Claims without an issuer are never encoded.
	c, err := domain.NewInvocationClaims(g, ids.New(), invNow, invTTL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Payload(); !errors.Is(err, domain.ErrInvocationInvalid) {
		t.Errorf("Payload without an issuer: %v", err)
	}
}

// testKey is the Ed25519 key of the parser tests.
var testKey = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize))

// rawToken builds a compact JWS from a literal header and a payload, signed with testKey.
func rawToken(header string, payload []byte) string {
	in := base64.RawURLEncoding.EncodeToString([]byte(header)) + "." + base64.RawURLEncoding.EncodeToString(payload)
	return in + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(testKey, []byte(in)))
}

// validPayload is the payload of a complete message grant.
func validPayload(t testing.TB) []byte {
	c, err := domain.NewInvocationClaims(grantOf(domain.TriggerMessage), ids.New(), invNow, invTTL)
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.WithIssuer(ids.New()).Payload()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// validHeader is the header the issuer writes.
const validHeader = `{"alg":"EdDSA","kid":"k1","typ":"agent-invocation+jwt"}`

// badHeaders are the headers TestParseInvocationToken_HeaderRules rejects.
func badHeaders() map[string]string {
	h := map[string]string{
		"typ missing": `{"alg":"EdDSA","kid":"k1"}`,
		"kid empty":   `{"alg":"EdDSA","kid":"","typ":"agent-invocation+jwt"}`,
		"kid missing": `{"alg":"EdDSA","typ":"agent-invocation+jwt"}`,
		"crit":        `{"alg":"EdDSA","kid":"k1","typ":"agent-invocation+jwt","crit":["exp"]}`,
		"alg missing": `{"kid":"k1","typ":"agent-invocation+jwt"}`,
		"kid number":  `{"alg":"EdDSA","kid":1,"typ":"agent-invocation+jwt"}`,
	}
	for _, alg := range []string{"none", "HS256", "ES256", "RS256", "Ed25519", "eddsa"} {
		h["alg "+alg] = `{"alg":"` + alg + `","kid":"k1","typ":"agent-invocation+jwt"}`
	}
	for _, typ := range []string{"JWT", "at+jwt", "terminal-session+jwt", "application/agent-invocation+jwt", "Agent-Invocation+JWT", "agent-invocation+jwt "} {
		h["typ "+typ] = `{"alg":"EdDSA","kid":"k1","typ":"` + typ + `"}`
	}
	return h
}

func TestParseInvocationToken_HeaderRules(t *testing.T) {
	t.Parallel()
	payload := validPayload(t)
	p, err := domain.ParseInvocationToken(rawToken(validHeader, payload))
	if err != nil {
		t.Fatalf("valid token: %v", err)
	}
	if p.KeyID() != "k1" {
		t.Errorf("kid %q", p.KeyID())
	}
	k, err := jose.JWKFromPublicKey(testKey.Public())
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Verify(k); err != nil {
		t.Errorf("Verify: %v", err)
	}
	for name, h := range badHeaders() {
		if _, err := domain.ParseInvocationToken(rawToken(h, payload)); !errors.Is(err, domain.ErrInvocationInvalid) {
			t.Errorf("%s: ParseInvocationToken = %v, want ErrInvocationInvalid", name, err)
		}
	}
	for name, tok := range map[string]string{"empty": "", "two segments": "a.b", "none without signature": rawToken(`{"alg":"none","typ":"agent-invocation+jwt","kid":"k1"}`, payload)[:10] + ".."} {
		if _, err := domain.ParseInvocationToken(tok); !errors.Is(err, domain.ErrInvocationInvalid) {
			t.Errorf("%s: ParseInvocationToken = %v", name, err)
		}
	}
}

func TestParsedInvocationToken_Claims(t *testing.T) {
	t.Parallel()
	c := claimsOf(t, grantOf(domain.TriggerInteraction))
	payload, err := c.Payload()
	if err != nil {
		t.Fatal(err)
	}
	p, err := domain.ParseInvocationToken(rawToken(validHeader, payload))
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.Claims()
	if err != nil || got != c {
		t.Fatalf("Claims = %+v, %v; want %+v", got, err, c)
	}
	// An unknown member is ignored (RFC 7519 section 4, ruling D-M2-06-7).
	var m map[string]any
	_ = json.Unmarshal(payload, &m)
	m["future_claim"] = "x"
	extended, _ := json.Marshal(m)
	p, _ = domain.ParseInvocationToken(rawToken(validHeader, extended))
	if got, err := p.Claims(); err != nil || got != c {
		t.Fatalf("with an unknown member: %+v, %v", got, err)
	}
	// Broken payloads are invalid.
	breakers := map[string]func(map[string]any){
		"iss missing":           func(m map[string]any) { delete(m, "iss") },
		"permission_version":    func(m map[string]any) { delete(m, "permission_version") },
		"iat string":            func(m map[string]any) { m["iat"] = "1" },
		"exp fraction":          func(m map[string]any) { m["exp"] = 1.5 },
		"upper-case aud":        func(m map[string]any) { m["aud"] = strings.ToUpper(m["aud"].(string)) },
		"aud array":             func(m map[string]any) { m["aud"] = []string{m["aud"].(string)} },
		"unknown level":         func(m map[string]any) { m["level"] = "owner" },
		"unknown kind":          func(m map[string]any) { m["trigger_kind"] = "mention" },
		"second object":         func(m map[string]any) { m["message_id"] = ids.New().String() },
		"run_id null":           func(m map[string]any) { m["run_id"] = nil },
		"dev_environment_id":    func(m map[string]any) { m["dev_environment_id"] = "not-an-id" },
		"case-folded member":    func(m map[string]any) { m["ISS"] = m["iss"]; delete(m, "iss") },
		"negative version":      func(m map[string]any) { m["permission_version"] = -2 },
		"interaction id number": func(m map[string]any) { m["interaction_id"] = 5 },
	}
	for name, f := range breakers {
		var m map[string]any
		_ = json.Unmarshal(payload, &m)
		f(m)
		b, _ := json.Marshal(m)
		p, err := domain.ParseInvocationToken(rawToken(validHeader, b))
		if err != nil {
			t.Fatalf("%s: parse: %v", name, err)
		}
		if _, err := p.Claims(); !errors.Is(err, domain.ErrInvocationInvalid) {
			t.Errorf("%s: Claims = %v, want ErrInvocationInvalid", name, err)
		}
	}
}

func TestCheckInvocationClaims_Order(t *testing.T) {
	t.Parallel()
	const leeway = 30 * time.Second
	g := grantOf(domain.TriggerMessage)
	c := claimsOf(t, g)
	iss := ids.MustParse(c.Iss)
	e := domain.InvocationExpectation{Issuer: iss, Audience: g.AgentID, RunID: g.RunID}
	iat := time.Unix(c.Iat, 0)
	exp := time.Unix(c.Exp, 0)
	if err := domain.CheckInvocationClaims(c, e, iat, leeway); err != nil {
		t.Fatalf("valid: %v", err)
	}
	// Expired and a foreign aud: invalid, not expired.
	foreign := e
	foreign.Audience = ids.New()
	if err := domain.CheckInvocationClaims(c, foreign, exp.Add(time.Hour), leeway); !errors.Is(err, domain.ErrInvocationInvalid) {
		t.Errorf("expired with a foreign aud: %v, want invalid", err)
	}
	for name, ex := range map[string]domain.InvocationExpectation{
		"foreign iss": {Issuer: ids.New(), Audience: g.AgentID, RunID: g.RunID},
		"wrong run":   {Issuer: iss, Audience: g.AgentID, RunID: ids.New()},
	} {
		if err := domain.CheckInvocationClaims(c, ex, iat, leeway); !errors.Is(err, domain.ErrInvocationInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// No run expected: any run passes.
	if err := domain.CheckInvocationClaims(c, domain.InvocationExpectation{Issuer: iss, Audience: g.AgentID}, iat, leeway); err != nil {
		t.Errorf("without a run expectation: %v", err)
	}
	// iat exactly at now plus leeway passes, one second later fails.
	if err := domain.CheckInvocationClaims(c, e, iat.Add(-leeway), leeway); err != nil {
		t.Errorf("iat at now plus leeway: %v", err)
	}
	if err := domain.CheckInvocationClaims(c, e, iat.Add(-leeway-time.Second), leeway); !errors.Is(err, domain.ErrInvocationInvalid) {
		t.Errorf("iat one second after now plus leeway: %v", err)
	}
	// exp plus leeway one second after now passes; exactly now is expired.
	if err := domain.CheckInvocationClaims(c, e, exp.Add(leeway-time.Second), leeway); err != nil {
		t.Errorf("one second before exp plus leeway: %v", err)
	}
	last := exp.Add(leeway)
	if err := domain.CheckInvocationClaims(c, e, last, leeway); !errors.Is(err, domain.ErrInvocationExpired) {
		t.Errorf("at exp plus leeway: %v, want expired", err)
	}
	ignore := e
	ignore.IgnoreExpiry = true
	if err := domain.CheckInvocationClaims(c, ignore, last.Add(24*time.Hour), leeway); err != nil {
		t.Errorf("IgnoreExpiry: %v", err)
	}
	// exp not after iat is invalid, also with IgnoreExpiry.
	bad := c
	bad.Exp = bad.Iat
	if err := domain.CheckInvocationClaims(bad, ignore, iat, leeway); !errors.Is(err, domain.ErrInvocationInvalid) {
		t.Errorf("exp equal to iat: %v", err)
	}
}

func FuzzParseInvocationToken(f *testing.F) {
	payload := validPayload(f)
	f.Add(rawToken(validHeader, payload))
	for _, h := range badHeaders() {
		f.Add(rawToken(h, payload))
	}
	f.Add("")
	f.Add("..")
	f.Fuzz(func(t *testing.T, s string) {
		p, err := domain.ParseInvocationToken(s)
		if err != nil {
			if !errors.Is(err, domain.ErrInvocationInvalid) {
				t.Fatalf("error %v does not wrap ErrInvocationInvalid", err)
			}
			return
		}
		head, _, _ := strings.Cut(s, ".")
		raw, derr := base64.RawURLEncoding.Strict().DecodeString(head)
		var h map[string]any
		if derr != nil || json.Unmarshal(raw, &h) != nil {
			t.Fatalf("accepted a token whose header does not decode: %q", s)
		}
		if h["alg"] != "EdDSA" || h["typ"] != "agent-invocation+jwt" || p.KeyID() == "" || h["kid"] != p.KeyID() {
			t.Fatalf("accepted header %v", h)
		}
		if _, ok := h["crit"]; ok {
			t.Fatalf("accepted crit: %v", h)
		}
		_, _ = p.Claims()
	})
}
