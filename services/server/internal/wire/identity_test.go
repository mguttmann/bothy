// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

func TestIdentityConfigBindsDefaults(t *testing.T) {
	t.Parallel()
	cfg, err := wire.IdentityConfig(loadValues(t, schema(t)))
	if err != nil {
		t.Fatal(err)
	}
	want := app.Config{ //nolint:gosec // G101: TokenPepperRef is a secret reference, no secret
		HandleMaxChars: 32, DisplayNameMaxChars: 64, DPoPNonceTTL: 5 * time.Minute, DPoPProofMaxAge: 60 * time.Second,
		ClockLeeway: 30 * time.Second, TokenPepperRef: "file:/run/bothy/secrets/token_pepper",
		PublicURL: "https://chat.example.com", PageSizeDefault: 50, PageSizeMax: 200, StepUpMaxAge: 5 * time.Minute,
		AccessTokenTTL: 10 * time.Minute, RefreshTokenTTL: 14 * 24 * time.Hour, SessionMaxAge: 30 * 24 * time.Hour,
		RefreshReuseGrace: 30 * time.Second, LastActiveGranularity: 5 * time.Minute,
		StepUpChallengeTTL: 2 * time.Minute, StepUpFailLimit: 5, LoginFailWindow: 15 * time.Minute, CleanupInterval: time.Hour,
		APITokenMaxTTL: 365 * 24 * time.Hour, PersonalTokenMaxTTL: 90 * 24 * time.Hour, DPoPRequiredFor: []string{"agent"},
		EnrollmentCodeTTL: 72 * time.Hour, EnrollmentCodeMaxAttempts: 5,
		DevicePendingTTL: 24 * time.Hour, DeviceVerificationCodeDigits: 6, PairingCodeTTL: 5 * time.Minute, PairingCodeChars: 8,
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("IdentityConfig = %+v, want %+v", cfg, want)
	}
}

func TestIdentityConfigBindsEnvironment(t *testing.T) {
	t.Parallel()
	v := loadValues(t, schema(t), "BOTHY_PRINCIPAL__HANDLE_MAX_CHARS=20", "BOTHY_PRINCIPAL__DISPLAY_NAME_MAX_CHARS=80",
		"BOTHY_AUTH__DPOP_NONCE_TTL=2m", "BOTHY_AUTH__DPOP_PROOF_MAX_AGE=10s", "BOTHY_AUTH__CLOCK_LEEWAY=0s",
		"BOTHY_API__PAGE_SIZE_DEFAULT=10", "BOTHY_API__PAGE_SIZE_MAX=20", "BOTHY_AUTH__DPOP_REQUIRED_FOR=agent,personal",
		"BOTHY_AUTH__API_TOKEN_MAX_TTL=30d", "BOTHY_AUTH__PERSONAL_TOKEN_MAX_TTL=7d")
	cfg, err := wire.IdentityConfig(v)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HandleMaxChars != 20 || cfg.DisplayNameMaxChars != 80 || cfg.DPoPNonceTTL != 2*time.Minute ||
		cfg.DPoPProofMaxAge != 10*time.Second || cfg.ClockLeeway != 0 || cfg.PageSizeDefault != 10 || cfg.PageSizeMax != 20 ||
		cfg.APITokenMaxTTL != 30*24*time.Hour || cfg.PersonalTokenMaxTTL != 7*24*time.Hour ||
		!reflect.DeepEqual(cfg.DPoPRequiredFor, []string{"agent", "personal"}) {
		t.Fatalf("IdentityConfig = %+v", cfg)
	}
}

func TestIdentityLimitsAtLeastOne(t *testing.T) {
	t.Parallel()
	for _, env := range []string{"BOTHY_PRINCIPAL__HANDLE_MAX_CHARS=0", "BOTHY_PRINCIPAL__DISPLAY_NAME_MAX_CHARS=0",
		"BOTHY_AUTH__DPOP_NONCE_TTL=500ms", "BOTHY_AUTH__DPOP_PROOF_MAX_AGE=0s", "BOTHY_API__PAGE_SIZE_DEFAULT=0", "BOTHY_API__PAGE_SIZE_MAX=0"} {
		if problems := load(t, env); len(problems) != 1 {
			t.Errorf("%s: problems %v, want one", env, problems)
		}
	}
}

func TestTokenPepper(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("token_pepper", strings.Repeat("ab", 32)+"\n")
	write("short", strings.Repeat("ab", 16))
	write("nothex", strings.Repeat("zz", 32))
	sp, err := secrets.NewProvider(secrets.Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	got, err := wire.TokenPepper(ctx, sp, "file:<secrets.dir>/token_pepper")
	if err != nil || len(got) != 32 || got[0] != 0xab {
		t.Fatalf("TokenPepper = %x, %v", got, err)
	}
	for _, name := range []string{"short", "nothex", "missing"} {
		_, err := wire.TokenPepper(ctx, sp, "file:<secrets.dir>/"+name)
		if err == nil {
			t.Errorf("%s: accepted", name)
			continue
		}
		if strings.Contains(err.Error(), "abab") || strings.Contains(err.Error(), "zzzz") {
			t.Errorf("%s: error names the value: %v", name, err)
		}
	}
}
