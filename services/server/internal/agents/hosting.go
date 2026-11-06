// SPDX-License-Identifier: Apache-2.0

package agents

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// HostCmd is the body of setAgentHost (schema HostSet).
type HostCmd struct {
	// HostedBy is the new hosting runner.
	HostedBy ids.ID
	Reason   string
	IfMatch  Precondition
}

// Hosting is the incoming port of the hosted agents (A1-03, docs/spec/26-builtin-agents.md 3):
// which Claude Code runner hosts which built-in agent, and the tokens of a built-in agent, which
// only its hosting runner receives. Every method authorizes the caller a with its action of 07
// section 2.13 and runs its writes in one unit of work with audit and events. Errors are
// kernel/errs errors.
type Hosting interface {
	// SetHost moves the built-in agent id to the runner cmd.HostedBy (agent.host.set, owner with
	// step-up) and revokes every token of the agent the old runner holds, in the same transaction.
	// Errors: validation_failed (/hosted_by host_not_capable, /reason), conflict with
	// not_a_builtin_agent, host_tool_class_ceiling or hosted_holder_mismatch, version_conflict.
	SetHost(ctx context.Context, a actor.Actor, id ids.ID, cmd HostCmd) (Agent, error)
	// ListHosted returns one page of the active agents whose hosted_by is the calling agent a
	// (agent.hosted.list, agent-self), ascending by id. Errors: cursor_invalid.
	ListHosted(ctx context.Context, a actor.Actor, p Page) (AgentPage, error)
	// IssueHostedToken issues a token of the built-in agent id to its hosting runner a
	// (agent.hosted_token.issue), bound to the DPoP key of a's token, and revokes the older ones
	// of that runner. Every caller other than the active hosting runner reporting hosts_agents
	// gets not_found, audited as a denial. Errors: not_found; conflict with dpop_required.
	//
	// Example:
	//
	//	issued, err := hosting.IssueHostedToken(ctx, runner, agentID)
	//	// issued.Secret.Reveal() is shown once in the answer, never logged.
	IssueHostedToken(ctx context.Context, a actor.Actor, id ids.ID) (HostedToken, error)
}

// HostedToken is the result of IssueHostedToken: the stored token of the built-in agent and its
// secret. The port packages do not import each other, so the agents port has its own types
// besides identity.IssuedToken.
type HostedToken struct {
	Token  HostedTokenInfo
	Secret Secret
}

// HostedTokenInfo is the stored token of a built-in agent (schema ApiToken), without its secret.
type HostedTokenInfo struct {
	ID, PrincipalID ids.ID
	// Kind is the token kind (agt).
	Kind string
	// Name is the label of the token.
	Name string
	// TokenPrefix is bothy_<kind>_ plus the first 6 characters of the random part.
	TokenPrefix string
	// DPoPJKT is the RFC 7638 thumbprint the token is bound to.
	DPoPJKT string
	// ExpiresAt is the expiry, agents.hosted_token_ttl after the issue.
	ExpiresAt time.Time
	// CreatedBy is the hosting runner.
	CreatedBy ids.ID
	CreatedAt time.Time
}

// redacted is the only rendering of a Secret.
const redacted = "[redacted]"

// Secret is the value of a hosted token: it renders as "[redacted]" through fmt, slog,
// encoding/json and encoding.TextMarshaler, so it never reaches a log, an event, an audit entry or
// an error by accident (SEC-LOG-1). Only Reveal hands out the value; the issueHostedAgentToken
// handler calls it to write the answer once.
type Secret struct {
	v *string
}

// NewSecret wraps v.
func NewSecret(v string) Secret { return Secret{v: &v} }

// Reveal returns the value; "" for the zero Secret.
func (s Secret) Reveal() string {
	if s.v == nil {
		return ""
	}
	return *s.v
}

// IsZero reports whether s holds no value.
func (s Secret) IsZero() bool { return s.v == nil }

// String returns "[redacted]".
func (Secret) String() string { return redacted }

// GoString returns "[redacted]".
func (Secret) GoString() string { return redacted }

// Format writes "[redacted]" for every verb.
func (Secret) Format(f fmt.State, _ rune) { _, _ = io.WriteString(f, redacted) }

// LogValue implements slog.LogValuer.
func (Secret) LogValue() slog.Value { return slog.StringValue(redacted) }

// MarshalJSON returns the JSON string "[redacted]".
func (Secret) MarshalJSON() ([]byte, error) { return []byte(`"` + redacted + `"`), nil }

// MarshalText returns "[redacted]".
func (Secret) MarshalText() ([]byte, error) { return []byte(redacted), nil }
