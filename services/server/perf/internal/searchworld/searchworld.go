// SPDX-License-Identifier: Apache-2.0

// Package searchworld builds the world of the search load test perf/search through the public API
// (brief M3-16 design note 4: everything that involves rights comes from a seeding server with
// raised rate limits, R-QA-11) and checks through the product that the bulk loader of
// perf/internal/searchbench wrote what the product writes (design note 6). Seed runs against an
// in-process harness server (the small-corpus tests of this package) and against the seeding
// process of perf/search alike: it takes the owner client and a function that creates a person.
//
// Example:
//
//	w, err := searchworld.Seed(ctx, srv.Owner, srv.CreateUser, plan)
//	rows, err := searchbench.BuildRows(corpus, w.IDs(), base)
//	err = searchworld.CheckThreads(ctx, w, rows, plan.ConsistencySamples)
package searchworld

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/OWNER/bothy/services/server/perf/internal/searchbench"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// Person is a seeded person with the client of its device.
type Person struct {
	Client *harness.Client
	ID     string
	Handle string
	Lang   searchbench.Lang
}

// World is the seeded world of a plan: the ids by the numbers of the plan, the people, and per
// searcher the set of channels the grants of Seed let it read, the truth of the checks (like
// harness.BuildMatrixWorld keeps it).
type World struct {
	Plan        *searchbench.Plan
	Spaces      []string
	Channels    []string
	MainThreads []string
	Authors     []Person
	Searchers   []Person
	visible     []map[string]bool
}

// IDs returns the ids the loader refers to.
func (w *World) IDs() searchbench.WorldIDs {
	authors := make([]string, len(w.Authors))
	for i, a := range w.Authors {
		authors[i] = a.ID
	}
	return searchbench.WorldIDs{Channels: w.Channels, MainThreads: w.MainThreads, Authors: authors}
}

// Visible returns the truth for searcher k: whether the grants of Seed let it read a channel.
func (w *World) Visible(k int) func(channelID string) bool {
	set := w.visible[k]
	return func(ch string) bool { return set[ch] }
}

// NewPerson creates a person with a local account and a signed-in device (harness Server
// CreateUser).
type NewPerson func(ctx context.Context, spec harness.UserSpec) (*harness.Client, error)

// seedParallel is how many seeding requests run at once.
const seedParallel = 16

// addresses numbers the simulated clients, so each person sends from its own address.
var addresses atomic.Int64

// clientAddress returns a documentation address (RFC 3849) of its own /64 for client i: the server
// limits public auth calls (the token refresh among them) per client address, so the people of the
// world must not share one.
func clientAddress(i int64) string {
	return fmt.Sprintf("2001:db8:5e%02x:%x::1", (i>>16)&0xff, i&0xffff)
}

// Seed builds the world of plan as the owner (design note 4): Spaces spaces (createSpace), each
// with TextChannelsPerSpace text channels, whose main threads come with them, and
// ForumChannelsPerSpace forum channels (createChannel); Authors people (createUser with
// local_account and one device through newPerson), the first half with locale de, the second en
// (updateMyProfile), each with write on every space; Searchers people, de and en alternating,
// searcher k with read on the spaces k mod Spaces and (k + Spaces/2) mod Spaces (createGrant).
// Handles and names are generated (space-01, text-01-1, forum-01-1, author-01, searcher-01),
// addresses at example.com. The seeding server must raise every rate limit. The error is the first
// failed step.
func Seed(ctx context.Context, owner *harness.Client, newPerson NewPerson, plan *searchbench.Plan) (*World, error) {
	w := &World{Plan: plan, Spaces: make([]string, plan.Spaces), Channels: make([]string, plan.Channels),
		MainThreads: make([]string, plan.Channels), Authors: make([]Person, plan.Authors), Searchers: make([]Person, plan.Searchers)}
	steps := []func() error{
		func() error { return w.seedSpaces(ctx, owner) },
		func() error { return w.seedChannels(ctx, owner) },
		func() error { return seedPeople(ctx, newPerson, "author", w.Authors, plan.AuthorLang) },
		func() error { return seedPeople(ctx, newPerson, "searcher", w.Searchers, plan.SearcherLang) },
		func() error { return w.seedGrants(ctx, owner) },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return nil, err
		}
	}
	return w, nil
}

// seedSpaces creates the spaces space-01 and on.
func (w *World) seedSpaces(ctx context.Context, owner *harness.Client) error {
	return parallel(ctx, w.Plan.Spaces, func(ctx context.Context, s int) error {
		var sp struct {
			ID string `json:"id"`
		}
		err := do(ctx, owner, http.MethodPost, "/spaces", map[string]any{"name": fmt.Sprintf("space-%02d", s+1)}, http.StatusCreated, &sp)
		w.Spaces[s] = sp.ID
		return err
	})
}

