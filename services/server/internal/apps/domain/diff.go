// SPDX-License-Identifier: Apache-2.0

package domain

// ManifestDiff is the difference of two manifests of one app (11 section 4.2): what a new
// manifest adds and removes. Every list is sorted; the audit entry of an upgrade carries it (M2-24
// design note 14). Commands are compared by their lower-case name, hosts by the lower-case host of
// the delivery addresses.
type ManifestDiff struct {
	ScopesAdded, ScopesRemoved               []string
	EventsAdded, EventsRemoved               []string
	CommandsAdded, CommandsRemoved           []string
	DeliveryHostsAdded, DeliveryHostsRemoved []string
}

// DiffManifests returns the difference from old to new.
//
// Example:
//
//	d := domain.DiffManifests(stored, uploaded)
//	_ = d.ScopesAdded // stay inactive until app.scopes.update
func DiffManifests(old, new Manifest) ManifestDiff {
	var d ManifestDiff
	d.ScopesAdded, d.ScopesRemoved = Delta(old.AllScopes(), new.AllScopes())
	d.EventsAdded, d.EventsRemoved = Delta(old.Events, new.Events)
	d.CommandsAdded, d.CommandsRemoved = Delta(old.CommandNames(), new.CommandNames())
	d.DeliveryHostsAdded, d.DeliveryHostsRemoved = Delta(old.DeliveryHosts(), new.DeliveryHosts())
	return d
}
