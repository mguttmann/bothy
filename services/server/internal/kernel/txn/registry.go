// SPDX-License-Identifier: Apache-2.0

package txn

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"
)

// Hook is a phase hook: authz for the two rights phases, sync and audit (02 "Ablauf"). It runs
// inside the transaction; an error rolls the unit back.
type Hook func(ctx context.Context) error

// Tracer starts a span named spanName and returns its context and the function that ends it
// with the error of the subscriber (nil for ok, else veto). platform/otel provides it and
// internal/wire sets it; without a tracer subscribers run without spans.
type Tracer func(ctx context.Context, spanName string) (context.Context, func(err error))

// The kinds of a Registration.
const (
	// KindHook is a phase hook.
	KindHook = "hook"
	// KindSubscribe is a subscription to an in-transaction event.
	KindSubscribe = "subscribe"
)

// Registration describes one hook or subscription as the golden file of internal/wire lists it
// (02 "Weitere Prüfungen im Gate"). Modules are derived from Go package paths, never declared.
type Registration struct {
	// Kind is KindHook or KindSubscribe.
	Kind string
	// Phase is the phase of a hook; PhaseNone for a subscription.
	Phase Phase
	// Event is "<module>.<TypeName>" of a subscription ("kernel.<TypeName>" for kernel
	// events); empty for a hook.
	Event string
	// Publisher is the module of the event type; empty for a hook.
	Publisher string
	// Subscriber is the module that owns the handler or hook.
	Subscriber string
	// Handler is the runtime function name of the handler or hook.
	Handler string
}

// subscriber is one registered handler of an event type.
type subscriber struct {
	module  string
	deliver func(ctx context.Context, ev any) error
}

// Registry holds the phase hooks and the in-transaction subscribers of a process. internal/wire
// fills it in one place and seals it before the first unit of work; platform/pg reads the hooks,
// Publish the subscribers. It is safe for concurrent use.
type Registry struct {
	mu     sync.RWMutex
	sealed bool
	hooks  map[Phase]Hook
	subs   map[reflect.Type][]subscriber
	regs   []Registration
	tracer Tracer
}

// NewRegistry returns an empty, unsealed registry.
func NewRegistry() *Registry {
	return &Registry{hooks: map[Phase]Hook{}, subs: map[reflect.Type][]subscriber{}}
}

// errSealed is the error of every change after Seal.
var errSealed = errors.New("txn: the registry is sealed")

// SetHook registers h for phase p. Allowed phases are PhaseRightsBegin, PhaseRightsCheck,
// PhaseSync and PhaseAudit, each at most once. h must be a function or method value of the
// owning module, so its module can be derived from its Go package.
//
// Errors: a phase that takes no hook, a second hook for p, a nil hook, a hook whose module
// cannot be derived, a sealed registry.
func (r *Registry) SetHook(p Phase, h Hook) error {
	if !hookPhase(p) {
		return fmt.Errorf("txn: phase %s takes no hook", p)
	}
	if h == nil {
		return fmt.Errorf("txn: nil hook for phase %s", p)
	}
	name, module, err := owner(h)
	if err != nil {
		return err
	}
	return r.setHook(p, h, name, module)
}

// setHook registers h for p with its derived runtime name and module.
func (r *Registry) setHook(p Phase, h Hook, name, module string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sealed {
		return errSealed
	}
	if _, dup := r.hooks[p]; dup {
		return fmt.Errorf("txn: phase %s already has a hook", p)
	}
	r.hooks[p] = h
	r.regs = append(r.regs, Registration{Kind: KindHook, Phase: p, Subscriber: module, Handler: name})
	return nil
}

// Subscribe registers handler for in-transaction events of type E; Publish delivers in the
// order of the Subscribe calls. E must be a named type of a module port or of kernel (02
// "In-Tx-Ereignisse"); handler must be a function or method value of the subscribing module.
// Whether the direction is allowed (higher module subscribes to lower) checks internal/wire
// against the module register.
//
// Errors: a nil handler, an event type or handler whose module cannot be derived, a sealed
// registry.
func Subscribe[E any](r *Registry, handler func(ctx context.Context, ev E) error) error {
	if handler == nil {
		return errors.New("txn: nil handler")
	}
	t := reflect.TypeFor[E]()
	publisher := ModuleOfPackage(t.PkgPath())
	if t.Name() == "" || publisher == "" {
		return fmt.Errorf("txn: event type %s is not a named type of a module or of kernel", t)
	}
	name, module, err := owner(handler)
	if err != nil {
		return err
	}
	return subscribe(r, handler, t, publisher, name, module)
}

