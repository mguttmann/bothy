// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"sync"
	"time"

	"github.com/riverqueue/river"
)

// Registry holds the handlers and periodic jobs of a process. internal/wire builds one for all
// roles: the api role needs the kinds to check its inserts, the worker role runs them. New takes
// the registry; registrations after that fail, so every process has a fixed list from its start.
// It is safe for concurrent use.
type Registry struct {
	mu       sync.Mutex
	kinds    map[string]*entry
	periodic []PeriodicJob
	sealed   bool
}

// entry is one registered kind.
type entry struct {
	typ  reflect.Type
	opts HandlerOptions
	// add registers the River worker of the kind; the closure keeps the type parameter.
	add func(w *river.Workers, d workerDeps) error
}

// queue returns the queue of the kind, QueueDefault for an empty HandlerOptions.Queue.
func (e *entry) queue() string {
	if e.opts.Queue == "" {
		return QueueDefault
	}
	return e.opts.Queue
}

// insertOpts returns the River insert options of the kind with scheduledAt (zero means now) and
// the River priority prio (PriorityHighest to PriorityLowest).
func (e *entry) insertOpts(scheduledAt time.Time, prio int) *river.InsertOpts {
	return &river.InsertOpts{Queue: e.queue(), MaxAttempts: e.opts.MaxAttempts, ScheduledAt: scheduledAt, Priority: prio}
}

// PeriodicJob is a job the worker inserts on a fixed interval, for example
// sync.partition_maintenance. Only the leader among the worker processes inserts it.
type PeriodicJob struct {
	// Args are the arguments of every inserted job; their kind must be registered.
	Args Args
	// Interval is the pause between two inserts; it must be positive.
	Interval time.Duration
	// RunOnStart inserts the job once right after the worker became leader as well.
	RunOnStart bool
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{kinds: map[string]*entry{}}
}

// errSealed reports a registration after New took the registry.
var errSealed = errors.New("jobs: the registry is in use by a runtime; register before jobs.New")

// Register adds the handler h of the argument type A with its options. It checks, so that a bad
// registration fails at the start of the process:
//
//   - A passes ValidateArgsType (only IDs and int64),
//   - the kind matches "<module>.<name>" and <module> is the module of A's package,
//   - the kind is not registered yet,
//   - o.Queue is empty or one of the five queues, o.MaxAttempts is not negative.
//
// The error names the kind or the type; it also reports a nil registry or handler and a
// registry that New already took.
//
// Example:
//
//	if err := jobs.Register(reg, scanHandler, jobs.HandlerOptions{Queue: jobs.QueueFiles}); err != nil {
//		return err
//	}
func Register[A Args](r *Registry, h Handler[A], o HandlerOptions) error {
	if r == nil {
		return errors.New("jobs: Register needs a registry")
	}
	if h == nil {
		return errors.New("jobs: Register needs a handler")
	}
	t := reflect.TypeFor[A]()
	if err := ValidateArgsType(t); err != nil {
		return err
	}
	var zero A
	kind := zero.Kind()
	if err := validateKind(kind, t); err != nil {
		return err
	}
	if o.Queue != "" && !knownQueue(o.Queue) {
		return fmt.Errorf("jobs: kind %q: unknown queue %q (want one of %v)", kind, o.Queue, Queues())
	}
	if o.MaxAttempts < 0 {
		return fmt.Errorf("jobs: kind %q: MaxAttempts must not be negative", kind)
	}
	e := &entry{typ: t, opts: o}
	e.add = func(w *river.Workers, d workerDeps) error {
		return river.AddWorkerSafely(w, newWorker(h, o, kind, d))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sealed {
		return errSealed
	}
	if _, dup := r.kinds[kind]; dup {
		return fmt.Errorf("jobs: kind %q is registered twice", kind)
	}
	r.kinds[kind] = e
	return nil
}

// AddPeriodic adds a periodic job. The error reports nil arguments, a kind without a handler,
// arguments of another type than the registered one (a pointer, for example) or that do not
// survive River's JSON encoding (a zero ids.ID), an interval that is not positive, or a registry
// that New already took.
func (r *Registry) AddPeriodic(p PeriodicJob) error {
	if p.Args == nil {
		return errors.New("jobs: AddPeriodic needs arguments")
	}
	kind := p.Args.Kind()
	if p.Interval <= 0 {
		return fmt.Errorf("jobs: periodic job %q needs a positive interval", kind)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sealed {
		return errSealed
	}
	e, ok := r.kinds[kind]
	if !ok {
		return fmt.Errorf("jobs: periodic job %q has no registered handler", kind)
	}
	if reflect.TypeOf(p.Args) != e.typ {
		return fmt.Errorf("jobs: periodic job %q has arguments of type %T, registered is %s", kind, p.Args, e.typ)
	}
	if err := roundTrip(p.Args, e.typ); err != nil {
		return fmt.Errorf("jobs: periodic job %q has arguments that cannot be read back: %w", kind, err)
	}
	r.periodic = append(r.periodic, p)
	return nil
}

// ArgTypes returns the argument type of every registered kind, keyed by kind. internal/wire
// tests it against ValidateArgsType (TestWireJobArgsOnlyIDs).
func (r *Registry) ArgTypes() map[string]reflect.Type {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]reflect.Type, len(r.kinds))
	for k, e := range r.kinds {
		out[k] = e.typ
	}
	return out
}

// Periodic returns a copy of the periodic jobs in the order they were added. internal/wire
// tests it (TestPartitionMaintenanceJobRegistered).
func (r *Registry) Periodic() []PeriodicJob {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]PeriodicJob(nil), r.periodic...)
}

// Queue returns the queue of a registered kind (QueueDefault for an empty HandlerOptions.Queue)
// and true, or "" and false for an unknown kind.
func (r *Registry) Queue(kind string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.kinds[kind]
	if !ok {
		return "", false
	}
	return e.queue(), true
}

// seal freezes the registry and returns its kinds and periodic jobs.
func (r *Registry) seal() (map[string]*entry, []PeriodicJob) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sealed = true
	return maps.Clone(r.kinds), append([]PeriodicJob(nil), r.periodic...)
}
