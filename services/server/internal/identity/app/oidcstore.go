// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/domain/oidcflow"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// LoginFlowRecord is a row of auth_login_flows (03 section 4.2, M1-41). Zero values stand for
// NULL. It never holds state, nonce or login_code, only their HMAC.
type LoginFlowRecord struct {
	ID, ProviderID ids.ID
	Mode           oidcflow.Mode
	// UserID is the person of a link flow.
	UserID ids.ID
	// StateHash and NonceHash are the HMAC of state and nonce towards the provider.
	StateHash, NonceHash []byte
	// ClientChallenge is the S256 challenge of the app; AppRedirectURI its redirect.
	ClientChallenge, AppRedirectURI string
	// LoginCodeHash and LoginCodeExpiresAt are set by the callback.
	LoginCodeHash      []byte
	LoginCodeExpiresAt time.Time
	// EnrollmentCodeID is the code checked at begin, consumed at complete.
	EnrollmentCodeID ids.ID
	// External is the verified identity while the status is code_issued; nil otherwise.
	External *oidcflow.ExternalIdentity
	Status   oidcflow.Status
	// Error is the code of a failed flow.
	Error                string
	CreatedAt, ExpiresAt time.Time
}

// LoginFlowStore is the outgoing port over auth_login_flows (implemented by adapters/postgres).
// Reads outside a unit of work use the pool; writes and locks run only inside one. Errors are
// kernel/errs errors without SQL or values.
type LoginFlowStore interface {
	// InsertLoginFlow inserts rec with the status started.
	InsertLoginFlow(ctx context.Context, rec LoginFlowRecord) error
	// LoginFlowByStateHash returns the flow with the state hash, without lock; found is false
	// when none has it.
	LoginFlowByStateHash(ctx context.Context, hash []byte) (rec LoginFlowRecord, found bool, err error)
	// LockLoginFlowByCodeHash returns the flow with the login_code hash, locked FOR UPDATE until
	// the end of the unit of work.
	LockLoginFlowByCodeHash(ctx context.Context, hash []byte) (rec LoginFlowRecord, found bool, err error)
	// IssueLoginCode moves the started flow id to code_issued with the external identity and the
	// login_code hash; issued is false when the flow was no longer started.
	IssueLoginCode(ctx context.Context, id ids.ID, ext oidcflow.ExternalIdentity, codeHash []byte, codeExpiresAt time.Time) (issued bool, err error)
	// FailLoginFlow moves a started or code_issued flow to failed with the error code and drops
	// the external identity; failed is false when the flow had ended already.
	FailLoginFlow(ctx context.Context, id ids.ID, code string) (failed bool, err error)
	// CompleteLoginFlow moves a code_issued flow to completed and drops the external identity;
	// completed is false when the flow was no longer code_issued.
	CompleteLoginFlow(ctx context.Context, id ids.ID) (completed bool, err error)
	// DeleteExpiredLoginFlows deletes the flows whose expires_at lies before now and returns how
	// many.
	DeleteExpiredLoginFlows(ctx context.Context, now time.Time) (int64, error)
}
