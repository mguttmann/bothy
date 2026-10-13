// SPDX-License-Identifier: Apache-2.0

package identity_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity"
)

// TestToken_Redacted: a token never renders its value through fmt, slog or encoding/json
// (SEC-LOG-1); only Reveal returns it.
func TestToken_Redacted(t *testing.T) {
	t.Parallel()
	const canary = "canary-token-value"
	tok := identity.NewToken(canary)
	set := identity.TokenSet{AccessToken: tok, RefreshToken: tok}
	var buf bytes.Buffer
	slog.New(slog.NewJSONHandler(&buf, nil)).Info("x", "token", tok, "set", set)
	j, err := json.Marshal(set)
	if err != nil {
		t.Fatal(err)
	}
	for _, out := range []string{
		fmt.Sprint(tok), fmt.Sprintf("%v %+v %#v %q %x %s", tok, set, set, tok, tok, tok), buf.String(), string(j),
	} {
		if strings.Contains(out, canary) {
			t.Errorf("token value in %q", out)
		}
	}
	if tok.Reveal() != canary || (identity.Token{}).Reveal() != "" || !(identity.Token{}).IsZero() || tok.IsZero() {
		t.Error("Reveal or IsZero")
	}
}

// TestEnums: the values of auth method and revoke reason of 03 auth_sessions.
func TestEnums(t *testing.T) {
	t.Parallel()
	for _, m := range []string{"oidc", "trusted_header", "passkey", "password_totp", "enrollment_code", "device_pairing"} {
		if !identity.AuthMethod(m).Valid() {
			t.Errorf("auth method %s", m)
		}
	}
	for _, r := range []string{"logout", "admin", "device_revoked", "refresh_reuse", "user_disabled", "expired", "inactive",
		"step_up_failed", "trusted_header_mismatch"} {
		if !identity.SessionRevokeReason(r).Valid() {
			t.Errorf("reason %s", r)
		}
	}
	if identity.AuthMethod("magic").Valid() || identity.SessionRevokeReason("stolen").Valid() {
		t.Error("unknown values are valid")
	}
}
