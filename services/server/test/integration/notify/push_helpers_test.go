// SPDX-License-Identifier: Apache-2.0

package notify_test

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	notifyjobs "github.com/OWNER/bothy/services/server/internal/notify/adapters/jobs"
	notifyapp "github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
	syncpkg "github.com/OWNER/bothy/services/server/internal/sync"
	syncapp "github.com/OWNER/bothy/services/server/internal/sync/app"
)

// finalized are the states of River's jobs that are no longer pending.
var finalized = []string{"completed", "cancelled", "discarded"}

// jobRow is one row of River's job table.
type jobRow struct {
	ID          int64
	Kind, State string
	ScheduledAt time.Time
	Args        json.RawMessage
}

// pending reports a job in a state that is not finalized.
func (j jobRow) pending() bool { return !slices.Contains(finalized, j.State) }

// pushJob is one job notify.push_send with its decoded arguments.
type pushJob struct {
	jobRow
	Args notifyjobs.PushSend
}

// Kind returns the kind of the hint of the job.
func (j pushJob) Kind() domain.Kind {
	k, _ := domain.KindFromCode(j.Args.KindCode)
	return k
}

// consume runs rounds of the consumer push of the world until a round finds no event, and returns the number of events processed. Errors fail the test.
func (w *world) consume() int {
	w.t.Helper()
	n, err := consumeOn(w.consumers)
	must(w.t, err)
	return n
}

// consumeOn runs rounds of the consumer push on svc until a round finds no event; it returns the
// events processed and the first error.
func consumeOn(svc *syncapp.ConsumerService) (int, error) {
	total := 0
	for {
		res, err := svc.Round(context.Background(), syncpkg.ConsumerPush)
		if err != nil {
			return total, err
		}
		if res.Outcome != syncapp.RoundProcessed {
			return total, nil
		}
		total += res.Processed
	}
}

// cursor returns last_seq of the consumer push.
func (w *world) cursor() int64 {
	return w.count(`SELECT last_seq FROM event_consumer_cursors WHERE name = 'push'`)
}

// head returns the highest seq of the event log.
func (w *world) head() int64 {
	return w.count(`SELECT coalesce(max(seq), 0) FROM event_log`)
}

// jobsOf returns the jobs of kind in id order.
func (w *world) jobsOf(kind string) []jobRow {
	w.t.Helper()
	rs, err := w.db.Query(context.Background(), `SELECT id, kind, state::text, scheduled_at, args FROM `+jobsSchema+`.river_job
		WHERE kind = $1 ORDER BY id`, kind)
	must(w.t, err)
	defer rs.Close()
	var out []jobRow
	for rs.Next() {
		var j jobRow
		must(w.t, rs.Scan(&j.ID, &j.Kind, &j.State, &j.ScheduledAt, &j.Args))
		out = append(out, j)
	}
	must(w.t, rs.Err())
	return out
}

// pendingOf returns the pending jobs of kind in id order.
func (w *world) pendingOf(kind string) []jobRow {
	return slices.DeleteFunc(w.jobsOf(kind), func(j jobRow) bool { return !j.pending() })
}

// pushJobs returns every job notify.push_send with its decoded arguments, in id order.
func (w *world) pushJobs() []pushJob {
	w.t.Helper()
	var out []pushJob
	for _, j := range w.jobsOf(notifyjobs.KindPushSend) {
		p := pushJob{jobRow: j}
		must(w.t, json.Unmarshal(j.Args, &p.Args))
		out = append(out, p)
	}
	return out
}

// pushJobsFor returns the jobs notify.push_send of recipient.
func (w *world) pushJobsFor(recipient ids.ID) []pushJob {
	return slices.DeleteFunc(w.pushJobs(), func(j pushJob) bool { return j.Args.RecipientID != recipient })
}

// hintsFor returns the distinct NotificationIDs of the jobs notify.push_send of recipient, in
// the order of their first job: the hints of the person (brief M3-19 "Tests").
func (w *world) hintsFor(recipient ids.ID) []ids.ID {
	var out []ids.ID
	for _, j := range w.pushJobsFor(recipient) {
		if !slices.Contains(out, j.Args.NotificationID) {
			out = append(out, j.Args.NotificationID)
		}
	}
	return out
}

// complete marks the job id completed, as River does after its handler returned.
func (w *world) complete(id int64) {
	w.sql(`UPDATE `+jobsSchema+`.river_job SET state = 'completed', finalized_at = now() WHERE id = $1`, id)
}