// subscribe registers handler for the event type t of publisher with the derived runtime name
// and module of the handler.
func subscribe[E any](r *Registry, handler func(ctx context.Context, ev E) error, t reflect.Type, publisher, name, module string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sealed {
		return errSealed
	}
	r.subs[t] = append(r.subs[t], subscriber{module: module, deliver: func(ctx context.Context, ev any) error {
		return handler(ctx, ev.(E)) //nolint:forcetypeassert // Publish looks the subscribers up by the type of ev
	}})
	r.regs = append(r.regs, Registration{
		Kind: KindSubscribe, Event: publisher + "." + t.Name(), Publisher: publisher,
		Subscriber: module, Handler: name,
	})
	return nil
}

// SetTracer sets the tracer of subscriber spans. It may be called before or after Seal; it only
// changes telemetry, never what runs.
func (r *Registry) SetTracer(t Tracer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tracer = t
}

// Seal ends the registration; SetHook and Subscribe fail from now on, and platform/pg runs units
// only with a sealed registry.
func (r *Registry) Seal() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sealed = true
}

// Sealed reports whether Seal was called.
func (r *Registry) Sealed() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sealed
}

// Hook returns the hook of phase p, or nil when none is registered (platform/pg skips the
// phase then).
func (r *Registry) Hook(p Phase) Hook {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.hooks[p]
}

// Registrations returns a copy of all registrations in the order they were made.
func (r *Registry) Registrations() []Registration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]Registration(nil), r.regs...)
}

// subscribers returns the subscribers of t and the tracer.
func (r *Registry) subscribers(t reflect.Type) ([]subscriber, Tracer, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.subs[t], r.tracer, r.sealed
}

// owner returns the runtime name of fn and its module. Hooks and handlers must belong to a
// module (02 "Ablauf": never closures in internal/wire), so the golden file shows the real owner.
func owner(fn any) (name, module string, err error) {
	f := runtime.FuncForPC(reflect.ValueOf(fn).Pointer())
	if f != nil {
		name = f.Name()
	}
	module = ModuleOfSymbol(name)
	if module == "" || module == kernelModule {
		return "", "", fmt.Errorf("txn: the module of %q cannot be derived; hooks and handlers must be functions or method values of the owning module", name)
	}
	return name, module, nil
}

// kernelModule is the module name of every package below internal/kernel.
const kernelModule = "kernel"

// internalSegment separates the server module path from the package path below internal.
const internalSegment = "/internal/"

// ModuleOfSymbol returns the module of a runtime function name such as
// "<root>/internal/chat/app.(*Service).OnAccessChanged-fm" ("chat"), following
// ModuleOfPackage, or "" when the name has no such package (a closure in internal/wire, a
// platform or cmd function, an empty name). A function of kernel yields "kernel".
func ModuleOfSymbol(symbol string) string {
	slash := strings.LastIndex(symbol, "/")
	if slash < 0 {
		return ""
	}
	dot := strings.Index(symbol[slash:], ".")
	if dot < 0 {
		return ""
	}
	return ModuleOfPackage(symbol[:slash+dot])
}

// ModuleOfPackage returns the module of a Go package path: the segment after the last
// "/internal/" ("kernel" for internal/kernel/..., also for its external test package). The
// infrastructure packages wire and platform are no modules and yield "", as does a path without
// "/internal/". The last occurrence counts so that test fixtures nested below a package
// (internal/platform/pg/internal/fakemodule) act as modules; product packages have only one.
func ModuleOfPackage(pkg string) string {
	i := strings.LastIndex(pkg, internalSegment)
	if i < 0 {
		return ""
	}
	module, _, _ := strings.Cut(pkg[i+len(internalSegment):], "/")
	module = strings.TrimSuffix(module, "_test")
	switch module {
	case "wire", "platform", "":
		return ""
	default:
		return module
	}
}
