// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// proofKey is a host login key with its stored form (hosts.auth_public_jwk, hosts.auth_jkt).
type proofKey struct {
	priv ed25519.PrivateKey
	jwk  []byte
	jkt  string
}

// newProofKey derives a fixed key from seed, so failures repeat.
func newProofKey(t testing.TB, seed string) proofKey {
	t.Helper()
	s := sha256.Sum256([]byte(seed))
	priv := ed25519.NewKeyFromSeed(s[:])
	k, err := jose.JWKFromPublicKey(priv.Public())
	if err != nil {
		t.Fatal(err)
	}
	return proofKey{priv: priv, jwk: k.Canonical(), jkt: k.Thumbprint()}
}

// b64 encodes base64url without padding.
func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// rawProof builds a compact JWS from header and claims (JSON text) signed with priv over
// header.payload.
func rawProof(priv ed25519.PrivateKey, header, claims string) string {
	input := b64([]byte(header)) + "." + b64([]byte(claims))
	return input + "." + b64(ed25519.Sign(priv, []byte(input)))
}

// The fixed exchange of the proof tests: hello at helloAt, identify at now, leeway one minute.
var (
	helloAt   = time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	proofNow  = helloAt.Add(2 * time.Second)
	leeway    = time.Minute
	challenge = b64([]byte("0123456789abcdef0123456789abcdef"))
)

// claimsAt are the claims of a proof of host for the challenge at iat.
func claimsAt(host ids.ID, iat time.Time) string {
	return `{"challenge":"` + challenge + `","host_id":"` + host.String() + `","iat":` + strconv.FormatInt(iat.Unix(), 10) + `}`
}

// header returns the header of a proof with kid.
func header(kid string) string { return `{"alg":"EdDSA","typ":"host-auth+jwt","kid":"` + kid + `"}` }

