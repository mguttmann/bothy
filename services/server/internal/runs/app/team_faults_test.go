// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// The team ports behind the fault gate: every call counts and the armed call fails.

type gatedTeamRepo struct {
	app.TeamRepository
	g *faultGate
}

func (r gatedTeamRepo) UpsertTeam(ctx context.Context, ch, sp ids.ID, at time.Time) (app.TeamRow, error) {
	if err := r.g.hit(); err != nil {
		return app.TeamRow{}, err
	}
	return r.TeamRepository.UpsertTeam(ctx, ch, sp, at)
}

func (r gatedTeamRepo) Team(ctx context.Context, ch ids.ID) (app.TeamRow, bool, error) {
	if err := r.g.hit(); err != nil {
		return app.TeamRow{}, false, err
	}
	return r.TeamRepository.Team(ctx, ch)
}

func (r gatedTeamRepo) LockTeam(ctx context.Context, ch ids.ID) (app.TeamRow, bool, error) {
	if err := r.g.hit(); err != nil {
		return app.TeamRow{}, false, err
	}
	return r.TeamRepository.LockTeam(ctx, ch)
}

func (r gatedTeamRepo) DeleteTeam(ctx context.Context, ch ids.ID) (bool, error) {
	if err := r.g.hit(); err != nil {
		return false, err
	}
	return r.TeamRepository.DeleteTeam(ctx, ch)
}

func (r gatedTeamRepo) SaveTeamBrake(ctx context.Context, ch ids.ID, b domain.TeamBrake, at time.Time) (app.TeamRow, error) {
	if err := r.g.hit(); err != nil {
		return app.TeamRow{}, err
	}
	return r.TeamRepository.SaveTeamBrake(ctx, ch, b, at)
}

func (r gatedTeamRepo) InsertTeamMember(ctx context.Context, m app.TeamMemberRow) error {
	if err := r.g.hit(); err != nil {
		return err
	}
	return r.TeamRepository.InsertTeamMember(ctx, m)
}

func (r gatedTeamRepo) TeamMembers(ctx context.Context, ch ids.ID) ([]app.TeamMemberRow, error) {
	if err := r.g.hit(); err != nil {
		return nil, err
	}
	return r.TeamRepository.TeamMembers(ctx, ch)
}

func (r gatedTeamRepo) TeamMemberByWorkThread(ctx context.Context, th ids.ID) (app.TeamMemberRow, bool, error) {
	if err := r.g.hit(); err != nil {
		return app.TeamMemberRow{}, false, err
	}
	return r.TeamRepository.TeamMemberByWorkThread(ctx, th)
}

func (r gatedTeamRepo) DeleteTeamMember(ctx context.Context, ch, m ids.ID) (bool, error) {
	if err := r.g.hit(); err != nil {
		return false, err
	}
	return r.TeamRepository.DeleteTeamMember(ctx, ch, m)
}

func (r gatedTeamRepo) InsertTeamInput(ctx context.Context, in app.TeamInputRow) (bool, error) {
	if err := r.g.hit(); err != nil {
		return false, err
	}
	return r.TeamRepository.InsertTeamInput(ctx, in)
}

func (r gatedTeamRepo) TeamInput(ctx context.Context, m ids.ID) (app.TeamInputRow, bool, error) {
	if err := r.g.hit(); err != nil {
		return app.TeamInputRow{}, false, err
	}
	return r.TeamRepository.TeamInput(ctx, m)
}

func (r gatedTeamRepo) TeamInputDelivered(ctx context.Context, src, m ids.ID) (bool, error) {
	if err := r.g.hit(); err != nil {
		return false, err
	}
	return r.TeamRepository.TeamInputDelivered(ctx, src, m)
}

func (r gatedTeamRepo) InsertTeamMirror(ctx context.Context, src, mirror, m ids.ID, at time.Time) (bool, error) {
	if err := r.g.hit(); err != nil {
		return false, err
	}
	return r.TeamRepository.InsertTeamMirror(ctx, src, mirror, m, at)
}

func (r gatedTeamRepo) TeamMirrored(ctx context.Context, src ids.ID) (bool, error) {
	if err := r.g.hit(); err != nil {
		return false, err
	}
	return r.TeamRepository.TeamMirrored(ctx, src)
}

type gatedTeamMsgs struct {
	app.TeamMessages
	g *faultGate
}

func (m gatedTeamMsgs) CopyMessage(ctx context.Context, cmd messaging.CopyCmd) (messaging.Copied, error) {
	if err := m.g.hit(); err != nil {
		return messaging.Copied{}, err
	}
	return m.TeamMessages.CopyMessage(ctx, cmd)
}

func (m gatedTeamMsgs) AppendSystemNote(ctx context.Context, th, author ids.ID, text string) (ids.ID, int64, error) {
	if err := m.g.hit(); err != nil {
		return ids.ID{}, 0, err
	}
	return m.TeamMessages.AppendSystemNote(ctx, th, author, text)
}

