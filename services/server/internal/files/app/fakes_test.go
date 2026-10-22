// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// uow runs fn in a unit in the work phase and runs the after-commit functions on success; inside
// a unit it joins it. Units run one at a time, as row locks would order them.
type uow struct {
	reg *txn.Registry
	mu  sync.Mutex
}

func newUoW() *uow {
	reg := txn.NewRegistry()
	reg.Seal()
	return &uow{reg: reg}
}

// newUoWWith seals reg after register added its subscribers.
func newUoWWith(t testing.TB, register func(*txn.Registry) error) *uow {
	t.Helper()
	reg := txn.NewRegistry()
	if err := register(reg); err != nil {
		t.Fatal(err)
	}
	reg.Seal()
	return &uow{reg: reg}
}

func (u *uow) Do(ctx context.Context, fn func(ctx context.Context) error, _ ...txn.Option) error {
	if txn.InUnit(ctx) {
		return fn(ctx)
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	unit, ctl := txn.NewControlledUnit(u.reg, txn.Options{})
	ctl.SetPhase(txn.PhaseWork)
	if err := fn(txn.WithUnit(ctx, unit)); err != nil {
		return err
	}
	ctl.Committed()
	return nil
}

// repo is an in-memory app.Repository; the writing methods need a unit of work.
type repo struct {
	mu       sync.Mutex
	files    map[ids.ID]app.FileRecord
	sessions map[ids.ID]app.SessionRecord
	parts    map[ids.ID]map[int]app.PartRecord
	// finished and failure record the writes of the scan pass.
	finished []app.ScanResult
	failure  map[ids.ID]string
	// usedBytes and openCount override the computed limits when set.
	usedBytes *int64
	openCount *int
}

func newRepo() *repo {
	return &repo{files: map[ids.ID]app.FileRecord{}, sessions: map[ids.ID]app.SessionRecord{}, parts: map[ids.ID]map[int]app.PartRecord{},
		failure: map[ids.ID]string{}}
}

var errNoUnit = errors.New("fake repo: write outside a unit of work")

func needUnit(ctx context.Context) error {
	if !txn.InUnit(ctx) {
		return errNoUnit
	}
	return nil
}

func (r *repo) LockPrincipalUploads(ctx context.Context, _ ids.ID) error { return needUnit(ctx) }

func (r *repo) QuotaUsedBytes(_ context.Context, p ids.ID) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.usedBytes != nil {
		return *r.usedBytes, nil
	}
	var sum int64
	for _, f := range r.files {
		if f.UploadedBy == p && !f.Deleted && f.UploadState != "failed" {
			sum += f.SizeBytes
		}
	}
	return sum, nil
}

func (r *repo) OpenSessions(_ context.Context, p ids.ID, now time.Time) (int, time.Time, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n, earliest := 0, time.Time{}
	for _, s := range r.sessions {
		if s.File.UploadedBy == p && s.State == domain.SessionOpen && s.ExpiresAt.After(now) {
			n++
			if earliest.IsZero() || s.ExpiresAt.Before(earliest) {
				earliest = s.ExpiresAt
			}
		}
	}
	if r.openCount != nil {
		return *r.openCount, now.Add(90 * time.Second), nil
	}
	return n, earliest, nil
}

func (r *repo) InsertUpload(ctx context.Context, u app.NewUpload) error {
	if err := needUnit(ctx); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.files[u.File.ID] = u.File
	s := u.Session
	s.File = u.File
	r.sessions[u.File.ID] = s
	return nil
}

func (r *repo) session(fileID, p ids.ID) (app.SessionRecord, bool) {
	s, ok := r.sessions[fileID]
	if !ok || s.File.UploadedBy != p || r.files[fileID].Deleted {
		return app.SessionRecord{}, false
	}
	s.File = r.files[fileID]
	return s, true
}

func (r *repo) Session(_ context.Context, fileID, p ids.ID) (app.SessionRecord, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.session(fileID, p)
	return s, ok, nil
}

func (r *repo) LockSession(ctx context.Context, fileID, p ids.ID, _ bool) (app.SessionRecord, bool, error) {
	if err := needUnit(ctx); err != nil {
		return app.SessionRecord{}, false, err
	}
	return r.Session(ctx, fileID, p)
}

