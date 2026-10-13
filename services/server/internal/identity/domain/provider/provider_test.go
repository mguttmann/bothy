// SPDX-License-Identifier: Apache-2.0

package provider_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity/domain/provider"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// refs accepts references of the form env:NAME or file:/path and refuses everything else, the way
// the parser of platform/secrets does for the two supported schemes.
type refs struct{}

func (refs) Valid(ref string) error {
	if strings.HasPrefix(ref, "env:") && len(ref) > 4 || strings.HasPrefix(ref, "file:/") {
		return nil
	}
	return errors.New("invalid reference")
}

// wantField asserts a validation_failed error with a field error at pointer.
func wantField(t *testing.T, err error, pointer string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.PublicCode() != errs.CodeValidationFailed {
		t.Fatalf("err = %v, want validation_failed at %s", err, pointer)
	}
	for _, f := range e.Fields {
		if f.Pointer == pointer {
			if strings.Contains(f.Detail, "s3cr3t") || strings.Contains(e.Detail, "s3cr3t") {
				t.Errorf("error repeats the input: %+v", e)
			}
			return
		}
	}
	t.Fatalf("fields %+v, want a field error at %s", e.Fields, pointer)
}

func TestProviderConfig_OIDC(t *testing.T) {
	t.Parallel()
	const basic = `{"issuer":"https://login.example.com","client_id":"bothy","client_auth":"client_secret_basic","client_secret_ref":"env:OIDC_SECRET"}`
	got, err := provider.ValidateOIDC(json.RawMessage(basic), refs{})
	if err != nil {
		t.Fatalf("valid basic config: %v", err)
	}
	if string(got) != basic {
		t.Errorf("normalized basic = %s, want %s", got, basic)
	}
	jwt := `{"client_auth":"private_key_jwt","issuer":"https://login.example.com/{tenantid}/v2.0","client_id":"bothy",
		"client_key_ref":"file:/run/secrets/oidc_key","allowed_tenant_ids":["tenant-a","tenant-b"],
		"app_redirect_uris":["com.example.bothy:/callback","https://app.example.com/callback"],"groups_claim":"groups",
		"client_key_id":"key-1","client_cert_x5t_s256":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}`
	got, err = provider.ValidateOIDC(json.RawMessage(jwt), refs{})
	if err != nil {
		t.Fatalf("valid private_key_jwt config: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatal(err)
	}
	if m["client_key_alg"] != "PS256" {
		t.Errorf("client_key_alg = %v, want the default PS256", m["client_key_alg"])
	}
	cases := []struct {
		name, body, pointer string
	}{
		{"missing issuer", `{"client_id":"bothy","client_auth":"client_secret_basic","client_secret_ref":"env:S"}`, "/config/issuer"},
		{"missing client_id", `{"issuer":"https://login.example.com","client_auth":"client_secret_basic","client_secret_ref":"env:S"}`, "/config/client_id"},
		{"missing client_auth", `{"issuer":"https://login.example.com","client_id":"bothy"}`, "/config/client_auth"},
		{"unknown client_auth", `{"issuer":"https://login.example.com","client_id":"bothy","client_auth":"none"}`, "/config/client_auth"},
		{"missing secret_ref", `{"issuer":"https://login.example.com","client_id":"bothy","client_auth":"client_secret_basic"}`, "/config/client_secret_ref"},
		{"missing key_ref", `{"issuer":"https://login.example.com","client_id":"bothy","client_auth":"private_key_jwt"}`, "/config/client_key_ref"},
		{"key_ref with basic", `{"issuer":"https://login.example.com","client_id":"bothy","client_auth":"client_secret_basic","client_secret_ref":"env:S","client_key_ref":"env:K"}`, "/config/client_key_ref"},
		{"secret_ref with jwt", `{"issuer":"https://login.example.com","client_id":"bothy","client_auth":"private_key_jwt","client_key_ref":"env:K","client_secret_ref":"env:S"}`, "/config/client_secret_ref"},
		{"key_alg with basic", `{"issuer":"https://login.example.com","client_id":"bothy","client_auth":"client_secret_basic","client_secret_ref":"env:S","client_key_alg":"RS256"}`, "/config/client_key_alg"},
		{"unknown key_alg", `{"issuer":"https://login.example.com","client_id":"bothy","client_auth":"private_key_jwt","client_key_ref":"env:K","client_key_alg":"HS256"}`, "/config/client_key_alg"},
		{"bad x5t", `{"issuer":"https://login.example.com","client_id":"bothy","client_auth":"private_key_jwt","client_key_ref":"env:K","client_cert_x5t_s256":"short"}`, "/config/client_cert_x5t_s256"},
		{"http issuer", `{"issuer":"http://login.example.com","client_id":"bothy","client_auth":"client_secret_basic","client_secret_ref":"env:S"}`, "/config/issuer"},
		{"issuer with query", `{"issuer":"https://login.example.com/?tenant=a","client_id":"bothy","client_auth":"client_secret_basic","client_secret_ref":"env:S"}`, "/config/issuer"},
		{"issuer with fragment", `{"issuer":"https://login.example.com/#a","client_id":"bothy","client_auth":"client_secret_basic","client_secret_ref":"env:S"}`, "/config/issuer"},
		{"issuer without host", `{"issuer":"https:///path","client_id":"bothy","client_auth":"client_secret_basic","client_secret_ref":"env:S"}`, "/config/issuer"},
		{"issuer with user info", `{"issuer":"https://alex@login.example.com","client_id":"bothy","client_auth":"client_secret_basic","client_secret_ref":"env:S"}`, "/config/issuer"},
		{"inline client_secret", `{"issuer":"https://login.example.com","client_id":"bothy","client_auth":"client_secret_basic","client_secret":"s3cr3t"}`, "/config/client_secret"},
		{"unknown member", `{"issuer":"https://login.example.com","client_id":"bothy","client_auth":"client_secret_basic","client_secret_ref":"env:S","scope":"x"}`, "/config/scope"},
		{"null value", `{"issuer":"https://login.example.com","client_id":"bothy","client_auth":"client_secret_basic","client_secret_ref":"env:S","groups_claim":null}`, "/config/groups_claim"},
		{"duplicate key", `{"issuer":"https://login.example.com","issuer":"https://other.example.com","client_id":"bothy","client_auth":"client_secret_basic","client_secret_ref":"env:S"}`, "/config/issuer"},
		{"invalid secret reference", `{"issuer":"https://login.example.com","client_id":"bothy","client_auth":"client_secret_basic","client_secret_ref":"s3cr3t"}`, "/config/client_secret_ref"},
		{"organizations issuer", `{"issuer":"https://login.example.com/organizations/v2.0","client_id":"bothy","client_auth":"client_secret_basic","client_secret_ref":"env:S"}`, "/config/issuer"},
		{"common issuer", `{"issuer":"https://login.example.com/common/v2.0","client_id":"bothy","client_auth":"client_secret_basic","client_secret_ref":"env:S"}`, "/config/issuer"},
		{"tenantid without list", `{"issuer":"https://login.example.com/{tenantid}/v2.0","client_id":"bothy","client_auth":"client_secret_basic","client_secret_ref":"env:S"}`, "/config/allowed_tenant_ids"},
		{"tenantid with empty list", `{"issuer":"https://login.example.com/{tenantid}/v2.0","client_id":"bothy","client_auth":"client_secret_basic","client_secret_ref":"env:S","allowed_tenant_ids":[]}`, "/config/allowed_tenant_ids"},
		{"tenant list without placeholder", `{"issuer":"https://login.example.com","client_id":"bothy","client_auth":"client_secret_basic","client_secret_ref":"env:S","allowed_tenant_ids":["a"]}`, "/config/allowed_tenant_ids"},
		{"bad tenant id", `{"issuer":"https://login.example.com/{tenantid}/v2.0","client_id":"bothy","client_auth":"client_secret_basic","client_secret_ref":"env:S","allowed_tenant_ids":["a/b"]}`, "/config/allowed_tenant_ids/0"},
		{"redirect http", `{"issuer":"https://login.example.com","client_id":"bothy","client_auth":"client_secret_basic","client_secret_ref":"env:S","app_redirect_uris":["http://127.0.0.1:8000/callback"]}`, "/config/app_redirect_uris/0"},
		{"redirect fragment", `{"issuer":"https://login.example.com","client_id":"bothy","client_auth":"client_secret_basic","client_secret_ref":"env:S","app_redirect_uris":["https://app.example.com/cb#x"]}`, "/config/app_redirect_uris/0"},
		{"redirect duplicate", `{"issuer":"https://login.example.com","client_id":"bothy","client_auth":"client_secret_basic","client_secret_ref":"env:S","app_redirect_uris":["com.example.app:/cb","com.example.app:/cb"]}`, "/config/app_redirect_uris/1"},
		{"wrong type", `{"issuer":"https://login.example.com","client_id":7,"client_auth":"client_secret_basic","client_secret_ref":"env:S"}`, "/config/client_id"},
		{"client_id with control", `{"issuer":"https://login.example.com","client_id":"bo\u0007thy","client_auth":"client_secret_basic","client_secret_ref":"env:S"}`, "/config/client_id"},
		{"not an object", `[]`, "/config"},
		{"empty groups_claim", `{"issuer":"https://login.example.com","client_id":"bothy","client_auth":"client_secret_basic","client_secret_ref":"env:S","groups_claim":""}`, "/config/groups_claim"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			_, err := provider.ValidateOIDC(json.RawMessage(c.body), refs{})
			wantField(t, err, c.pointer)
		})
	}
}

