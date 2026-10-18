// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Form of the enable list services/server/api/operations.yaml (plan decision P-04, format in
// services/server/api/README.md).
var (
	milestoneForm = regexp.MustCompile(`^(M[1-8]|V[1-3])$`)
	ticketForm    = regexp.MustCompile(`^(M[1-8]|V[1-3])-[0-9]{2}$`)
)

// sectionEnabled and sectionLater are the two sections of the list, as keys and in problems.
const (
	sectionEnabled = "enabled"
	sectionLater   = "later"
)

// sectionGatewayOps is the optional section of the gateway ops (M1-64, gatewayops.go).
const sectionGatewayOps = "gateway_ops"

// requiredListKeys are the keys every list has; listKeys and laterKeys are the only keys the list
// knows, anything else is a typo that would otherwise be ignored.
var (
	requiredListKeys = []string{"version", sectionEnabled, sectionLater}
	listKeys         = []string{"version", sectionEnabled, sectionLater, sectionGatewayOps}
	laterKeys        = []string{"operation_id", "milestone", "ticket"}
)

// laterEntry is one operation that has no handler yet, with the milestone (and optionally the
// ticket) that builds it.
type laterEntry struct {
	OperationID string `yaml:"operation_id"`
	Milestone   string `yaml:"milestone"`
	Ticket      string `yaml:"ticket"`
}

// enableList is the decoded enable list.
type enableList struct {
	Version int          `yaml:"version"`
	Enabled []string     `yaml:"enabled"`
	Later   []laterEntry `yaml:"later"`
	// GatewayOps is the section gateway_ops; nil when the list has none (gatewayops.go).
	GatewayOps *gatewayOpsList `yaml:"gateway_ops"`
}

// readList loads and checks the enable list at path on its own: version, keys, sort order,
// duplicates, milestone and ticket forms. The checks against the spec follow in checkListAgainst.
// It returns nil when the file cannot be used at all.
func readList(path string, p *problemsError) *enableList {
	doc, _, err := readYAML(path)
	if err != nil {
		p.add(path, "%v", err)
		return nil
	}
	root := doc.Content[0]
	knownKeys(path, "", root, listKeys, p)
	for _, k := range requiredListKeys {
		if value(root, k) == nil {
			p.add(path, "missing key %s", k)
		}
	}
	for i, e := range items(value(root, sectionLater)) {
		knownKeys(path, fmt.Sprintf("later[%d]", i), e, laterKeys, p)
	}
	var list enableList
	if err := doc.Decode(&list); err != nil {
		p.add(path, "%v", err)
		return nil
	}
	if list.Version != 1 {
		p.add(path, "version %d is not supported (want 1)", list.Version)
		return nil
	}

	checkSorted(path, sectionEnabled, list.Enabled, p)
	ids := make([]string, len(list.Later))
	for i, e := range list.Later {
		ids[i] = e.OperationID
		checkLaterEntry(path, i, e, p)
	}
	checkSorted(path, sectionLater, ids, p)
	checkUnique(path, sectionEnabled, list.Enabled, p)
	checkUnique(path, sectionLater, ids, p)
	for _, id := range list.Enabled {
		if id == "" {
			p.add(path, "enabled: empty operation ID")
		}
	}
	return &list
}

// knownKeys reports every key of the mapping n that is not in allowed; where prefixes the
// problem with the place in the file.
func knownKeys(path, where string, n *yaml.Node, allowed []string, p *problemsError) {
	for _, k := range keys(n) {
		if !slices.Contains(allowed, k) {
			at := path
			if where != "" {
				at += ": " + where
			}
			p.add(at, "unknown key %q (known: %s)", k, strings.Join(allowed, ", "))
		}
	}
}

// checkLaterEntry checks the fields of one later entry.
func checkLaterEntry(path string, i int, e laterEntry, p *problemsError) {
	if e.OperationID == "" {
		p.add(path, "later[%d]: missing operation_id", i)
		return
	}
	at := path + ": later: " + e.OperationID
	switch {
	case e.Milestone == "":
		p.add(at, "missing milestone")
	case !milestoneForm.MatchString(e.Milestone):
		p.add(at, "milestone %q is not one of M1 to M8 or V1 to V3", e.Milestone)
	}
	if e.Ticket == "" {
		return
	}
	switch {
	case !ticketForm.MatchString(e.Ticket):
		p.add(at, "ticket %q does not have the form <milestone>-<two digits>, for example M1-31", e.Ticket)
	case milestoneForm.MatchString(e.Milestone) && !strings.HasPrefix(e.Ticket, e.Milestone+"-"):
		p.add(at, "ticket %q does not belong to milestone %s", e.Ticket, e.Milestone)
	}
}

// checkSorted reports every entry of a section that sorts before its predecessor in byte order
// (the order of strings.Compare, which is what `sort` with LC_ALL=C gives).
func checkSorted(path, section string, ids []string, p *problemsError) {
	for i := 1; i < len(ids); i++ {
		if ids[i] < ids[i-1] {
			p.add(path, "%s: not sorted ascending by byte order: %s comes after %s", section, ids[i], ids[i-1])
		}
	}
}

// checkUnique reports every ID that a section lists more than once, once per ID.
func checkUnique(path, section string, ids []string, p *problemsError) {
	count := map[string]int{}
	for _, id := range ids {
		count[id]++
		if count[id] == 2 && id != "" {
			p.add(path, "%s: %s is listed more than once", section, id)
		}
	}
}

// checkListAgainst compares the list with the operations under paths of the spec: every entry
// names one, no entry is in both sections, and every operation is in a section.
func checkListAgainst(listPath, specPath string, list *enableList, ops []opMeta, p *problemsError) {
	known := map[string]bool{}
	for _, op := range ops {
		known[op.ID] = true
	}
	enabled := map[string]bool{}
	for _, id := range list.Enabled {
		enabled[id] = true
		if id != "" && !known[id] {
			p.add(listPath, "enabled: %s is not an operation under paths of %s", id, specPath)
		}
	}
	later := checkLaterAgainst(listPath, specPath, list.Later, known, enabled, p)
	for _, op := range ops {
		if !enabled[op.ID] && !later[op.ID] {
			p.add(listPath, "%s (%s %s) is in neither enabled nor later", op.ID, op.Method, strings.TrimPrefix(op.Path, baseURL))
		}
	}
}

// checkLaterAgainst reports every later entry that names no operation under paths of the spec or
// that enabled lists as well, and returns the operation IDs under later.
func checkLaterAgainst(listPath, specPath string, entries []laterEntry, known, enabled map[string]bool, p *problemsError) map[string]bool {
	later := map[string]bool{}
	for _, e := range entries {
		if e.OperationID == "" {
			continue
		}
		later[e.OperationID] = true
		if !known[e.OperationID] {
			p.add(listPath, "later: %s is not an operation under paths of %s", e.OperationID, specPath)
		}
		if enabled[e.OperationID] {
			p.add(listPath, "%s is in enabled and in later", e.OperationID)
		}
	}
	return later
}
