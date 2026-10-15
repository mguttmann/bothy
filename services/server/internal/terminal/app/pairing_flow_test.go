// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi-validator/schema_validation"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/apitoken"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
	"github.com/OWNER/bothy/services/server/test/terminalworld"
)

// hostService plays the host service of a host in the tests: fresh X25519 and Ed25519 keys.
type hostService struct {
	noise, auth []byte
}

// newHostService generates the two key pairs; the private keys never leave the test.
func newHostService(t *testing.T) hostService {
	t.Helper()
	x, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return hostService{noise: x.PublicKey().Bytes(), auth: pub}
}

// fingerprint is the fingerprint of the keys of h.
func (h hostService) fingerprint(t *testing.T) string {
	t.Helper()
	fp, err := domain.ComputeFingerprint(h.noise, h.auth)
	if err != nil {
		t.Fatal(err)
	}
	return fp
}

// prefix is the first eight characters of the compact fingerprint in lower case.
func (h hostService) prefix(t *testing.T) string {
	return strings.ToLower(domain.CompactFingerprint(h.fingerprint(t))[:8])
}

// pairRequest is the body of pairHost, editable per test.
type pairRequest map[string]any

// request returns the body of pairHost with code: platform linux, the profiles shell and claude
// and the session users bothy-s-robin, bothy-s-alex and bothy-s-sam with the label "Project A".
func (h hostService) request(code string) pairRequest {
	enc := base64.RawURLEncoding
	users := []any{}
	for _, n := range []string{"bothy-s-robin", "bothy-s-alex", "bothy-s-sam"} {
		users = append(users, map[string]any{"os_user": n, "workdirs": []any{"Project A"}})
	}
	return pairRequest{
		"pairing_code": code, "noise_public_key": enc.EncodeToString(h.noise),
		"auth_public_jwk": map[string]any{"kty": "OKP", "crv": "Ed25519", "x": enc.EncodeToString(h.auth)},
		"platform":        "linux", "os_version": "Debian 13", "host_version": "1.0.0",
		"profiles": []any{
			map[string]any{"name": "shell", "display_name": "Shell", "kind": "shell"},
			map[string]any{"name": "claude", "display_name": "Claude Code", "kind": "claude"},
		},
		"session_users": users,
	}
}

// json encodes r.
func (r pairRequest) json(t *testing.T) string {
	t.Helper()
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// pairWorld is the world of the brief: Robin and Alex are owners, Sam is a person without owner
// rights; the hosts come from createHost.
type pairWorld struct {
	*terminalworld.World
	t *testing.T
}

// newPairWorld builds the world without fixture hosts.
func newPairWorld(t *testing.T) *pairWorld {
	t.Helper()
	w := terminalworld.NewEmpty(t)
	w.Robin = w.Person("robin", true)
	w.Alex = w.Person("alex", true)
	w.Sam = w.Person("sam", false)
	// The instance row that bothy migrate up writes (pairHost answers its instance_id).
	w.SQL(`INSERT INTO instance (instance_id, schema_version, sync_epoch) VALUES ($1, 1, $2) ON CONFLICT DO NOTHING`, ids.New().String(), ids.New().String())
	return &pairWorld{World: w, t: t}
}

// create creates a host named name as Robin and returns its id and pairing code.
func (p *pairWorld) create(name string) (ids.ID, string) {
	p.t.Helper()
	r := p.Do("robin", http.MethodPost, terminalworld.HostsPath, `{"name":"`+name+`","reason":"New host"}`, terminalworld.Key()...)
	if r.Header.Get("Cache-Control") != "no-store" {
		p.t.Fatalf("createHost without Cache-Control: no-store")
	}
	m := p.Want(r, http.StatusCreated, "")
	return ids.MustParse(m["host_id"].(string)), m["pairing_code"].(string)
}

// pair redeems code with the keys of h as the host service (anonymous).
func (p *pairWorld) pair(body string) *httpservertest.Response {
	return p.Do("", http.MethodPost, terminalworld.HostsPath+"/pair", body)
}

// repair issues a new pairing code as Robin and returns it.
func (p *pairWorld) repair(id ids.ID) string {
	p.t.Helper()
	r := p.Do("robin", http.MethodPost, terminalworld.HostPath(id)+"/pairing-code", `{"reason":"Host reinstalled"}`, terminalworld.Key()...)
	if r.Header.Get("Cache-Control") != "no-store" {
		p.t.Fatalf("issueHostPairingCode without Cache-Control: no-store")
	}
	return p.Want(r, http.StatusCreated, "")["pairing_code"].(string)
}

// confirmBody is a body of confirmHost with prefix, the declaration organization, no server host
// and the assignments.
func confirmBody(prefix string, assign map[ids.ID]string) string {
	users := []map[string]string{}
	for u, n := range assign {
		users = append(users, map[string]string{"user_id": u.String(), "os_user": n})
	}
	b, _ := json.Marshal(map[string]any{
		"fingerprint_prefix": prefix, "llm_credentials": "organization", "declared_server_host": false,
		"session_users": users, "reason": "Build machine",
	})
	return string(b)
}

// confirm sends a confirmation as Robin with the current If-Match and a fresh key.
func (p *pairWorld) confirm(id ids.ID, body string) *httpservertest.Response {
	return p.Do("robin", http.MethodPost, terminalworld.HostPath(id)+"/confirm", body, p.Keyed(id)...)
}

// active creates, pairs and confirms a host with Robin on bothy-s-robin; it returns the id and the
// host service.
func (p *pairWorld) active(name string) (ids.ID, hostService) {
	p.t.Helper()
	id, code := p.create(name)
	hs := newHostService(p.t)
	p.Want(p.pair(hs.request(code).json(p.t)), http.StatusCreated, "")
	p.Want(p.confirm(id, confirmBody(hs.prefix(p.t), map[ids.ID]string{p.Robin: "bothy-s-robin"})), http.StatusOK, "")
	return id, hs
}

// state reads hosts.state.
func (p *pairWorld) state(id ids.ID) string {
	return p.Text(`SELECT state FROM hosts WHERE id = $1`, id.String())
}

// failures reads hosts.fingerprint_failures.
func (p *pairWorld) failures(id ids.ID) int {
	return p.Count(`SELECT fingerprint_failures FROM hosts WHERE id = $1`, id.String())
}

// keysNull reports the three key columns NULL.
func (p *pairWorld) keysNull(id ids.ID) bool {
	return p.Count(`SELECT count(*) FROM hosts WHERE id = $1 AND noise_public_key IS NULL AND auth_public_jwk IS NULL AND auth_jkt IS NULL`, id.String()) == 1
}

// hmacOf is the HMAC of a code with the pepper of the world.
func hmacOf(code string) []byte { return apitoken.Hash(terminalworld.Pepper, code) }

// leaked returns a short label of s for failure messages: never the value itself.
func leaked(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:4])
}