// TestCheckHostProof (M2-43 Design note 3 d): the valid proof; every algorithm but EdDSA, every
// other typ, a kid of another key (unknown key), the right kid with the signature of another key,
// a wrong challenge, another host, iat at both bounds and one second outside, extra and missing
// claims, two or four parts and a payload that is no JSON are refused.
func TestCheckHostProof(t *testing.T) {
	t.Parallel()
	k, other := newProofKey(t, "proof/host"), newProofKey(t, "proof/other")
	host := ids.New()
	stored := domain.HostLoginKey{Found: true, State: domain.StateActive, JKT: k.jkt, JWK: k.jwk}
	check := func(proof string) domain.ProofVerdict {
		return domain.CheckHostProof(domain.ProofCheck{Proof: proof, HostID: host, Challenge: challenge, HelloAt: helloAt, Now: proofNow,
			Leeway: leeway}, stored)
	}
	valid := rawProof(k.priv, header(k.jkt), claimsAt(host, proofNow))
	if got := check(valid); got != domain.ProofValid {
		t.Fatalf("valid proof: %v", got)
	}
	ec, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	es256, err := jose.SignCompact(jose.ES256Signer{Key: ec}, jose.Header{Typ: "host-auth+jwt", Kid: k.jkt}, []byte(claimsAt(host, proofNow)))
	if err != nil {
		t.Fatal(err)
	}
	none := b64([]byte(`{"alg":"none","typ":"host-auth+jwt","kid":"`+k.jkt+`"}`)) + "." + b64([]byte(claimsAt(host, proofNow))) + "."
	hs := rawProof(k.priv, `{"alg":"HS256","typ":"host-auth+jwt","kid":"`+k.jkt+`"}`, claimsAt(host, proofNow))
	invalid := map[string]string{
		"alg none":              none,
		"alg HS256":             hs,
		"alg ES256":             es256,
		"typ agent invocation":  rawProof(k.priv, `{"alg":"EdDSA","typ":"agent-invocation+jwt","kid":"`+k.jkt+`"}`, claimsAt(host, proofNow)),
		"typ terminal session":  rawProof(k.priv, `{"alg":"EdDSA","typ":"terminal-session+jwt","kid":"`+k.jkt+`"}`, claimsAt(host, proofNow)),
		"typ other case":        rawProof(k.priv, `{"alg":"EdDSA","typ":"HOST-AUTH+JWT","kid":"`+k.jkt+`"}`, claimsAt(host, proofNow)),
		"no kid":                rawProof(k.priv, `{"alg":"EdDSA","typ":"host-auth+jwt"}`, claimsAt(host, proofNow)),
		"signature of another":  rawProof(other.priv, header(k.jkt), claimsAt(host, proofNow)),
		"wrong challenge":       rawProof(k.priv, header(k.jkt), strings.Replace(claimsAt(host, proofNow), challenge, b64([]byte("x")), 1)),
		"another host":          rawProof(k.priv, header(k.jkt), claimsAt(ids.New(), proofNow)),
		"iat before the window": rawProof(k.priv, header(k.jkt), claimsAt(host, helloAt.Add(-leeway-time.Second))),
		"iat after the window":  rawProof(k.priv, header(k.jkt), claimsAt(host, proofNow.Add(leeway+time.Second))),
		"extra claim":           rawProof(k.priv, header(k.jkt), strings.TrimSuffix(claimsAt(host, proofNow), "}")+`,"exp":1}`),
		"missing claim":         rawProof(k.priv, header(k.jkt), `{"challenge":"`+challenge+`","host_id":"`+host.String()+`"}`),
		"iat as string":         rawProof(k.priv, header(k.jkt), `{"challenge":"`+challenge+`","host_id":"`+host.String()+`","iat":"1"}`),
		"two parts":             strings.Join(strings.Split(valid, ".")[:2], "."),
		"four parts":            valid + ".x",
		"payload no JSON":       b64([]byte(header(k.jkt))) + "." + b64([]byte("not json")) + "." + b64([]byte("sig")),
		"empty":                 "",
	}
	for name, proof := range invalid {
		if got := check(proof); got != domain.ProofInvalid {
			t.Errorf("%s: %v, want invalid", name, got)
		}
	}
	for name, iat := range map[string]time.Time{"lower bound": helloAt.Add(-leeway), "upper bound": proofNow.Add(leeway)} {
		if got := check(rawProof(k.priv, header(k.jkt), claimsAt(host, iat))); got != domain.ProofValid {
			t.Errorf("iat at the %s: %v, want valid", name, got)
		}
	}
	if got := check(rawProof(other.priv, header(other.jkt), claimsAt(host, proofNow))); got != domain.ProofUnknownKey {
		t.Errorf("kid of another key: %v, want unknown key", got)
	}
	for name, key := range map[string]domain.HostLoginKey{
		"no host":    {},
		"pairing":    {Found: true, State: domain.StatePairing},
		"no key yet": {Found: true, State: domain.StateConfirming},
	} {
		got := domain.CheckHostProof(domain.ProofCheck{Proof: valid, HostID: host, Challenge: challenge, HelloAt: helloAt, Now: proofNow, Leeway: leeway}, key)
		if got != domain.ProofUnknownKey {
			t.Errorf("%s: %v, want unknown key", name, got)
		}
	}
}

