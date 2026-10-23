// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/netip"
	"regexp"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/apps/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The audit attributes of the app actions (M2-24 design note 14, SEC-AUD-5): ids, codes,
// integers, booleans and lists of codes; never names, descriptions, addresses, data_egress or the
// manifest.
const (
	attrAppID                     = "app_id"
	attrInstallationID            = "installation_id"
	attrSlug                      = "slug"
	attrManifestSHA256            = "manifest_sha256"
	attrPreviousManifestSHA256    = "previous_manifest_sha256"
	attrGrantedScopes             = "granted_scopes"
	attrEventSubscriptions        = "event_subscriptions"
	attrDeliveryHosts             = "delivery_hosts"
	attrReinstall                 = "reinstall"
	attrReason                    = "reason"
	attrNoChange                  = "no_change"
	attrManifestScopesAdded       = "manifest_scopes_added"
	attrManifestScopesRemoved     = "manifest_scopes_removed"
	attrManifestEventsAdded       = "manifest_events_added"
	attrManifestEventsRemoved     = "manifest_events_removed"
	attrManifestCommandsAdded     = "manifest_commands_added"
	attrManifestCommandsRemoved   = "manifest_commands_removed"
	attrDeliveryHostsAdded        = "delivery_hosts_added"
	attrGrantedScopesAdded        = "granted_scopes_added"
	attrGrantedScopesRemoved      = "granted_scopes_removed"
	attrEventSubscriptionsAdded   = "event_subscriptions_added"
	attrEventSubscriptionsRemoved = "event_subscriptions_removed"
	attrTokensRevoked             = "tokens_revoked"
	attrGrantsRevoked             = "grants_revoked"
	attrCommandsAdded             = "commands_added"
	attrCommandsChanged           = "commands_changed"
	attrCommandsRemoved           = "commands_removed"
	attrComponentsDisabled        = "components_disabled"
)

// codes returns list as an audit list of codes (never nil).
func codes(list []string) auditing.Value {
	vs := make([]auditing.Value, 0, len(list))
	for _, s := range list {
		vs = append(vs, auditing.Code(s))
	}
	return auditing.List(vs...)
}

// codePattern and codeMaxLen are the shape of an audit code (audit_details.v1): a value taken
// from a manifest outside it cannot stand in the audit as itself.
var codePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.:-]*$`)

const codeMaxLen = 64

// manifestCode returns a value taken from a manifest (a delivery host, a command name, the slug)
// as an audit code. A value that is no audit code, such as an address literal (which the audit
// never takes, 20 section 4.5) or a value longer than a code, stands as "sha256:" and the first 32
// hex digits of its SHA-256, so a value the check rules allow never makes the audit entry, and
// with it the install or upgrade, fail.
func manifestCode(v string) auditing.Value {
	if _, err := netip.ParseAddr(v); err == nil || len(v) > codeMaxLen || !codePattern.MatchString(v) {
		sum := sha256.Sum256([]byte(v))
		v = "sha256:" + hex.EncodeToString(sum[:16])
	}
	return auditing.Code(v)
}

// manifestCodes returns values taken from a manifest as an audit list of codes (never nil), each
// through manifestCode.
func manifestCodes(list []string) auditing.Value {
	vs := make([]auditing.Value, 0, len(list))
	for _, v := range list {
		vs = append(vs, manifestCode(v))
	}
	return auditing.List(vs...)
}

// hexHash is the lower-case hex form of a SHA-256.
func hexHash(b []byte) auditing.Value { return auditing.Code(hex.EncodeToString(b)) }

// record writes the allow entry of act on the app id with attrs inside the unit of ctx.
func (s *Service) record(ctx context.Context, act authorize.Action, id ids.ID, attrs auditing.Attrs) error {
	return s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(act), Decision: auditing.DecisionAllow, ResourceKind: string(authorize.ResourceApp), ResourceID: id, Attrs: attrs,
	})
}

// baseAttrs are app_id and installation_id.
func baseAttrs(app, inst ids.ID) auditing.Attrs {
	return auditing.Attrs{attrAppID: auditing.ID(app), attrInstallationID: auditing.ID(inst)}
}

// noChange marks a call without effect (06 section 2).
func noChange(attrs auditing.Attrs) auditing.Attrs {
	attrs[attrNoChange] = auditing.Bool(true)
	return attrs
}

// upgradeAttrs are the attributes of an upgrade with the difference of the manifests and of the
// consented lists.
func upgradeAttrs(app StoredApp, inst InstallationRecord, newHash []byte, diff domain.ManifestDiff, next domain.Grants, reason string) auditing.Attrs {
	_, scopesRemoved := domain.Delta(inst.GrantedScopes, next.Scopes)
	_, eventsRemoved := domain.Delta(inst.EventSubscriptions, next.Events)
	attrs := baseAttrs(app.PrincipalID, inst.ID)
	attrs[attrManifestSHA256] = hexHash(newHash)
	attrs[attrPreviousManifestSHA256] = hexHash(app.ManifestSHA256)
	attrs[attrManifestScopesAdded] = codes(diff.ScopesAdded)
	attrs[attrManifestScopesRemoved] = codes(diff.ScopesRemoved)
	attrs[attrManifestEventsAdded] = codes(diff.EventsAdded)
	attrs[attrManifestEventsRemoved] = codes(diff.EventsRemoved)
	attrs[attrManifestCommandsAdded] = manifestCodes(diff.CommandsAdded)
	attrs[attrManifestCommandsRemoved] = manifestCodes(diff.CommandsRemoved)
	attrs[attrDeliveryHostsAdded] = manifestCodes(diff.DeliveryHostsAdded)
	attrs[attrGrantedScopesRemoved] = codes(scopesRemoved)
	attrs[attrEventSubscriptionsRemoved] = codes(eventsRemoved)
	attrs[attrReason] = auditing.ReasonText(reason)
	return attrs
}

// reasonText is the reason as the one free text of an audit entry.
func reasonText(reason string) auditing.Value { return auditing.ReasonText(reason) }

// sortedSet returns a sorted copy of list without duplicates; never nil.
func sortedSet(list []string) []string {
	out := append([]string{}, list...)
	slices.Sort(out)
	return slices.Compact(out)
}

// idList is a list of IDs as an audit value (the commands of a lifecycle action, M2-25 design note
// 11; SEC-AUD-5 allows IDs).
func idList(list []ids.ID) auditing.Value {
	vs := make([]auditing.Value, 0, len(list))
	for _, id := range list {
		vs = append(vs, auditing.ID(id))
	}
	return auditing.List(vs...)
}