// seedChannels creates the text and forum channels of every space; a text channel comes with its
// main thread, a forum channel without one.
func (w *World) seedChannels(ctx context.Context, owner *harness.Client) error {
	plan := w.Plan
	return parallel(ctx, plan.Channels, func(ctx context.Context, c int) error {
		s := plan.ChannelSpace(c)
		kind := plan.ChannelKind(c)
		n := c%plan.ChannelsPerSpace + 1
		if kind == searchbench.ForumChannel {
			n -= plan.TextChannelsPerSpace
		}
		var ch struct {
			ID           string `json:"id"`
			MainThreadID string `json:"main_thread_id"`
		}
		err := do(ctx, owner, http.MethodPost, "/spaces/"+w.Spaces[s]+"/channels",
			map[string]any{"kind": string(kind), "name": fmt.Sprintf("%s-%02d-%d", kind, s+1, n)}, http.StatusCreated, &ch)
		if err == nil && (kind == searchbench.TextChannel) != (ch.MainThreadID != "") {
			err = fmt.Errorf("searchworld: channel %d of kind %s has main thread %q", c, kind, ch.MainThreadID)
		}
		w.Channels[c], w.MainThreads[c] = ch.ID, ch.MainThreadID
		return err
	})
}

// seedPeople creates the people prefix-01 and on into out, each from its own client address, and
// sets the locale of lang(i) with updateMyProfile.
func seedPeople(ctx context.Context, newPerson NewPerson, prefix string, out []Person, lang func(int) searchbench.Lang) error {
	return parallel(ctx, len(out), func(ctx context.Context, i int) error {
		handle := fmt.Sprintf("%s-%02d", prefix, i+1)
		c, err := newPerson(ctx, harness.UserSpec{Handle: handle, DisplayName: handle, Email: handle + "@example.com"})
		if err != nil {
			return err
		}
		c.ClientIP = clientAddress(addresses.Add(1))
		out[i] = Person{Client: c, ID: c.PrincipalID, Handle: handle, Lang: lang(i)}
		return do(ctx, c, http.MethodPatch, "/me/profile", map[string]any{"locale": lang(i).Locale()}, http.StatusOK, nil)
	})
}

// seedGrants gives every author write on every space and every searcher read on its two spaces,
// and keeps the channels each searcher may read.
func (w *World) seedGrants(ctx context.Context, owner *harness.Client) error {
	plan := w.Plan
	type grant struct{ subject, space, level string }
	var grants []grant
	for _, a := range w.Authors {
		for _, s := range w.Spaces {
			grants = append(grants, grant{a.ID, s, "write"})
		}
	}
	w.visible = make([]map[string]bool, plan.Searchers)
	for k, p := range w.Searchers {
		w.visible[k] = map[string]bool{}
		for _, s := range plan.SearcherSpaces(k) {
			grants = append(grants, grant{p.ID, w.Spaces[s], "read"})
		}
		for c := range plan.Channels {
			if plan.Visible(k, c) {
				w.visible[k][w.Channels[c]] = true
			}
		}
	}
	return parallel(ctx, len(grants), func(ctx context.Context, i int) error {
		g := grants[i]
		return do(ctx, owner, http.MethodPost, "/grants", map[string]any{"subject_kind": "user", "subject_id": g.subject,
			"target_kind": "space", "target_id": g.space, "level": g.level, "reason": "search load test"}, http.StatusCreated, nil)
	})
}

// do sends a request as c with a step-up when asked and decodes the answer into out (nil skips
// it); another status than want is an error naming method, path and status.
func do(ctx context.Context, c *harness.Client, method, path string, body any, want int, out any) error {
	resp, err := harness.DoWithStepUp(ctx, c, harness.Request{Method: method, Path: path, Body: body})
	if err != nil {
		return err
	}
	if resp.Status != want {
		return fmt.Errorf("searchworld: %s %s = %d %s", method, path, resp.Status, resp.Body)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(resp.Body, out); err != nil {
		return fmt.Errorf("searchworld: %s %s: %w", method, path, err)
	}
	return nil
}

// parallel runs f for 0..n-1 on up to seedParallel goroutines and returns the first error; after
// an error no further call starts.
func parallel(ctx context.Context, n int, f func(ctx context.Context, i int) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var next atomic.Int64
	var once sync.Once
	var first error
	var wg sync.WaitGroup
	for range min(seedParallel, max(n, 1)) {
		wg.Go(func() {
			for {
				i := int(next.Add(1) - 1)
				if i >= n || ctx.Err() != nil {
					return
				}
				if err := f(ctx, i); err != nil {
					once.Do(func() { first = err; cancel() })
					return
				}
			}
		})
	}
	wg.Wait()
	return first
}

// Refresh renews the token pair of every person (refreshToken, then Client.Adopt) before a phase,
// so no access token expires inside a measured window and no sample pays a renewal.
func Refresh(ctx context.Context, people []Person) error {
	return parallel(ctx, len(people), func(ctx context.Context, i int) error {
		c := people[i].Client
		_, refresh := c.Tokens()
		resp := c.DoAnonymous(ctx, harness.Request{Method: http.MethodPost, Path: "/auth/token/refresh", Body: map[string]any{"refresh_token": refresh}})
		if resp.Status != http.StatusOK {
			return fmt.Errorf("searchworld: refreshToken of %s = %d", people[i].Handle, resp.Status)
		}
		return c.Adopt(resp.Body)
	})
}
