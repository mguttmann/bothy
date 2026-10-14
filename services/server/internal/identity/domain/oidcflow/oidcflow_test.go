// SPDX-License-Identifier: Apache-2.0

package oidcflow_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/identity/domain/oidcflow"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// fieldOf returns the pointer of the one field error of err, "" for none.
func fieldOf(err error) string {
	var e *errs.Error
	if !errors.As(err, &e) || len(e.Fields) != 1 || e.Code != errs.CodeValidationFailed {
		return ""
	}
	return e.Fields[0].Pointer
}

func TestRedirectURIRules(t *testing.T) {
	t.Parallel()
	allowed := []string{"com.example.teamchat:/login", "https://app.example.com/oidc"}
	ok := []string{
		"http://127.0.0.1:1/callback", "http://127.0.0.1:65535/callback", "http://127.0.0.1:49152/callback",
		"http://[::1]:8000/callback", "http://[::1]:1/callback",
		"com.example.teamchat:/login", "https://app.example.com/oidc",
	}
	for _, u := range ok {
		if err := oidcflow.CheckAppRedirectURI(u, allowed); err != nil {
			t.Errorf("%s: %v", u, err)
		}
	}
	bad := []string{
		"", "http://localhost:8000/callback", "http://127.0.0.1/callback", "http://127.0.0.1:0/callback",
		"http://127.0.0.1:65536/callback", "http://127.0.0.1:080/callback", "http://127.0.0.2:8000/callback",
		"http://192.0.2.1:8000/callback", "http://example.com:8000/callback", "http://user@127.0.0.1:8000/callback",
		"http://127.0.0.1:8000/callback?x=1", "http://127.0.0.1:8000/callback#f", "http://127.0.0.1:8000/other",
		"http://127.0.0.1:8000/callback/", "http://127.0.0.1:8000/", "http://[::1]:8000/x", "http://[::ffff:127.0.0.1]:8000/callback",
		"https://127.0.0.1:8000/callback", "HTTP://127.0.0.1:8000/callback", "http://127.0.0.1:8000/Callback",
		"http://127.0.0.1:+80/callback", "http://127.0.0.1:8000/%63allback",
		// exact match only for the configured entries
		"com.example.teamchat:/login/", "com.example.teamchat:/Login", "https://app.example.com/oidc?a=1",
		"https://app.example.com/oidc#x", "https://app.example.com/oidc/", "com.example.other:/login",
		"https://evil.example.org/oidc",
	}
	for _, u := range bad {
		if err := oidcflow.CheckAppRedirectURI(u, allowed); fieldOf(err) != "/app_redirect_uri" {
			t.Errorf("%q: %v, want validation_failed at /app_redirect_uri", u, err)
		}
	}
	// A configured entry with userinfo or a fragment never matches, even exactly.
	if err := oidcflow.CheckAppRedirectURI("https://u@app.example.com/x", []string{"https://u@app.example.com/x"}); err == nil {
		t.Error("an entry with userinfo matched")
	}
}

func TestClientChallengeFormat(t *testing.T) {
	t.Parallel()
	good := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	if err := oidcflow.CheckClientChallenge(good); err != nil {
		t.Fatal(err)
	}
	for _, c := range []string{"", good[:42], good + "A", good[:42] + "=", good[:42] + "+", good[:42] + "/", strings.Repeat("A", 43) + " ", "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cé"} {
		if err := oidcflow.CheckClientChallenge(c); fieldOf(err) != "/client_challenge" {
			t.Errorf("%q: %v", c, err)
		}
	}
	// The verifier follows RFC 7636: 43 to 128 unreserved characters.
	for _, v := range []string{strings.Repeat("a", 43), strings.Repeat("Z", 128), "a-._~" + strings.Repeat("0", 38)} {
		if !oidcflow.ValidVerifier(v) {
			t.Errorf("%q refused", v)
		}
	}
	for _, v := range []string{"", strings.Repeat("a", 42), strings.Repeat("a", 129), strings.Repeat("a", 42) + "+", strings.Repeat("a", 42) + " "} {
		if oidcflow.ValidVerifier(v) {
			t.Errorf("%q accepted", v)
		}
	}
}

