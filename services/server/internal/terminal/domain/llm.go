// SPDX-License-Identifier: Apache-2.0

package domain

import "github.com/OWNER/bothy/services/server/internal/kernel/ids"

// The declarations of the LLM login of a host (hosts.llm_credentials, 24 section 7.2).
const (
	LLMNone         = "none"
	LLMPersonal     = "personal"
	LLMOrganization = "organization"
)

// FieldNotAllowed marks a member that must not come with the other values of the request (M2-40:
// credential_holder_id with none or organization).
const FieldNotAllowed = "not_allowed"

// Declaration is the declaration of the LLM login of a host: none, personal or organization, and
// the holder exactly with personal (zero otherwise).
type Declaration struct {
	Credentials string
	HolderID    ids.ID
}

// ValidateDeclaration checks d (24 section 7.2, Design note 10 of M2-40): a known value; personal
// without a holder gives validation_failed with required at /credential_holder_id; a holder with
// none or organization gives not_allowed there. Whether the holder is a person is the use case's
// check (unknown_user). It returns nil or the error of the first failing rule; M2-39's field
// errors are kernel/errs errors with one entry, so this function returns one too.
func ValidateDeclaration(d Declaration) error {
	switch d.Credentials {
	case LLMNone, LLMOrganization:
		if !d.HolderID.IsZero() {
			return FieldError("/credential_holder_id", FieldNotAllowed, "a credential holder needs personal")
		}
	case LLMPersonal:
		if d.HolderID.IsZero() {
			return FieldError("/credential_holder_id", FieldRequired, "personal needs a credential holder")
		}
	default:
		return FieldError("/llm_credentials", FieldInvalid, "the declaration must be none, personal or organization")
	}
	return nil
}

// Same reports the same declaration (06 section 2 "Ohne Wirkung").
func (d Declaration) Same(o Declaration) bool { return d == o }

// EndsForeignSessions reports a change from before to after that ends the sessions of everybody
// but the new holder (24 section 7.3 point 3, 03 section 6.8): after is personal, and before was
// not personal or named another holder. Back to organization or none ends nothing here.
func EndsForeignSessions(before, after Declaration) bool {
	return after.Credentials == LLMPersonal && (before.Credentials != LLMPersonal || before.HolderID != after.HolderID)
}
