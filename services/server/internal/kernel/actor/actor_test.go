// SPDX-License-Identifier: Apache-2.0

package actor_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/netip"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

func TestAnonymous(t *testing.T) {
	t.Parallel()
	a := actor.Anonymous()
	if a.Kind != actor.KindAnonymous || !a.IsAnonymous() || !a.ID.IsZero() {
		t.Fatalf("Anonymous() = %+v", a)
	}
	u := actor.Actor{ID: ids.New(), Kind: actor.KindUser}
	if u.IsAnonymous() {
		t.Fatal("a user is not anonymous")
	}
}

func TestContextRoundTrip(t *testing.T) {
	t.Parallel()
	if _, ok := actor.FromContext(context.Background()); ok {
		t.Fatal("empty context has no actor")
	}
	want := actor.Actor{ID: ids.New(), Kind: actor.KindAgent, Scopes: []string{"message.read"}}
	got, ok := actor.FromContext(actor.WithActor(context.Background(), want))
	if !ok || got.ID != want.ID || got.Kind != want.Kind || len(got.Scopes) != 1 {
		t.Fatalf("FromContext = %+v, %v", got, ok)
	}
}

func TestCredentialsPresent(t *testing.T) {
	t.Parallel()
	if (actor.Credentials{}).Present() {
		t.Fatal("no Authorization header means no credentials")
	}
	if !(actor.Credentials{Authorization: "DPoP x"}).Present() {
		t.Fatal("an Authorization header means credentials")
	}
	if !(actor.Credentials{ConsoleCookie: actor.NewCookieValue("v")}).Present() {
		t.Fatal("the console cookie means credentials (M6-01)")
	}
}

// TestCookieValueRedacts: the console cookie never renders its value (SEC-LOG-1, M6-01).
func TestCookieValueRedacts(t *testing.T) {
	t.Parallel()
	c := actor.NewCookieValue("canary-cookie-value")
	var buf bytes.Buffer
	slog.New(slog.NewJSONHandler(&buf, nil)).Info("x", slog.Any("cookie", c))
	j, _ := json.Marshal(struct{ C actor.CookieValue }{c})
	for _, out := range []string{fmt.Sprint(c), fmt.Sprintf("%v %+v %#v %s", c, c, c, c), buf.String(), string(j)} {
		if strings.Contains(out, "canary") {
			t.Fatalf("value rendered: %s", out)
		}
	}
	if c.Reveal() != "canary-cookie-value" || c.IsZero() || !(actor.CookieValue{}).IsZero() {
		t.Fatal("Reveal or IsZero")
	}
}

func TestAnonymousOnly(t *testing.T) {
	t.Parallel()
	var auth actor.Authenticator = actor.AnonymousOnly{}
	ctx := context.Background()
	a, err := auth.Authenticate(ctx, actor.Credentials{Method: "GET", HTU: "https://chat.example.com/api/v1/x", ClientIP: netip.MustParseAddr("192.0.2.1")})
	if err != nil || !a.IsAnonymous() {
		t.Fatalf("without Authorization: %+v, %v", a, err)
	}
	_, err = auth.Authenticate(ctx, actor.Credentials{Authorization: "Bearer abc"})
	if got := errs.KindOf(err); got != errs.KindUnauthenticated {
		t.Fatalf("with Authorization: kind %s, err %v", got, err)
	}
	e, _ := errs.As(err)
	if e.PublicCode() != errs.CodeUnauthenticated || e.Headers.Get("WWW-Authenticate") == "" {
		t.Fatalf("error %+v must be unauthenticated with WWW-Authenticate", e)
	}
	if err := auth.Revalidate(ctx, actor.Anonymous()); err != nil {
		t.Fatalf("Revalidate(anonymous) = %v", err)
	}
	if err := auth.Revalidate(ctx, actor.Actor{ID: ids.New(), Kind: actor.KindUser}); errs.KindOf(err) != errs.KindUnauthenticated {
		t.Fatalf("Revalidate(user) = %v, want unauthenticated", err)
	}
}

// TestCredentialState: the state the authenticator read is reported only when DeviceState is set
// (M1-34); a principal built elsewhere reports none, so authz keeps its credential source.
func TestCredentialState(t *testing.T) {
	t.Parallel()
	if _, _, _, observed := (actor.Actor{ID: ids.New(), Kind: actor.KindUser, SessionActive: true}).CredentialState(); observed {
		t.Fatal("an actor without DeviceState reports observed facts")
	}
	a := actor.Actor{ID: ids.New(), Kind: actor.KindUser, SessionActive: true, DeviceState: actor.DevicePending}
	active, state, key, observed := a.CredentialState()
	if !observed || !active || state != actor.DevicePending || key {
		t.Fatalf("CredentialState = %v %q %v %v", active, state, key, observed)
	}
}

// TestActingFor_PresentAndBinding (M2-14): the zero value means no Invocation-Token header; a
// binding with a person but without a status reads as bound (fail closed); the invalid binding
// carries no IDs.
func TestActingFor_PresentAndBinding(t *testing.T) {
	t.Parallel()
	if (actor.ActingFor{}).Present() {
		t.Fatal("the zero value is no header")
	}
	if st, u, r, th := (actor.Actor{Kind: actor.KindAgent}).ActingForBinding(); st != "" || !u.IsZero() || !r.IsZero() || !th.IsZero() {
		t.Fatalf("no header: %q %v %v %v", st, u, r, th)
	}
	alex, run, thread := ids.New(), ids.New(), ids.New()
	bound := actor.Actor{Kind: actor.KindAgent, ActingFor: actor.ActingFor{Status: actor.ActingForBound, UserID: alex, RunID: run, ThreadID: thread, JTI: "j"}}
	if st, u, r, th := bound.ActingForBinding(); st != "bound" || u != alex || r != run || th != thread {
		t.Fatalf("bound: %q %v %v %v", st, u, r, th)
	}
	legacy := actor.Actor{Kind: actor.KindAgent, ActingFor: actor.ActingFor{UserID: alex}}
	if st, _, _, _ := legacy.ActingForBinding(); st != "bound" {
		t.Fatalf("a person without status: %q, want bound", st)
	}
	inv := actor.Actor{Kind: actor.KindAgent, ActingFor: actor.ActingFor{Status: actor.ActingForInvalid}}
	if st, u, _, _ := inv.ActingForBinding(); st != "invalid" || !u.IsZero() || !inv.ActingFor.Present() {
		t.Fatalf("invalid: %q %v", st, u)
	}
	if !(actor.Credentials{InvocationTokens: []string{"x"}}).Present() {
		t.Fatal("the Invocation-Token header alone is a credential (401 without Authorization)")
	}
}