func TestPKCES256Vector(t *testing.T) {
	t.Parallel()
	const verifier, challenge = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk", "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	if got := oidcflow.S256(verifier); got != challenge {
		t.Fatalf("S256 = %s, want %s", got, challenge)
	}
	if !oidcflow.VerifierMatches(verifier, challenge) {
		t.Fatal("the RFC 7636 vector did not match")
	}
	if oidcflow.VerifierMatches(verifier[:42]+"Y", challenge) || oidcflow.VerifierMatches("short", oidcflow.S256("short")) {
		t.Fatal("a wrong or malformed verifier matched")
	}
}

func TestServerVerifierDerivation(t *testing.T) {
	t.Parallel()
	pepper := bytes.Repeat([]byte{7}, 32)
	a, b := ids.New(), ids.New()
	va := oidcflow.ServerVerifier(pepper, a)
	if va != oidcflow.ServerVerifier(pepper, a) {
		t.Fatal("not deterministic")
	}
	if va == oidcflow.ServerVerifier(pepper, b) || va == oidcflow.ServerVerifier(bytes.Repeat([]byte{8}, 32), a) {
		t.Fatal("same verifier for another flow or pepper")
	}
	if len(va) != 43 || !oidcflow.ValidVerifier(va) {
		t.Fatalf("verifier %q is not 43 unreserved characters", va)
	}
}

func TestHashesAndSecrets(t *testing.T) {
	t.Parallel()
	pepper := bytes.Repeat([]byte{1}, 32)
	if bytes.Equal(oidcflow.Hash(pepper, oidcflow.HashState, "x"), oidcflow.Hash(pepper, oidcflow.HashNonce, "x")) {
		t.Fatal("state and nonce hash share their context")
	}
	s, err := oidcflow.NewSecret(bytes.NewReader(bytes.Repeat([]byte{9}, 32)))
	if err != nil || len(s) != 43 {
		t.Fatalf("secret %q: %v", s, err)
	}
	if _, err := oidcflow.NewSecret(bytes.NewReader([]byte{1})); err == nil {
		t.Fatal("short random source accepted")
	}
	lc, err := oidcflow.NewLoginCode(bytes.NewReader(bytes.Repeat([]byte{9}, 32)))
	if err != nil || !strings.HasPrefix(lc, "lc_") || len(lc) != 46 {
		t.Fatalf("login code %q: %v", lc, err)
	}
}

func TestFlowStateMachine(t *testing.T) {
	t.Parallel()
	all := []oidcflow.Status{oidcflow.StatusStarted, oidcflow.StatusCodeIssued, oidcflow.StatusCompleted, oidcflow.StatusFailed}
	allowed := map[[2]oidcflow.Status]bool{
		{oidcflow.StatusStarted, oidcflow.StatusCodeIssued}:   true,
		{oidcflow.StatusCodeIssued, oidcflow.StatusCompleted}: true,
		{oidcflow.StatusStarted, oidcflow.StatusFailed}:       true,
		{oidcflow.StatusCodeIssued, oidcflow.StatusFailed}:    true,
	}
	for _, from := range all {
		for _, to := range all {
			if got := oidcflow.CanTransition(from, to); got != allowed[[2]oidcflow.Status{from, to}] {
				t.Errorf("%s -> %s = %v", from, to, got)
			}
		}
	}
	if oidcflow.CanTransition("x", oidcflow.StatusFailed) {
		t.Error("unknown state")
	}
}

// claimsFixture is a valid set of claims against expectation e.
func claimsFixture() (oidcflow.Claims, oidcflow.Expectation) {
	now := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	pepper := bytes.Repeat([]byte{3}, 32)
	c := oidcflow.Claims{
		Issuer: "https://idp.example.com", Subject: "s-1", Audience: []string{"chat"}, Nonce: "n-1",
		Expiry: now.Add(5 * time.Minute), IssuedAt: now.Add(-time.Minute), HasExpiry: true, HasIssuedAt: true,
	}
	e := oidcflow.Expectation{
		Issuer: "https://idp.example.com", ClientID: "chat", NonceHash: oidcflow.Hash(pepper, oidcflow.HashNonce, "n-1"), Pepper: pepper,
		FlowCreatedAt: now.Add(-2 * time.Minute), Now: now, Leeway: 30 * time.Second,
	}
	return c, e
}