type gatedTeamReaders struct {
	app.TeamReaders
	g *faultGate
}

func (r gatedTeamReaders) Readers(ctx context.Context, ch ids.ID) ([]ids.ID, error) {
	if err := r.g.hit(); err != nil {
		return nil, err
	}
	return r.TeamReaders.Readers(ctx, ch)
}

type gatedTeamSched struct {
	app.TeamScheduler
	g *faultGate
}

func (s gatedTeamSched) ScheduleTeamInput(ctx context.Context, m ids.ID) error {
	if err := s.g.hit(); err != nil {
		return err
	}
	return s.TeamScheduler.ScheduleTeamInput(ctx, m)
}

// gatedTeamWorld is a team world whose use cases and team ports all go through one fault gate,
// set up (team on, @coding and @review) before the gate is armed.
func gatedTeamWorld(t *testing.T) (*teamWorld, *faultGate) {
	t.Helper()
	w := newTeamWorld(t, defaultTeamConfig())
	g := &faultGate{}
	w.gate = g
	w.build()
	if err := w.svc.BindTeams(w.cfg, app.TeamDeps{
		Repo: gatedTeamRepo{w.trepo, g}, Messages: gatedTeamMsgs{w.tmsgs, g}, Readers: gatedTeamReaders{w.readers, g},
		Owners: gatedOwners{w.owners, g}, Schedule: gatedTeamSched{w.sched, g},
	}); err != nil {
		t.Fatal(err)
	}
	w.setUp()
	return w, g
}

// TestTeam_InfrastructureErrorsEndTheOperation sweeps every port call of the team operations and
// the job runs.team_input: the use case fails at call n with an infrastructure error, which the
// operation returns (so the unit of work rolls back and the job retries) instead of continuing
// or hiding it. The sweep ends at the first n the operation no longer reaches.
func TestTeam_InfrastructureErrorsEndTheOperation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		prep func(w *teamWorld) func() error
	}{
		{"enable", func(w *teamWorld) func() error {
			return func() error { _, err := w.svc.Teams().Enable(asUser(w.alex), w.homeB); return err }
		}},
		{"get", func(w *teamWorld) func() error {
			return func() error { _, err := w.svc.Teams().Get(asUser(w.alex), w.team); return err }
		}},
		{"add member", func(w *teamWorld) func() error {
			if err := w.svc.Teams().RemoveMember(asUser(w.alex), w.team, w.review.ID); err != nil {
				w.t.Fatal(err)
			}
			return func() error {
				_, err := w.svc.Teams().AddMember(asUser(w.alex), runs.AddTeamMember{ChannelID: w.team, AgentID: w.texter, HomeChannelID: w.homeB, Handle: "review"})
				return err
			}
		}},
		{"remove member", func(w *teamWorld) func() error {
			return func() error { return w.svc.Teams().RemoveMember(asUser(w.alex), w.team, w.review.ID) }
		}},
		{"stop", func(w *teamWorld) func() error {
			w.reply(w.coding, "working")
			return func() error { _, err := w.svc.Teams().Stop(asUser(w.alex), w.team); return err }
		}},
		{"disable", func(w *teamWorld) func() error {
			return func() error { return w.svc.Teams().Disable(asUser(w.alex), w.team) }
		}},
		{"owner input", func(w *teamWorld) func() error {
			m := w.ownerMessage(w.alex, "@coding build feature X")
			return func() error { return w.svc.TeamInput(context.Background(), m) }
		}},
		{"member reply", func(w *teamWorld) func() error {
			r := w.reply(w.coding, "@review please review")
			if err := w.repo.RecordAnswer(context.Background(), r, w.runOf[r]); err != nil {
				w.t.Fatal(err)
			}
			return func() error { return w.svc.TeamInput(context.Background(), r) }
		}},
		{"refused member reply", func(w *teamWorld) func() error {
			r := w.replyBy(w.coding, "@review please review", w.sam)
			if err := w.repo.RecordAnswer(context.Background(), r, w.runOf[r]); err != nil {
				w.t.Fatal(err)
			}
			return func() error { return w.svc.TeamInput(context.Background(), r) }
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			for at := 1; at < 200; at++ {
				w, g := gatedTeamWorld(t)
				op := c.prep(w)
				g.mu.Lock()
				g.armed, g.at, g.n = true, at, 0
				g.mu.Unlock()
				err := op()
				if !g.fired {
					if err != nil && errors.Is(err, errInjected) {
						t.Fatalf("call %d: injected error without a fired gate", at)
					}
					if at == 1 {
						t.Fatal("the operation calls no gated port")
					}
					return
				}
				if !errors.Is(err, errInjected) {
					t.Fatalf("call %d: the operation returned %v, want the injected error", at, err)
				}
			}
			t.Fatal("the sweep did not end")
		})
	}
}
