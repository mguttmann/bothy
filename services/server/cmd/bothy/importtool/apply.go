// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/api/restclient"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
)

// cursorVersion is the version of the cursor object the tool owns (22 7.3; docs/DECISIONS.md
// D-M8-04-8).
const cursorVersion = 1

// cursorDoc is the cursor object of a source: the highest fully taken-over message per Discord
// channel (main thread) and thread, and the run lease (22 7.3, 7.4).
type cursorDoc struct {
	Version  int               `json:"version"`
	Run      *leaseDoc         `json:"run,omitempty"`
	Channels map[string]string `json:"channels"`
	Threads  map[string]string `json:"threads"`
}

// leaseDoc is the run lease: the ID of the writing run and the time of its last renewal.
type leaseDoc struct {
	ID        string `json:"id"`
	RenewedAt string `json:"renewed_at"`
}

// decodeCursor reads the stored cursor; an empty or foreign object is a fresh cursor.
func decodeCursor(raw *restclient.JsonObject) cursorDoc {
	c := cursorDoc{Version: cursorVersion, Channels: map[string]string{}, Threads: map[string]string{}}
	if raw == nil || len(*raw) == 0 {
		return c
	}
	b, err := json.Marshal(*raw)
	if err != nil {
		return c
	}
	var d cursorDoc
	if json.Unmarshal(b, &d) != nil || d.Version != cursorVersion {
		return c
	}
	if d.Channels == nil {
		d.Channels = map[string]string{}
	}
	if d.Threads == nil {
		d.Threads = map[string]string{}
	}
	return d
}

// object returns the cursor as the JSON object of setImportCursor.
func (c cursorDoc) object() (restclient.JsonObject, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	var out restclient.JsonObject
	return out, json.Unmarshal(b, &out)
}

// newRunID returns a random run ID for the lease.
func newRunID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// writer holds the cursor and the run lease of an --apply run and sends every write of the run.
// One mutex serializes the lease renewal and the cursor writes of the batches (brief design note
// 8), so a renewal never overwrites a newer cursor and a lost lease stops both.
type writer struct {
	api    bothyAPI
	reads  bothyReads
	src    restclient.Uuid
	clk    clock.Clock
	runID  string
	stale  time.Duration
	mu     sync.Mutex
	cur    cursorDoc
	stats  map[string]any
	server time.Time // the server time of the lease write, from its Date header
}

// acquire reads the source and its cursor and takes the lease: a lease of another run renewed
// less than import.lease_stale_after ago ends the run with exit 3 unless takeover (22 7.4).
func (w *writer) acquire(ctx context.Context, takeover bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	src, err := w.reads.source(ctx, w.src)
	if err != nil {
		return err
	}
	w.cur = decodeCursor(src.Cursor)
	if l := w.cur.Run; l != nil && l.ID != w.runID && !takeover {
		at, err := time.Parse(time.RFC3339Nano, l.RenewedAt)
		if err == nil && w.clk.Now().Sub(at) < w.stale {
			return &leaseError{msg: "another run holds the lease of the source (renewed less than import.lease_stale_after ago); " +
				"wait for it or use --takeover"}
		}
	}
	return w.write(ctx)
}

// check re-reads the lease: when another run took it over, the run stops writing at once (22 7.4).
// The caller holds w.mu.
func (w *writer) check(ctx context.Context) error {
	src, err := w.reads.source(ctx, w.src)
	if err != nil {
		return err
	}
	if src.State != restclient.ImportSourceStateRunning {
		return &APIError{Op: "getImport", Status: http.StatusConflict, Code: "conflict", Reason: reasonNotRunning}
	}
	if l := decodeCursor(src.Cursor).Run; l == nil || l.ID != w.runID {
		return &leaseError{msg: "another run took over the lease of the source (--takeover); this run stopped writing"}
	}
	return nil
}

// write stores the cursor with a renewed lease and the tool counters. The caller holds w.mu.
func (w *writer) write(ctx context.Context) error {
	w.cur.Run = &leaseDoc{ID: w.runID, RenewedAt: w.clk.Now().UTC().Format(time.RFC3339Nano)}
	return w.put(ctx)
}

