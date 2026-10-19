// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
)

// errEntry is wrapped by every rejection of an entry; the recorder turns it into kind internal,
// because an invalid entry is a programming error of the caller (19 SEC-AUD-5).
var errEntry = errors.New("audit: invalid entry")

// invalid returns the error of an entry that breaks rule.
func invalid(rule string) error { return fmt.Errorf("%w: %s", errEntry, rule) }

// buildRow checks e and returns its row without seq, prev_hash, hash and IP fields: the actor
// from kernel/actor, the correlation ID from kernel/reqmeta (absent when it is not a UUID),
// occurred_at = now cut to milliseconds, a new id, details from the attributes. The details are
// checked against audit_details.v1 and the row against the canonical form, so an entry that
// would fail in the hook fails here, at the caller.
func buildRow(ctx context.Context, e auditing.Entry, now time.Time) (domain.Row, error) {
	if err := checkEntry(e); err != nil {
		return domain.Row{}, err
	}
	details, err := detailsOf(e.Attrs)
	if err != nil {
		return domain.Row{}, err
	}
	r := domain.Row{
		ID: ids.New(), OccurredAt: domain.TruncateTime(now), Action: e.Action, Decision: string(e.Decision),
		DenyReason: string(e.Reason), ResourceKind: e.ResourceKind, ResourceID: e.ResourceID, Details: details,
	}
	if a, ok := actor.FromContext(ctx); ok && !a.IsAnonymous() {
		r.ActorPrincipalID, r.ActorDeviceID, r.ActorSessionID = a.ID, a.DeviceID, a.SessionID
	}
	if id, err := domain.ParseUUID(reqmeta.CorrelationID(ctx)); err == nil {
		r.CorrelationID = id
	}
	if err := checkRow(r); err != nil {
		return domain.Row{}, err
	}
	return r, nil
}

// checkRow checks the details schema and the canonical form of r with a placeholder seq.
func checkRow(r domain.Row) error {
	if err := domain.DetailsV1(r.Details).Validate(); err != nil {
		return fmt.Errorf("%w: %w", errEntry, err)
	}
	r.Seq = 1
	if _, err := domain.CanonicalRow(r); err != nil {
		return fmt.Errorf("%w: %w", errEntry, err)
	}
	return nil
}

// checkEntry checks the columns of e: action and resource kind are codes, the decision is one of
// three, a reason is a code, empty for allow and required for deny.
func checkEntry(e auditing.Entry) error {
	if !domain.ValidCode(e.Action) {
		return invalid("action is not a code")
	}
	if e.ResourceKind != "" && !domain.ValidCode(e.ResourceKind) {
		return invalid("resource kind is not a code")
	}
	if e.Reason != "" && !domain.ValidCode(string(e.Reason)) {
		return invalid("reason is not a code")
	}
	switch e.Decision {
	case auditing.DecisionAllow:
		if e.Reason != "" {
			return invalid("an allow carries no reason")
		}
	case auditing.DecisionDeny:
		if e.Reason == "" {
			return invalid("a deny needs a reason")
		}
	case auditing.DecisionError:
	default:
		return invalid("decision is not allow, deny or error")
	}
	return nil
}

// detailsOf converts the attributes into the details object. ip_hmac, ip_key_id and aggregate
// belong to the recorder; reason takes only a ReasonText, violations only a Violations value,
// os_user only an OSUserName in its form, and none of these values is accepted under another key.
func detailsOf(attrs auditing.Attrs) (map[string]any, error) {
	d := make(map[string]any, len(attrs))
	for k, v := range attrs {
		switch {
		case k == domain.DetailIPHMAC || k == domain.DetailIPKeyID || k == domain.DetailAggregate:
			return nil, invalid("attribute " + k + " is reserved for the recorder")
		case (k == domain.DetailReason) != (v.Kind() == auditing.KindReasonText):
			return nil, invalid("a reason text belongs under reason and nowhere else")
		case (k == domain.DetailViolations) != (v.Kind() == auditing.KindViolations):
			return nil, invalid("violations belong under violations and nowhere else")
		case (k == domain.DetailOSUser) != (v.Kind() == auditing.KindOSUserName):
			return nil, invalid("a session user name belongs under os_user and nowhere else")
		case k == domain.DetailOSUser && !domain.ValidOSUser(v.Data().(string)):
			return nil, invalid("os_user is not the name of a session user")
		case !domain.ValidKey(k):
			return nil, invalid("attribute key does not match ^[a-z][a-z0-9_]*$")
		}
		data, err := valueData(v, true)
		if err != nil {
			return nil, err
		}
		d[k] = data
	}
	return d, nil
}

// valueData returns the JSON data of v; lists may hold only scalars of one kind.
func valueData(v auditing.Value, top bool) (any, error) {
	switch v.Kind() {
	case auditing.KindID, auditing.KindCode, auditing.KindReasonText, auditing.KindOSUserName, auditing.KindInt, auditing.KindBool:
		return v.Data(), nil
	case auditing.KindTime:
		t, _ := v.Data().(time.Time)
		return domain.FormatTime(t), nil
	case auditing.KindList:
		return listData(v, top)
	case auditing.KindViolations:
		vs, _ := v.Data().([]auditing.Violation)
		return violationsData(vs)
	case auditing.KindInvalid:
		return nil, invalid("attribute value has no kind")
	default:
		return nil, invalid("attribute value has an unknown kind")
	}
}

// listData converts a list value.
func listData(v auditing.Value, top bool) (any, error) {
	elems, _ := v.Data().([]auditing.Value)
	if !top {
		return nil, invalid("lists do not nest")
	}
	out := make([]any, 0, len(elems))
	for _, e := range elems {
		if e.Kind() != elems[0].Kind() || e.Kind() == auditing.KindReasonText || e.Kind() == auditing.KindViolations ||
			e.Kind() == auditing.KindOSUserName {
			return nil, invalid("a list holds scalars of one kind")
		}
		data, err := valueData(e, false)
		if err != nil {
			return nil, err
		}
		out = append(out, data)
	}
	return out, nil
}

// violationsData converts the violations of i1_violation; every id is required.
func violationsData(vs []auditing.Violation) (any, error) {
	out := make([]any, 0, len(vs))
	for _, v := range vs {
		if v.AgentID.IsZero() || v.CredentialHolderID.IsZero() {
			return nil, invalid("a violation names its agent and credential holder")
		}
		principals := make([]any, 0, len(v.PrincipalIDs))
		for _, p := range v.PrincipalIDs {
			if p.IsZero() {
				return nil, invalid("a violation names its principals")
			}
			principals = append(principals, p.String())
		}
		out = append(out, map[string]any{
			"code": v.Code, "agent_id": v.AgentID.String(), "credential_holder_id": v.CredentialHolderID.String(),
			"principal_ids": principals,
		})
	}
	return out, nil
}

// internalError turns an error that is not a kernel/errs error (the rejection of an entry, a
// key error) into kind internal; its text names a rule, never a value.
func internalError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := errs.As(err); ok {
		return err
	}
	return errs.Wrap(errs.CodeInternalError, err, err.Error())
}
