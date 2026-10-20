// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// cleanedPrefix marks the bytes the fake codec returns as the cleaned original.
const cleanedPrefix = "cleaned:"

// codec is a scripted ImageCodec: the cleaned original is cleanedPrefix plus the input, the
// variants are fixed bytes per variant.
type codec struct {
	mu        sync.Mutex
	cleanErr  error
	renderErr error
	// second makes every Body after the first yield these bytes (the input changed).
	second []byte
	bodies int
	// renderedAt records the store objects that existed when RenderVariants ran.
	renderedAt []string
	store      *store
	info       app.ImageInfo
}

func (c *codec) CleanOriginal(ctx context.Context, _ domain.OriginalPlan, open app.Opener, _ int64, _ app.CleanLimits) (app.CleanedOriginal, error) {
	if c.cleanErr != nil {
		return app.CleanedOriginal{}, c.cleanErr
	}
	r, err := open(ctx)
	if err != nil {
		return app.CleanedOriginal{}, err
	}
	in, err := io.ReadAll(r)
	_ = r.Close()
	if err != nil {
		return app.CleanedOriginal{}, err
	}
	out := append([]byte(cleanedPrefix), in...)
	return app.CleanedOriginal{Size: int64(len(out)), SHA256: sha256.Sum256(out), MIMEType: "image/jpeg", Width: 30, Height: 40,
		Body: func(context.Context) (io.ReadCloser, error) {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.bodies++
			if c.second != nil {
				return io.NopCloser(bytes.NewReader(c.second)), nil
			}
			return io.NopCloser(bytes.NewReader(out)), nil
		}}, nil
}

func (c *codec) Inspect(context.Context, string, app.Opener) (app.ImageInfo, error) {
	return c.info, nil
}

func (c *codec) RenderVariants(_ context.Context, _ app.Opener, spec app.RenderSpec) ([]app.RenderedVariant, error) {
	c.mu.Lock()
	c.store.mu.Lock()
	for k := range c.store.objects {
		c.renderedAt = append(c.renderedAt, k)
	}
	c.store.mu.Unlock()
	c.mu.Unlock()
	if c.renderErr != nil {
		return nil, c.renderErr
	}
	var out []app.RenderedVariant
	for _, v := range spec.Variants {
		out = append(out, app.RenderedVariant{Variant: v.Variant, MIMEType: "image/jpeg", Width: v.LongestEdge, Height: v.LongestEdge,
			Data: []byte("variant " + string(v.Variant))})
	}
	return out, nil
}

// movies returns a fixed MovieInfo or error.
type movies struct {
	info app.MovieInfo
	err  error
}

func (m movies) ReadMovieInfo(r io.ReaderAt, size int64) (app.MovieInfo, error) {
	b := make([]byte, 4)
	if _, err := r.ReadAt(b, size-4); err != nil {
		return app.MovieInfo{}, err
	}
	return m.info, m.err
}

var imageCfg = app.ImageConfig{MaxPixels: 1000, JPEGQuality: 90, ProcessingTimeout: time.Minute, MaxConcurrent: 2, SmallPx: 10,
	MediumPx: 20, LargePx: 30, ThumbnailJPEGQuality: 80}

// imageEnv is the job over the fakes.
type imageEnv struct {
	proc      *app.ImageProcessor
	repo      *repo
	store     *store
	codec     *codec
	events    *events
	hooks     *app.ImageHooks
	mu        sync.Mutex
	finalized []ids.ID
}

func newImageEnv(t *testing.T, mv app.MovieReader) *imageEnv {
	t.Helper()
	e := &imageEnv{repo: newRepo(), store: newStore(t), events: &events{}, hooks: &app.ImageHooks{}}
	e.codec = &codec{store: e.store}
	if mv == nil {
		mv = movies{}
	}
	u := newUoWWith(t, func(r *txn.Registry) error {
		return txn.Subscribe(r, func(_ context.Context, ev files.FileFinalized) error {
			e.mu.Lock()
			defer e.mu.Unlock()
			e.finalized = append(e.finalized, ev.FileID)
			return nil
		})
	})
	p, err := app.NewImageProcessor(imageCfg, app.ImageDeps{Repo: e.repo, Store: e.store, UoW: u, Events: e.events,
		Channels: channels{space: ids.New()}, Codec: e.codec, Movies: mv, Hooks: e.hooks})
	if err != nil {
		t.Fatal(err)
	}
	e.proc = p
	return e
}