// TestIdentifyOutcome (M2-43 Design note 3): the close code of an identify in the order of the
// note, the first match wins.
func TestIdentifyOutcome(t *testing.T) {
	t.Parallel()
	ok := domain.IdentifyFacts{ProtocolVersion: 1, LiveSessions: true, HostVersion: "1.0.0", Proof: domain.ProofValid, State: domain.StateActive}
	cases := map[string]struct {
		mod  func(*domain.IdentifyFacts)
		want domain.CloseCode
	}{
		"active":                       {func(*domain.IdentifyFacts) {}, 0},
		"confirming":                   {func(f *domain.IdentifyFacts) { f.State = domain.StateConfirming }, 0},
		"version 2 with broken proof":  {func(f *domain.IdentifyFacts) { f.ProtocolVersion, f.Proof = 2, domain.ProofInvalid }, domain.CodeHostVersionUnsupported},
		"unknown host":                 {func(f *domain.IdentifyFacts) { f.Proof, f.State = domain.ProofUnknownKey, "" }, domain.CodeHostUnpaired},
		"pairing":                      {func(f *domain.IdentifyFacts) { f.Proof, f.State = domain.ProofUnknownKey, domain.StatePairing }, domain.CodeHostUnpaired},
		"replaced key":                 {func(f *domain.IdentifyFacts) { f.Proof = domain.ProofUnknownKey }, domain.CodeHostUnpaired},
		"bad signature":                {func(f *domain.IdentifyFacts) { f.Proof = domain.ProofInvalid }, domain.CodeHostAuthFailed},
		"timeout":                      {func(f *domain.IdentifyFacts) { f.TimedOut = true }, domain.CodeHostAuthFailed},
		"another op first":             {func(f *domain.IdentifyFacts) { f.OtherOp = true }, domain.CodeHostAuthFailed},
		"disabled":                     {func(f *domain.IdentifyFacts) { f.State = domain.StateDisabled }, domain.CodeHostDisabled},
		"retired":                      {func(f *domain.IdentifyFacts) { f.State = domain.StateRetired }, domain.CodeHostDisabled},
		"no live_sessions":             {func(f *domain.IdentifyFacts) { f.LiveSessions = false }, domain.CodeDecodeError},
		"session_id on control":        {func(f *domain.IdentifyFacts) { f.SessionID = true }, domain.CodeDecodeError},
		"host_version no SemVer":       {func(f *domain.IdentifyFacts) { f.HostVersion = "v1.0" }, domain.CodeDecodeError},
		"live_sessions and bad proof":  {func(f *domain.IdentifyFacts) { f.LiveSessions, f.Proof = false, domain.ProofInvalid }, domain.CodeDecodeError},
		"timeout before version check": {func(f *domain.IdentifyFacts) { f.TimedOut, f.ProtocolVersion = true, 2 }, domain.CodeHostAuthFailed},
	}
	for name, c := range cases {
		f := ok
		c.mod(&f)
		if got := domain.IdentifyOutcome(f); got != c.want {
			t.Errorf("%s: %d, want %d", name, got, c.want)
		}
	}
}

// TestReconcileLiveSessions (M2-43 Design note 6, TM-T16): opening A, active B, detached C, active
// D, closed E with access_revoked; live = B, E and an unknown X: C and D end with host_restarted,
// A and B stay, E gets session.close with its stored reason and X with access_revoked.
func TestReconcileLiveSessions(t *testing.T) {
	t.Parallel()
	host := ids.New()
	a, b, c, d, e, x := ids.New(), ids.New(), ids.New(), ids.New(), ids.New(), ids.New()
	open := []domain.Session{
		{ID: a, HostID: host, State: "opening"}, {ID: b, HostID: host, State: "active"},
		{ID: c, HostID: host, State: "detached"}, {ID: d, HostID: host, State: "active"},
	}
	listed := map[ids.ID]domain.ListedSession{
		e: {Found: true, HostID: host, State: "closed", CloseReason: domain.CloseAccessRevoked},
		x: {},
	}
	got := domain.ReconcileLiveSessions(host, open, []ids.ID{b, e, x, e}, listed)
	if !slices.Equal(got.End, []ids.ID{c, d}) {
		t.Errorf("end %v, want C and D", got.End)
	}
	want := []domain.SessionClose{{SessionID: e, Reason: domain.CloseAccessRevoked}, {SessionID: x, Reason: domain.CloseAccessRevoked}}
	if !slices.Equal(got.Close, want) {
		t.Errorf("close %v, want %v", got.Close, want)
	}
	// A session of another host and a closed session with another reason.
	f, g := ids.New(), ids.New()
	got = domain.ReconcileLiveSessions(host, nil, []ids.ID{f, g}, map[ids.ID]domain.ListedSession{
		f: {Found: true, HostID: ids.New(), State: "active"},
		g: {Found: true, HostID: host, State: "closed", CloseReason: domain.CloseHostRekeyed},
	})
	want = []domain.SessionClose{{SessionID: f, Reason: domain.CloseAccessRevoked}, {SessionID: g, Reason: domain.CloseHostRekeyed}}
	if len(got.End) != 0 || !slices.Equal(got.Close, want) {
		t.Errorf("other host: %+v", got)
	}
}

