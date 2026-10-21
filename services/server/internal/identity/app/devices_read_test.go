// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// allowAll is an Authorizer that allows every call.
type allowAll struct{}

func (allowAll) Authorize(context.Context, authorize.Principal, authorize.Action, authorize.Resource) (authorize.Decision, error) {
	return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
}

func (allowAll) Scope(context.Context, authorize.Principal, authorize.Action) (authorize.Scope, error) {
	return authorize.Scope{}, nil
}

// Observe implements authorize.Authorizer; the fake keeps no cache.
func (allowAll) Observe(context.Context, authorize.LogEvent) {}

func devicesWith(t *testing.T, f *fixture, az authorize.Authorizer) *app.Devices {
	t.Helper()
	d, err := app.NewDevices(app.Config{PageSizeDefault: 50, PageSizeMax: 200}, app.DevicesDeps{
		Store: f.repo, People: f.repo, UoW: f.uow, Authorizer: az, Cursors: nopCursors{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func wantDeviceCode(t *testing.T, err error, code errs.Code) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.PublicCode() != code {
		t.Fatalf("err = %v, want %s", err, code)
	}
}

func TestNewDevices_RejectsMissingDepsAndPageSizes(t *testing.T) {
	t.Parallel()
	if _, err := app.NewDevices(app.Config{PageSizeDefault: 1, PageSizeMax: 1}, app.DevicesDeps{}); err == nil {
		t.Error("missing collaborators accepted")
	}
	f := newFixture(t)
	deps := app.DevicesDeps{Store: f.repo, People: f.repo, UoW: f.uow, Authorizer: allowAll{}, Cursors: nopCursors{}}
	if _, err := app.NewDevices(app.Config{PageSizeDefault: 10, PageSizeMax: 5}, deps); err == nil {
		t.Error("default above maximum accepted")
	}
}

// TestDevices_GetAndList reads a registered device through Get and ListMine, pages with a limit
// of one, and refuses a cursor that decodes to no position.
func TestDevices_GetAndList(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	devs := devicesWith(t, f, allowAll{})
	ctx := context.Background()
	alex := f.add(domain.KindUser, "alexr")
	r1, _, _ := registration(alex.ID)
	r2, _, _ := registration(alex.ID)
	for _, r := range []identity.RegisterDevice{r1, r2} {
		if _, err := devs.Register(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	a := actor.Actor{ID: alex.ID}
	got, err := devs.Get(ctx, a, r1.DeviceID)
	if err != nil || got.ID != r1.DeviceID {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	_, err = devs.Get(ctx, a, ids.New())
	wantDeviceCode(t, err, errs.CodeNotFound)
	page, err := devs.ListMine(ctx, a, identity.Page{Limit: 1})
	if err != nil || len(page.Items) != 1 || page.NextCursor == "" {
		t.Fatalf("ListMine limit 1 = %+v, %v", page, err)
	}
	if page, err := devs.ListMine(ctx, a, identity.Page{}); err != nil || len(page.Items) != 2 || page.NextCursor != "" {
		t.Fatalf("ListMine = %+v, %v", page, err)
	}
	_, err = devs.ListMine(ctx, a, identity.Page{Cursor: "c"})
	wantDeviceCode(t, err, errs.CodeCursorInvalid)
	k, err := devs.ActiveKey(ctx, r1.DeviceID, identity.KeyPurposeDPoP)
	if err != nil || k.DeviceID != r1.DeviceID {
		t.Fatalf("ActiveKey = %+v, %v", k, err)
	}
	_, err = devs.ActiveKey(ctx, ids.New(), identity.KeyPurposeDPoP)
	wantDeviceCode(t, err, errs.CodeNotFound)
}

// TestDevices_DeniedAndUnknownAreNotFound: a denied Get and a list for an unknown person are the
// same not_found as a missing device.
func TestDevices_DeniedAndUnknownAreNotFound(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()
	alex := f.add(domain.KindUser, "alexd")
	r, _, _ := registration(alex.ID)
	if _, err := devicesWith(t, f, allowAll{}).Register(ctx, r); err != nil {
		t.Fatal(err)
	}
	denied := devicesWith(t, f, denyAll{})
	_, err := denied.Get(ctx, actor.Actor{ID: ids.New()}, r.DeviceID)
	wantDeviceCode(t, err, errs.CodeNotFound)
	_, err = denied.ListMine(ctx, actor.Actor{ID: alex.ID}, identity.Page{})
	wantDeviceCode(t, err, errs.CodeNotFound)
	_, err = devicesWith(t, f, allowAll{}).ListForUser(ctx, actor.Actor{ID: alex.ID}, ids.New(), identity.Page{})
	wantDeviceCode(t, err, errs.CodeNotFound)
}