// assertNowhere checks that s appears in no stored row, event, idempotency row, job or log line.
func (p *pairWorld) assertNowhere(s, what string) {
	p.t.Helper()
	p.AssertNowhere(s, what+" "+leaked(s))
	queries := []string{`SELECT count(*) FROM hosts WHERE row_to_json(hosts)::text LIKE '%' || $1 || '%'`}
	// River creates its tables when the job runner starts; this world has none, so no job holds it.
	if p.Count(`SELECT count(*) FROM pg_tables WHERE tablename = 'river_job'`) == 1 {
		queries = append(queries, `SELECT count(*) FROM river_job WHERE args::text LIKE '%' || $1 || '%'`)
	}
	for _, q := range queries {
		if n := p.Count(q, s); n != 0 {
			p.t.Errorf("%s %s found in %s", what, leaked(s), q)
		}
	}
}

// auditAndLogsFree checks that s appears in no audit entry and no log line.
func (p *pairWorld) auditAndLogsFree(s, what string) {
	p.t.Helper()
	if n := p.Count(`SELECT count(*) FROM audit_events WHERE details::text LIKE '%' || $1 || '%'`, s); n != 0 {
		p.t.Errorf("%s in %d audit entries", what, n)
	}
	for _, r := range append(p.H.Logs.Records(), p.Logs.Records()...) {
		if strings.Contains(r.Message, s) {
			p.t.Errorf("%s in a log message", what)
		}
		for _, v := range r.Attrs {
			if strings.Contains(v, s) {
				p.t.Errorf("%s in a log attribute", what)
			}
		}
	}
}

// validateSchema validates raw against the schema name of the contract.
func validateSchema(t *testing.T, name string, raw []byte) {
	t.Helper()
	doc, err := libopenapi.NewDocument(rest.Spec)
	if err != nil {
		t.Fatal(err)
	}
	model, err := doc.BuildV3Model()
	if err != nil {
		t.Fatal(err)
	}
	proxy, ok := model.Model.Components.Schemas.Get(name)
	if !ok {
		t.Fatalf("no schema %s", name)
	}
	if valid, problems := schema_validation.NewSchemaValidator().ValidateSchemaBytes(proxy.Schema(), raw); !valid {
		for _, pr := range problems {
			t.Errorf("schema %s: %s %s", name, pr.Message, pr.Reason)
		}
	}
}

// TestPairingCode_OnlyHMACStored (03 point 9, 24 section 3.2): after createHost and after
// issueHostPairingCode the code is stored nowhere, pairing_code_hash is its HMAC, and both answers
// carry Cache-Control: no-store.
func TestPairingCode_OnlyHMACStored(t *testing.T) {
	t.Parallel()
	p := newPairWorld(t)
	id, code := p.create("Build server")
	again := p.repair(id)
	for _, c := range []string{code, again} {
		p.assertNowhere(c, "pairing code")
	}
	var hash []byte
	if err := p.Owner.QueryRow(context.Background(), `SELECT pairing_code_hash FROM hosts WHERE id = $1`, id.String()).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if !hmac.Equal(hash, hmacOf(again)) || hmac.Equal(hash, hmacOf(code)) {
		t.Fatal("pairing_code_hash is not the HMAC of the current code")
	}
}

// TestPairHost_HappyPath (24 section 3.2, Design note 3): 201 with the fingerprint and the
// instance; the host is confirming with its keys and reports; the rights version rises by one;
// one host.updated without key_changed; one audit allow host.pair.
func TestPairHost_HappyPath(t *testing.T) {
	t.Parallel()
	p := newPairWorld(t)
	id, code := p.create("Build server")
	hs := newHostService(t)
	before, events := p.PermissionVersion(), len(p.Events("host.updated"))
	m := p.Want(p.pair(hs.request(code).json(t)), http.StatusCreated, "")
	if m["host_id"] != id.String() || m["key_fingerprint"] != hs.fingerprint(t) || m["instance_id"] == nil || len(m) != 3 {
		t.Fatalf("answer %v", m)
	}
	if p.state(id) != "confirming" || p.PermissionVersion() != before+1 {
		t.Fatalf("state %s, rights version %d after %d", p.state(id), p.PermissionVersion(), before)
	}
	var jwk, jkt, platform, osVersion, hostVersion, profiles string
	var gen int
	var noise []byte
	if err := p.Owner.QueryRow(context.Background(), `SELECT noise_public_key, auth_public_jwk::text, auth_jkt, key_generation, platform,
		reported_os_version, reported_host_version, reported_profiles::text FROM hosts WHERE id = $1 AND paired_at IS NOT NULL
		AND pairing_code_hash IS NULL AND pairing_expires_at IS NULL AND key_fingerprint = $2`, id.String(), hs.fingerprint(t)).
		Scan(&noise, &jwk, &jkt, &gen, &platform, &osVersion, &hostVersion, &profiles); err != nil {
		t.Fatal(err)
	}
	var members map[string]any
	_ = json.Unmarshal([]byte(jwk), &members)
	if !hmac.Equal(noise, hs.noise) || len(members) != 3 || members["kty"] != "OKP" || gen != 1 || platform != "linux" ||
		osVersion != "Debian 13" || hostVersion != "1.0.0" || !strings.Contains(profiles, `"claude"`) || jkt == "" {
		t.Fatalf("stored pairing: gen %d, platform %s, jwk members %d", gen, platform, len(members))
	}
	evs := p.Events("host.updated")
	if len(evs) != events+1 {
		t.Fatalf("%d host.updated", len(evs)-events)
	}
	if _, ok := evs[len(evs)-1].Payload["key_changed"]; ok {
		t.Fatal("pairing sets key_changed")
	}
	allows := p.Allows("host.pair")
	if len(allows) != 1 || allows[0].ResourceID != id.String() || allows[0].Details["fingerprint"] != strings.ToLower(domain.CompactFingerprint(hs.fingerprint(t))) ||
		allows[0].Details["key_generation"] != float64(1) || allows[0].Details["platform"] != "linux" {
		t.Fatalf("audit %+v", allows)
	}
}

