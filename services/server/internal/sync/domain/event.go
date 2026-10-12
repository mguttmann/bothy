// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// The actor kinds of event_log.actor_kind and of the envelope member actor.kind
// (docs/spec/03-data-model.md, R-DM-29).
const (
	// ActorUser is a human principal.
	ActorUser = "user"
	// ActorAgent is an agent principal.
	ActorAgent = "agent"
	// ActorApp is an app principal.
	ActorApp = "app"
	// ActorSystem is a system action of the server without a principal (05 rule 10).
	ActorSystem = "system"
)

// ReceiveSelf is the receive action of a personal event that only its audience receives.
const ReceiveSelf = "self"

// ActorKinds returns the four actor kinds in the order of the table check of event_log.
func ActorKinds() []string { return []string{ActorUser, ActorAgent, ActorApp, ActorSystem} }

// typePattern is the form of an event type: lower-case segments with underscores, at least two,
// separated by dots ("message.created", "agent_session.changed").
var typePattern = regexp.MustCompile(`^[a-z][a-z_]*(\.[a-z][a-z_]*)+$`)

// maxSchemaVersion is the largest value of event_log.schema_version (smallint).
const maxSchemaVersion = math.MaxInt16

// EventFields are the fields of a durable event that the rules check. The app layer fills it
// from kernel/eventlog.Event, which this package may not import (a kernel port, not
// kernel-core); the has-flags say whether an ID field is set.
type EventFields struct {
	// Type is the event type, for example "message.created".
	Type string
	// SchemaVersion is the version of the payload schema of the type.
	SchemaVersion int
	// ActorKind is one of ActorKinds, or empty for an event without actor.
	ActorKind string
	// HasActorPrincipal reports whether the actor principal ID is set.
	HasActorPrincipal bool
	// ResourceKind is the kind of the affected object, for example "message".
	ResourceKind string
	// ReceiveAction is the action a receiver needs on the resource, or ReceiveSelf.
	ReceiveAction string
	// HasAudience reports whether the audience principal ID is set (a personal event).
	HasAudience bool
	// Payload is the JSON payload.
	Payload []byte
}

// ValidateEvent checks f against the rules of a durable event (03 event_log, 04 "Envelope"):
// the type matches "<segment>.<segment>[...]" in lower case, the schema version is between 1
// and 32767, resource kind and receive action are set, ReceiveSelf needs an audience, the actor
// kind is empty or one of ActorKinds, user, agent and app need a principal while an empty kind
// and system forbid one, and the payload passes CheckPayload with maxPayloadBytes.
//
// A violation is a programming error of the producer, so the error has kind internal. It names
// the rule and the type, never payload content.
//
// Example:
//
//	err := domain.ValidateEvent(domain.EventFields{Type: "thread.closed", ...}, 65536)
func ValidateEvent(f EventFields, maxPayloadBytes int) error {
	var problems []string
	if !typePattern.MatchString(f.Type) {
		problems = append(problems, "the type does not match "+typePattern.String())
	}
	if f.SchemaVersion < 1 || f.SchemaVersion > maxSchemaVersion {
		problems = append(problems, fmt.Sprintf("schema_version must be between 1 and %d", maxSchemaVersion))
	}
	if f.ResourceKind == "" {
		problems = append(problems, "resource_kind is empty")
	}
	if f.ReceiveAction == "" {
		problems = append(problems, "receive_action is empty")
	}
	if f.ReceiveAction == ReceiveSelf && !f.HasAudience {
		problems = append(problems, "receive_action self needs an audience principal")
	}
	problems = append(problems, actorProblems(f.ActorKind, f.HasActorPrincipal)...)
	if err := CheckPayload(f.Payload, maxPayloadBytes); err != nil {
		problems = append(problems, err.Error())
	}
	if len(problems) > 0 {
		return errs.New(errs.CodeInternalError,
			fmt.Sprintf("sync: event %q: %s", printable(f.Type), strings.Join(problems, "; ")))
	}
	return nil
}

// actorProblems checks the actor kind against the table check of R-DM-29: a principal exactly
// for user, agent and app.
func actorProblems(kind string, hasPrincipal bool) []string {
	if kind != "" && !slices.Contains(ActorKinds(), kind) {
		return []string{"unknown actor kind"}
	}
	needs := kind == ActorUser || kind == ActorAgent || kind == ActorApp
	switch {
	case needs && !hasPrincipal:
		return []string{"actor kind " + kind + " needs an actor principal"}
	case !needs && hasPrincipal:
		return []string{"an actor principal needs the actor kind user, agent or app"}
	default:
		return nil
	}
}

// errPayload is the base of every payload problem; its text never carries payload content.
var errPayload = errors.New("payload")