// reportLimits are small bounds, so every rule is cheap to break.
var reportLimits = domain.ReportLimits{OSUserMaxChars: 16, LabelMaxChars: 12, MaxEntries: 3}

// goodControlReport is a report that passes ValidateReport: every check once, one failed HD6 per account.
func goodControlReport() domain.Report {
	var doc []domain.DoctorCheck
	for _, c := range domain.DoctorCheckNames() {
		doc = append(doc, domain.DoctorCheck{Check: c, OK: true})
	}
	doc = append(doc, domain.DoctorCheck{Check: "hd6_llm_login", OK: false, Code: "subscription_login_detected", SessionUser: "bothy-s-alex"})
	return domain.Report{
		Profiles:     []domain.Profile{{Name: "shell", DisplayName: "Shell", Kind: "shell"}},
		SessionUsers: []domain.ReportedSessionUser{{OSUser: "bothy-s-alex", Workdirs: []string{"Project A"}}},
		Doctor:       doc, OSVersion: "Debian 13",
	}
}

// TestValidateReport (M2-43 Design note 8): one case per rule; an empty session_users passes.
func TestValidateReport(t *testing.T) {
	t.Parallel()
	if f := domain.ValidateReport(goodControlReport(), reportLimits); len(f) != 0 {
		t.Fatalf("good report: %+v", f)
	}
	empty := goodControlReport()
	empty.SessionUsers = []domain.ReportedSessionUser{}
	if f := domain.ValidateReport(empty, reportLimits); len(f) != 0 {
		t.Fatalf("empty session_users: %+v", f)
	}
	cases := map[string]struct {
		mod     func(*domain.Report)
		pointer string
	}{
		"check missing": {func(r *domain.Report) { r.Doctor = r.Doctor[1:] }, "/doctor"},
		"unknown check": {func(r *domain.Report) { r.Doctor[0].Check = "hd12_more" }, "/doctor/0/check"},
		"check and name twice": {func(r *domain.Report) {
			r.Doctor = append(r.Doctor, r.Doctor[len(r.Doctor)-1])
		}, "/doctor/12"},
		"name on a passed check": {func(r *domain.Report) { r.Doctor[0].SessionUser = "bothy-s-alex" }, "/doctor/0/session_user"},
		"name not a session user": {func(r *domain.Report) {
			r.Doctor[len(r.Doctor)-1].SessionUser = "../etc"
		}, "/doctor/11/session_user"},
		"name over host.os_user_max_chars in doctor": {func(r *domain.Report) {
			r.Doctor[len(r.Doctor)-1].SessionUser = strings.Repeat("a", 17)
		}, "/doctor/11/session_user"},
		"code outside its pattern": {func(r *domain.Report) { r.Doctor[0].Code = "Bad Code" }, "/doctor/0/code"},
		"session users equal without case": {func(r *domain.Report) {
			r.SessionUsers = append(r.SessionUsers, domain.ReportedSessionUser{OSUser: "BOTHY-S-ALEX", Workdirs: []string{}})
		}, "/session_users/1/os_user"},
		"name over host.os_user_max_chars in session_users": {func(r *domain.Report) {
			r.SessionUsers[0].OSUser = strings.Repeat("a", 17)
		}, "/session_users/0/os_user"},
		"profile twice":                         {func(r *domain.Report) { r.Profiles = append(r.Profiles, r.Profiles[0]) }, "/profiles/1/name"},
		"display name with a control character": {func(r *domain.Report) { r.Profiles[0].DisplayName = "She\u0007ll" }, "/profiles/0/display_name"},
		"label over terminal.report_label_max_chars": {func(r *domain.Report) {
			r.SessionUsers[0].Workdirs[0] = strings.Repeat("x", 13)
		}, "/session_users/0/workdirs/0"},
		"os_version over the label length": {func(r *domain.Report) { r.OSVersion = strings.Repeat("x", 13) }, "/os_version"},
		"os_version bidi":                  {func(r *domain.Report) { r.OSVersion = "Debian\u202e13" }, "/os_version"},
		"profiles over terminal.report_max_entries": {func(r *domain.Report) {
			for i := range 3 {
				r.Profiles = append(r.Profiles, domain.Profile{Name: "p" + strconv.Itoa(i), DisplayName: "P", Kind: "command"})
			}
		}, "/profiles"},
		"session_users over terminal.report_max_entries": {func(r *domain.Report) {
			for i := range 3 {
				r.SessionUsers = append(r.SessionUsers, domain.ReportedSessionUser{OSUser: "bothy-s-" + strconv.Itoa(i), Workdirs: []string{}})
			}
		}, "/session_users"},
		"workdirs over terminal.report_max_entries": {func(r *domain.Report) {
			r.SessionUsers[0].Workdirs = []string{"a", "b", "c", "d"}
		}, "/session_users/0/workdirs"},
	}
	for name, c := range cases {
		r := goodControlReport()
		c.mod(&r)
		f := domain.ValidateReport(r, reportLimits)
		if !slices.ContainsFunc(f, func(e errs.FieldError) bool { return e.Pointer == c.pointer }) {
			t.Errorf("%s: %+v, want a finding at %s", name, f, c.pointer)
		}
	}
}

