// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
)

// PersonalData is the participant of instance in the privacy requests (M7-06,
// docs/spec/20-privacy.md sections 2, 4.4 and 5.2). instance_settings and feature_flags are
// configuration of the instance (class config); they name a person only as the last changer
// (updated_by) and, in the rollout of a flag, in the list user_ids. Export writes nothing: the
// values are settings of the instance, not data of the person, and the changes the person made as
// an owner are in the audit entries of audit.jsonl. Erase changes nothing: updated_by stays as
// evidence with the pseudonymous ID and is overwritten with the next change (20 section 2:
// "overwritten on change", "on change"). The ID in a rollout list also stays: removing it would
// be a flag change of its own (a new version and the events of feature_flag.update, made by
// nobody), while the ID of a disabled, pseudonymized person matches no one who can still sign in;
// an owner who changes the flag must drop it (a put that names a deleted person is refused).
type PersonalData struct{}

// Compile-time check: PersonalData is a participant of kernel/personaldata.
var _ personaldata.Participant = PersonalData{}

// NewPersonalData returns the participant; it needs no storage, because it reads and writes
// nothing.
func NewPersonalData() PersonalData { return PersonalData{} }

// Module returns "instance".
func (PersonalData) Module() string { return "instance" }

// Tables returns the tables of instance with personal data of 20 section 2; instance,
// server_signing_keys and sequence_counters have none.
func (PersonalData) Tables() []string { return []string{"instance_settings", "feature_flags"} }

// Export implements personaldata.Participant and writes nothing (see PersonalData).
func (PersonalData) Export(context.Context, personaldata.Export, personaldata.Writer) error {
	return nil
}

// Erase implements personaldata.Participant and changes nothing in any step (see PersonalData).
func (PersonalData) Erase(context.Context, personaldata.Step, personaldata.Erasure) error {
	return nil
}
