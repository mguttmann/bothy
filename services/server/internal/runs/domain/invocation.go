// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// The protected header of an invocation token (05 section 5, 09 4.5 "Header"; RFC 8725 sections
// 3.1 and 3.11): exactly these values, so no other JWS of the server (access tokens at+jwt,
// terminal session tokens terminal-session+jwt) passes as an invocation token.
const (
	// InvocationAlg is the one JWS algorithm of invocation tokens.
	InvocationAlg = jose.AlgEdDSA
	// InvocationTyp is the explicit type of the header, compared byte for byte.
	InvocationTyp = "agent-invocation+jwt"
)

// TriggerKind is the claim trigger_kind of an invocation token (05 section 5).
type TriggerKind string

// The trigger kinds of 05 section 5.
const (
	// TriggerMessage: a message opened the run (agent.invocation).
	TriggerMessage TriggerKind = "message"
	// TriggerFollowup: a follow-up message to an active run (agent.followup).
	TriggerFollowup TriggerKind = "followup"
	// TriggerInteraction: a component was used (interaction.created).
	TriggerInteraction TriggerKind = "interaction"
	// TriggerCommand: a command was executed (command.invoked).
	TriggerCommand TriggerKind = "command"
	// TriggerAutocomplete: the server asks for suggestions (command.autocomplete_requested).
	TriggerAutocomplete TriggerKind = "autocomplete"
)

// TriggerKinds returns the trigger kinds in the order of 05 section 5.
func TriggerKinds() []TriggerKind {
	return []TriggerKind{TriggerMessage, TriggerFollowup, TriggerInteraction, TriggerCommand, TriggerAutocomplete}
}

// The values of the claims level, billing_mode and tool_class: exactly the enums Level,
// BillingMode and ToolClass of docs/api/openapi.yaml, kept local so the domain imports no type of
// the agents module (Design note 1 of M2-06).
var (
	invocationLevels       = []string{"read", "write", "trigger"}
	invocationBillingModes = []string{string(BillingPersonal), string(BillingOrganization)}
	invocationToolClasses  = []string{"chat", "scoped", "host"}
)

// ErrInvocationInvalid is wrapped by every error about an invocation token or grant that is not
// only an expiry: a header outside the profile, a payload that breaks the claim rules, a foreign
// iss, aud or run, an iat in the future. The wrapping error names the rule, never a value of the
// token.
var ErrInvocationInvalid = errors.New("runs: invalid invocation token")

// ErrInvocationExpired is returned by CheckInvocationClaims when every other check holds and only
// exp plus the leeway has passed.
var ErrInvocationExpired = errors.New("runs: invocation token expired")

// InvocationGrant is what the server decided for one delivery to an agent; Issue turns it into
// the claims. Zero IDs are absent. Which of RunID, MessageID, InteractionID and CommandID must be
// set follows the table of Design note 2 of M2-06 (05 section 5):
//
//	message, followup: RunID and MessageID
//	interaction:       RunID and InteractionID
//	command:           RunID and CommandID; RunID may be zero only with Control (R-AZ-25)
//	autocomplete:      CommandID, never RunID
type InvocationGrant struct {
	// AgentID becomes aud, TriggeredBy sub.
	AgentID, TriggeredBy ids.ID
	// TriggerKind is the claim trigger_kind.
	TriggerKind TriggerKind
	// RunID is the run; zero only for autocomplete.
	RunID ids.ID
	// MessageID, InteractionID and CommandID: exactly one, the triggering object of the kind.
	MessageID, InteractionID, CommandID ids.ID
	// ThreadID, ChannelID and SpaceID are the location; never zero.
	ThreadID, ChannelID, SpaceID ids.ID
	// Level is the effective level of TriggeredBy on the channel at delivery time.
	Level string
	// BillingMode and ToolClass are the configured values of the agent at delivery time.
	BillingMode, ToolClass string
	// PermissionVersion is the permission version at delivery time; never negative.
	PermissionVersion int64
	// Control marks a command.invoked of a control command (M2-16 design note 13 c): only such a
	// command grant may lack RunID, because the owner has no active run in the thread (R-AZ-25).
	// Any other trigger kind with Control is refused (fail closed).
	Control bool
}