// TestPairHost_StoresReportedSessionUsers (PLAN row): reported_session_users equals the report in
// its order, and getHost shows it.
func TestPairHost_StoresReportedSessionUsers(t *testing.T) {
	t.Parallel()
	p := newPairWorld(t)
	id, code := p.create("Build server")
	hs := newHostService(t)
	p.Want(p.pair(hs.request(code).json(t)), http.StatusCreated, "")
	want := `[{"os_user": "bothy-s-robin", "workdirs": ["Project A"]}, {"os_user": "bothy-s-alex", "workdirs": ["Project A"]}, {"os_user": "bothy-s-sam", "workdirs": ["Project A"]}]`
	if got := p.Text(`SELECT reported_session_users::text FROM hosts WHERE id = $1`, id.String()); got != want {
		t.Fatalf("reported_session_users %s", got)
	}
	host := p.Want(p.Do("robin", http.MethodGet, terminalworld.HostPath(id), ""), http.StatusOK, "")
	list, _ := host["reported_session_users"].([]any)
	if len(list) != 3 || list[0].(map[string]any)["os_user"] != "bothy-s-robin" || list[2].(map[string]any)["workdirs"].([]any)[0] != "Project A" {
		t.Fatalf("getHost reported_session_users %v", list)
	}
}

// TestTMT20_PairingCodeRejections (TM-T20): a used, expired, mistyped, replaced or unknown code
// gives 401 unauthenticated with equal bodies, the host stays unchanged, and the denials of
// host.pair are aggregated in the audit.
func TestTMT20_PairingCodeRejections(t *testing.T) {
	t.Parallel()
	p := newPairWorld(t)
	used, usedCode := p.create("Used")
	p.Want(p.pair(newHostService(t).request(usedCode).json(t)), http.StatusCreated, "")
	expired, expiredCode := p.create("Expired")
	replaced, replacedCode := p.create("Replaced")
	p.repair(replaced)
	unknown, err := apitoken.Generate(nil, apitoken.KindHostPairingCode)
	if err != nil {
		t.Fatal(err)
	}
	last := byte('A')
	if unknown[len(unknown)-1] == last {
		last = 'B'
	}
	mistyped := unknown[:len(unknown)-1] + string(last)
	versions := map[ids.ID]int64{used: p.HostVersion(used), expired: p.HostVersion(expired), replaced: p.HostVersion(replaced)}
	p.Clk.Advance(terminalworld.HostConfig.PairingCodeTTL + time.Second)
	var first *httpservertest.Response
	for _, code := range []string{usedCode, expiredCode, mistyped, replacedCode, unknown} {
		r := p.pair(newHostService(t).request(code).json(t))
		p.Want(r, http.StatusUnauthorized, "unauthenticated")
		if first == nil {
			first = r
			continue
		}
		if string(terminalworld.StripCorrelation(first.Body)) != string(terminalworld.StripCorrelation(r.Body)) {
			t.Fatalf("bodies differ: %s / %s", first.Body, r.Body)
		}
	}
	for id, v := range versions {
		if p.HostVersion(id) != v {
			t.Errorf("host %s changed", id)
		}
	}
	denials := 0
	for _, r := range p.Audits("host.pair") {
		if r.Decision == "deny" && r.Reason == "unauthenticated" {
			denials++
		}
	}
	if denials == 0 {
		t.Fatal("no aggregated denial of host.pair")
	}
}

// TestPairHost_ReportRules (Design note 4): one case per rule group gives 400 with its pointer
// and code, and the same code then pairs.
func TestPairHost_ReportRules(t *testing.T) {
	t.Parallel()
	p := newPairWorld(t)
	_, code := p.create("Build server")
	hs := newHostService(t)
	cases := []struct {
		name          string
		edit          func(r pairRequest)
		pointer, code string
	}{
		{"noise key", func(r pairRequest) { r["noise_public_key"] = base64.RawURLEncoding.EncodeToString(make([]byte, 32)) }, "/noise_public_key", "invalid_key"},
		{"jwk member", func(r pairRequest) { r["auth_public_jwk"].(map[string]any)["kid"] = "k1" }, "/auth_public_jwk/kid", "unknown_member"},
		{"no session users", func(r pairRequest) { r["session_users"] = []any{} }, "/session_users", "empty"},
		{"duplicate names", func(r pairRequest) {
			r["session_users"] = []any{map[string]any{"os_user": "a", "workdirs": []any{}}, map[string]any{"os_user": "A", "workdirs": []any{}}}
		}, "/session_users/1/os_user", "duplicate"},
		{"label too long", func(r pairRequest) {
			r["profiles"].([]any)[0].(map[string]any)["display_name"] = strings.Repeat("d", terminalworld.HostConfig.ReportLabelMaxChars+1)
		}, "/profiles/0/display_name", "too_long"},
		{"too many entries", func(r pairRequest) {
			list := []any{}
			for i := range terminalworld.HostConfig.ReportMaxEntries + 1 {
				list = append(list, map[string]any{"os_user": "user" + string(rune('a'+i)), "workdirs": []any{}})
			}
			r["session_users"] = list
		}, "/session_users", "limit_reached"},
		{"host version", func(r pairRequest) { r["host_version"] = "v1.0.0" }, "/host_version", "invalid_format"},
	}
	for _, c := range cases {
		body := hs.request(code)
		c.edit(body)
		pr := p.Want(p.pair(body.json(t)), http.StatusBadRequest, "validation_failed")
		found := false
		for _, e := range pr["errors"].([]any) {
			f := e.(map[string]any)
			found = found || f["pointer"] == c.pointer && f["code"] == c.code
		}
		if !found {
			t.Errorf("%s: %v", c.name, pr["errors"])
		}
	}
	p.Want(p.pair(hs.request(code).json(t)), http.StatusCreated, "")
}

