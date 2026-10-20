// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
)

var errBoom = errors.New("boom")

// TestNew_RejectsAnIncompleteSetup: a missing collaborator, a payload limit below 1 and an
// instance default outside the levels end New; without a clock and a log New still succeeds.
func TestNew_RejectsAnIncompleteSetup(t *testing.T) {
	t.Parallel()
	full := app.Deps{Store: &fakeStore{}, UoW: noUoW{}, Authorizer: noAuthz{}, Events: &appender{}, Paths: &fakePaths{},
		Settings: &fakeSettings{}, Users: &fakeUsers{}}
	ok := app.Config{EventPayloadMaxBytes: 4096, UserDefaultLevel: "mentions"}
	noStore := full
	noStore.Store = nil
	cases := map[string]struct {
		cfg app.Config
		d   app.Deps
	}{
		"missing store":     {ok, noStore},
		"payload limit 0":   {app.Config{EventPayloadMaxBytes: 0, UserDefaultLevel: "mentions"}, full},
		"unknown level":     {app.Config{EventPayloadMaxBytes: 4096, UserDefaultLevel: "loud"}, full},
		"empty level value": {app.Config{EventPayloadMaxBytes: 4096}, full},
	}
	for name, c := range cases {
		if _, err := app.New(c.cfg, c.d); err == nil {
			t.Errorf("%s: New succeeded", name)
		}
	}
	if _, err := app.New(ok, full); err != nil {
		t.Fatalf("defaults: %v", err)
	}
}

// stepAuthz allows every decision except call number failAt (1-based), which returns errBoom.
type stepAuthz struct {
	authorize.Authorizer
	calls, failAt int
}

func (a *stepAuthz) Authorize(context.Context, authorize.Principal, authorize.Action, authorize.Resource) (authorize.Decision, error) {
	a.calls++
	if a.calls == a.failAt {
		return authorize.Decision{}, errBoom
	}
	return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
}

// listStore returns the stored rows of List or an error.
type listStore struct {
	fakeStore
	rows []notify.Preference
	err  error
}

func (s *listStore) Preferences(context.Context, ids.ID) ([]notify.Preference, error) {
	return s.rows, s.err
}

// pathErr fails every path read.
type pathErr struct{ fakePaths }

func (*pathErr) NotificationPath(context.Context, string, ids.ID) (chat.NotificationPath, error) {
	return chat.NotificationPath{}, errBoom
}

// TestList_PassesErrorsOn: an error of the authorizer (settings.read or the read check of a row),
// of the store and of the chat path read ends List with that error, also for an anonymous caller.
func TestList_PassesErrorsOn(t *testing.T) {
	t.Parallel()
	th := ids.New()
	row := notify.Preference{Target: notify.Target{Kind: chat.TargetThread, ID: th}, Level: "all", Version: 1}
	okPaths := &fakePaths{path: chat.NotificationPath{ThreadID: th}}
	cases := map[string]struct {
		store *listStore
		paths app.PathSource
		authz *stepAuthz
	}{
		"settings.read fails": {&listStore{}, okPaths, &stepAuthz{failAt: 1}},
		"store fails":         {&listStore{err: errBoom}, okPaths, &stepAuthz{}},
		"path read fails":     {&listStore{rows: []notify.Preference{row}}, &pathErr{}, &stepAuthz{}},
		"row check fails":     {&listStore{rows: []notify.Preference{row}}, okPaths, &stepAuthz{failAt: 2}},
	}
	for name, c := range cases {
		svc, err := app.New(app.Config{EventPayloadMaxBytes: 4096, UserDefaultLevel: "mentions"}, app.Deps{
			Store: c.store, UoW: noUoW{}, Authorizer: c.authz, Events: &appender{}, Paths: c.paths, Settings: &fakeSettings{}, Users: &fakeUsers{},
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Preferences().List(context.Background()); !errors.Is(err, errBoom) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// failingPaths fails ChannelIDsOfSpace or ThreadIDsOfChannels.
type failingPaths struct {
	fakePaths
	channelsErr, threadsErr error
}

func (p *failingPaths) ChannelIDsOfSpace(context.Context, ids.ID) ([]ids.ID, error) {
	return nil, p.channelsErr
}

func (p *failingPaths) ThreadIDsOfChannels(context.Context, []ids.ID) ([]ids.ID, error) {
	return nil, p.threadsErr
}

// failingStore fails the cleanup delete.
type failingStore struct{ fakeStore }

func (*failingStore) DeletePreferencesOfTargets(context.Context, ids.ID, []ids.ID, []ids.ID) ([]app.PreferenceRow, error) {
	return nil, errBoom
}

// failingAppender fails every append.
type failingAppender struct{}

func (failingAppender) Append(context.Context, ...eventlog.Event) error { return errBoom }

// TestSubscribers_PassErrorsOn: Bind refuses nil collaborators, and every error of chat, the
// store and the appender ends the archive subscriber with that error, so the unit rolls back.
func TestSubscribers_PassErrorsOn(t *testing.T) {
	t.Parallel()
	if err := app.NewSubscribers(&fakeStore{}).Bind(nil, &fakePaths{}); err == nil {
		t.Fatal("Bind accepted a nil appender")
	}
	row := app.PreferenceRow{UserID: ids.New(), Preference: notify.Preference{Target: notify.Target{Kind: chat.TargetSpace, ID: ids.New()}, Level: "all", Version: 1}}
	type run func(*app.Subscribers) error
	space := func(s *app.Subscribers) error {
		return s.OnSpaceArchived(context.Background(), chat.SpaceArchived{SpaceID: ids.New()})
	}
	channel := func(s *app.Subscribers) error {
		return s.OnChannelArchived(context.Background(), chat.ChannelArchived{ChannelID: ids.New()})
	}
	cases := map[string]struct {
		store  app.Store
		events eventlog.Appender
		paths  app.PathSource
		call   run
	}{
		"space: channels fail":  {&fakeStore{}, &appender{}, &failingPaths{channelsErr: errBoom}, space},
		"space: threads fail":   {&fakeStore{}, &appender{}, &failingPaths{threadsErr: errBoom}, space},
		"channel: threads fail": {&fakeStore{}, &appender{}, &failingPaths{threadsErr: errBoom}, channel},
		"delete fails":          {&failingStore{}, &appender{}, &failingPaths{}, channel},
		"append fails":          {&fakeStore{deleted: []app.PreferenceRow{row}}, failingAppender{}, &failingPaths{}, space},
	}
	for name, c := range cases {
		s := app.NewSubscribers(c.store)
		if err := s.Bind(c.events, c.paths); err != nil {
			t.Fatal(err)
		}
		if err := c.call(s); !errors.Is(err, errBoom) {
			t.Errorf("%s: %v", name, err)
		}
	}
}