func (r *repo) SetStorageUploadID(ctx context.Context, fileID ids.ID, id string) (bool, error) {
	if err := needUnit(ctx); err != nil {
		return false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.sessions[fileID]
	if s.StorageUploadID != "" || s.State != domain.SessionOpen {
		return false, nil
	}
	s.StorageUploadID = id
	r.sessions[fileID] = s
	return true, nil
}

func (r *repo) Parts(_ context.Context, fileID ids.ID) ([]app.PartRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []app.PartRecord
	for _, p := range r.parts[fileID] {
		out = append(out, p)
	}
	slices.SortFunc(out, func(a, b app.PartRecord) int { return a.PartNumber - b.PartNumber })
	return out, nil
}

func (r *repo) Part(_ context.Context, fileID ids.ID, n int) (app.PartRecord, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.parts[fileID][n]
	return p, ok, nil
}

func (r *repo) InsertPart(ctx context.Context, p app.PartRecord) (bool, error) {
	if err := needUnit(ctx); err != nil {
		return false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.parts[p.FileID][p.PartNumber]; ok {
		return false, nil
	}
	if r.parts[p.FileID] == nil {
		r.parts[p.FileID] = map[int]app.PartRecord{}
	}
	r.parts[p.FileID][p.PartNumber] = p
	return true, nil
}

func (r *repo) CompleteSession(ctx context.Context, fileID ids.ID, at time.Time) error {
	if err := needUnit(ctx); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.sessions[fileID]
	s.State, s.CompletedAt = domain.SessionCompleted, at
	r.sessions[fileID] = s
	return nil
}

func (r *repo) AbortSession(ctx context.Context, fileID ids.ID) error {
	if err := needUnit(ctx); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.sessions[fileID]
	s.State = domain.SessionAborted
	r.sessions[fileID] = s
	f := r.files[fileID]
	f.UploadState = "failed"
	r.files[fileID] = f
	delete(r.parts, fileID)
	return nil
}

func (r *repo) File(_ context.Context, fileID ids.ID) (app.FileRecord, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	f, ok := r.files[fileID]
	if !ok || f.Deleted {
		return app.FileRecord{}, false, nil
	}
	return f, true, nil
}

// store is an in-memory app.ObjectStore that records every call and fails, with a test error, a
// call made inside a unit of work (02 8.5).
type store struct {
	t       testing.TB
	mu      sync.Mutex
	objects map[string][]byte
	uploads map[string]map[int][]byte
	calls   map[string]int
	next    int
}

func newStore(t testing.TB) *store {
	return &store{t: t, objects: map[string][]byte{}, uploads: map[string]map[int][]byte{}, calls: map[string]int{}}
}

func (s *store) enter(ctx context.Context, op string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls[op]++
	if txn.InUnit(ctx) {
		s.t.Errorf("object store %s inside a unit of work", op)
		return errors.New("object store call inside a unit of work")
	}
	return nil
}

func (s *store) count(op string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls[op]
}

func (s *store) total() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, c := range s.calls {
		n += c
	}
	return n
}

func readExactly(body io.Reader, size int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(body, size+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) != size {
		return nil, app.ErrSizeMismatch
	}
	return b, nil
}

func (s *store) Put(ctx context.Context, key string, body io.Reader, size int64) error {
	if err := s.enter(ctx, "Put"); err != nil {
		return err
	}
	b, err := readExactly(body, size)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = b
	return nil
}

func (s *store) Get(ctx context.Context, key string, _ *app.ByteRange) (io.ReadCloser, error) {
	if err := s.enter(ctx, "Get"); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.objects[key]
	if !ok {
		return nil, app.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (s *store) Head(ctx context.Context, key string) (app.ObjectInfo, error) {
	if err := s.enter(ctx, "Head"); err != nil {
		return app.ObjectInfo{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.objects[key]
	if !ok {
		return app.ObjectInfo{}, app.ErrNotFound
	}
	return app.ObjectInfo{Size: int64(len(b))}, nil
}

func (s *store) Delete(ctx context.Context, key string) error {
	if err := s.enter(ctx, "Delete"); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, key)
	return nil
}

func (s *store) CreateMultipart(ctx context.Context, _ string) (string, error) {
	if err := s.enter(ctx, "CreateMultipart"); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	id := fmt.Sprintf("upload-%d", s.next)
	s.uploads[id] = map[int][]byte{}
	return id, nil
}

func (s *store) UploadPart(ctx context.Context, _, uploadID string, n int, body io.Reader, size int64) (app.Part, error) {
	if err := s.enter(ctx, "UploadPart"); err != nil {
		return app.Part{}, err
	}
	b, err := readExactly(body, size)
	if err != nil {
		return app.Part{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.uploads[uploadID]
	if !ok {
		return app.Part{}, app.ErrUploadNotFound
	}
	u[n] = b
	sum := sha256.Sum256(b)
	return app.Part{Number: n, ETag: fmt.Sprintf(`"s3-%x"`, sum[:4])}, nil
}

func (s *store) CompleteMultipart(ctx context.Context, _, _ string, _ []app.Part) error {
	return s.enter(ctx, "CompleteMultipart")
}

func (s *store) AbortMultipart(ctx context.Context, _, uploadID string) error {
	if err := s.enter(ctx, "AbortMultipart"); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.uploads, uploadID)
	return nil
}

func (s *store) PresignUploadPart(ctx context.Context, _, _ string, _ int, _ int64, _ time.Duration) (app.PresignedPart, error) {
	if err := s.enter(ctx, "PresignUploadPart"); err != nil {
		return app.PresignedPart{}, err
	}
	return app.PresignedPart{}, app.ErrNotSupported
}

// authorizer decides by a rule per action; nil rules allow.
type authorizer struct {
	rules map[authorize.Action]authorize.Decision
}

func (a *authorizer) Authorize(_ context.Context, _ authorize.Principal, act authorize.Action, _ authorize.Resource) (authorize.Decision, error) {
	if d, ok := a.rules[act]; ok {
		return d, nil
	}
	return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
}

func (a *authorizer) Scope(context.Context, authorize.Principal, authorize.Action) (authorize.Scope, error) {
	return authorize.Scope{}, nil
}

func (a *authorizer) Observe(context.Context, authorize.LogEvent) {}

// events keeps the appended events; Append needs a unit of work.
type events struct {
	mu   sync.Mutex
	list []eventlog.Event
}

func (e *events) Append(ctx context.Context, evs ...eventlog.Event) error {
	if err := needUnit(ctx); err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.list = append(e.list, evs...)
	return nil
}

// channels locates every channel in one space.
type channels struct{ space ids.ID }

func (c channels) SpaceOf(context.Context, ids.ID) (ids.ID, error) { return c.space, nil }

// names keeps names as they are.
type names struct{}

func (names) NFC(s string) string { return s }

// types maps a media type to its extensions.
type types map[string][]string

func (t types) Extensions(mt string) []string { return t[mt] }

// freeSpace reports a fixed number of free bytes.
type freeSpace struct{ free int64 }

func (f freeSpace) FreeBytes(context.Context) (int64, error) { return f.free, nil }

// testCfg is the configuration of the app tests: parts of 4 bytes.
var testCfg = app.Config{
	MaxUploadBytes: 64, PartBytes: 4, SessionTTL: time.Hour, MaxOpenSessions: 3, QuotaBytes: 0, NameMaxBytes: 64,
	MinFreeBytes: 10,
}

// env is a service over the fakes.
type env struct {
	svc    *app.Service
	repo   *repo
	store  *store
	az     *authorizer
	events *events
	clk    *clock.Fake
	uow    *uow
	scans  *scanQueue
	alex   actor.Actor
	ctx    context.Context
}

// newEnv builds the service with the part strategy of driver ("fs" or "s3").
func newEnv(t testing.TB, driver string, cfg app.Config) *env {
	t.Helper()
	e := &env{repo: newRepo(), store: newStore(t), az: &authorizer{rules: map[authorize.Action]authorize.Decision{}}, events: &events{},
		clk: clock.NewFake(time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)), uow: newUoW(),
		alex: actor.Actor{ID: ids.New(), Kind: actor.KindUser}}
	parts := app.NewFSPartStore(e.store)
	var fs app.FreeSpace = freeSpace{free: 1 << 30}
	if driver == "s3" {
		parts, fs = app.NewS3PartStore(e.store, e.repo, e.uow), nil
	}
	e.scans = &scanQueue{}
	svc, err := app.New(cfg, app.Deps{Repo: e.repo, Parts: parts, UoW: e.uow, Authorizer: e.az, Events: e.events,
		Channels: channels{space: ids.New()}, Names: names{}, Types: types{"image/png": {".png"}}, FreeSpace: fs, Clock: e.clk,
		ScanQueue: e.scans})
	if err != nil {
		t.Fatal(err)
	}
	e.svc = svc
	e.ctx = actor.WithActor(context.Background(), e.alex)
	return e
}

// digestOf is the Content-Digest header of b.
func digestOf(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha-256=:" + base64.StdEncoding.EncodeToString(sum[:]) + ":"
}