// seed stores a scanned file of mime with content as revision 1 in upload state state.
func (e *imageEnv) seed(t *testing.T, mime, state string, content []byte) app.FileRecord {
	t.Helper()
	id := ids.New()
	key, err := domain.OriginalKey(id, 1)
	if err != nil {
		t.Fatal(err)
	}
	f := app.FileRecord{ID: id, ObjectKey: key, SHA256: sha256.Sum256(content), SizeBytes: int64(len(content)), MIMEType: mime,
		OriginalName: "canary-photo.jpg", UploadedBy: ids.New(), Purpose: "attachment", ScopeChannelID: ids.New(), UploadState: state,
		ScanStatus: "skipped", TranscriptStatus: "none"}
	e.repo.files[id] = f
	e.repo.sessions[id] = app.SessionRecord{FileID: id, State: domain.SessionCompleted, File: f}
	e.store.objects[key] = content
	return f
}

func (e *imageEnv) keys() []string {
	e.store.mu.Lock()
	defer e.store.mu.Unlock()
	var out []string
	for k := range e.store.objects {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// TestProcessImageCommitsOriginalBeforeThumbnails: T1 commits the cleaned original before any
// variant is written; a render error leaves the file complete without thumbnails.
func TestProcessImageCommitsOriginalBeforeThumbnails(t *testing.T) {
	t.Parallel()
	e := newImageEnv(t, nil)
	f := e.seed(t, "image/jpeg", "pending", []byte("a photo of Sam"))
	if err := e.proc.ProcessImage(context.Background(), f.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := e.repo.imageFile(f.ID)
	k2, _ := domain.OriginalKey(f.ID, 2)
	want := append([]byte(cleanedPrefix), "a photo of Sam"...)
	if got.UploadState != "complete" || got.ObjectKey != k2 || got.SHA256 != sha256.Sum256(want) || got.SizeBytes != int64(len(want)) ||
		got.Width != 30 || got.Height != 40 || !bytes.Equal(e.store.objects[k2], want) {
		t.Fatalf("file %+v", got)
	}
	if _, ok := e.store.objects[f.ObjectKey]; ok {
		t.Error("revision 1 remains")
	}
	if !slices.Equal(e.repo.writes, []string{"T1", "T2"}) {
		t.Errorf("writes %v", e.repo.writes)
	}
	for _, k := range e.codec.renderedAt {
		if strings.HasSuffix(k, "/small") || strings.HasSuffix(k, "/medium") || strings.HasSuffix(k, "/large") {
			t.Errorf("variant %s written before the render", k)
		}
	}
	if len(got.Thumbnails) != 3 || len(e.events.list) != 2 || len(e.finalized) != 1 {
		t.Errorf("thumbnails %d, events %d, finalized %d", len(got.Thumbnails), len(e.events.list), len(e.finalized))
	}
	view := app.FileView(got)
	if len(view.Thumbnails) != 3 || view.Thumbnails[0].Variant != "small" || view.Thumbnails[2].Variant != "large" {
		t.Errorf("view thumbnails %+v", view.Thumbnails)
	}
	for _, ev := range e.events.list {
		if bytes.Contains(ev.Payload, []byte("null")) || bytes.Contains(ev.Payload, []byte(cleanedPrefix)) {
			t.Errorf("payload %s", ev.Payload)
		}
	}

	// A render error: complete, no thumbnails, no retry.
	e2 := newImageEnv(t, nil)
	e2.codec.renderErr = app.ErrImageUndecodable
	f2 := e2.seed(t, "image/png", "pending", []byte("pixels"))
	if err := e2.proc.ProcessImage(context.Background(), f2.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := e2.repo.imageFile(f2.ID); got.UploadState != "complete" || len(got.Thumbnails) != 0 || len(e2.events.list) != 1 {
		t.Errorf("render error: %+v, %d events", got, len(e2.events.list))
	}
	// A transient render error is retried.
	e3 := newImageEnv(t, nil)
	transient := errors.New("store down")
	e3.codec.renderErr = transient
	f3 := e3.seed(t, "image/png", "pending", []byte("pixels"))
	if err := e3.proc.ProcessImage(context.Background(), f3.ID); !errors.Is(err, transient) {
		t.Errorf("transient render error: %v", err)
	}
}

// TestProcessImageTwoPassMismatchRetries: a second pass with other bytes returns an error and
// writes no row and no object.
func TestProcessImageTwoPassMismatchRetries(t *testing.T) {
	t.Parallel()
	for _, second := range [][]byte{[]byte("cleaned:another photo"), []byte("cleaned:a photo of Robin!"), []byte("short")} {
		e := newImageEnv(t, nil)
		e.codec.second = second
		f := e.seed(t, "image/jpeg", "pending", []byte("a photo of Robin"))
		if err := e.proc.ProcessImage(context.Background(), f.ID); err == nil {
			t.Fatalf("%q: no error", second)
		}
		got, _ := e.repo.imageFile(f.ID)
		if got.UploadState != "pending" || got.ObjectKey != f.ObjectKey || len(e.repo.writes) != 0 || len(e.events.list) != 0 {
			t.Errorf("%q: file %+v, writes %v", second, got, e.repo.writes)
		}
		if k := e.keys(); len(k) != 1 || k[0] != f.ObjectKey {
			t.Errorf("%q: objects %v", second, k)
		}
	}
}

// TestProcessImageMalformedFails (design note 10b): a JPEG the filter refuses fails with
// image_malformed, file.updated and FileFinalized; no object of the file remains.
func TestProcessImageMalformedFails(t *testing.T) {
	t.Parallel()
	e := newImageEnv(t, nil)
	e.codec.cleanErr = app.ErrImageMalformed
	f := e.seed(t, "image/jpeg", "pending", []byte("\xff\xd8truncated"))
	if err := e.proc.ProcessImage(context.Background(), f.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := e.repo.imageFile(f.ID)
	if got.UploadState != "failed" || e.repo.failure[f.ID] != "image_malformed" || len(e.events.list) != 1 || len(e.finalized) != 1 {
		t.Errorf("file %+v, reason %q, events %d", got, e.repo.failure[f.ID], len(e.events.list))
	}
	if k := e.keys(); len(k) != 0 {
		t.Errorf("objects %v", k)
	}
	// A GIF that cannot be decoded stays revision 1 and becomes complete without thumbnails.
	e2 := newImageEnv(t, nil)
	e2.codec.cleanErr = app.ErrImageUndecodable
	e2.codec.info = app.ImageInfo{Width: 7, Height: 5, Frames: 1}
	g := e2.seed(t, "image/gif", "pending", []byte("GIF89a"))
	if err := e2.proc.ProcessImage(context.Background(), g.ID); err != nil {
		t.Fatal(err)
	}
	got, _ = e2.repo.imageFile(g.ID)
	if got.UploadState != "complete" || got.ObjectKey != g.ObjectKey || got.Width != 7 || len(got.Thumbnails) != 0 || len(e2.events.list) != 1 {
		t.Errorf("unchanged GIF %+v", got)
	}
}

// TestProcessImageDeletedMidway (design note 10e): deleted between the object write and T1, the
// row stays, no event, and the object of revision 2 is removed.
func TestProcessImageDeletedMidway(t *testing.T) {
	t.Parallel()
	e := newImageEnv(t, nil)
	f := e.seed(t, "image/png", "pending", []byte("pixels"))
	e.hooks.AfterPut = func(context.Context, ids.ID) { e.repo.setDeleted(f.ID) }
	if err := e.proc.ProcessImage(context.Background(), f.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := e.repo.imageFile(f.ID)
	if got.UploadState != "pending" || got.ObjectKey != f.ObjectKey || len(e.events.list) != 0 {
		t.Errorf("file %+v, events %d", got, len(e.events.list))
	}
	if k := e.keys(); len(k) != 1 || k[0] != f.ObjectKey {
		t.Errorf("objects %v", k)
	}
}

// TestProcessImageRetryAfterFailedCommit and run twice: T1 fails once, the retry ends with
// revision 2, no leftover object and one event per transaction; a third run changes nothing.
func TestProcessImageRetryAfterFailedCommit(t *testing.T) {
	t.Parallel()
	e := newImageEnv(t, nil)
	f := e.seed(t, "image/jpeg", "pending", []byte("a photo"))
	failOnce := errors.New("crash before commit")
	e.hooks.BeforeCommit = func(context.Context, ids.ID) error {
		err := failOnce
		failOnce = nil
		return err
	}
	if err := e.proc.ProcessImage(context.Background(), f.ID); err == nil {
		t.Fatal("first run: no error")
	}
	for range 2 {
		if err := e.proc.ProcessImage(context.Background(), f.ID); err != nil {
			t.Fatal(err)
		}
	}
	k2, _ := domain.OriginalKey(f.ID, 2)
	want := []string{k2}
	for _, v := range []domain.Variant{domain.VariantLarge, domain.VariantMedium, domain.VariantSmall} {
		k, _ := domain.VariantKey(f.ID, v)
		want = append(want, k)
	}
	slices.Sort(want)
	if k := e.keys(); !slices.Equal(k, want) {
		t.Errorf("objects %v, want %v", k, want)
	}
	if len(e.events.list) != 2 || len(e.finalized) != 1 {
		t.Errorf("events %d, finalized %d", len(e.events.list), len(e.finalized))
	}
	puts := e.store.count("Put")
	if err := e.proc.ProcessImage(context.Background(), f.ID); err != nil || e.store.count("Put") != puts || len(e.events.list) != 2 {
		t.Errorf("a run over a done file: %v, puts %d, events %d", err, e.store.count("Put")-puts, len(e.events.list))
	}
}

// TestProcessImageContainerValues (design note 3e): an MP4 gets size and duration once; a box
// error leaves them absent.
func TestProcessImageContainerValues(t *testing.T) {
	t.Parallel()
	e := newImageEnv(t, movies{info: app.MovieInfo{Width: 1080, Height: 1920, HasSize: true, DurationMS: 4500, HasDuration: true}})
	f := e.seed(t, "video/mp4", "complete", []byte("....moov...."))
	for range 2 {
		if err := e.proc.ProcessImage(context.Background(), f.ID); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := e.repo.imageFile(f.ID)
	if got.Width != 1080 || got.Height != 1920 || got.DurationMS != 4500 || len(e.events.list) != 1 {
		t.Errorf("file %+v, events %d", got, len(e.events.list))
	}
	e2 := newImageEnv(t, movies{err: errors.New("box beyond its parent")})
	f2 := e2.seed(t, "video/quicktime", "complete", []byte("....moov...."))
	if err := e2.proc.ProcessImage(context.Background(), f2.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := e2.repo.imageFile(f2.ID); got.Width != 0 || got.DurationMS != 0 || len(e2.events.list) != 0 {
		t.Errorf("broken boxes: %+v", got)
	}
}

// TestProcessImageConcurrencyLimit: never more runs at once than file.image.max_concurrent.
func TestProcessImageConcurrencyLimit(t *testing.T) {
	t.Parallel()
	e := newImageEnv(t, nil)
	var mu sync.Mutex
	active, peak := 0, 0
	e.hooks.BeforeRun = func(context.Context, ids.ID) error {
		mu.Lock()
		active++
		peak = max(peak, active)
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		return nil
	}
	e.hooks.AfterRun = func(ids.ID) {
		mu.Lock()
		active--
		mu.Unlock()
	}
	var seeded []app.FileRecord
	for range 6 {
		seeded = append(seeded, e.seed(t, "application/pdf", "complete", []byte("%PDF")))
	}
	var wg sync.WaitGroup
	for _, f := range seeded {
		wg.Go(func() { _ = e.proc.ProcessImage(context.Background(), f.ID) })
	}
	wg.Wait()
	if peak != imageCfg.MaxConcurrent {
		t.Errorf("peak %d, want %d", peak, imageCfg.MaxConcurrent)
	}
}