func TestIDTokenClaimValidation(t *testing.T) {
	t.Parallel()
	base, exp := claimsFixture()
	if iss, err := oidcflow.CheckIDToken(base, exp); err != nil || iss != "https://idp.example.com" {
		t.Fatalf("valid claims: %s %v", iss, err)
	}
	now, leeway := exp.Now, exp.Leeway
	type tc struct {
		name string
		mod  func(c *oidcflow.Claims, e *oidcflow.Expectation)
		ok   bool
	}
	cases := []tc{
		{"foreign iss", func(c *oidcflow.Claims, _ *oidcflow.Expectation) { c.Issuer = "https://idp.example.org" }, false},
		{"iss with trailing slash", func(c *oidcflow.Claims, _ *oidcflow.Expectation) { c.Issuer += "/" }, false},
		{"aud without client", func(c *oidcflow.Claims, _ *oidcflow.Expectation) { c.Audience = []string{"other"} }, false},
		{"aud empty", func(c *oidcflow.Claims, _ *oidcflow.Expectation) { c.Audience = nil }, false},
		{"two audiences without azp", func(c *oidcflow.Claims, _ *oidcflow.Expectation) { c.Audience = []string{"chat", "other"} }, false},
		{"two audiences with azp", func(c *oidcflow.Claims, _ *oidcflow.Expectation) {
			c.Audience, c.AZP = []string{"chat", "other"}, "chat"
		}, true},
		{"azp other", func(c *oidcflow.Claims, _ *oidcflow.Expectation) { c.AZP = "other" }, false},
		{"azp equal", func(c *oidcflow.Claims, _ *oidcflow.Expectation) { c.AZP = "chat" }, true},
		{"exp missing", func(c *oidcflow.Claims, _ *oidcflow.Expectation) { c.HasExpiry = false }, false},
		{"exp just inside leeway", func(c *oidcflow.Claims, _ *oidcflow.Expectation) { c.Expiry = now.Add(-leeway + time.Second) }, true},
		{"exp at leeway", func(c *oidcflow.Claims, _ *oidcflow.Expectation) { c.Expiry = now.Add(-leeway) }, false},
		{"expired", func(c *oidcflow.Claims, _ *oidcflow.Expectation) { c.Expiry = now.Add(-time.Hour) }, false},
		{"iat missing", func(c *oidcflow.Claims, _ *oidcflow.Expectation) { c.HasIssuedAt = false }, false},
		{"iat at future leeway", func(c *oidcflow.Claims, _ *oidcflow.Expectation) { c.IssuedAt = now.Add(leeway) }, true},
		{"iat beyond future leeway", func(c *oidcflow.Claims, _ *oidcflow.Expectation) { c.IssuedAt = now.Add(leeway + time.Second) }, false},
		{"iat at flow start minus leeway", func(c *oidcflow.Claims, e *oidcflow.Expectation) { c.IssuedAt = e.FlowCreatedAt.Add(-leeway) }, true},
		{"iat before the flow", func(c *oidcflow.Claims, e *oidcflow.Expectation) {
			c.IssuedAt = e.FlowCreatedAt.Add(-leeway - time.Second)
		}, false},
		{"wrong nonce", func(c *oidcflow.Claims, _ *oidcflow.Expectation) { c.Nonce = "n-2" }, false},
		{"missing nonce", func(c *oidcflow.Claims, _ *oidcflow.Expectation) { c.Nonce = "" }, false},
		{"empty sub", func(c *oidcflow.Claims, _ *oidcflow.Expectation) { c.Subject = "" }, false},
		{"sub with NUL", func(c *oidcflow.Claims, _ *oidcflow.Expectation) { c.Subject = "a\x00b" }, false},
		{"tid ignored without template", func(c *oidcflow.Claims, _ *oidcflow.Expectation) { c.TenantID = "t9" }, true},
	}
	for _, k := range cases {
		c, e := claimsFixture()
		c.Audience = append([]string(nil), c.Audience...)
		k.mod(&c, &e)
		_, err := oidcflow.CheckIDToken(c, e)
		if k.ok != (err == nil) {
			t.Errorf("%s: err = %v, want ok %v", k.name, err, k.ok)
		}
		if err != nil && !errors.Is(err, oidcflow.ErrClaims) {
			t.Errorf("%s: error %v does not wrap ErrClaims", k.name, err)
		}
	}
}

