// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// uniqueOpts are River's unique options of InsertUniqueTx: by args, blocking in every state that
// is not finalized. River requires available, pending, running and scheduled in ByState;
// retryable is added because a job waiting for its next attempt still stands. The states are
// untyped constants so that only the river package is imported; the read of PendingTx names the
// same five states.
func uniqueOpts() river.UniqueOpts {
	u := river.UniqueOpts{ByArgs: true}
	u.ByState = append(u.ByState, "available", "pending", "retryable", "running", "scheduled")
	return u
}

// InsertUniqueTx implements Inserter with River's unique jobs by args and state: River hashes
// the kind and the compared args into the job's unique key, and a unique index over the key and
// the states of uniqueOpts turns a second insert into a skip that returns the standing row.
func (i inserter) InsertUniqueTx(ctx context.Context, args Args, opts *InsertOptions) (InsertResult, error) {
	tx, e, err := i.prepare(ctx, "InsertUniqueTx", args)
	if err != nil {
		return InsertResult{}, err
	}
	o := e.insertOpts(scheduledAt(opts))
	o.UniqueOpts = uniqueOpts()
	res, err := i.rt.client.InsertTx(ctx, tx, args, o)
	if err != nil {
		return InsertResult{}, fmt.Errorf("jobs: unique insert %s: %w", args.Kind(), err)
	}
	if res == nil || res.Job == nil {
		return InsertResult{}, errs.New(errs.CodeInternalError, "jobs: unique insert of "+args.Kind()+" returned no job")
	}
	return InsertResult{Inserted: !res.UniqueSkippedAsDuplicate, ScheduledAt: res.Job.ScheduledAt.UTC()}, nil
}

// PendingTx implements Inserter. River keeps only a hash of the compared args, so the read
// compares the stored JSON instead: every compared field present in args must be equal in the
// job (jsonb equality, also for lists of IDs), and every compared field absent from the encoding
// of args (omitempty) must be absent in the job, as River's hash would differ otherwise.
func (i inserter) PendingTx(ctx context.Context, args Args) (time.Time, bool, error) {
	tx, e, err := i.prepare(ctx, "PendingTx", args)
	if err != nil {
		return time.Time{}, false, err
	}
	want, absent, err := comparedArgs(args, e.typ)
	if err != nil {
		return time.Time{}, false, errs.Wrap(errs.CodeInternalError, err, "jobs: PendingTx of "+args.Kind())
	}
	q := "SELECT scheduled_at FROM " + pgx.Identifier{i.rt.cfg.Schema, "river_job"}.Sanitize() +
		` WHERE kind = $1 AND state IN ('available', 'pending', 'retryable', 'running', 'scheduled')
		AND args @> $2::jsonb
		AND NOT EXISTS (SELECT 1 FROM jsonb_each($2::jsonb) AS w(key, value) WHERE args -> w.key IS DISTINCT FROM w.value)
		AND NOT (args ?| $3::text[])
		ORDER BY scheduled_at, id LIMIT 1`
	var at time.Time
	err = tx.QueryRow(ctx, q, args.Kind(), want, absent).Scan(&at)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("jobs: read the pending %s: %w", args.Kind(), err)
	}
	return at.UTC(), true, nil
}

// comparedArgs returns the JSON object of the compared fields of args that its encoding holds,
// and the JSON names of the compared fields it omits. The compared fields are those tagged
// river:"unique" when t tags any, else all fields: River's rule for unique jobs by args.
func comparedArgs(args Args, t reflect.Type) (string, []string, error) {
	b, err := json.Marshal(args)
	if err != nil {
		return "", nil, err
	}
	var encoded map[string]json.RawMessage
	if err := json.Unmarshal(b, &encoded); err != nil {
		return "", nil, err
	}
	names := uniqueNames(t)
	if len(names) == 0 {
		names = jsonNames(t)
	}
	want := make(map[string]json.RawMessage, len(names))
	absent := []string{}
	for _, n := range names {
		if v, ok := encoded[n]; ok {
			want[n] = v
		} else {
			absent = append(absent, n)
		}
	}
	out, err := json.Marshal(want)
	return string(out), absent, err
}

// uniqueNames returns the JSON names of the fields of t tagged river:"unique" (the tag may list
// more comma-separated values, trimmed of spaces as River trims them), sorted.
func uniqueNames(t reflect.Type) []string {
	var out []string
	for i := range t.NumField() {
		f := t.Field(i)
		name, ok := jsonName(f)
		if !ok {
			continue
		}
		for _, v := range strings.Split(f.Tag.Get("river"), ",") {
			if strings.TrimSpace(v) == "unique" {
				out = append(out, name)
				break
			}
		}
	}
	slices.Sort(out)
	return out
}

// jsonNames returns the JSON names of every encoded field of t.
func jsonNames(t reflect.Type) []string {
	var out []string
	for i := range t.NumField() {
		if name, ok := jsonName(t.Field(i)); ok {
			out = append(out, name)
		}
	}
	return out
}

// jsonName returns the key encoding/json uses for f; ok is false for an unexported field or the
// tag "-". ValidateArgsType allows only flat structs, so no embedded field needs care here.
func jsonName(f reflect.StructField) (string, bool) {
	if !f.IsExported() {
		return "", false
	}
	tag := f.Tag.Get("json")
	if tag == "-" {
		return "", false
	}
	if name, _, _ := strings.Cut(tag, ","); name != "" {
		return name, true
	}
	return f.Name, true
}