// TestPairHost_RejectsPrivateJWKMember: d is 400 unknown_member at /auth_public_jwk/d, and its value
// is stored and logged nowhere.
func TestPairHost_RejectsPrivateJWKMember(t *testing.T) {
	t.Parallel()
	p := newPairWorld(t)
	_, code := p.create("Build server")
	body := newHostService(t).request(code)
	secret := base64.RawURLEncoding.EncodeToString([]byte("EXAMPLE private key material 01"))
	body["auth_public_jwk"].(map[string]any)["d"] = secret
	pr := p.Want(p.pair(body.json(t)), http.StatusBadRequest, "validation_failed")
	if c, ptr := terminalworld.Field(pr); c != "unknown_member" || ptr != "/auth_public_jwk/d" {
		t.Fatalf("field %s at %s", c, ptr)
	}
	if strings.Contains(string(mustJSON(t, pr)), secret) {
		t.Fatal("the problem repeats the private member")
	}
	p.assertNowhere(secret, "private JWK member")
}

// mustJSON encodes v.
func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestPairHost_KeyInUse: the Ed25519 key of another host is 409 conflict host_key_in_use, and the
// code stays valid.
func TestPairHost_KeyInUse(t *testing.T) {
	t.Parallel()
	p := newPairWorld(t)
	_, first := p.create("First")
	_, second := p.create("Second")
	a := newHostService(t)
	p.Want(p.pair(a.request(first).json(t)), http.StatusCreated, "")
	b := newHostService(t)
	b.auth = a.auth
	pr := p.Want(p.pair(b.request(second).json(t)), http.StatusConflict, "conflict")
	if pr["reason"] != "host_key_in_use" {
		t.Fatalf("reason %v", pr["reason"])
	}
	p.Want(p.pair(newHostService(t).request(second).json(t)), http.StatusCreated, "")
}

// TestConfirmHost_HappyPath (24 section 3.2): Robin confirms with the lower-case prefix,
// organization, server host and two assignments; the host is active with its ETag and the stored
// columns, the rights version rose, host.updated and the confirm entry exist, and no step-up was
// consumed.
func TestConfirmHost_HappyPath(t *testing.T) {
	t.Parallel()
	p := newPairWorld(t)
	id, code := p.create("Build server")
	hs := newHostService(t)
	p.Want(p.pair(hs.request(code).json(t)), http.StatusCreated, "")
	before, events := p.PermissionVersion(), len(p.Events("host.updated"))
	body := strings.Replace(confirmBody(hs.prefix(t), map[ids.ID]string{p.Robin: "bothy-s-robin", p.Sam: "bothy-s-sam"}),
		`"declared_server_host":false`, `"declared_server_host":true`, 1)
	r := p.confirm(id, body)
	m := p.Want(r, http.StatusOK, "")
	if m["state"] != "active" || r.Header.Get("ETag") == "" || m["declared_server_host"] != true || len(m["session_users"].([]any)) != 2 {
		t.Fatalf("answer %v", m)
	}
	if n := p.Count(`SELECT count(*) FROM hosts WHERE id = $1 AND confirmed_by = $2 AND confirmed_at IS NOT NULL AND llm_credentials = 'organization'
		AND llm_credentials_declared_by = $2 AND declared_server_host AND fingerprint_failures = 0`, id.String(), p.Robin.String()); n != 1 {
		t.Fatal("confirmation columns not stored")
	}
	if p.Count(`SELECT count(*) FROM host_session_users WHERE host_id = $1 AND revoked_at IS NULL`, id.String()) != 2 {
		t.Fatal("assignments not stored")
	}
	if p.PermissionVersion() <= before || len(p.Events("host.updated")) != events+1 {
		t.Fatal("rights version or host.updated missing")
	}
	var confirm int
	for _, a := range p.Allows("host.manage") {
		if a.Details["operation"] == "confirm" && a.Details["result"] == "confirmed" && a.Details["llm_credentials"] == "organization" {
			confirm++
		}
	}
	if confirm != 1 || p.Count(`SELECT count(*) FROM auth_sessions WHERE step_up_consumed_at IS NOT NULL`) != 0 {
		t.Fatalf("%d confirm entries, or a step-up was consumed", confirm)
	}
}

// TestTMT19_FingerprintMaxAttempts (TM-T19), with a fresh Idempotency-Key per attempt and without
// the header: every wrong prefix counts down attempts_remaining with the same If-Match; the last
// one falls back to pairing without keys; the old code and the right prefix no longer work; a new
// code and new keys pair again with key_generation 2 and confirm.
func TestTMT19_FingerprintMaxAttempts(t *testing.T) {
	t.Parallel()
	for _, keyed := range []bool{true, false} {
		p := newPairWorld(t)
		id, code := p.create("Build server")
		hs := newHostService(t)
		p.Want(p.pair(hs.request(code).json(t)), http.StatusCreated, "")
		v := p.HostVersion(id)
		wrong := "zzzzzzzz"
		if strings.HasPrefix(hs.prefix(t), wrong) {
			wrong = "yyyyyyyy"
		}
		maxAttempts := terminalworld.HostConfig.FingerprintMaxAttempts
		for i := 1; i <= maxAttempts; i++ {
			headers := terminalworld.IfMatch(v)
			if keyed {
				headers = append(headers, terminalworld.Key()...)
			}
			pr := p.Want(p.Do("robin", http.MethodPost, terminalworld.HostPath(id)+"/confirm", confirmBody(wrong, nil), headers...), http.StatusConflict, "fingerprint_mismatch")
			if pr["attempts_remaining"] != float64(maxAttempts-i) {
				t.Fatalf("keyed %v attempt %d: attempts_remaining %v", keyed, i, pr["attempts_remaining"])
			}
			if i < maxAttempts && (p.HostVersion(id) != v || p.failures(id) != i) {
				t.Fatalf("keyed %v attempt %d: version %d, failures %d", keyed, i, p.HostVersion(id), p.failures(id))
			}
		}
		if p.state(id) != "pairing" || !p.keysNull(id) || p.failures(id) != 0 {
			t.Fatalf("keyed %v: state %s after the last attempt", keyed, p.state(id))
		}
		p.Want(p.pair(newHostService(t).request(code).json(t)), http.StatusUnauthorized, "unauthenticated")
		p.Want(p.confirm(id, confirmBody(hs.prefix(t), nil)), http.StatusConflict, "invalid_state_transition")
		evs := p.Events("host.updated")
		if evs[len(evs)-1].Payload["key_changed"] != true {
			t.Fatalf("keyed %v: the fallback event has no key_changed", keyed)
		}
		again := newHostService(t)
		p.Want(p.pair(again.request(p.repair(id)).json(t)), http.StatusCreated, "")
		if p.Count(`SELECT key_generation FROM hosts WHERE id = $1`, id.String()) != 2 {
			t.Fatalf("keyed %v: key_generation", keyed)
		}
		p.Want(p.confirm(id, confirmBody(again.prefix(t), nil)), http.StatusOK, "")
	}
}

