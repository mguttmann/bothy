// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// Args are the arguments of one job kind. The type is a struct whose exported fields are only
// IDs (ValidateArgsType); Kind names the job, "<module>.<name>", for example "files.scan".
// River stores the arguments as JSON in its job table, so the struct must marshal to JSON.
//
// Example:
//
//	type ScanArgs struct {
//		File ids.ID `json:"file"`
//	}
//
//	func (ScanArgs) Kind() string { return "files.scan" }
type Args interface { // NOSONAR: the arguments of a job, not an action; see M1-14/16.
	// Kind returns the stable kind of the job; it must not depend on the field values.
	Kind() string
}

// Job is one run of a job: the arguments plus what the handler may need to know about the run.
type Job[A Args] struct {
	// ID is the job's row id in the job table; it is stable across attempts.
	ID int64
	// Attempt counts the runs of this job, 1 for the first one.
	Attempt int
	// Args are the arguments the job was inserted with.
	Args A
}

// Handler works the jobs of one kind. Work must be idempotent (02 "Rules for jobs" item 1),
// pass ctx on to everything it calls and return ctx.Err() when ctx ends; returning nil marks
// the job as done, any error makes River retry it (item 2).
type Handler[A Args] interface {
	// Work runs the job once. The error is recorded on the job row (never logged with the
	// arguments); nil means success.
	Work(ctx context.Context, job Job[A]) error
}

// HandlerOptions configure how the jobs of one kind are queued and retried.
type HandlerOptions struct {
	// Queue is one of the five queues (QueueDefault and the others); empty means QueueDefault.
	Queue string
	// MaxAttempts is how often a job runs before it is discarded; 0 means River's default.
	MaxAttempts int
	// NextRetry returns the pause before the next attempt after attempt failed; nil means
	// River's default backoff. Chapters with their own retry rule set it (02 "Rules for jobs"
	// item 4).
	NextRetry func(attempt int) time.Duration
	// Timeout is River's job timeout for the kind: zero means River's default (README "River
	// defaults"), a negative value no River timeout at all, for a handler that bounds its own work
	// (files.scan, docs/DECISIONS.md D-M3-04-2).
	Timeout time.Duration
}

// The queues of docs/spec/02-architecture.md "Queues".
const (
	// QueueDefault holds short domain jobs.
	QueueDefault = "default"
	// QueueFiles holds scans, thumbnails and transcription (CPU and memory).
	QueueFiles = "files"
	// QueueDelivery holds push and webhook delivery (network).
	QueueDelivery = "delivery"
	// QueueMaintenance holds cleanup, partitions and checks.
	QueueMaintenance = "maintenance"
	// QueuePrivacy holds export and erasure.
	QueuePrivacy = "privacy"
)

// Queues returns the five queue names in the order of 02 "Queues".
func Queues() []string {
	return []string{QueueDefault, QueueFiles, QueueDelivery, QueueMaintenance, QueuePrivacy}
}

// knownQueue reports whether q is one of the five queues.
func knownQueue(q string) bool {
	switch q {
	case QueueDefault, QueueFiles, QueueDelivery, QueueMaintenance, QueuePrivacy:
		return true
	default:
		return false
	}
}

// kindPattern is the form of a job kind: "<module>.<name>" in lower case with underscores.
var kindPattern = regexp.MustCompile(`^[a-z][a-z_]*\.[a-z][a-z_]*$`)

// The field types job arguments may have: an ID, a list of IDs, or an int64 (sequence numbers
// and versions).
var (
	idType     = reflect.TypeFor[ids.ID]()
	idListType = reflect.TypeFor[[]ids.ID]()
	int64Type  = reflect.TypeFor[int64]()
)

// ValidateArgsType checks that t is a valid job argument type: a struct (an empty struct is
// allowed) whose exported fields are each kernel/ids.ID, []kernel/ids.ID or int64. Strings, byte
// slices, maps, interfaces, pointers and nested structs are refused, because the job table is
// one more place where data would live (02 "Usage": arguments carry only IDs, never content or
// secrets). Unexported fields are refused too: they would not reach the job table, so the
// handler would see their zero value.
//
// Register calls it, so a bad type fails at the start of the process. The error names the type
// and the first offending field.
func ValidateArgsType(t reflect.Type) error {
	if t == nil {
		return fmt.Errorf("jobs: argument type is nil")
	}
	if t.Kind() != reflect.Struct {
		return fmt.Errorf("jobs: argument type %s must be a struct, not %s", t, t.Kind())
	}
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			return fmt.Errorf("jobs: argument type %s: field %s is unexported and would not be stored", t, f.Name)
		}
		switch f.Type {
		case idType, idListType, int64Type:
		default:
			return fmt.Errorf("jobs: argument type %s: field %s has type %s; only ids.ID, []ids.ID and int64 are allowed (02 \"Usage\": only IDs)", t, f.Name, f.Type)
		}
	}
	return nil
}

// validateKind checks the kind of the argument type t: the pattern "<module>.<name>" and that
// <module> is the module of t's package (the segment after the last /internal/, as the txn
// registry derives it).
func validateKind(kind string, t reflect.Type) error {
	if !kindPattern.MatchString(kind) {
		return fmt.Errorf("jobs: kind %q of %s must match %s", kind, t, kindPattern)
	}
	module := txn.ModuleOfPackage(t.PkgPath())
	if module == "" {
		return fmt.Errorf("jobs: kind %q: the argument type %s lies in no module package", kind, t)
	}
	if !strings.HasPrefix(kind, module+".") {
		return fmt.Errorf("jobs: kind %q of %s must start with its module %q", kind, t, module+".")
	}
	return nil
}
