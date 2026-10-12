// SPDX-License-Identifier: Apache-2.0

package actor_test

import (
	"context"
	"net/netip"
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