// TestConfirmHost_WrongPrefixFormatNotCounted (Design note 6): a prefix with O or of the wrong
// length is 400 invalid_format and no attempt; a keyed mismatch counts once and its replay returns
// the stored 409 without counting again.
func TestConfirmHost_WrongPrefixFormatNotCounted(t *testing.T) {
	t.Parallel()
	p := newPairWorld(t)
	id, code := p.create("Build server")
	hs := newHostService(t)
	p.Want(p.pair(hs.request(code).json(t)), http.StatusCreated, "")
	for _, bad := range []string{"OOOOOOOO", "ABC"} {
		pr := p.Want(p.confirm(id, confirmBody(bad, nil)), http.StatusBadRequest, "validation_failed")
		if c, ptr := terminalworld.Field(pr); c != "invalid_format" || ptr != "/fingerprint_prefix" {
			t.Fatalf("%s: %s at %s", bad, c, ptr)
		}
	}
	if p.failures(id) != 0 {
		t.Fatal("a malformed prefix counted")
	}
	wrong := "zzzzzzzz"
	if strings.HasPrefix(hs.prefix(t), wrong) {
		wrong = "yyyyyyyy"
	}
	headers := p.Keyed(id)
	first := p.Do("robin", http.MethodPost, terminalworld.HostPath(id)+"/confirm", confirmBody(wrong, nil), headers...)
	p.Want(first, http.StatusConflict, "fingerprint_mismatch")
	if p.failures(id) != 1 {
		t.Fatalf("failures %d after one mismatch", p.failures(id))
	}
	replay := p.Do("robin", http.MethodPost, terminalworld.HostPath(id)+"/confirm", confirmBody(wrong, nil), headers...)
	p.Want(replay, http.StatusConflict, "fingerprint_mismatch")
	if replay.Header.Get("Idempotent-Replay") != "true" || p.failures(id) != 1 {
		t.Fatalf("replay header %q, failures %d", replay.Header.Get("Idempotent-Replay"), p.failures(id))
	}
}

// confirming pairs a new host with fresh keys and returns it with its host service.
func (p *pairWorld) confirming(name string) (ids.ID, hostService) {
	p.t.Helper()
	id, code := p.create(name)
	hs := newHostService(p.t)
	p.Want(p.pair(hs.request(code).json(p.t)), http.StatusCreated, "")
	return id, hs
}

// unchanged checks that a refused confirmation stored nothing.
func (p *pairWorld) unchanged(id ids.ID, rows int, what string) {
	p.t.Helper()
	if p.state(id) != "confirming" || p.Count(`SELECT count(*) FROM host_session_users WHERE host_id = $1`, id.String()) != rows ||
		p.Count(`SELECT count(*) FROM hosts WHERE id = $1 AND llm_credentials_declared_by IS NULL`, id.String()) != 1 && rows == 0 ||
		p.failures(id) != 0 {
		p.t.Fatalf("%s: the refused confirmation changed the host", what)
	}
}

// TestTMT28b_ConfirmSessionUserTaken (TM-T28 (b) over confirmHost, ruling T12).
func TestTMT28b_ConfirmSessionUserTaken(t *testing.T) {
	t.Parallel()
	p := newPairWorld(t)
	// (1) A name that belonged to Sam goes to Alex after a re-pairing.
	id, code := p.create("Build server")
	hs := newHostService(t)
	p.Want(p.pair(hs.request(code).json(t)), http.StatusCreated, "")
	p.Want(p.confirm(id, confirmBody(hs.prefix(t), map[ids.ID]string{p.Sam: "bothy-s-sam"})), http.StatusOK, "")
	p.Want(p.Do("robin", http.MethodPatch, terminalworld.HostPath(id), `{"revoke_session_users":["`+p.Sam.String()+`"],"reason":"Sam left"}`, p.Keyed(id)...), http.StatusOK, "")
	again := newHostService(t)
	p.Want(p.pair(again.request(p.repair(id)).json(t)), http.StatusCreated, "")
	pr := p.Want(p.confirm(id, confirmBody(again.prefix(t), map[ids.ID]string{p.Alex: "bothy-s-sam"})), http.StatusConflict, "conflict")
	if pr["reason"] != "session_user_taken" {
		t.Fatalf("(1) reason %v", pr["reason"])
	}
	p.unchanged(id, 1, "(1)")
	// (2) Sam and Alex on one name; (3) Sam on two names.
	two, hs2 := p.confirming("Second")
	body := `{"fingerprint_prefix":"` + hs2.prefix(t) + `","llm_credentials":"organization","declared_server_host":false,"session_users":[` +
		`{"user_id":"` + p.Sam.String() + `","os_user":"bothy-s-sam"},{"user_id":"` + p.Alex.String() + `","os_user":"bothy-s-sam"}],"reason":"x"}`
	if pr := p.Want(p.confirm(two, body), http.StatusConflict, "conflict"); pr["reason"] != "session_user_taken" {
		t.Fatalf("(2) %v", pr)
	}
	p.unchanged(two, 0, "(2)")
	body = `{"fingerprint_prefix":"` + hs2.prefix(t) + `","llm_credentials":"organization","declared_server_host":false,"session_users":[` +
		`{"user_id":"` + p.Sam.String() + `","os_user":"bothy-s-sam"},{"user_id":"` + p.Sam.String() + `","os_user":"bothy-s-alex"}],"reason":"x"}`
	if pr := p.Want(p.confirm(two, body), http.StatusConflict, "conflict"); pr["reason"] != "session_user_taken" {
		t.Fatalf("(3) %v", pr)
	}
	p.unchanged(two, 0, "(3)")
	// (4) Robin kept his assignment through a re-pairing and gets a second one.
	three, hs3 := p.active("Third")
	next := newHostService(t)
	p.Want(p.pair(next.request(p.repair(three)).json(t)), http.StatusCreated, "")
	_ = hs3
	if pr := p.Want(p.confirm(three, confirmBody(next.prefix(t), map[ids.ID]string{p.Robin: "bothy-s-alex"})), http.StatusConflict, "conflict"); pr["reason"] != "session_user_taken" {
		t.Fatalf("(4) %v", pr)
	}
	if p.state(three) != "confirming" || p.Count(`SELECT count(*) FROM host_session_users WHERE host_id = $1`, three.String()) != 1 {
		t.Fatal("(4) changed the host")
	}
}

