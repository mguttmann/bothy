// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
)

// PersonalData is the participant of billing in the privacy requests (M7-06,
// docs/spec/20-privacy.md sections 2, 4.4 and 5.2). cost_entries is personal only through the run
// (20 section 2: "attributable to a person through the run"), and it holds no column of the
// person. Export writes nothing: the costs of the person's runs are in runs.jsonl
// (runs.cost_micros, written by the participant of runs). Erase changes nothing: the entries stay
// until retention.cost removes them, because they are the evidence of the costs of the instance.
type PersonalData struct{}

// Compile-time check: PersonalData is a participant of kernel/personaldata.
var _ personaldata.Participant = PersonalData{}

// NewPersonalData returns the participant; it needs no storage, because it reads and writes
// nothing.
func NewPersonalData() PersonalData { return PersonalData{} }

// Module returns "billing".
func (PersonalData) Module() string { return "billing" }

// Tables returns cost_entries, the table of billing with personal data of 20 section 2;
// price_table has none.
func (PersonalData) Tables() []string { return []string{"cost_entries"} }

// Export implements personaldata.Participant and writes nothing (see PersonalData).
func (PersonalData) Export(context.Context, personaldata.Export, personaldata.Writer) error {
	return nil
}

// Erase implements personaldata.Participant and changes nothing in any step (see PersonalData).
func (PersonalData) Erase(context.Context, personaldata.Step, personaldata.Erasure) error {
	return nil
}
