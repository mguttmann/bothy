// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The kinds of a feature flag (docs/spec/16-betrieb.md 6.2, ADR 0064).
const (
	// FlagKindRelease switches a new function on step by step; it ends at RemoveBy.
	FlagKindRelease = "release"
	// FlagKindOption is a lasting product switch of the owner.
	FlagKindOption = "option"
)

// The subject kinds of an evaluation; only a person can be in a rollout.
const (
	// SubjectUser is a person.
	SubjectUser = "user"
	// SubjectAgent is an agent.
	SubjectAgent = "agent"
	// SubjectApp is an app.
	SubjectApp = "app"
)

// FlagSpec is the registration of one feature flag, the domain form of
// instance.FlagDefinition (the app layer converts).
type FlagSpec struct {
	// Key is the flag key, for example dm.enabled.
	Key string
	// Kind is FlagKindRelease or FlagKindOption.
	Kind string
	// Description is the English text the console shows.
	Description string
	// Module is the owning module, a key under modules: of architecture/modules.yaml.
	Module string
	// RemoveBy is the server version by which a release flag is removed; "" for an option.
	RemoveBy string
	// Default is the value of a flag without a row.
	Default bool
}

// flagKeyPattern is the form of a flag key: dot-separated lower-case segments, at least two; it
// fits the KeyPath pattern of the API.
var flagKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)

// CheckRegistry checks specs against the registry rules (brief M1-47 "Registry rules") and the
// known module keys and returns one English finding per defect, each naming the key; nil when
// the registry is valid. It never fails: the caller decides what a finding means (a unit test,
// make flag-check).
//
// Example:
//
//	findings := domain.CheckRegistry(specs, []string{"chat", "files", "identity"})
func CheckRegistry(specs []FlagSpec, modules []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range specs {
		if seen[s.Key] {
			out = append(out, fmt.Sprintf("flag %q: duplicate key", s.Key))
			continue
		}
		seen[s.Key] = true
		if !flagKeyPattern.MatchString(s.Key) {
			out = append(out, fmt.Sprintf("flag %q: key does not match %s", s.Key, flagKeyPattern))
		}
		if strings.TrimSpace(s.Description) == "" {
			out = append(out, fmt.Sprintf("flag %q: blank description", s.Key))
		}
		if !slices.Contains(modules, s.Module) {
			out = append(out, fmt.Sprintf("flag %q: module %q is not in the module register", s.Key, s.Module))
		}
		if f := checkKind(s); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// checkKind returns the finding about kind and remove_by of s; "" when they fit.
func checkKind(s FlagSpec) string {
	switch s.Kind {
	case FlagKindRelease:
		if ValidateSemVer(s.RemoveBy) != nil {
			return fmt.Sprintf("flag %q: a release flag needs remove_by as SemVer without v", s.Key)
		}
	case FlagKindOption:
		if s.RemoveBy != "" {
			return fmt.Sprintf("flag %q: an option flag has no remove_by", s.Key)
		}
	default:
		return fmt.Sprintf("flag %q: kind %q is neither release nor option", s.Key, s.Kind)
	}
	return ""
}

// ReachedRemoveBy reports whether version has reached removeBy by SemVer precedence (a
// pre-release of removeBy has not). Errors: either value is not SemVer 2.0.0.
func ReachedRemoveBy(version, removeBy string) (bool, error) {
	c, err := CompareSemVer(version, removeBy)
	if err != nil {
		return false, err
	}
	return c >= 0, nil
}

// Evaluate returns the value of a flag: the global value, inverted when the subject is in the
// rollout (16 6.2 "Auswertung").
func Evaluate(global, inRollout bool) bool { return global != inRollout }

// Rollout is the stored rollout of a flag (schema feature_flag_rollout.v1): the people and groups
// for which the flag has the opposite of its global value.
type Rollout struct {
	// UserIDs are people.
	UserIDs []ids.ID
	// GroupIDs are groups.
	GroupIDs []ids.ID
}

// Empty reports whether r names nobody.
func (r Rollout) Empty() bool { return len(r.UserIDs) == 0 && len(r.GroupIDs) == 0 }

// Equal reports whether r and o name the same IDs in the same order; compare normalized values.
func (r Rollout) Equal(o Rollout) bool {
	return slices.Equal(r.UserIDs, o.UserIDs) && slices.Equal(r.GroupIDs, o.GroupIDs)
}

// InRollout reports whether the subject of kind with principalID, a member of the valid groups
// validGroups, is in r. Only a person can be: agents and apps get the global value, even when
// their ID stands in user_ids (16 6.2).
func InRollout(r Rollout, kind string, principalID ids.ID, validGroups []ids.ID) bool {
	if kind != SubjectUser {
		return false
	}
	if slices.Contains(r.UserIDs, principalID) {
		return true
	}
	for _, g := range validGroups {
		if slices.Contains(r.GroupIDs, g) {
			return true
		}
	}
	return false
}

// NormalizeRollout returns r with both lists sorted and without duplicates, and nil for an empty
// list, the stored form; r is not changed.
func NormalizeRollout(r Rollout) Rollout {
	return Rollout{UserIDs: normalizeIDs(r.UserIDs), GroupIDs: normalizeIDs(r.GroupIDs)}
}

// normalizeIDs sorts a copy of list and drops duplicates; nil for an empty list.
func normalizeIDs(list []ids.ID) []ids.ID {
	if len(list) == 0 {
		return nil
	}
	out := slices.Clone(list)
	slices.SortFunc(out, func(a, b ids.ID) int { return bytes.Compare(a[:], b[:]) })
	return slices.Compact(out)
}
