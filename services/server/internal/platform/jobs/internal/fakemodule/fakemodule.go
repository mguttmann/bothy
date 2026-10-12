// SPDX-License-Identifier: Apache-2.0

// Package fakemodule is a stand-in module for the tests of platform/jobs. Register checks that the
// part of a job kind before the dot is the module of the argument type's package (the segment
// after the last /internal/), so the test arguments live here as module "fakemodule". Only tests
// import it.
package fakemodule

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

// Echo is the argument of the test job fakemodule.echo.
type Echo struct {
	// Target is an ID the test checks after the job ran.
	Target ids.ID `json:"target"`
	// Seq tells jobs of one test apart.
	Seq int64 `json:"seq"`
}

// Kind returns "fakemodule.echo".
func (Echo) Kind() string { return "fakemodule.echo" }

// Block is the argument of the test job fakemodule.block, whose handler waits for its context.
type Block struct{}

// Kind returns "fakemodule.block".
func (Block) Kind() string { return "fakemodule.block" }

// Tick is the argument of the periodic test job fakemodule.tick.
type Tick struct{}

// Kind returns "fakemodule.tick".
func (Tick) Kind() string { return "fakemodule.tick" }

// WrongModule claims the kind of another module; Register must refuse it.
type WrongModule struct{}

// Kind returns "chat.wrong", a kind of module chat.
func (WrongModule) Kind() string { return "chat.wrong" }

// BadKind has a kind that does not match the kind pattern.
type BadKind struct{}

// Kind returns "FakeModule-Bad".
func (BadKind) Kind() string { return "FakeModule-Bad" }

// WithText carries a string, which job arguments must never do.
type WithText struct {
	// Body would be content in the job table.
	Body string `json:"body"`
}

// Kind returns "fakemodule.with_text".
func (WithText) Kind() string { return "fakemodule.with_text" }

// Func adapts a function to jobs.Handler for the test jobs.
type Func[A jobs.Args] func(ctx context.Context, job jobs.Job[A]) error

// Work calls f.
func (f Func[A]) Work(ctx context.Context, job jobs.Job[A]) error { return f(ctx, job) }

// Hook returns a phase hook owned by fakemodule that calls fn; the txn registry accepts hooks
// only from module packages.
func Hook(fn func(ctx context.Context) error) txn.Hook {
	return func(ctx context.Context) error { return fn(ctx) }
}
