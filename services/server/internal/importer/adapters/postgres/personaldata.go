// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
)

// PersonalData is the participant of importer in the privacy requests (M7-06,
// docs/spec/20-privacy.md sections 2, 4.4 and 5.2). import_sources names the person who created an
// import (created_by), and import_items holds identifiers of the source system, also foreign
// person IDs in the origin key; both belong to the class import with its own period
// (retention.import_after_completion). Export writes nothing: the identifiers are those of the
// source system, not data the person gave this instance, and the created_by of an import is an
// administrative act that the audit entries of audit.jsonl record. Erase changes nothing: the rows
// stay with the pseudonymous ID until retention.import_after_completion removes them, because a
// running or resumed import needs its items to stay idempotent.
type PersonalData struct{}

// Compile-time check: PersonalData is a participant of kernel/personaldata.
var _ personaldata.Participant = PersonalData{}

// NewPersonalData returns the participant; it needs no storage, because it reads and writes
// nothing.
func NewPersonalData() PersonalData { return PersonalData{} }

// Module returns "importer".
func (PersonalData) Module() string { return "importer" }

// Tables returns the tables of importer with personal data of 20 section 2.
func (PersonalData) Tables() []string { return []string{"import_sources", "import_items"} }

// Export implements personaldata.Participant and writes nothing (see PersonalData).
func (PersonalData) Export(context.Context, personaldata.Export, personaldata.Writer) error {
	return nil
}

// Erase implements personaldata.Participant and changes nothing in any step (see PersonalData).
func (PersonalData) Erase(context.Context, personaldata.Step, personaldata.Erasure) error {
	return nil
}