// InvocationClaims are the claims of an invocation token, members in the order of the table of
// 05 section 5 (ruling D-M2-06-3): IDs as canonical lower-case UUID strings, iat and exp as
// NumericDate seconds, aud a single string, absent claims omitted and never null.
// dev_environment_id belongs to the dev environment milestone (V2-02) and is never written here.
type InvocationClaims struct {
	Iss               string `json:"iss"`
	Aud               string `json:"aud"`
	Sub               string `json:"sub"`
	Jti               string `json:"jti"`
	Iat               int64  `json:"iat"`
	Exp               int64  `json:"exp"`
	TriggerKind       string `json:"trigger_kind"`
	RunID             string `json:"run_id,omitzero"`
	MessageID         string `json:"message_id,omitzero"`
	InteractionID     string `json:"interaction_id,omitzero"`
	CommandID         string `json:"command_id,omitzero"`
	ThreadID          string `json:"thread_id"`
	ChannelID         string `json:"channel_id"`
	SpaceID           string `json:"space_id"`
	Level             string `json:"level"`
	BillingMode       string `json:"billing_mode"`
	ToolClass         string `json:"tool_class"`
	DevEnvironmentID  string `json:"dev_environment_id,omitzero"`
	PermissionVersion int64  `json:"permission_version"`
}

// InvocationExpectation is what the verifier of the server compares the claims with: its own
// instance, the agent of the request and, when set, the run (09 4.5 reason table "run wrong").
// IgnoreExpiry skips only the expiry (renewInvocationToken, 09 4.5.1 step 1, and later SR9).
type InvocationExpectation struct {
	Issuer, Audience, RunID ids.ID
	IgnoreExpiry            bool
}

// NewInvocationClaims returns the claims of g with the token id jti, issued at now truncated to
// whole seconds and valid for ttl (agent.invocation_token_ttl, truncated to whole seconds). The
// issuer is not part of the grant, because it is the instance and not a decision about the
// delivery: the app layer sets it with WithIssuer, and Payload refuses claims without one.
// Errors: ErrInvocationInvalid (wrapped) for a grant that breaks the table of Design note 2 of
// M2-06, a zero jti or a ttl below one second; a programming error of the caller, so the issuer
// signs nothing (fail closed).
//
// Example:
//
//	c, err := domain.NewInvocationClaims(g, ids.New(), now, cfg.TTL)
//	c = c.WithIssuer(instanceID)
//	payload, err := c.Payload()
func NewInvocationClaims(g InvocationGrant, jti ids.ID, now time.Time, ttl time.Duration) (InvocationClaims, error) {
	if jti.IsZero() || ttl < time.Second {
		return InvocationClaims{}, fmt.Errorf("%w: zero jti or ttl below one second", ErrInvocationInvalid)
	}
	if g.Control && g.TriggerKind != TriggerCommand {
		return InvocationClaims{}, fmt.Errorf("%w: control on a grant that is not a command", ErrInvocationInvalid)
	}
	if g.TriggerKind == TriggerCommand && g.RunID.IsZero() && !g.Control {
		return InvocationClaims{}, fmt.Errorf("%w: a command grant without run that is not a control command", ErrInvocationInvalid)
	}
	iat := now.Unix()
	c := InvocationClaims{
		Aud: idText(g.AgentID), Sub: idText(g.TriggeredBy), Jti: jti.String(), Iat: iat, Exp: iat + int64(ttl/time.Second),
		TriggerKind: string(g.TriggerKind), RunID: idText(g.RunID), MessageID: idText(g.MessageID),
		InteractionID: idText(g.InteractionID), CommandID: idText(g.CommandID), ThreadID: idText(g.ThreadID),
		ChannelID: idText(g.ChannelID), SpaceID: idText(g.SpaceID), Level: g.Level, BillingMode: g.BillingMode,
		ToolClass: g.ToolClass, PermissionVersion: g.PermissionVersion,
	}
	if err := c.validateShape(true); err != nil {
		return InvocationClaims{}, err
	}
	return c, nil
}

// WithIssuer returns c with iss set to the instance id.
func (c InvocationClaims) WithIssuer(iss ids.ID) InvocationClaims {
	c.Iss = idText(iss)
	return c
}

// idText is the canonical text of id, "" for the zero ID.
func idText(id ids.ID) string {
	if id.IsZero() {
		return ""
	}
	return id.String()
}

// Payload returns the JSON payload of c. Errors: ErrInvocationInvalid (wrapped) when c breaks the
// claim rules (an issuer missing above all), so a broken claim set is never signed.
func (c InvocationClaims) Payload() ([]byte, error) {
	if err := c.validate(true); err != nil {
		return nil, err
	}
	return json.Marshal(c)
}

// validate is validateShape plus a canonical issuer.
func (c InvocationClaims) validate(issued bool) error {
	if !canonicalID(c.Iss) {
		return fmt.Errorf("%w: iss is missing or not canonical", ErrInvocationInvalid)
	}
	return c.validateShape(issued)
}

