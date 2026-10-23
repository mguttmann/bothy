// SPDX-License-Identifier: Apache-2.0

// Package pushworld builds and reads the world of the load test perf/push_handoff (brief M3-24
// design note 5): one space, the channels of the plan, senders with write and recipients with
// read on their channel, the level mentions per recipient, and one apns registration per
// recipient, written as a fixture row because registerPush (M3-22) is not part of the measured
// path. Everything except the registrations goes through the public API of a harness server; the
// registrations and the read-only queries of the checks use its admin pool, before and after the
// measurement only.
//
// Example:
//
//	w, err := pushworld.Seed(ctx, srv, plan)
//	sent := w.Send(ctx, planned, canary, due)
//	occurred, err := pushworld.Occurred(ctx, srv.AdminDB(), messageIDs)
package pushworld

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/perf/internal/pushhandoff"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// Channel is a text channel of the world with its main thread.
type Channel struct{ ID, Thread string }

// Recipient is a person who is mentioned and holds one apns registration.
type Recipient struct {
	Client *harness.Client
	Handle string
	// Registration is the ID of the push registration; its token stays inside World.
	Registration string
}

// World is the seeded world of a plan.
type World struct {
	Space      string
	Channels   []Channel
	Senders    []*harness.Client
	Recipients []Recipient
	tokens     map[string]pushhandoff.Registration
}

// Registrations returns the seeded registrations by device token, for the matcher; the tokens
// never leave the test.
func (w *World) Registrations() map[string]pushhandoff.Registration { return w.tokens }

// The fixture values of a registration (design note 5).
const (
	appID       = "com.example.chat"
	environment = "production"
	// seedParallel is how many seeding requests run at once.
	seedParallel = 32
)

// Seed builds the world of p through s: a space, p.Channels text channels, p.Senders senders and
// p.Recipients recipients (createUser with local_account, redeemEnrollmentCode with one device),
// the grants write and read per channel, the level mentions of each recipient on its channel
// (putNotificationPreference), and one apns registration per recipient (a fixture row through
// the admin pool, device and newest session of the recipient). Seeding sends no message. The
// server must raise every rate limit except rl.message. The error is the first failed step.
func Seed(ctx context.Context, s *harness.Server, p pushhandoff.Plan) (*World, error) {
	w := &World{tokens: map[string]pushhandoff.Registration{}}
	var space struct {
		ID string `json:"id"`
	}
	if err := do(ctx, s.Owner, http.MethodPost, "/spaces", map[string]any{"name": "push handoff"}, http.StatusCreated, &space); err != nil {
		return nil, err
	}
	w.Space = space.ID
	w.Channels = make([]Channel, p.Channels)
	if err := parallel(ctx, p.Channels, func(ctx context.Context, i int) error {
		var c struct {
			ID           string `json:"id"`
			MainThreadID string `json:"main_thread_id"`
		}
		err := do(ctx, s.Owner, http.MethodPost, "/spaces/"+space.ID+"/channels", map[string]any{"name": fmt.Sprintf("team-%03d", i+1), "kind": "text"}, http.StatusCreated, &c)
		w.Channels[i] = Channel{ID: c.ID, Thread: c.MainThreadID}
		return err
	}); err != nil {
		return nil, err
	}
	w.Senders = make([]*harness.Client, p.Senders)
	if err := parallel(ctx, p.Senders, func(ctx context.Context, i int) error {
		c, err := user(ctx, s, fmt.Sprintf("sender-%03d", i+1))
		if err != nil {
			return err
		}
		w.Senders[i] = c
		return grant(ctx, s, c.PrincipalID, w.Channels[i/p.SendersPerChannel].ID, "write")
	}); err != nil {
		return nil, err
	}
	w.Recipients = make([]Recipient, p.Recipients)
	var mu sync.Mutex
	if err := parallel(ctx, p.Recipients, func(ctx context.Context, i int) error {
		handle := fmt.Sprintf("recipient-%03d", i+1)
		c, err := user(ctx, s, handle)
		if err != nil {
			return err
		}
		ch := w.Channels[i/p.RecipientsPerChannel]
		if err := grant(ctx, s, c.PrincipalID, ch.ID, "read"); err != nil {
			return err
		}
		if err := do(ctx, c, http.MethodPut, "/me/notification-preferences/channel/"+ch.ID, map[string]any{"level": "mentions"}, http.StatusOK, nil); err != nil {
			return err
		}
		reg, token, err := register(ctx, s, c)
		if err != nil {
			return err
		}
		w.Recipients[i] = Recipient{Client: c, Handle: handle, Registration: reg}
		mu.Lock()
		w.tokens[token] = pushhandoff.Registration{ID: reg, Recipient: c.PrincipalID}
		mu.Unlock()
		return nil
	}); err != nil {
		return nil, err
	}
	return w, nil
}

