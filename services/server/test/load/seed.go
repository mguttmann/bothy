// SPDX-License-Identifier: Apache-2.0

package load

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/test/smoke"
)

// seedWorkers bounds the parallel calls of the seeding.
const seedWorkers = 32

// Group is one group space: its text channel and the main thread the members write into.
type Group struct {
	Space, Channel, Thread rest.Uuid
}

// World is what Seed built: the owner session, the people in index order and their groups.
type World struct {
	Owner  *Person
	People []*Person
	Groups []Group
}

// Seed builds the world of people people as the owner through the public API: the owner session
// from the enrollment code ownerCode (a device with a step-up key, the step-up repeated whenever an
// operation asks for it), one space with one text channel per GroupSize people, every person
// created with a local account (createUser), granted write on its group space and enrolled as its
// own device from its own client address. The server must allow the seeding's calls: the owner
// alone makes two administrative calls per person, so a run raises the rate limits of the
// seeding (tools/load) and measures with the defaults. Errors name the step, never a value.
func (d *Driver) Seed(ctx context.Context, ownerCode string, gen Generator, people int) (*World, error) {
	if people <= 0 {
		return nil, errors.New("load: the world needs at least one person")
	}
	owner, err := d.enroll(ctx, ownerCode, &Person{Index: -1, Spec: PersonSpec{Handle: "alex"}, Address: ClientAddress(-1)}, true)
	if err != nil {
		return nil, fmt.Errorf("load: owner session: %w", err)
	}
	w := &World{Owner: owner, People: make([]*Person, people), Groups: make([]Group, (people+GroupSize-1)/GroupSize)}
	d.logf("seed: %d groups", len(w.Groups))
	if err := parallel(ctx, len(w.Groups), func(ctx context.Context, g int) error {
		grp, err := d.group(ctx, owner, g)
		w.Groups[g] = grp
		return err
	}); err != nil {
		return nil, err
	}
	d.logf("seed: %d people", people)
	if err := parallel(ctx, people, func(ctx context.Context, i int) error {
		p, err := d.person(ctx, owner, gen, i, w.Groups[GroupOf(i)].Space)
		w.People[i] = p
		return err
	}); err != nil {
		return nil, err
	}
	d.logf("seed: done")
	return w, nil
}

// group creates the space and text channel of group g.
func (d *Driver) group(ctx context.Context, owner *Person, g int) (Group, error) {
	var space rest.Space
	if err := d.admin(ctx, owner, request{op: "createSpace", method: http.MethodPost, path: "/spaces", body: rest.SpaceCreate{Name: fmt.Sprintf("Load %d", g+1)},
		idempotent: true, want: http.StatusCreated, out: &space}); err != nil {
		return Group{}, err
	}
	var ch rest.Channel
	if err := d.admin(ctx, owner, request{op: "createChannel", method: http.MethodPost, path: "/spaces/" + space.Id.String() + "/channels",
		body: rest.ChannelCreate{Kind: rest.ChannelCreateKindText, Name: "general"}, idempotent: true, want: http.StatusCreated, out: &ch}); err != nil {
		return Group{}, err
	}
	if ch.MainThreadId == nil {
		return Group{}, errors.New("load: createChannel returned no main_thread_id")
	}
	return Group{Space: space.Id, Channel: ch.Id, Thread: *ch.MainThreadId}, nil
}

// person creates person i with a local account, grants write on space and enrolls its device.
func (d *Driver) person(ctx context.Context, owner *Person, gen Generator, i int, space rest.Uuid) (*Person, error) {
	spec := gen.Person(i)
	var created rest.UserCreated
	// A map, not rest.UserCreate: its e-mail type comes from a package outside the black box.
	body := map[string]any{"handle": spec.Handle, "display_name": spec.DisplayName, "email": spec.Email, "local_account": true}
	if err := d.admin(ctx, owner, request{op: "createUser", method: http.MethodPost, path: "/users", body: body, idempotent: true,
		want: http.StatusCreated, out: &created}); err != nil {
		return nil, err
	}
	if created.Enrollment == nil {
		return nil, errors.New("load: createUser returned no enrollment code")
	}
	level := rest.LevelWrite
	if err := d.admin(ctx, owner, request{op: "createGrant", method: http.MethodPost, path: "/grants", body: rest.GrantCreate{
		SubjectKind: rest.GrantSubjectKindUser, SubjectId: created.User.Id, TargetKind: rest.TargetKindSpace, TargetId: &space, Level: &level,
		Reason: "capacity load test",
	}, idempotent: true, want: http.StatusCreated}); err != nil {
		return nil, err
	}
	return d.enroll(ctx, created.Enrollment.Code, &Person{Index: i, Spec: spec, ID: created.User.Id, Address: ClientAddress(i)}, false)
}

