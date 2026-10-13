// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/instance/domain"
)

func TestDiscoveryMinClientVersions(t *testing.T) {
	t.Parallel()
	if got := domain.MinClientVersions(domain.ClientVersionConfig{}); got == nil || len(got) != 0 {
		t.Errorf("nothing set: %#v, want an empty non-nil list", got)
	}
	got := domain.MinClientVersions(domain.ClientVersionConfig{
		Min:         map[string]string{"windows": "1.1.0", "ios": "1.0.0"},
		Recommended: map[string]string{"ios": "1.2.0", "android": "1.3.0"},
	})
	want := []domain.ClientVersion{
		{Platform: "ios", MinVersion: "1.0.0", RecommendedVersion: "1.2.0"},
		{Platform: "android", MinVersion: domain.NoMinVersion, RecommendedVersion: "1.3.0"},
		{Platform: "windows", MinVersion: "1.1.0"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if domain.NoMinVersion != "0.0.0" {
		t.Errorf("NoMinVersion = %q", domain.NoMinVersion)
	}
}

func TestDiscoveryConstants(t *testing.T) {
	t.Parallel()
	if got := domain.APIVersions(); !reflect.DeepEqual(got, []string{"v1"}) {
		t.Errorf("APIVersions() = %v", got)
	}
	if got := domain.DiscoveryFeatures(); got == nil || len(got) != 0 {
		t.Errorf("DiscoveryFeatures() = %#v, want empty", got)
	}
	for _, p := range []string{domain.GatewayURL, domain.JWKSURI, domain.WebhookKeysURI, domain.LogoURL} {
		if !strings.HasPrefix(p, "/api/v1/") {
			t.Errorf("%s is not below /api/v1/", p)
		}
	}
}

func TestLocalPasswordEnabled(t *testing.T) {
	t.Parallel()
	local := []string{"oidc", "local"}
	if !domain.LocalPasswordEnabled(true, local) {
		t.Error("config on with a local provider: false")
	}
	if domain.LocalPasswordEnabled(false, local) {
		t.Error("config off: true")
	}
	if domain.LocalPasswordEnabled(true, []string{"oidc", "trusted_header"}) {
		t.Error("no local provider: true")
	}
}

func TestContentETag(t *testing.T) {
	t.Parallel()
	a := domain.ContentETag([]byte(`{"a":1}`))
	if a != domain.ContentETag([]byte(`{"a":1}`)) {
		t.Error("not stable")
	}
	if a == domain.ContentETag([]byte(`{"a":2}`)) {
		t.Error("two bodies share an ETag")
	}
	if len(a) != 24 || a[0] != '"' || a[23] != '"' || strings.ContainsAny(a[1:23], "+/=") {
		t.Errorf("ETag %s is not a quoted base64url text of 22 characters", a)
	}
}
