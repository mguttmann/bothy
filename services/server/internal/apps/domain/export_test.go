// SPDX-License-Identifier: Apache-2.0

package domain

import "slices"

// ReplyContractTables returns the hand-kept tables of the reply decoder for the contract test:
// the member names of MessageCreate, ComponentCreate and CommandChoice and the enums of kind,
// type, required_capability and restricted_to, each sorted.
func ReplyContractTables() map[string][]string {
	keys := func(s *shape) []string {
		out := make([]string, 0, len(s.members))
		for k := range s.members {
			out = append(out, k)
		}
		slices.Sort(out)
		return out
	}
	sorted := func(v []string) []string {
		out := slices.Clone(v)
		slices.Sort(out)
		return out
	}
	return map[string][]string{
		"MessageCreate":        keys(messageShape),
		"ComponentCreate":      keys(componentShape),
		"CommandChoice":        keys(answerShape.members["choices"].items),
		"MessageCreate.kind":   sorted(replyKinds),
		"ComponentType":        sorted(componentTypes),
		"Capability":           sorted(capabilities),
		"ComponentRestriction": sorted(componentRestricts),
	}
}