// storedFrom is the host after r was stored.
func storedFrom(r domain.Report) domain.Host {
	return domain.Host{Profiles: r.Profiles, ReportedUsers: r.SessionUsers, Doctor: r.Doctor, ReportedIsServerHost: &r.IsServerHost,
		RequireLocalConfirm: &r.RequireLocalConfirm, OSVersion: r.OSVersion}
}

// TestDiffReport (M2-43 Design note 8 a and b): no change; profiles only (event without audit);
// doctor (audit); names added and removed; the server-host flag; os_version only.
func TestDiffReport(t *testing.T) {
	t.Parallel()
	base := goodControlReport()
	if d := domain.DiffReport(storedFrom(base), base); d.Changed() || d.Audit() {
		t.Fatalf("no change: %+v", d)
	}
	if d := domain.DiffReport(domain.Host{}, base); !d.Changed() || !d.Audit() {
		t.Fatalf("first report: %+v", d)
	}
	profiles := goodControlReport()
	profiles.Profiles[0].DisplayName = "Bash"
	if d := domain.DiffReport(storedFrom(base), profiles); !d.Profiles || !d.Changed() || d.Audit() {
		t.Errorf("profiles: %+v", d)
	}
	doctor := goodControlReport()
	doctor.Doctor[0].OK, doctor.Doctor[0].Code = false, "user_missing"
	if d := domain.DiffReport(storedFrom(base), doctor); !d.Doctor || !d.Audit() || !slices.Equal(d.Codes(), []string{"doctor"}) {
		t.Errorf("doctor: %+v", d)
	}
	names := goodControlReport()
	names.SessionUsers = []domain.ReportedSessionUser{{OSUser: "bothy-s-sam", Workdirs: []string{}}}
	d := domain.DiffReport(storedFrom(base), names)
	if !d.SessionUsers || !d.Audit() || !slices.Equal(d.Added, []string{"bothy-s-sam"}) || !slices.Equal(d.Removed, []string{"bothy-s-alex"}) {
		t.Errorf("names: %+v", d)
	}
	flag := goodControlReport()
	flag.IsServerHost = true
	if d := domain.DiffReport(storedFrom(base), flag); !d.IsServerHost || !d.Audit() || !d.TurnsServerHostOn() || !slices.Equal(d.Codes(), []string{"is_server_host"}) {
		t.Errorf("flag: %+v", d)
	}
	osv := goodControlReport()
	osv.OSVersion = "Debian 14"
	if d := domain.DiffReport(storedFrom(base), osv); !d.OSVersion || !d.Changed() || d.Audit() {
		t.Errorf("os_version: %+v", d)
	}
}

// TestSessionsEndedByReport (M2-43 Design note 8 c, SR10): when the flag turns on, every open
// session of a person who is not an active owner ends; otherwise none.
func TestSessionsEndedByReport(t *testing.T) {
	t.Parallel()
	alex1, alex2, robin := ids.New(), ids.New(), ids.New()
	open := []domain.SessionOwner{{ID: alex1}, {ID: robin, ActiveOwner: true}, {ID: alex2}}
	if got := domain.SessionsEndedByReport(false, true, open); !slices.Equal(got, []ids.ID{alex1, alex2}) {
		t.Errorf("turned on: %v", got)
	}
	for _, c := range [][2]bool{{true, true}, {false, false}, {true, false}} {
		if got := domain.SessionsEndedByReport(c[0], c[1], open); len(got) != 0 {
			t.Errorf("%v: %v", c, got)
		}
	}
}

