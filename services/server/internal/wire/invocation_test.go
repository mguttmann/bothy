// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	runsapp "github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestInvocationTokenConfig binds agent.invocation_token_ttl and auth.clock_leeway (M2-06): the
// chapter defaults, and values from the environment.
func TestInvocationTokenConfig(t *testing.T) {
	t.Parallel()
	v, err := config.Load(schema(t), config.Sources{Env: testEnv()})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got, err := wire.InvocationTokenConfigFrom(v)
	if err != nil {
		t.Fatal(err)
	}
	if got.TTL != 60*time.Second || got.ClockLeeway != 30*time.Second {
		t.Fatalf("defaults %+v", got)
	}
	v, err = config.Load(schema(t), config.Sources{Env: append(testEnv(), "BOTHY_AGENT__INVOCATION_TOKEN_TTL=2m", "BOTHY_AUTH__CLOCK_LEEWAY=5s")})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, err = wire.InvocationTokenConfigFrom(v); err != nil || got.TTL != 2*time.Minute || got.ClockLeeway != 5*time.Second {
		t.Fatalf("from the environment: %+v, %v", got, err)
	}
}

// noKeys is an InvocationKeys that is never called.
type noKeys struct{}

func (noKeys) Signer(context.Context, instance.Purpose) (instance.Signer, error) { return nil, nil }
func (noKeys) VerificationKey(context.Context, instance.Purpose, string) (instance.PublicKey, error) {
	return instance.PublicKey{}, nil
}

// noState is an InstanceState that is never called.
type noState struct{}

func (noState) Load(context.Context) (instance.Record, error) { return instance.Record{}, nil }

// TestRunsBindInvocationTokens: the runs module gets its invocation tokens once, after the
// signing keys exist; a second bind and a broken configuration fail.
func TestRunsBindInvocationTokens(t *testing.T) {
	t.Parallel()
	r := &wire.Runs{}
	cfg := runsapp.InvocationTokenConfig{TTL: time.Minute, ClockLeeway: 30 * time.Second}
	if err := r.BindInvocationTokens(runsapp.InvocationTokenConfig{}, noKeys{}, noState{}, clock.System()); err == nil {
		t.Fatal("a zero TTL was accepted")
	}
	if err := r.BindInvocationTokens(cfg, noKeys{}, noState{}, clock.System()); err != nil || r.Invocations == nil {
		t.Fatalf("bind: %v, %v", err, r.Invocations)
	}
	if err := r.BindInvocationTokens(cfg, noKeys{}, noState{}, clock.System()); err == nil {
		t.Fatal("a second bind was accepted")
	}
}