// enroll redeems code as a new device of p with fresh keys; withStepUp registers a step-up key.
func (d *Driver) enroll(ctx context.Context, code string, p *Person, withStepUp bool) (*Person, error) {
	var err error
	if p.key, err = smoke.NewKey(); err != nil {
		return nil, err
	}
	p.DeviceID = NewUUIDv7()
	reg := rest.DeviceRegistration{DeviceId: p.DeviceID, Name: "Load test", Platform: rest.DevicePlatformMacos,
		Alg: rest.DeviceRegistrationAlgES256, PublicJwk: p.key.PublicJWK()}
	if withStepUp {
		if p.stepUp, err = smoke.NewKey(); err != nil {
			return nil, err
		}
		reg.StepUpKey = &rest.DeviceKeyRegistration{Alg: rest.DeviceKeyRegistrationAlgES256, PublicJwk: p.stepUp.PublicJWK()}
	}
	var tr rest.TokenResponse
	if _, err := d.do(ctx, p, p.key, request{op: "redeemEnrollmentCode", method: http.MethodPost, path: "/auth/enroll/redeem",
		body: rest.EnrollRedeemRequest{EnrollmentCode: code, Device: reg}, want: http.StatusCreated, out: &tr, anonymous: true}); err != nil {
		return nil, err
	}
	p.adopt(tr, time.Now())
	if p.ID == (rest.Uuid{}) {
		p.ID = tr.Device.UserId
	}
	return p, nil
}

// admin sends r as the owner; a 401 step_up_required is answered with a step-up with the device's
// step-up key and r is repeated once (08 "Step-up flow").
func (d *Driver) admin(ctx context.Context, owner *Person, r request) error {
	status, err := d.do(ctx, owner, owner.key, r)
	var se *StatusError
	if err == nil || status != http.StatusUnauthorized || !errors.As(err, &se) || se.Code != "step_up_required" {
		return err
	}
	if err := d.stepUp(ctx, owner); err != nil {
		return err
	}
	_, err = d.do(ctx, owner, owner.key, r)
	return err
}

// stepUp completes a step-up of the owner's session with its step-up key.
func (d *Driver) stepUp(ctx context.Context, owner *Person) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	var ch rest.StepUpChallenge
	if _, err := d.do(ctx, owner, owner.key, request{op: "createStepUpChallenge", method: http.MethodPost, path: "/auth/step-up/challenge",
		want: http.StatusOK, out: &ch}); err != nil {
		return err
	}
	owner.mu.Lock()
	session := owner.session
	owner.mu.Unlock()
	sig, err := smoke.StepUpSignature(owner.stepUp, ch.Challenge, session, time.Now())
	if err != nil {
		return err
	}
	_, err = d.do(ctx, owner, owner.key, request{op: "completeStepUpWithDeviceKey", method: http.MethodPost, path: "/auth/step-up/device-key",
		body: rest.StepUpDeviceKeyRequest{ChallengeId: ch.ChallengeId, Signature: sig}, want: http.StatusOK})
	return err
}

// parallel runs f for 0..n-1 on at most seedWorkers goroutines (parallelN).
func parallel(ctx context.Context, n int, f func(ctx context.Context, i int) error) error {
	return parallelN(ctx, n, seedWorkers, f)
}

// parallelN runs f for 0..n-1 on at most workers goroutines and returns the first error; after an
// error the remaining calls see a cancelled context.
func parallelN(ctx context.Context, n, workers int, f func(ctx context.Context, i int) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	next := make(chan int)
	var wg sync.WaitGroup
	var once sync.Once
	var first error
	for range min(workers, n) {
		wg.Go(func() {
			for i := range next {
				if err := f(ctx, i); err != nil {
					once.Do(func() { first = err; cancel() })
				}
			}
		})
	}
feed:
	for i := range n {
		select {
		case next <- i:
		case <-ctx.Done():
			break feed
		}
	}
	close(next)
	wg.Wait()
	if first == nil && ctx.Err() != nil && n > 0 {
		first = ctx.Err()
	}
	return first
}
