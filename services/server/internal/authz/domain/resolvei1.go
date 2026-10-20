// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"strconv"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The pointers of the request member resolve_i1 (OpenAPI ResolveI1, M2-21).
const (
	pointerResolveAgents = "/resolve_i1/switch_to_organization"
	pointerResolveReason = "/resolve_i1/reason"
	// fieldTooFewItems is the code the request validator gives for minItems (platform/httpserver).
	fieldTooFewItems = "too_few_items"
)

// CheckResolveI1 validates the guided flow resolve_i1 (05 rule 1.6; M2-21 design note 2) beyond
// its schema: agentIDs is not empty and holds no zero and no repeated id (the pointer names the
// index of the repeated entry), and reason, trimmed, is not blank, passes SEC-IN-4 and has at most
// reasonMaxChars characters (audit.reason_max_chars, SEC-AUD-5). It returns every violation in one
// validation_failed. Errors: validation_failed; internal for a reasonMaxChars below 1, a wiring
// error that must never mean "no limit".
//
// Example:
//
//	err := domain.CheckResolveI1([]ids.ID{assistent}, "Second owner as deputy", 500) // nil
func CheckResolveI1(agentIDs []ids.ID, reason string, reasonMaxChars int) error {
	if reasonMaxChars < 1 {
		return internalError("audit.reason_max_chars is not bound")
	}
	var fields []errs.FieldError
	add := func(err error) {
		if e, ok := errs.As(err); ok {
			fields = append(fields, e.Fields...)
		}
	}
	if len(agentIDs) == 0 {
		add(fieldError(pointerResolveAgents, fieldTooFewItems, "switch_to_organization needs at least one agent"))
	}
	seen := make(map[ids.ID]bool, len(agentIDs))
	for i, id := range agentIDs {
		pointer := pointerResolveAgents + "/" + strconv.Itoa(i)
		switch {
		case id.IsZero():
			add(fieldError(pointer, fieldInvalid, "the agent id is not valid"))
		case seen[id]:
			add(fieldError(pointer, fieldInvalid, "the agent is listed twice"))
		}
		seen[id] = true
	}
	add(checkText(pointerResolveReason, "reason", strings.TrimSpace(reason), true, reasonMaxChars))
	if len(fields) == 0 {
		return nil
	}
	return errs.New(errs.CodeValidationFailed, fields[0].Detail).WithFields(fields...)
}