// put sends setImportCursor with the current cursor and counters. The caller holds w.mu.
func (w *writer) put(ctx context.Context) error {
	obj, err := w.cur.object()
	if err != nil {
		return err
	}
	body := restclient.SetImportCursorJSONRequestBody{Cursor: obj}
	if w.stats != nil {
		st := restclient.JsonObject(maps.Clone(w.stats))
		body.Stats = &st
	}
	resp, err := w.api.SetImportCursorWithResponse(ctx, w.src, body)
	if err != nil {
		return err
	}
	if resp.JSON200 == nil {
		return apiError("setImportCursor", resp.HTTPResponse, resp.Body)
	}
	if w.server.IsZero() {
		if at, err := http.ParseTime(resp.HTTPResponse.Header.Get("Date")); err == nil {
			w.server = at
		}
	}
	return nil
}

// renew checks and renews the lease (the ticker of the run).
func (w *writer) renew(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.check(ctx); err != nil {
		return err
	}
	return w.write(ctx)
}

// beforeBatch checks the lease before a write (22 7.4).
func (w *writer) beforeBatch(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.check(ctx)
}

// advance moves the cursor of a main thread (channel true) or thread to last after a successful
// batch, renews the lease and stores the counters in the same setImportCursor call (22 7.3).
func (w *writer) advance(ctx context.Context, channel bool, id, last string, stats map[string]any) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.check(ctx); err != nil {
		return err
	}
	if channel {
		w.cur.Channels[id] = last
	} else {
		w.cur.Threads[id] = last
	}
	w.stats = stats
	return w.write(ctx)
}

// release clears the lease at the end of the run and stores the final counters.
func (w *writer) release(ctx context.Context, stats map[string]any) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.check(ctx); err != nil {
		return err
	}
	w.stats = stats
	w.cur.Run = nil
	return w.put(ctx)
}

// position returns the stored cursor of a main thread or thread ("0" for none).
func (w *writer) position(channel bool, id string) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	m := w.cur.Threads
	if channel {
		m = w.cur.Channels
	}
	if v, ok := m[id]; ok && v != "" {
		return v
	}
	return "0"
}

// renewLoop renews the lease every import.follow.poll_interval while ctx lives, independent of the
// batches (brief design note 8): long rate-limit waits or uploads never let the lease age past
// import.lease_stale_after. A failed renewal ends the run through stop.
func (w *writer) renewLoop(ctx context.Context, every time.Duration, sleep Sleeper, stop context.CancelCauseFunc) {
	for {
		if err := sleep(ctx, every); err != nil {
			return
		}
		if err := w.renew(ctx); err != nil {
			if ctx.Err() == nil {
				stop(err)
			}
			return
		}
	}
}

// sendBatch sends one batch (applyImportBatch) and returns its items by origin key.
func sendBatch(ctx context.Context, api bothyAPI, src restclient.Uuid, b restclient.ImportBatch) (map[string]restclient.ImportItem, error) {
	resp, err := api.ApplyImportBatchWithResponse(ctx, src, b)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, apiError("applyImportBatch", resp.HTTPResponse, resp.Body)
	}
	out := make(map[string]restclient.ImportItem, len(resp.JSON200.Items))
	for _, it := range resp.JSON200.Items {
		out[it.OriginKey] = it
	}
	if len(out) != len(b.Items) {
		return nil, fmt.Errorf("bothy: applyImportBatch answered %d items for %d", len(out), len(b.Items))
	}
	return out, nil
}

// object builds an ImportObject from typed data.
func object(key, kind string, data any, sha string) (restclient.ImportObject, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return restclient.ImportObject{}, err
	}
	var obj restclient.JsonObject
	if err := json.Unmarshal(b, &obj); err != nil {
		return restclient.ImportObject{}, err
	}
	o := restclient.ImportObject{OriginKey: key, ItemKind: restclient.ImportObjectItemKind(kind), Data: obj}
	if sha != "" {
		o.ContentSha256 = &sha
	}
	return o, nil
}