// TestTMT28e_ConfirmNameNotReported (TM-T28 (e) over confirmHost): a name the host did not report
// is 400 session_user_not_on_host at /session_users/0/os_user, and nothing is stored.
func TestTMT28e_ConfirmNameNotReported(t *testing.T) {
	t.Parallel()
	p := newPairWorld(t)
	id, hs := p.confirming("Build server")
	pr := p.Want(p.confirm(id, confirmBody(hs.prefix(t), map[ids.ID]string{p.Sam: "bothy-s-other"})), http.StatusBadRequest, "validation_failed")
	if c, ptr := terminalworld.Field(pr); c != "session_user_not_on_host" || ptr != "/session_users/0/os_user" {
		t.Fatalf("%s at %s", c, ptr)
	}
	p.unchanged(id, 0, "(e)")
}

// TestConfirmHost_I6Violation (TM-T28, 03 section 6.8): with a grant terminal to Sam, personal with
// Robin is 409 i6_violation, the host stays confirming and the deny entry exists; organization
// confirms.
func TestConfirmHost_I6Violation(t *testing.T) {
	t.Parallel()
	p := newPairWorld(t)
	id, hs := p.confirming("Build server")
	p.Want(p.Do("robin", http.MethodPost, "/api/v1/grants", `{"subject_kind":"user","subject_id":"`+p.Sam.String()+
		`","target_kind":"host","target_id":"`+id.String()+`","capabilities":["terminal"],"reason":"Sam works there"}`, terminalworld.Key()...), http.StatusCreated, "")
	body := strings.Replace(confirmBody(hs.prefix(t), nil), `"llm_credentials":"organization"`,
		`"llm_credentials":"personal","credential_holder_id":"`+p.Robin.String()+`"`, 1)
	p.Want(p.confirm(id, body), http.StatusConflict, "i6_violation")
	if p.state(id) != "confirming" {
		t.Fatal("the host left confirming")
	}
	denied := false
	for _, r := range p.Audits("host.manage") {
		denied = denied || r.Decision == "deny" && r.Reason == "i6_violation"
	}
	if !denied {
		t.Fatal("no deny entry of i6_violation")
	}
	p.Want(p.confirm(id, confirmBody(hs.prefix(t), nil)), http.StatusOK, "")
}

// TestConfirmHost_RightsAndPreconditions: 428, 412 with current, 409 for pairing and active,
// 403 owner_only for the assignee Sam, 404 for Sam on a foreign host, 401 without step-up.
func TestConfirmHost_RightsAndPreconditions(t *testing.T) {
	t.Parallel()
	p := newPairWorld(t)
	id, hs := p.confirming("Build server")
	path := terminalworld.HostPath(id) + "/confirm"
	body := confirmBody(hs.prefix(t), nil)
	p.Want(p.Do("robin", http.MethodPost, path, body, terminalworld.Key()...), http.StatusPreconditionRequired, "precondition_required")
	if pr := p.Want(p.Do("robin", http.MethodPost, path, body, append(terminalworld.IfMatch(p.HostVersion(id)+5), terminalworld.Key()...)...),
		http.StatusPreconditionFailed, "version_conflict"); pr["current"] == nil {
		t.Fatal("412 without current")
	}
	pairing, _ := p.create("Pairing")
	p.Want(p.confirm(pairing, body), http.StatusConflict, "invalid_state_transition")
	active, _ := p.active("Active")
	p.Want(p.confirm(active, body), http.StatusConflict, "invalid_state_transition")
	p.Want(p.Do("robin", http.MethodPatch, terminalworld.HostPath(id), `{"assigned_user_id":"`+p.Sam.String()+`","reason":"Sam's"}`, p.Keyed(id)...), http.StatusOK, "")
	if pr := p.Want(p.Do("sam", http.MethodPost, path, body, p.Keyed(id)...), http.StatusForbidden, "forbidden"); pr["reason"] != "owner_only" {
		t.Fatalf("assignee: %v", pr)
	}
	other, _ := p.confirming("Other")
	foreign := p.Do("sam", http.MethodPost, terminalworld.HostPath(other)+"/confirm", body, p.Keyed(other)...)
	random := p.Do("sam", http.MethodPost, terminalworld.HostPath(ids.New())+"/confirm", body, append(terminalworld.IfMatch(1), terminalworld.Key()...)...)
	p.Want(foreign, http.StatusNotFound, "not_found")
	terminalworld.SameAnswer(t, foreign, random)
	p.Session("robin-nostep", p.Robin, time.Time{})
	p.Want(p.Do("robin-nostep", http.MethodPost, path, body, p.Keyed(id)...), http.StatusUnauthorized, "step_up_required")
}