// CheckPayload reports whether p is a valid event payload (04 payload rules, ADR 0053): one JSON
// object in valid UTF-8, smaller than maxBytes bytes (04 "smaller than
// sync.event_payload_max_bytes"), without a JSON null at any depth, and in a form jsonb stores
// without growing or refusing it: no number with an exponent (jsonb writes 1e100000 out in full),
// no character U+0000 and no unpaired surrogate escape. A producer
// with a larger object builds a reduced payload and sets truncated (04 rule 2); this check never
// cuts. The error names the rule only.
func CheckPayload(p []byte, maxBytes int) error {
	if len(p) >= maxBytes {
		return fmt.Errorf("%w is %d bytes; it must be smaller than sync.event_payload_max_bytes (%d)", errPayload, len(p), maxBytes)
	}
	if !utf8.Valid(p) {
		return fmt.Errorf("%w is not valid UTF-8", errPayload)
	}
	if err := checkSurrogates(p); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(p))
	dec.UseNumber()
	first, err := dec.Token()
	if err != nil {
		return fmt.Errorf("%w is not valid JSON", errPayload)
	}
	if d, ok := first.(json.Delim); !ok || d != '{' {
		return fmt.Errorf("%w must be a JSON object", errPayload)
	}
	if err := checkTokens(dec); err != nil {
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w has data after the JSON object", errPayload)
	}
	return nil
}

// checkTokens reads the rest of the object whose opening brace dec has read and rejects JSON
// null and U+0000 in keys and strings.
func checkTokens(dec *json.Decoder) error {
	for depth := 1; depth > 0; {
		tok, err := dec.Token()
		if err != nil {
			// encoding/json refuses nesting beyond its depth limit with the same error.
			return fmt.Errorf("%w is not valid JSON or nested deeper than the decoder reads", errPayload)
		}
		switch v := tok.(type) {
		case nil:
			return fmt.Errorf("%w contains JSON null (ADR 0053)", errPayload)
		case json.Delim:
			depth += delimStep(v)
		case string:
			if strings.ContainsRune(v, 0) {
				return fmt.Errorf("%w contains the character U+0000, which jsonb cannot store", errPayload)
			}
		case json.Number:
			if err := checkNumber(string(v)); err != nil {
				return err
			}
		}
	}
	return nil
}

// The digit limits of PostgreSQL numeric, which jsonb uses for every number.
const (
	maxIntegerDigits  = 131072
	maxFractionDigits = 16383
)

// checkNumber rejects a number jsonb would grow or refuse: one with an exponent (jsonb writes
// 1e100000 out in full, so its size is unbounded) and one with more digits before or after the
// point than numeric holds.
func checkNumber(n string) error {
	if strings.ContainsAny(n, "eE") {
		return fmt.Errorf("%w contains a number with an exponent; jsonb stores it expanded, so its size is unbounded", errPayload)
	}
	intPart, frac, _ := strings.Cut(strings.TrimPrefix(n, "-"), ".")
	if len(intPart) > maxIntegerDigits || len(frac) > maxFractionDigits {
		return fmt.Errorf("%w contains a number with more digits than jsonb holds (%d before, %d after the point)",
			errPayload, maxIntegerDigits, maxFractionDigits)
	}
	return nil
}

// checkSurrogates rejects a \u escape of a UTF-16 surrogate that is not part of a high-low pair:
// encoding/json replaces it silently, jsonb refuses it at the insert.
func checkSurrogates(p []byte) error {
	for i := 0; i < len(p); i++ {
		if p[i] != '\\' {
			continue
		}
		n, ok := escapeLen(p, i)
		if !ok {
			return fmt.Errorf("%w contains an unpaired UTF-16 surrogate escape", errPayload)
		}
		i += n - 1
	}
	return nil
}

// escapeLen returns the length of the escape at p[i] (a backslash): 12 for a surrogate pair, 2
// for any other escape (the decoder checks those), and false for an unpaired surrogate.
func escapeLen(p []byte, i int) (int, bool) {
	if i+1 >= len(p) || p[i+1] != 'u' {
		return 2, true
	}
	v, ok := hex4(p, i+2)
	switch {
	case !ok || v < 0xD800 || v > 0xDFFF:
		return 2, true
	case v >= 0xDC00:
		return 0, false
	}
	if i+7 >= len(p) || p[i+6] != '\\' || p[i+7] != 'u' {
		return 0, false
	}
	low, ok := hex4(p, i+8)
	if !ok || low < 0xDC00 || low > 0xDFFF {
		return 0, false
	}
	return 12, true
}

// hex4 reads four hex digits of p at i.
func hex4(p []byte, i int) (uint32, bool) {
	if i+4 > len(p) {
		return 0, false
	}
	v, err := strconv.ParseUint(string(p[i:i+4]), 16, 32)
	return uint32(v), err == nil
}

// delimStep is +1 for an opening and -1 for a closing delimiter.
func delimStep(d json.Delim) int {
	if d == '{' || d == '[' {
		return 1
	}
	return -1
}

// printable cuts s to 64 bytes and replaces control characters, so an error never carries a
// long or binary type name.
func printable(s string) string {
	if len(s) > 64 {
		s = s[:64]
	}
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return '?'
		}
		return r
	}, s)
}
