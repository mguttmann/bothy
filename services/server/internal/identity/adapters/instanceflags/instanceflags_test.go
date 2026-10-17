// SPDX-License-Identifier: Apache-2.0

package instanceflags_test

import (
	"context"
	"errors"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/instanceflags"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// recordFlags answers on and records the subject and key it was asked for.
type recordFlags struct {
	on   bool
	err  error
	subj instance.FlagSubject
	key  string
}

func (r *recordFlags) Enabled(_ context.Context, s instance.FlagSubject, key string) (bool, error) {
	r.subj, r.key = s, key
	return r.on, r.err
}

func (r *recordFlags) EvaluateAll(context.Context, instance.FlagSubject) ([]instance.EvaluatedFlag, error) {
	return nil, nil
}

func TestSwitch_AsksTheFlagForTheCaller(t *testing.T) {
	t.Parallel()
	f := &recordFlags{on: true}
	sw, err := instanceflags.New(f)
	if err != nil {
		t.Fatal(err)
	}
	sam := ids.New()
	on, err := sw.PersonalTokensEnabled(context.Background(), sam)
	if err != nil || !on || f.key != instance.FlagTokensPersonalEnabled || f.subj != (instance.FlagSubject{PrincipalID: sam, Kind: "user"}) {
		t.Fatalf("person: %v %v %+v %s", on, err, f.subj, f.key)
	}
	agent := ids.New()
	ctx := actor.WithActor(context.Background(), actor.Actor{ID: agent, Kind: actor.KindAgent})
	if _, err := sw.PersonalTokensEnabled(ctx, agent); err != nil || f.subj.Kind != "agent" {
		t.Errorf("agent caller: %+v %v", f.subj, err)
	}
	f.err = errors.New("down")
	if _, err := sw.PersonalTokensEnabled(context.Background(), sam); !errors.Is(err, f.err) {
		t.Error("error lost")
	}
	if _, err := instanceflags.New(nil); err == nil {
		t.Error("nil flags accepted")
	}
}