// TestIssuePairingCode_Rights: 403 owner_only for the assignee, 401 without step-up, 409 for a
// retired host, 400 too_long for a long reason.
func TestIssuePairingCode_Rights(t *testing.T) {
	t.Parallel()
	p := newPairWorld(t)
	id, _ := p.create("Build server")
	path := terminalworld.HostPath(id) + "/pairing-code"
	p.Want(p.Do("robin", http.MethodPatch, terminalworld.HostPath(id), `{"assigned_user_id":"`+p.Sam.String()+`","reason":"Sam's"}`, p.Keyed(id)...), http.StatusOK, "")
	if pr := p.Want(p.Do("sam", http.MethodPost, path, `{"reason":"x"}`, terminalworld.Key()...), http.StatusForbidden, "forbidden"); pr["reason"] != "owner_only" {
		t.Fatalf("assignee: %v", pr)
	}
	p.Session("robin-nostep", p.Robin, time.Time{})
	p.Want(p.Do("robin-nostep", http.MethodPost, path, `{"reason":"x"}`, terminalworld.Key()...), http.StatusUnauthorized, "step_up_required")
	long := `{"reason":"` + strings.Repeat("r", terminalworld.HostConfig.ReasonMaxChars+1) + `"}`
	if c, _ := terminalworld.Field(p.Want(p.Do("robin", http.MethodPost, path, long, terminalworld.Key()...), http.StatusBadRequest, "validation_failed")); c != "too_long" {
		t.Fatalf("long reason: %s", c)
	}
	retired, _ := p.active("Retired")
	p.Want(p.Do("robin", http.MethodPost, terminalworld.HostPath(retired)+"/state", `{"state":"retired","reason":"Gone"}`, p.Keyed(retired)...), http.StatusOK, "")
	p.Want(p.Do("robin", http.MethodPost, terminalworld.HostPath(retired)+"/pairing-code", `{"reason":"x"}`, terminalworld.Key()...), http.StatusConflict, "invalid_state_transition")
}

// TestRekey_FromActive (PLAN row): re-pairing an active host drops the keys, ends its active and
// detached session with host_rekeyed, raises the rights version, hides the key members, and
// host.updated with key_changed reaches every owner and not Sam.
func TestRekey_FromActive(t *testing.T) {
	t.Parallel()
	p := newPairWorld(t)
	id, hs := p.active("Build server")
	su := ids.MustParse(p.Text(`SELECT id::text FROM host_session_users WHERE host_id = $1 AND user_id = $2`, id.String(), p.Robin.String()))
	s1 := p.TerminalSession(id, p.Robin, su, "active")
	s2 := p.TerminalSession(id, p.Robin, su, "detached")
	before := p.PermissionVersion()
	p.repair(id)
	if p.state(id) != "pairing" || !p.keysNull(id) || p.PermissionVersion() <= before {
		t.Fatal("re-pairing did not drop the keys")
	}
	for _, s := range []ids.ID{s1, s2} {
		if p.Text(`SELECT coalesce(close_reason, '') FROM terminal_sessions WHERE id = $1 AND state = 'closed'`, s.String()) != "host_rekeyed" {
			t.Fatalf("session %s not closed with host_rekeyed", s)
		}
	}
	host := p.Want(p.Do("robin", http.MethodGet, terminalworld.HostPath(id), ""), http.StatusOK, "")
	if _, ok := host["key_fingerprint"]; ok {
		t.Fatal("getHost shows the dropped fingerprint")
	}
	if _, ok := host["noise_public_key"]; ok {
		t.Fatal("getHost shows a dropped key")
	}
	evs := p.Events("host.updated")
	ev := evs[len(evs)-1]
	if ev.Payload["key_changed"] != true || strings.Contains(ev.Raw, base64.RawURLEncoding.EncodeToString(hs.noise)) ||
		strings.Contains(ev.Raw, base64.RawURLEncoding.EncodeToString(hs.auth)) {
		t.Fatalf("event %s", ev.Raw)
	}
	validateSchema(t, "HostUpdatedPayload", []byte(ev.Raw))
	receives := func(name string) bool {
		d, err := p.Az.Authorize(authorize.Filter(context.Background()), p.Actors.Get(name), authorize.ActionHostRead,
			authorize.Resource{Kind: authorize.ResourceHost, ID: id})
		if err != nil {
			t.Fatal(err)
		}
		return d.Allowed()
	}
	if !receives("robin") || !receives("alex") || receives("sam") {
		t.Fatal("receive decision of host.updated")
	}
}

// TestRekey_AuditNamesBothFingerprints (PLAN row): the pairing_code entry names the old
// fingerprint; the next host.pair names old and new fingerprint with key_generation 2.
func TestRekey_AuditNamesBothFingerprints(t *testing.T) {
	t.Parallel()
	p := newPairWorld(t)
	id, hs := p.active("Build server")
	old := strings.ToLower(domain.CompactFingerprint(hs.fingerprint(t)))
	again := newHostService(t)
	p.Want(p.pair(again.request(p.repair(id)).json(t)), http.StatusCreated, "")
	found := false
	for _, a := range p.Allows("host.manage") {
		found = found || a.Details["operation"] == "pairing_code" && a.Details["old_fingerprint"] == old
	}
	if !found {
		t.Fatal("no pairing_code entry with the old fingerprint")
	}
	allows := p.Allows("host.pair")
	last := allows[len(allows)-1].Details
	if last["old_fingerprint"] != old || last["fingerprint"] != strings.ToLower(domain.CompactFingerprint(again.fingerprint(t))) ||
		last["fingerprint"] == old || last["key_generation"] != float64(2) {
		t.Fatalf("host.pair details %v", last)
	}
}

// TestPairHost_RejectsReusedKeys (T-TM-05): after a re-pairing the old key pair is 409
// host_key_reused and the code stays valid; a new pair gives 201.
func TestPairHost_RejectsReusedKeys(t *testing.T) {
	t.Parallel()
	p := newPairWorld(t)
	id, hs := p.active("Build server")
	code := p.repair(id)
	if pr := p.Want(p.pair(hs.request(code).json(t)), http.StatusConflict, "conflict"); pr["reason"] != "host_key_reused" {
		t.Fatalf("reason %v", pr["reason"])
	}
	p.Want(p.pair(newHostService(t).request(code).json(t)), http.StatusCreated, "")
}