// TestValidateHostSessionEnd (M2-43 Design note 10): the accepted reasons, close_detail exactly
// with open_failed (any detail) and host_policy_violation (three details), exit_code only with
// process_exited.
func TestValidateHostSessionEnd(t *testing.T) {
	t.Parallel()
	code := 3
	good := []domain.HostSessionEnd{
		{Reason: domain.CloseClientClosed}, {Reason: domain.CloseProcessExited, ExitCode: &code}, {Reason: domain.CloseProcessExited},
		{Reason: domain.CloseIdleTimeout}, {Reason: domain.CloseDetachExpired},
		{Reason: domain.CloseHostPolicyViolated, Detail: domain.DetailSubscriptionLoginDetected},
		{Reason: domain.CloseHostPolicyViolated, Detail: domain.DetailHelperUntrusted},
		{Reason: domain.CloseHostPolicyViolated, Detail: domain.DetailSharedLocation},
	}
	for _, d := range domain.CloseDetails() {
		good = append(good, domain.HostSessionEnd{Reason: domain.CloseOpenFailed, Detail: d})
	}
	for _, e := range good {
		if !domain.ValidateHostSessionEnd(e) {
			t.Errorf("refused %+v", e)
		}
	}
	bad := []domain.HostSessionEnd{
		{Reason: domain.CloseAccessRevoked}, {Reason: domain.CloseHostRestarted}, {Reason: domain.CloseMaxDuration},
		{Reason: domain.CloseOpenFailed}, {Reason: domain.CloseHostPolicyViolated},
		{Reason: domain.CloseHostPolicyViolated, Detail: domain.DetailPTYFailed},
		{Reason: domain.CloseClientClosed, Detail: domain.DetailPTYFailed}, {Reason: domain.CloseClientClosed, ExitCode: &code},
		{Reason: domain.CloseOpenFailed, Detail: "made_up"}, {Reason: "made_up"},
	}
	for _, e := range bad {
		if domain.ValidateHostSessionEnd(e) {
			t.Errorf("accepted %+v", e)
		}
	}
}

// TestHeartbeatDeadline (M2-43 Design note 9): interval times (miss limit + 1) after the last
// heartbeat; open exactly at the deadline, closed after it.
func TestHeartbeatDeadline(t *testing.T) {
	t.Parallel()
	last := helloAt
	dl := domain.HeartbeatDeadline(last, 20*time.Second, 3)
	if !dl.Equal(last.Add(80 * time.Second)) {
		t.Fatalf("deadline %v", dl)
	}
	if domain.HeartbeatMissed(dl, dl) || !domain.HeartbeatMissed(dl, dl.Add(time.Nanosecond)) {
		t.Fatal("open exactly at the deadline, closed after it")
	}
}

// TestSupportedProtocolVersionsMatchAsyncAPI holds SupportedProtocolVersions equal to
// x-protocol-versions of the channel hostControl.
func TestSupportedProtocolVersionsMatchAsyncAPI(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "docs", "api", "asyncapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	i := strings.Index(s, "\n  hostControl:\n")
	if i < 0 {
		t.Fatal("no channel hostControl")
	}
	m := regexp.MustCompile(`x-protocol-versions: \[([0-9, ]*)\]`).FindStringSubmatch(s[i:])
	if m == nil {
		t.Fatal("no x-protocol-versions in hostControl")
	}
	var want []int
	if err := json.Unmarshal([]byte("["+m[1]+"]"), &want); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(domain.SupportedProtocolVersions(), want) {
		t.Fatalf("SupportedProtocolVersions %v, asyncapi %v", domain.SupportedProtocolVersions(), want)
	}
}

// splitProof splits a compact JWS at its dots.
func splitProof(s string) []string { return strings.Split(s, ".") }

// b64Decode decodes base64url without padding.
func b64Decode(s string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(s) }
