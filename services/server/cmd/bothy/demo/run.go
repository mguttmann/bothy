// SPDX-License-Identifier: Apache-2.0

package demo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// ErrNotEmpty is the error of Bootstrap when the setup of the instance is already complete: the
// instance is not empty (16 16.1), and nothing was written.
var ErrNotEmpty = errors.New("demo: the instance is not empty")

// Options configure Run.
type Options struct {
	// DataSet is the loaded and validated data set (Load).
	DataSet *DataSet
	// Seed drives every choice of the warm-up (Plan).
	Seed uint64
	// APIURL is the address the command dials (loopback in the stack), PublicURL the
	// server.public_url the DPoP proofs name.
	APIURL, PublicURL string
	// Exchange is the exchange directory (16 16.5) or empty: then no agent token is created and
	// the host stays pairing.
	Exchange string
	// Pace is the wait between two warm-up prompts; a negative value takes values.pace.
	Pace time.Duration
	// FingerprintChars is terminal.fingerprint_confirm_chars of the server: how many characters
	// of the host fingerprint confirmHost takes (only with Exchange).
	FingerprintChars int
	// Bootstrap creates the owner with the system action instance.bootstrap and returns the
	// enrollment code; ErrNotEmpty when the setup is already complete. It is the first write.
	Bootstrap func(ctx context.Context, owner Person) (code string, err error)
	// Stdout receives the new enrollment code of the owner as the only line.
	Stdout io.Writer
	// Log receives progress; it never carries a code, a token or a key.
	Log *slog.Logger
	// Sleep waits d or until ctx ends; nil is the real wait.
	Sleep func(ctx context.Context, d time.Duration) error
}

// Result is what Run created, by name of the data set: the IDs of the people, agents, spaces,
// channels and the host, and the threads of the live prompts (by title). Tests and the stack
// runner read it; it holds no secret.
type Result struct {
	OwnerID     string
	People      map[string]string
	Agents      map[string]string
	Spaces      map[string]string
	Channels    map[string]string
	HostID      string
	LiveThreads map[string]string
}

// realSleep waits d or until ctx ends.
func realSleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Run turns an empty instance into the demo of the data set (16 16.1): the owner through
// Bootstrap, then every record through the public API in the sessions of the people (16 16.2),
// the host and the agents through the exchange directory when set (16 16.5), the warm-up and the
// live prompts (16 16.4), and at the end a new enrollment code of the owner on Stdout.
//
// Errors: ErrNotEmpty from Bootstrap (nothing written); a StatusError or a transport error of a
// call; errWaitExpired of a wait of the exchange. After the bootstrap the instance keeps what was
// written.
//
// Example:
//
//	res, err := demo.Run(ctx, demo.Options{DataSet: ds, Seed: 1, APIURL: api, PublicURL: pub, Pace: -1,
//		Bootstrap: boot, Stdout: os.Stdout, Log: log})
func Run(ctx context.Context, o Options) (*Result, error) {
	if o.Sleep == nil {
		o.Sleep = realSleep
	}
	if o.Log == nil {
		o.Log = slog.New(slog.DiscardHandler)
	}
	if o.Pace < 0 {
		o.Pace = time.Duration(o.DataSet.Values.Pace)
	}
	if o.Exchange != "" {
		if err := checkExchangeDir(o.Exchange); err != nil {
			return nil, err
		}
	}
	tr, err := newTransport(o.APIURL, o.PublicURL, o.Sleep)
	if err != nil {
		return nil, err
	}
	owner := o.DataSet.Owner()
	code, err := o.Bootstrap(ctx, owner)
	if err != nil {
		return nil, err
	}
	o.Log.InfoContext(ctx, "demo: owner created")
	r := &runner{o: o, tr: tr, ds: o.DataSet, res: &Result{
		People: map[string]string{}, Agents: map[string]string{}, Spaces: map[string]string{},
		Channels: map[string]string{}, LiveThreads: map[string]string{},
	}, sessions: map[string]*session{}, channels: map[string]channelRef{}}
	if err := r.run(ctx, owner, code); err != nil {
		return r.res, err
	}
	return r.res, nil
}

// runner holds the state of one Run.
type runner struct {
	o        Options
	tr       *transport
	ds       *DataSet
	res      *Result
	owner    *session
	sessions map[string]*session
	channels map[string]channelRef
}

// channelRef is a created channel: its ID, kind and main thread (text channels only).
type channelRef struct {
	id, kind, mainThread string
}

// channelKey is the key of a channel in runner.channels and Result.Channels.
func channelKey(space, channel string) string { return space + "/" + channel }

// run performs the steps after the bootstrap in the order of 16 16.1 to 16.5.
func (r *runner) run(ctx context.Context, owner Person, code string) error {
	s, err := newSession(r.tr)
	if err != nil {
		return err
	}
	tok, err := s.enroll(ctx, code, owner.DisplayName+" demo device")
	if err != nil {
		return err
	}
	r.owner = s
	r.res.OwnerID = tok.Session.UserId.String()
	r.sessions[owner.Handle] = s
	r.res.People[owner.Handle] = r.res.OwnerID
	steps := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"prices", r.prices},
		{"people", r.people},
		{"agents", r.agents},
		{"spaces", r.spaces},
		{"threads", r.threads},
		{"operators", r.operators},
		{"host", r.host},
		{"agent tokens", r.agentTokens},
		{"warm-up", r.warmup},
		{"live prompts", r.live},
	}
	for _, st := range steps {
		if err := st.fn(ctx); err != nil {
			return fmt.Errorf("%s: %w", st.name, err)
		}
		r.o.Log.InfoContext(ctx, "demo: step done", slog.String("step", st.name))
	}
	return r.finalCode(ctx)
}

// finalCode issues a new enrollment code of the owner (createEnrollmentCode, purpose recovery)
// and writes it as the only line on Stdout (16 16.1 point 4).
func (r *runner) finalCode(ctx context.Context) error {
	var out struct {
		Code string `json:"code"`
	}
	if err := r.owner.ownerPost(ctx, "createEnrollmentCode", "/users/"+r.res.OwnerID+"/enrollment-codes", nil, http.StatusCreated, &out); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(r.o.Stdout, out.Code); err != nil {
		return fmt.Errorf("demo: write the enrollment code: %w", err)
	}
	return nil
}
