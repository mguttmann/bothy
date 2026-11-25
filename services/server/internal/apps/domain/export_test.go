// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"encoding/base64"
	"errors"
	"slices"
	"strings"
)

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

// ErrSecretFormat reports, for the tests only, a value that is not whsec_ and standard base64.
var ErrSecretFormat = errors.New("apps: the value is no whsec_ secret")

// ParseSecret returns, for the tests only, the key bytes of a whsec_ secret. Errors:
// ErrSecretFormat; it never repeats the value.
func ParseSecret(s string) ([]byte, error) {
	rest, ok := strings.CutPrefix(s, SecretPrefix)
	if !ok {
		return nil, ErrSecretFormat
	}
	key, err := base64.StdEncoding.DecodeString(rest)
	if err != nil || len(key) == 0 {
		return nil, ErrSecretFormat
	}
	return key, nil
}

// AppSubscribableTypes returns the subscribable types, ascending, in a fresh slice.
func AppSubscribableTypes() []string { return slices.Clone(appSubscribable) }

// AgentSubscribableTypes returns the types an agent subscription may name, ascending, in a fresh
// slice.
func AgentSubscribableTypes() []string {
	out := append(AppSubscribableTypes(), agentOnly...)
	slices.Sort(out)
	return out
}