// validateShape checks the claim rules of Design note 2 apart from iss: every ID canonical, the members of the trigger
// kind present and no other triggering object, known enums, exp after iat, a permission version
// that is not negative. issued says whether dev_environment_id must be absent (the issuer never
// writes it); a verified token may carry it as a canonical ID.
func (c InvocationClaims) validateShape(issued bool) error {
	if err := c.validateIDs(issued); err != nil {
		return err
	}
	if err := c.validateTrigger(); err != nil {
		return err
	}
	if !oneOf(c.Level, invocationLevels) || !oneOf(c.BillingMode, invocationBillingModes) || !oneOf(c.ToolClass, invocationToolClasses) {
		return fmt.Errorf("%w: unknown level, billing_mode or tool_class", ErrInvocationInvalid)
	}
	if c.PermissionVersion < 0 || c.Exp <= c.Iat {
		return fmt.Errorf("%w: negative permission_version or exp not after iat", ErrInvocationInvalid)
	}
	return nil
}

// validateIDs checks that the required IDs are present and every ID is canonical; issued forbids
// dev_environment_id, which the issuer never writes.
func (c InvocationClaims) validateIDs(issued bool) error {
	for _, s := range []string{c.Aud, c.Sub, c.Jti, c.ThreadID, c.ChannelID, c.SpaceID} {
		if !canonicalID(s) {
			return fmt.Errorf("%w: a required ID is missing or not canonical", ErrInvocationInvalid)
		}
	}
	for _, s := range []string{c.RunID, c.MessageID, c.InteractionID, c.CommandID} {
		if s != "" && !canonicalID(s) {
			return fmt.Errorf("%w: an ID is not canonical", ErrInvocationInvalid)
		}
	}
	if c.DevEnvironmentID != "" && (issued || !canonicalID(c.DevEnvironmentID)) {
		return fmt.Errorf("%w: dev_environment_id", ErrInvocationInvalid)
	}
	return nil
}

// validateTrigger checks the table of Design note 2: exactly one triggering object, the one of
// the trigger kind, and run_id absent exactly for autocomplete.
func (c InvocationClaims) validateTrigger() error {
	var object string
	switch TriggerKind(c.TriggerKind) {
	case TriggerMessage, TriggerFollowup:
		object = c.MessageID
	case TriggerInteraction:
		object = c.InteractionID
	case TriggerCommand, TriggerAutocomplete:
		object = c.CommandID
	default:
		return fmt.Errorf("%w: unknown trigger_kind", ErrInvocationInvalid)
	}
	set := 0
	for _, s := range []string{c.MessageID, c.InteractionID, c.CommandID} {
		if s != "" {
			set++
		}
	}
	if object == "" || set != 1 {
		return fmt.Errorf("%w: exactly one triggering object of the trigger kind", ErrInvocationInvalid)
	}
	// A command may lack run_id: a control command of an owner without an active run in the thread
	// (R-AZ-25, M2-16); the issuer allows that only for a control grant (NewInvocationClaims).
	switch k := TriggerKind(c.TriggerKind); {
	case k == TriggerAutocomplete && c.RunID != "", k != TriggerAutocomplete && k != TriggerCommand && c.RunID == "":
		return fmt.Errorf("%w: run_id does not fit the trigger kind", ErrInvocationInvalid)
	}
	return nil
}

// canonicalID reports whether s is a UUIDv7 in canonical lower-case form.
func canonicalID(s string) bool {
	id, err := ids.Parse(s)
	return err == nil && id.String() == s
}

// oneOf reports whether s is one of values.
func oneOf(s string, values []string) bool {
	for _, v := range values {
		if s == v {
			return true
		}
	}
	return false
}

// ParsedInvocationToken is a compact JWS whose header passed the invocation profile; its
// signature is not verified yet.
type ParsedInvocationToken struct {
	jws jose.JWS
}

// ParseInvocationToken parses token with the JOSE profile of ADR 0067 restricted to EdDSA and the
// type agent-invocation+jwt (Design note 5a of M2-06): kernel/jose refuses none, HS*, crit, jku,
// x5u, x5c and zip and compares typ without case; this function then compares typ byte for byte
// (09 4.5 item 2 says "exactly") and requires a non-empty kid. A header jwk is never used. Errors:
// ErrInvocationInvalid (wrapped); the message never contains the token.
//
// Example:
//
//	p, err := domain.ParseInvocationToken(token)
//	key, err := keys.VerificationKey(ctx, instance.PurposeInvocation, p.KeyID())
func ParseInvocationToken(token string) (ParsedInvocationToken, error) {
	j, err := jose.ParseCompactJWS(token, []string{InvocationAlg}, InvocationTyp)
	if err != nil {
		return ParsedInvocationToken{}, fmt.Errorf("%w: the JWS breaks the profile", ErrInvocationInvalid)
	}
	if j.Header.Alg != InvocationAlg || j.Header.Typ != InvocationTyp || j.Header.Kid == "" {
		return ParsedInvocationToken{}, fmt.Errorf("%w: header alg, typ or kid", ErrInvocationInvalid)
	}
	return ParsedInvocationToken{jws: j}, nil
}

