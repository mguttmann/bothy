// SPDX-License-Identifier: Apache-2.0

package main

// inputs is everything the generator reads, after validation.
type inputs struct {
	specData []byte          // openapi.yaml as read, the source of the filtered copy
	ops      []opMeta        // every operation under paths, sorted by ID
	enabled  map[string]bool // operation IDs of the enabled section
	actions  []catalogAction // every action of actions.yaml, sorted by ID
}

// loadInputs reads and validates the spec, the action catalogue and the enable list. The checks
// run over all operations, enabled or not, so a broken spec fails before any operation of it is
// enabled. The error is a *problemsError naming every problem, each with its file and the
// offending operation ID or entry.
func loadInputs(specPath, catalogPath, listPath string) (*inputs, error) {
	p := &problemsError{}
	actions := readCatalog(catalogPath, p)
	list := readList(listPath, p)
	doc, data, err := readYAML(specPath)
	if err != nil {
		p.add(specPath, "%v", err)
		return nil, p.err()
	}
	in := &inputs{specData: data, actions: actions, enabled: map[string]bool{}}
	ops, overrides := readOperations(specPath, data, catalogPath, actions, p)
	checkRequestSchemasClosed(specPath, doc, p)
	in.ops = ops
	if list != nil {
		checkListAgainst(listPath, specPath, list, in.ops, p)
		for _, id := range list.Enabled {
			in.enabled[id] = true
			if overrides[id] {
				p.add(specPath, "%s is enabled but has security requirements of its own; the route table records only public (security: []) or the root security, so the HTTP chain would treat it as the root security (docs/DECISIONS.md M1-17/19)", id)
			}
		}
	}
	if err := p.err(); err != nil {
		return nil, err
	}
	return in, nil
}