// asPushJob maps the stored arguments onto the PushJob of the use case, as the handler of the
// jobs adapter does.
func (w *world) asPushJob(a notifyjobs.PushSend) notifyapp.PushJob {
	w.t.Helper()
	kind, err := domain.KindFromCode(a.KindCode)
	must(w.t, err)
	return notifyapp.PushJob{
		NotificationID: a.NotificationID, RegistrationID: a.RegistrationID, RecipientID: a.RecipientID, ThreadID: a.ThreadID,
		MessageID: a.MessageID, TargetID: a.TargetID, Cleared: a.Cleared, Kind: kind, Quiet: a.Quiet == 1, Count: a.Count,
		DueAt: time.UnixMicro(a.DueAtUnixMicro).UTC(), Attempt: a.Attempt, ReleaseSeq: a.ReleaseSeq,
	}
}

// send runs the job notify.push_send j through Push.Send at the clock of the world and, when it
// returns nil, completes it; it returns the error of the use case.
func (w *world) send(j pushJob) error {
	w.t.Helper()
	if err := w.push.Push.Send(context.Background(), w.asPushJob(j.Args)); err != nil {
		return err
	}
	w.complete(j.ID)
	return nil
}

// sendPending runs every pending job notify.push_send due at the clock (send) and fails the test
// on an error.
func (w *world) sendPending() {
	w.t.Helper()
	now := w.clk.Now()
	for _, j := range w.pushJobs() {
		if j.pending() && !j.ScheduledAt.After(now) {
			must(w.t, w.send(j))
		}
	}
}

// flushDue runs every pending notify.coalesce_window (its handler does nothing) and
// notify.coalesce_flush (Push.Flush) scheduled at or before the clock, then completes them.
func (w *world) flushDue() {
	w.t.Helper()
	now := w.clk.Now()
	for _, j := range w.pendingOf(notifyjobs.KindCoalesceWindow) {
		if !j.ScheduledAt.After(now) {
			w.complete(j.ID)
		}
	}
	for _, j := range w.pendingOf(notifyjobs.KindCoalesceFlush) {
		if j.ScheduledAt.After(now) {
			continue
		}
		var a notifyjobs.CoalesceFlush
		must(w.t, json.Unmarshal(j.Args, &a))
		must(w.t, w.push.Push.Flush(context.Background(), a.UserID, a.ThreadID, time.UnixMicro(a.EndUnixMicro).UTC()))
		w.complete(j.ID)
	}
}

// badgeSyncsDue runs every pending notify.badge_sync_hold (its handler does nothing) and
// notify.badge_sync (Push.BadgeSync) scheduled at or before the clock, then completes them.
func (w *world) badgeSyncsDue() {
	w.t.Helper()
	now := w.clk.Now()
	for _, j := range w.pendingOf(notifyjobs.KindBadgeSyncHold) {
		if !j.ScheduledAt.After(now) {
			w.complete(j.ID)
		}
	}
	for _, j := range w.pendingOf(notifyjobs.KindBadgeSync) {
		if j.ScheduledAt.After(now) {
			continue
		}
		var a notifyjobs.BadgeSync
		must(w.t, json.Unmarshal(j.Args, &a))
		must(w.t, w.push.Push.BadgeSync(context.Background(), a.UserID, a.AfterSeq))
		w.complete(j.ID)
	}
}

// advance moves the fake clock of the world by d.
func (w *world) advance(d time.Duration) { w.clk.Advance(d) }

// collect reads the metrics of the push path.
func (w *world) collect() metricdata.ResourceMetrics {
	w.t.Helper()
	var rm metricdata.ResourceMetrics
	must(w.t, w.reader.Collect(context.Background(), &rm))
	return rm
}

// counter returns the value of the int64 counter name at the data point whose attributes hold
// every key and value of want; 0 without one.
func (w *world) counter(name string, want map[string]string) int64 {
	var total int64
	for _, sm := range w.collect().ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				w.t.Fatalf("metric %s is %T, want an int64 sum", name, m.Data)
			}
			for _, dp := range sum.DataPoints {
				match := true
				for k, v := range want {
					if got, ok := dp.Attributes.Value(attribute.Key(k)); !ok || got.AsString() != v {
						match = false
					}
				}
				if match {
					total += dp.Value
				}
			}
		}
	}
	return total
}

// skipped returns bothy.push.skipped with bothy.reason = reason.
func (w *world) skipped(reason string) int64 {
	return w.counter("bothy.push.skipped", map[string]string{attr.Reason: reason})
}

// sent returns bothy.push.sent with bothy.push.provider = provider and bothy.outcome = outcome.
func (w *world) sent(provider, outcome string) int64 {
	return w.counter("bothy.push.sent", map[string]string{attr.PushProvider: provider, attr.Outcome: outcome})
}

// logMark returns the number of captured records of the module log and the push log, the mark
// of logSince.
func (w *world) logMark() [2]int {
	return [2]int{len(w.log.Records()), len(w.pushLog.Records())}
}

// logSince returns the records of the module log and the push log captured after mark.
func (w *world) logSince(mark [2]int) []httpservertest.Record {
	return append(w.log.Records()[mark[0]:], w.pushLog.Records()[mark[1]:]...)
}