func TestProviderConfig_TrustedHeader(t *testing.T) {
	t.Parallel()
	got, err := provider.ValidateTrustedHeader(json.RawMessage(`{"login_header":"x-user-login","name_header":"X-User-Name"}`))
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"login_header":"X-User-Login","name_header":"X-User-Name"}`; string(got) != want {
		t.Errorf("normalized = %s, want %s", got, want)
	}
	if _, err := provider.ValidateTrustedHeader(json.RawMessage(`{"login_header":"X-User-Login"}`)); err != nil {
		t.Errorf("without name_header: %v", err)
	}
	cases := []struct{ name, body, pointer string }{
		{"missing login_header", `{"name_header":"X-User-Name"}`, "/config/login_header"},
		{"empty login_header", `{"login_header":""}`, "/config/login_header"},
		{"space", `{"login_header":"X User"}`, "/config/login_header"},
		{"colon", `{"login_header":"X-User:Login"}`, "/config/login_header"},
		{"name with space", `{"login_header":"X-User","name_header":"X Name"}`, "/config/name_header"},
		{"same header twice", `{"login_header":"X-User","name_header":"x-user"}`, "/config/name_header"},
		{"unknown member", `{"login_header":"X-User","trusted":true}`, "/config/trusted"},
		{"null", `{"login_header":null}`, "/config/login_header"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			_, err := provider.ValidateTrustedHeader(json.RawMessage(c.body))
			wantField(t, err, c.pointer)
		})
	}
}

func TestProviderConfig_Local(t *testing.T) {
	t.Parallel()
	got, err := provider.ValidateLocal(json.RawMessage(` { } `))
	if err != nil || string(got) != `{}` {
		t.Fatalf("{} = %s, %v", got, err)
	}
	wantField(t, func() error { _, err := provider.ValidateLocal(json.RawMessage(`{"a":1}`)); return err }(), "/config/a")
	wantField(t, func() error { _, err := provider.ValidateLocal(json.RawMessage(`null`)); return err }(), "/config")
}

func TestProviderName(t *testing.T) {
	t.Parallel()
	n, err := provider.NewName("  Company SSO  ", 64)
	if err != nil || n != "Company SSO" {
		t.Fatalf("NewName = %q, %v", n, err)
	}
	for _, bad := range []string{"", "   ", "Company\u202eSSO", "Company\nSSO", strings.Repeat("a", 65)} {
		if _, err := provider.NewName(bad, 64); err == nil {
			t.Errorf("NewName(%q) accepted", bad)
		} else {
			wantField(t, err, "/name")
		}
	}
}

func TestKind_Valid(t *testing.T) {
	t.Parallel()
	for _, k := range []provider.Kind{provider.KindOIDC, provider.KindTrustedHeader, provider.KindLocal} {
		if !k.Valid() {
			t.Errorf("%s invalid", k)
		}
	}
	if provider.Kind("saml").Valid() {
		t.Error("saml valid")
	}
	if _, err := provider.Validate("saml", json.RawMessage(`{}`), refs{}); err == nil {
		t.Error("Validate accepted an unknown kind")
	}
}