// TestRekey_FromConfirmingDisabledAndPairing (24 sections 3.3 and 3.4).
func TestRekey_FromConfirmingDisabledAndPairing(t *testing.T) {
	t.Parallel()
	p := newPairWorld(t)
	keyChanged := func() any {
		evs := p.Events("host.updated")
		return evs[len(evs)-1].Payload["key_changed"]
	}
	confirming, _ := p.confirming("Confirming")
	p.repair(confirming)
	if !p.keysNull(confirming) || keyChanged() != true {
		t.Fatal("from confirming")
	}
	disabled, _ := p.active("Disabled")
	p.SQL(`UPDATE hosts SET state = 'disabled', disabled_reason = 'duplicate_connection', disabled_at = now() WHERE id = $1`, disabled.String())
	p.repair(disabled)
	if p.state(disabled) != "pairing" || p.Count(`SELECT count(*) FROM hosts WHERE id = $1 AND disabled_reason IS NULL AND disabled_at IS NULL`, disabled.String()) != 1 {
		t.Fatal("from disabled with duplicate_connection")
	}
	pairing, first := p.create("Pairing")
	p.repair(pairing)
	if _, ok := p.Events("host.updated")[len(p.Events("host.updated"))-1].Payload["key_changed"]; ok {
		t.Fatal("from pairing with key_changed")
	}
	p.Want(p.pair(newHostService(t).request(first).json(t)), http.StatusUnauthorized, "unauthenticated")
}

// TestPairingNeverLeaksSecrets (SEC-AUD-5, SEC-LOG-1): over a full pairing, a mismatch and a
// re-pairing, no code, HMAC, public key or working directory label appears in audit, events,
// problems or logs; the audit details carry only the keys of Design note 11.
func TestPairingNeverLeaksSecrets(t *testing.T) {
	t.Parallel()
	p := newPairWorld(t)
	id, code := p.create("Build server")
	hs := newHostService(t)
	body := hs.request(code)
	body["session_users"] = []any{map[string]any{"os_user": "bothy-s-robin", "workdirs": []any{"Secret project dir"}}}
	p.Want(p.pair(body.json(t)), http.StatusCreated, "")
	wrong := "zzzzzzzz"
	if strings.HasPrefix(hs.prefix(t), wrong) {
		wrong = "yyyyyyyy"
	}
	problem := p.confirm(id, confirmBody(wrong, nil))
	p.Want(problem, http.StatusConflict, "fingerprint_mismatch")
	second := p.repair(id)
	enc := base64.RawURLEncoding
	for what, s := range map[string]string{
		"code": code, "second code": second, "hmac": hex.EncodeToString(hmacOf(code)), "hmac b64": enc.EncodeToString(hmacOf(second)),
		"noise key": enc.EncodeToString(hs.noise), "auth key": enc.EncodeToString(hs.auth), "workdir": "Secret project dir",
	} {
		switch what {
		case "noise key", "auth key":
			// The public keys are part of the hosts row and of getHost while stored; elsewhere never.
			p.AssertNowhere(s, what)
		case "workdir":
			// A working directory label is part of the Host representation (reported_session_users),
			// so it is in the row and in host.updated; never in the audit, a problem or a log.
			p.auditAndLogsFree(s, what)
		default:
			p.assertNowhere(s, what)
		}
		if strings.Contains(string(problem.Body), s) {
			t.Errorf("%s in a problem body", what)
		}
	}
	allowed := map[string]bool{
		"operation": true, "reason": true, "from_state": true, "to_state": true, "old_fingerprint": true, "key_generation": true,
		"sessions_ended": true, "result": true, "fingerprint": true, "attempts_remaining": true, "llm_credentials": true,
		"credential_holder_id": true, "declared_server_host": true, "platform": true, "no_change": true, "change": true, "user_id": true,
		"os_user": true, "to_assigned_user_id": true, "from_assigned_user_id": true, "name_changed": true,
	}
	for _, action := range []string{"host.manage", "host.pair"} {
		for _, r := range p.Audits(action) {
			for k := range r.Details {
				if !allowed[k] && k != "ip_hmac" && k != "ip_key_id" && k != "aggregate" {
					t.Errorf("%s: detail %s", action, k)
				}
			}
		}
	}
}

// TestPairingConcurrency (Design note 13): two redemptions of one code give one 201 and one 401;
// two confirmations give one 200 and one 412 or 409; a confirmation racing a re-pairing never
// leaves an active host without keys.
func TestPairingConcurrency(t *testing.T) {
	t.Parallel()
	p := newPairWorld(t)
	_, code := p.create("Build server")
	statuses := parallel(func() int { return p.pair(newHostService(t).request(code).json(t)).StatusCode },
		func() int { return p.pair(newHostService(t).request(code).json(t)).StatusCode })
	if (statuses[0] != 201 || statuses[1] != 401) && (statuses[0] != 401 || statuses[1] != 201) {
		t.Fatalf("parallel pairHost: %v", statuses)
	}
	id, hs := p.confirming("Second")
	body := confirmBody(hs.prefix(t), nil)
	h1, h2 := p.Keyed(id), p.Keyed(id)
	statuses = parallel(func() int {
		return p.Do("robin", http.MethodPost, terminalworld.HostPath(id)+"/confirm", body, h1...).StatusCode
	},
		func() int {
			return p.Do("robin", http.MethodPost, terminalworld.HostPath(id)+"/confirm", body, h2...).StatusCode
		})
	ok, other := 0, 0
	for _, s := range statuses {
		switch s {
		case http.StatusOK:
			ok++
		case http.StatusPreconditionFailed, http.StatusConflict:
			other++
		}
	}
	if ok != 1 || other != 1 {
		t.Fatalf("parallel confirmHost: %v", statuses)
	}
	race, rhs := p.confirming("Race")
	rbody := confirmBody(rhs.prefix(t), nil)
	rh := p.Keyed(race)
	parallel(func() int {
		return p.Do("robin", http.MethodPost, terminalworld.HostPath(race)+"/confirm", rbody, rh...).StatusCode
	},
		func() int {
			return p.Do("robin", http.MethodPost, terminalworld.HostPath(race)+"/pairing-code", `{"reason":"x"}`, terminalworld.Key()...).StatusCode
		})
	if p.state(race) == "active" && p.keysNull(race) {
		t.Fatal("an active host without keys")
	}
}

// parallel runs the functions at once and returns their results in order.
func parallel(fns ...func() int) []int {
	out := make([]int, len(fns))
	var wg sync.WaitGroup
	for i, f := range fns {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out[i] = f()
		}()
	}
	wg.Wait()
	return out
}