// Send posts the planned message m as its sender into the main thread of its channel
// (createMessage): a fresh UUIDv7 client_msg_id, a content_md with canary, and for a mention the
// handles of its recipients in content_md and their IDs in mention_ids. It records the scheduled
// time due, the status and the message ID.
func (w *World) Send(ctx context.Context, m pushhandoff.Planned, canary string, due time.Time) pushhandoff.Send {
	ch := w.Channels[m.Channel]
	text := "load " + canary
	var mentionIDs []string
	for _, r := range m.Mentions {
		text += " @" + w.Recipients[r].Handle
		mentionIDs = append(mentionIDs, w.Recipients[r].Client.PrincipalID)
	}
	body := map[string]any{"client_msg_id": ids.New().String(), "content_md": text}
	if len(mentionIDs) > 0 {
		body["mention_ids"] = mentionIDs
	}
	resp := w.Senders[m.Sender].Do(ctx, harness.Request{Method: http.MethodPost, Path: "/threads/" + ch.Thread + "/messages", Body: body})
	out := pushhandoff.Send{Thread: ch.Thread, Recipients: mentionIDs, Scheduled: due, Status: resp.Status}
	if resp.Status != http.StatusCreated {
		out.Body = strings.ReplaceAll(string(resp.Body), canary, "<canary>")
		return out
	}
	var msg struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp.Body, &msg); err != nil || msg.ID == "" {
		out.Status, out.Body = 0, "createMessage answered 201 without an id"
		return out
	}
	out.MessageID = msg.ID
	return out
}

// Querier is the admin pool of the measured database.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Occurred returns occurred_at of message.created per message ID (t0 of 13 section 9), read in
// one query on the index (resource_kind, resource_id).
func Occurred(ctx context.Context, q Querier, messageIDs []string) (map[string]time.Time, error) {
	rows, err := q.Query(ctx, `SELECT resource_id::text, occurred_at FROM event_log
		WHERE resource_kind = 'message' AND resource_id = ANY($1::uuid[]) AND type = 'message.created'`, messageIDs)
	if err != nil {
		return nil, err
	}
	out := map[string]time.Time{}
	for rows.Next() {
		var id string
		var at time.Time
		if err := rows.Scan(&id, &at); err != nil {
			rows.Close()
			return nil, err
		}
		out[id] = at
	}
	return out, rows.Err()
}

// Unhealthy returns the IDs of push registrations with disabled_at set or failure_count above
// zero.
func Unhealthy(ctx context.Context, q Querier) ([]string, error) {
	rows, err := q.Query(ctx, `SELECT id::text FROM push_registrations WHERE disabled_at IS NOT NULL OR failure_count > 0 ORDER BY id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// ConsumerLag returns max(event_log.seq) minus the last_seq of the consumer push: the events
// the consumer has not read yet.
func ConsumerLag(ctx context.Context, q Querier) (int64, error) {
	var lag int64
	err := q.QueryRow(ctx, `SELECT coalesce((SELECT max(seq) FROM event_log), 0)
		- coalesce((SELECT last_seq FROM event_consumer_cursors WHERE name = 'push'), 0)`).Scan(&lag)
	return lag, err
}

// user creates the person handle with an address at example.com.
func user(ctx context.Context, s *harness.Server, handle string) (*harness.Client, error) {
	return s.CreateUser(ctx, harness.UserSpec{Handle: handle, DisplayName: handle, Email: handle + "@example.com"})
}

// grant gives subject level on channel as the owner (createGrant).
func grant(ctx context.Context, s *harness.Server, subject, channel, level string) error {
	return do(ctx, s.Owner, http.MethodPost, "/grants", map[string]any{
		"subject_kind": "user", "subject_id": subject, "target_kind": "channel", "target_id": channel, "level": level, "reason": "load test",
	}, http.StatusCreated, nil)
}

// register writes the apns registration of c: its device, the newest session of that device, a
// token of 64 random hex digits; it returns the registration ID and the token.
func register(ctx context.Context, s *harness.Server, c *harness.Client) (string, string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	id, token := ids.New().String(), hex.EncodeToString(b)
	tag, err := s.AdminDB().Exec(ctx, `INSERT INTO push_registrations (id, user_id, device_id, auth_session_id, provider, token, environment, app_id)
		SELECT $1, $2, $3, s.id, 'apns', $4, $5, $6 FROM auth_sessions s WHERE s.device_id = $3 ORDER BY s.created_at DESC, s.id DESC LIMIT 1`,
		id, c.PrincipalID, c.DeviceID, token, environment, appID)
	if err != nil {
		return "", "", fmt.Errorf("pushworld: registration of %s: %w", c.PrincipalID, err)
	}
	if tag.RowsAffected() != 1 {
		return "", "", fmt.Errorf("pushworld: registration of %s: no session of its device", c.PrincipalID)
	}
	return id, token, nil
}

// do sends a request as c with a step-up when asked and decodes the answer into out (nil skips
// it); a status other than want is an error.
func do(ctx context.Context, c *harness.Client, method, path string, body any, want int, out any) error {
	resp, err := harness.DoWithStepUp(ctx, c, harness.Request{Method: method, Path: path, Body: body})
	if err != nil {
		return err
	}
	if resp.Status != want {
		return fmt.Errorf("pushworld: %s %s = %d %s", method, path, resp.Status, resp.Body)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(resp.Body, out)
}

// parallel runs f for 0..n-1 on up to seedParallel goroutines and returns the first error.
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