// KeyID returns the kid of the header.
func (p ParsedInvocationToken) KeyID() string { return p.jws.Header.Kid }

// Verify checks the signature with k. Errors: ErrInvocationInvalid (wrapped).
func (p ParsedInvocationToken) Verify(k jose.JWK) error {
	if err := p.jws.Verify(k); err != nil {
		return fmt.Errorf("%w: signature", ErrInvocationInvalid)
	}
	return nil
}

// Claims decodes the payload (Design note 5d): every member of the trigger kind present with its
// JSON type (strings for IDs and enums, integers for iat, exp and permission_version), member
// names matched exactly, enums known. Unknown members are ignored (RFC 7519 section 4, ruling
// D-M2-06-7), so a token of a newer server stays verifiable during an upgrade. Errors:
// ErrInvocationInvalid (wrapped).
func (p ParsedInvocationToken) Claims() (InvocationClaims, error) {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(p.jws.Payload, &members); err != nil {
		return InvocationClaims{}, fmt.Errorf("%w: payload", ErrInvocationInvalid)
	}
	var c InvocationClaims
	strs := []struct {
		name     string
		dst      *string
		required bool
	}{
		{"iss", &c.Iss, true}, {"aud", &c.Aud, true}, {"sub", &c.Sub, true}, {"jti", &c.Jti, true},
		{"trigger_kind", &c.TriggerKind, true}, {"run_id", &c.RunID, false}, {"message_id", &c.MessageID, false},
		{"interaction_id", &c.InteractionID, false}, {"command_id", &c.CommandID, false}, {"thread_id", &c.ThreadID, true},
		{"channel_id", &c.ChannelID, true}, {"space_id", &c.SpaceID, true}, {"level", &c.Level, true},
		{"billing_mode", &c.BillingMode, true}, {"tool_class", &c.ToolClass, true},
		{"dev_environment_id", &c.DevEnvironmentID, false},
	}
	for _, f := range strs {
		raw, ok := members[f.name]
		if !ok {
			if f.required {
				return InvocationClaims{}, fmt.Errorf("%w: claim %s is missing", ErrInvocationInvalid, f.name)
			}
			continue
		}
		if !jsonString(raw, f.dst) || *f.dst == "" {
			return InvocationClaims{}, fmt.Errorf("%w: claim %s is not a non-empty string", ErrInvocationInvalid, f.name)
		}
	}
	for _, f := range []struct {
		name string
		dst  *int64
	}{{"iat", &c.Iat}, {"exp", &c.Exp}, {"permission_version", &c.PermissionVersion}} {
		raw, ok := members[f.name]
		if !ok || json.Unmarshal(raw, f.dst) != nil {
			return InvocationClaims{}, fmt.Errorf("%w: claim %s is not an integer", ErrInvocationInvalid, f.name)
		}
	}
	if err := c.validate(false); err != nil {
		return InvocationClaims{}, err
	}
	return c, nil
}

// jsonString decodes raw into dst when raw is a JSON string (null is not one).
func jsonString(raw json.RawMessage, dst *string) bool {
	if len(raw) == 0 || raw[0] != '"' {
		return false
	}
	return json.Unmarshal(raw, dst) == nil
}

// CheckInvocationClaims compares c with e at now (Design note 5 steps e to g of M2-06), leeway
// being auth.clock_leeway: iss equals e.Issuer, aud equals e.Audience, run_id equals e.RunID when
// set; iat not later than now plus leeway; exp after iat; then, unless e.IgnoreExpiry, now before
// exp plus leeway. So ErrInvocationExpired comes only when everything else holds (09 4.5: "exp
// passed, otherwise valid and current"). Errors: ErrInvocationInvalid (wrapped) or
// ErrInvocationExpired.
func CheckInvocationClaims(c InvocationClaims, e InvocationExpectation, now time.Time, leeway time.Duration) error {
	if e.Issuer.IsZero() || c.Iss != e.Issuer.String() || e.Audience.IsZero() || c.Aud != e.Audience.String() {
		return fmt.Errorf("%w: iss or aud", ErrInvocationInvalid)
	}
	if !e.RunID.IsZero() && c.RunID != e.RunID.String() {
		return fmt.Errorf("%w: run_id", ErrInvocationInvalid)
	}
	if time.Unix(c.Iat, 0).After(now.Add(leeway)) || c.Exp <= c.Iat {
		return fmt.Errorf("%w: iat in the future or exp not after iat", ErrInvocationInvalid)
	}
	if !e.IgnoreExpiry && !now.Before(time.Unix(c.Exp, 0).Add(leeway)) {
		return ErrInvocationExpired
	}
	return nil
}
