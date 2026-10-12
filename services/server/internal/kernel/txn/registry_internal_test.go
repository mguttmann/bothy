// SPDX-License-Identifier: Apache-2.0

package txn

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// These tests register hooks and handlers under a chosen module through the unexported half of
// SetHook and Subscribe, because every function of this package belongs to kernel, which the
// derivation rejects as owner. The derivation itself is tested in TestModuleOfSymbol and, end to
// end with a real module package, in the unit-of-work tests of platform/pg.

// evA and evB are event types of this package; their publisher is kernel.
type (
	evA struct{ n int }
	evB struct{ n int }
)

func ctxIn(reg *Registry, p Phase) context.Context {
	u, setPhase := NewUnit(reg, Options{})
	setPhase(p)
	return WithUnit(context.Background(), u)
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// subAs subscribes fn as a handler of module.
func subAs[E any](t *testing.T, r *Registry, module string, fn func(context.Context, E) error) {
	t.Helper()
	ty := reflect.TypeFor[E]()
	must(t, subscribe(r, fn, ty, ModuleOfPackage(ty.PkgPath()), "example/internal/"+module+".handler", module))
}

func TestPublish_RegistrationOrder(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	var got []string
	subAs(t, r, "runs", func(context.Context, evA) error { got = append(got, "runs"); return nil })
	subAs(t, r, "agents", func(context.Context, evA) error { got = append(got, "agents"); return nil })
	subAs(t, r, "chat", func(context.Context, evB) error { got = append(got, "other type"); return nil })
	r.Seal()
	must(t, Publish(ctxIn(r, PhaseWork), evA{}))
	if want := []string{"runs", "agents"}; !slices.Equal(got, want) {
		t.Fatalf("delivery order = %v, want %v", got, want)
	}
}

func TestPublish_VetoStopsLaterSubscribers(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	veto := errs.New(errs.CodeConflict, "no")
	later := false
	subAs(t, r, "runs", func(context.Context, evA) error { return veto })
	subAs(t, r, "agents", func(context.Context, evA) error { later = true; return nil })
	r.Seal()
	if err := Publish(ctxIn(r, PhaseRightsCheck), evA{}); err != veto { //nolint:errorlint // the veto must come back unchanged
		t.Fatalf("Publish = %v, want the veto unchanged", err)
	}
	if later {
		t.Fatal("a subscriber after the veto ran")
	}
}

func TestPublish_NestedPublishFromSubscriber(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	var got []string
	subAs(t, r, "runs", func(ctx context.Context, ev evA) error {
		got = append(got, "runs:"+CurrentPhase(ctx).String())
		return Publish(ctx, evB(ev))
	})
	subAs(t, r, "agents", func(ctx context.Context, _ evB) error {
		got = append(got, "agents:"+CurrentPhase(ctx).String())
		return nil
	})
	r.Seal()
	must(t, Publish(ctxIn(r, PhaseWork), evA{n: 1}))
	if want := []string{"runs:work", "agents:work"}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestPublish_WrongPhase(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	ran := false
	subAs(t, r, "runs", func(context.Context, evA) error { ran = true; return nil })
	r.Seal()
	for _, p := range []Phase{PhaseSync, PhaseAudit, PhaseRightsBegin, PhaseNone} {
		err := Publish(ctxIn(r, p), evA{})
		if errs.KindOf(err) != errs.KindInternal || !strings.Contains(err.Error(), p.String()) {
			t.Errorf("Publish in phase %s = %v, want kind internal naming the phase", p, err)
		}
	}
	if ran {
		t.Fatal("a subscriber ran in a phase without events")
	}
}

// span is one span the fake tracer recorded.
type span struct {
	name    string
	outcome string
}

func TestPublish_SpanPerSubscriber(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	var spans []span
	type spanKey struct{}
	r.SetTracer(func(ctx context.Context, name string) (context.Context, func(error)) {
		i := len(spans)
		spans = append(spans, span{name: name})
		return context.WithValue(ctx, spanKey{}, name), func(err error) {
			spans[i].outcome = "ok"
			if err != nil {
				spans[i].outcome = "veto"
			}
		}
	})
	veto := errors.New("veto")
	subAs(t, r, "runs", func(ctx context.Context, _ evA) error {
		if ctx.Value(spanKey{}) != "txn.subscribe runs" {
			t.Error("the subscriber did not get the context of its span")
		}
		return nil
	})
	subAs(t, r, "agents", func(context.Context, evA) error { return veto })
	r.Seal()
	if err := Publish(ctxIn(r, PhaseWork), evA{}); !errors.Is(err, veto) {
		t.Fatalf("Publish = %v, want the veto", err)
	}
	want := []span{{"txn.subscribe runs", "ok"}, {"txn.subscribe agents", "veto"}}
	if !slices.Equal(spans, want) {
		t.Fatalf("spans = %v, want %v", spans, want)
	}
}

func TestRegistry_SetHookTwice(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	noop := func(context.Context) error { return nil }
	must(t, r.setHook(PhaseSync, noop, "a", "sync"))
	if err := r.setHook(PhaseSync, noop, "b", "sync"); err == nil {
		t.Fatal("a second hook for the same phase was accepted")
	}
}

func TestRegistry_SealRejectsChanges(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	if r.Sealed() {
		t.Fatal("a new registry is sealed")
	}
	r.Seal()
	if !r.Sealed() {
		t.Fatal("Seal did not seal")
	}
	if err := r.setHook(PhaseSync, func(context.Context) error { return nil }, "h", "sync"); !errors.Is(err, errSealed) {
		t.Errorf("SetHook after Seal = %v", err)
	}
	ty := reflect.TypeFor[evA]()
	if err := subscribe(r, func(context.Context, evA) error { return nil }, ty, "kernel", "h", "runs"); !errors.Is(err, errSealed) {
		t.Errorf("Subscribe after Seal = %v", err)
	}
	if len(r.Registrations()) != 0 {
		t.Fatal("a rejected registration was recorded")
	}
}

func TestRegistry_Registrations(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	subAs(t, r, "runs", func(context.Context, evA) error { return nil })
	must(t, r.setHook(PhaseAudit, func(context.Context) error { return nil }, "audit.h", "audit"))
	must(t, r.setHook(PhaseRightsBegin, func(context.Context) error { return nil }, "authz.h", "authz"))
	got := r.Registrations()
	want := []Registration{
		{Kind: KindSubscribe, Event: "kernel.evA", Publisher: "kernel", Subscriber: "runs", Handler: "example/internal/runs.handler"},
		{Kind: KindHook, Phase: PhaseAudit, Subscriber: "audit", Handler: "audit.h"},
		{Kind: KindHook, Phase: PhaseRightsBegin, Subscriber: "authz", Handler: "authz.h"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("registrations = %+v\nwant %+v", got, want)
	}
	got[0].Handler = "changed"
	if r.Registrations()[0].Handler == "changed" {
		t.Fatal("Registrations returns the internal slice")
	}
	if r.Hook(PhaseAudit) == nil || r.Hook(PhaseSync) != nil {
		t.Fatal("Hook does not return the registered hooks")
	}
}