func TestIDTokenTenantTemplate(t *testing.T) {
	t.Parallel()
	c, e := claimsFixture()
	e.Issuer = "https://login.example.com/{tenantid}/v2.0"
	e.AllowedTenants = []string{"t1", "t2"}
	c.Issuer, c.TenantID = "https://login.example.com/t2/v2.0", "t2"
	if iss, err := oidcflow.CheckIDToken(c, e); err != nil || iss != c.Issuer {
		t.Fatalf("allowed tenant: %s %v", iss, err)
	}
	for name, mod := range map[string]func(*oidcflow.Claims){
		"tenant not allowed":   func(c *oidcflow.Claims) { c.Issuer, c.TenantID = "https://login.example.com/t3/v2.0", "t3" },
		"tid differs from iss": func(c *oidcflow.Claims) { c.TenantID = "t1" },
		"tid missing":          func(c *oidcflow.Claims) { c.TenantID = "" },
		"literal template":     func(c *oidcflow.Claims) { c.Issuer, c.TenantID = e.Issuer, "{tenantid}" },
	} {
		cc := c
		mod(&cc)
		if _, err := oidcflow.CheckIDToken(cc, e); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	// The iss response parameter (RFC 9207) of a template issuer.
	if !oidcflow.IssuerMatches(e.Issuer, e.AllowedTenants, "https://login.example.com/t1/v2.0") ||
		oidcflow.IssuerMatches(e.Issuer, e.AllowedTenants, "https://login.example.com/t3/v2.0") ||
		oidcflow.IssuerMatches(e.Issuer, e.AllowedTenants, "https://login.example.com/t1/x/v2.0") ||
		!oidcflow.IssuerMatches("https://idp.example.com", nil, "https://idp.example.com") ||
		oidcflow.IssuerMatches("https://idp.example.com", nil, "https://idp.example.com/") {
		t.Fatal("IssuerMatches")
	}
}

func TestParseClaims(t *testing.T) {
	t.Parallel()
	c, err := oidcflow.ParseClaims([]byte(`{"iss":"https://i.example.com","sub":"s","aud":"chat","exp":1700000000,"iat":1699999990,
		"nonce":"n","tid":"t","name":"Alex","preferred_username":"alex@example.com","email":"alex@example.com","auth_time":1699999980,
		"amr":["pwd","mfa"],"azp":"chat","extra":{"x":[1]}}`))
	if err != nil {
		t.Fatal(err)
	}
	if c.Issuer != "https://i.example.com" || len(c.Audience) != 1 || c.Audience[0] != "chat" || !c.HasExpiry || c.Expiry.Unix() != 1700000000 ||
		c.IssuedAt.Unix() != 1699999990 || c.AuthTime.Unix() != 1699999980 || len(c.AMR) != 2 || c.Name != "Alex" || c.AZP != "chat" || c.TenantID != "t" {
		t.Fatalf("%+v", c)
	}
	c, err = oidcflow.ParseClaims([]byte(`{"iss":"x","sub":"s","aud":["a","b"],"exp":1.7e9,"iat":1700000000.5}`))
	if err != nil || len(c.Audience) != 2 || c.Expiry.Unix() != 1700000000 {
		t.Fatalf("%+v %v", c, err)
	}
	for _, raw := range []string{`{"aud":1}`, `{"exp":"1"}`, `{"sub":1}`, `{"amr":"pwd"}`, `{"iat":true}`, `[]`, `{"aud":[1]}`, `{"exp":1e300}`} {
		if _, err := oidcflow.ParseClaims([]byte(raw)); !errors.Is(err, oidcflow.ErrClaims) {
			t.Errorf("%s: %v", raw, err)
		}
	}
}

func TestExternalIdentityFromClaims(t *testing.T) {
	t.Parallel()
	c, _ := claimsFixture()
	c.Name, c.PreferredUsername, c.Email, c.AMR = "", "alex@example.com", "alex@example.com", []string{"pwd"}
	x := oidcflow.ExternalFromClaims(c, "https://idp.example.com")
	if x.Issuer != "https://idp.example.com" || x.Subject != "s-1" || x.DisplayName != "alex@example.com" || x.EmailHint != "alex@example.com" ||
		!x.AuthTime.Equal(c.IssuedAt) || len(x.AMR) != 1 || x.PreferredUsername != "alex@example.com" {
		t.Fatalf("%+v", x)
	}
	c.Name, c.AuthTime = "Alex", c.IssuedAt.Add(-time.Hour)
	x = oidcflow.ExternalFromClaims(c, "https://idp.example.com")
	if x.DisplayName != "Alex" || !x.AuthTime.Equal(c.AuthTime) {
		t.Fatalf("%+v", x)
	}
}

func TestProvisionedHandle(t *testing.T) {
	t.Parallel()
	cases := []struct {
		pref, name string
		max        int
		want       string
	}{
		{"Alex.Example@example.com", "Alex", 32, "alex.example"},
		{"", "Sam Example", 32, "sam-example"},
		{"", "", 32, "user"},
		{"__!!robin", "", 32, "robin"},
		{"@example.com", "Robin", 32, "robin"},
		{"!!!", "", 32, "user"},
		{"Zoë.Ünal", "", 32, "zo-.-nal"},
		{"averyveryverylonglocalpartthatexceedsthelimit", "", 10, "averyveryv"},
		{"a b\tc", "", 32, "a-b-c"},
		{"", "\u202eRobin", 32, "robin"},
	}
	for _, c := range cases {
		if got := oidcflow.ProvisionedHandle(c.pref, c.name, c.max); got != c.want {
			t.Errorf("ProvisionedHandle(%q, %q, %d) = %q, want %q", c.pref, c.name, c.max, got, c.want)
		}
	}
	// Collisions: a suffix of "-" and four characters, within the limit.
	if got := oidcflow.HandleWithSuffix("alex", "k3x9", 32); got != "alex-k3x9" {
		t.Fatalf("suffix %q", got)
	}
	if got := oidcflow.HandleWithSuffix("abcdefghij", "k3x9", 10); got != "abcde-k3x9" || len(got) != 10 {
		t.Fatalf("suffix within limit %q", got)
	}
	if got := oidcflow.HandleWithSuffix("abc", "k3x9", 4); got != "k3x9" {
		t.Fatalf("tiny limit %q", got)
	}
	if got := oidcflow.HandleWithSuffix("abc", "k3x9", 2); got != "k3" {
		t.Fatalf("tinier limit %q", got)
	}
	s, err := oidcflow.NewHandleSuffix(bytes.NewReader(bytes.Repeat([]byte{200}, 16)))
	if err != nil || len(s) != 4 || strings.Trim(s, "abcdefghijklmnopqrstuvwxyz0123456789") != "" {
		t.Fatalf("suffix %q: %v", s, err)
	}
}

func TestDisplayNameStripping(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"Alex":                        "Alex",
		"  Alex  ":                    "Alex",
		"Al\u202eex":                  "Alex",
		"Al\u200eex\u2066":            "Alex",
		"Sam\x00\x07\nExample":        "SamExample",
		"Robin\u200b":                 "Robin",
		"\xffRobin":                   "Robin",
		"\u202e\u200b \t":             "",
		strings.Repeat("ü", 70):       strings.Repeat("ü", 64),
		strings.Repeat("a", 63) + " ": strings.Repeat("a", 63),
	}
	for in, want := range cases {
		got := oidcflow.SanitizeDisplayName(in, 64)
		if got != want || !utf8.ValidString(got) {
			t.Errorf("SanitizeDisplayName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExternalIdentityJSON(t *testing.T) {
	t.Parallel()
	x := oidcflow.ExternalIdentity{Issuer: "https://i.example.com", Subject: "s", AuthTime: time.Unix(1700000000, 0).UTC(), AMR: []string{"pwd"}}
	b, err := x.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"schema":"external_identity.v1"`) {
		t.Fatalf("%s", b)
	}
	back, err := oidcflow.ParseExternalIdentity(b)
	if err != nil || back.Subject != "s" || !back.AuthTime.Equal(x.AuthTime) || back.AMR[0] != "pwd" {
		t.Fatalf("%+v %v", back, err)
	}
	if _, err := oidcflow.ParseExternalIdentity([]byte(`{"schema":"other","issuer":"i","subject":"s"}`)); err == nil {
		t.Fatal("other schema accepted")
	}
}

// TestGroupsClaimParsing reads the configured group claim of an ID token (brief M1-42 "Reading
// the claim at the callback"): an array of strings is present (deduplicated), an empty array is
// present without values, a missing claim is absent, the claim name in _claim_names is the Entra
// overage, and another JSON type or a non-string element fails.
func TestGroupsClaimParsing(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, payload, state string
		groups               []string
		fail                 bool
	}{
		{name: "array of strings", payload: `{"groups":["support","marketing","support"]}`, state: oidcflow.GroupsPresent, groups: []string{"marketing", "support"}},
		{name: "empty array", payload: `{"groups":[]}`, state: oidcflow.GroupsPresent},
		{name: "absent", payload: `{"sub":"s"}`, state: oidcflow.GroupsAbsent},
		{name: "entra overage", payload: `{"_claim_names":{"groups":"src1"},"_claim_sources":{"src1":{"endpoint":"https://graph.example.com"}}}`,
			state: oidcflow.GroupsOverage},
		{name: "overage of another claim", payload: `{"_claim_names":{"roles":"src1"},"groups":["support"]}`, state: oidcflow.GroupsPresent,
			groups: []string{"support"}},
		{name: "wrong type", payload: `{"groups":"support"}`, fail: true},
		{name: "null", payload: `{"groups":null}`, fail: true},
		{name: "non-string element", payload: `{"groups":["support",7]}`, fail: true},
		{name: "null element", payload: `{"groups":["support",null]}`, fail: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			claims, err := oidcflow.ParseClaims([]byte(c.payload))
			if err != nil {
				t.Fatal(err)
			}
			state, groups, err := oidcflow.GroupsFromClaims(claims, "groups")
			if c.fail {
				if !errors.Is(err, oidcflow.ErrClaims) || strings.Contains(err.Error(), "support") {
					t.Fatalf("error %v", err)
				}
				return
			}
			if err != nil || state != c.state || strings.Join(groups, ",") != strings.Join(c.groups, ",") {
				t.Fatalf("state %q groups %v err %v, want %q %v", state, groups, err, c.state, c.groups)
			}
		})
	}
	x := oidcflow.ExternalIdentity{Issuer: "https://i.example.com", Subject: "s", AuthTime: time.Unix(1700000000, 0).UTC(),
		GroupsState: oidcflow.GroupsPresent, Groups: []string{"support"}}
	b, err := x.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	back, err := oidcflow.ParseExternalIdentity(b)
	if err != nil || back.GroupsState != oidcflow.GroupsPresent || strings.Join(back.Groups, ",") != "support" {
		t.Fatalf("%s: %+v %v", b, back, err)
	}
	for _, bad := range []string{
		`{"schema":"external_identity.v1","issuer":"i","subject":"s","auth_time":1,"groups_state":"maybe"}`,
		`{"schema":"external_identity.v1","issuer":"i","subject":"s","auth_time":1,"groups_state":"absent","groups":["support"]}`,
		`{"schema":"external_identity.v1","issuer":"i","subject":"s","auth_time":1,"groups":["support"]}`,
	} {
		if _, err := oidcflow.ParseExternalIdentity([]byte(bad)); err == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
}
