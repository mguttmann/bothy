// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
)

// PersonalData is the participant of the gateway in the privacy requests
// (docs/spec/20-privacy.md sections 2, 4.4 and 5.2). gateway_presence holds only the live
// connections of people for the routing of notifications (R-DM-51): which device is connected
// right now and what it focuses. Export writes nothing: a row is no stored record, it ends with
// its connection. Erase changes nothing: the revocation of the person's login sessions by
// identity closes their connections, the node deletes the rows of ended connections with its next
// flush, and the rows of a node that stopped are ignored and deleted after cluster.presence_ttl.
type PersonalData struct{}

// Compile-time check: PersonalData is a participant of kernel/personaldata.
var _ personaldata.Participant = PersonalData{}

// NewPersonalData returns the participant; it needs no storage, because it reads and writes
// nothing.
func NewPersonalData() PersonalData { return PersonalData{} }

// Module returns "gateway".
func (PersonalData) Module() string { return "gateway" }

// Tables returns gateway_presence, the table of the gateway with personal data of 20 section 2.
func (PersonalData) Tables() []string { return []string{"gateway_presence"} }

// Export implements personaldata.Participant and writes nothing (see PersonalData).
func (PersonalData) Export(context.Context, personaldata.Export, personaldata.Writer) error {
	return nil
}

// Erase implements personaldata.Participant and changes nothing in any step (see PersonalData).
func (PersonalData) Erase(context.Context, personaldata.Step, personaldata.Erasure) error {
	return nil
}
