// SPDX-License-Identifier: Apache-2.0

package domain

// defaultConsumerName is the consumer of an agent that names none (04 section 6.3 step 3), its
// main stream.
const defaultConsumerName = "default"

// AutocompleteCandidate is one connection of a command's owner in this process, as the relay of
// command.autocomplete_requested sees it (brief M2-17 design note 5).
type AutocompleteCandidate struct {
	// Consumer is the consumer name of the connection ("default" when the agent named none).
	Consumer string
	// Live reports the live phase: for an agent after its consumer window queued catchup.complete;
	// ephemeral frames never go out before it (M1-66).
	Live bool
	// Attached orders the connections by the time they attached; higher is newer.
	Attached uint64
}

// PickAutocompleteConnection chooses the one connection that gets an autocomplete request: the
// live connection of consumer default, otherwise the most recently attached live connection, as
// with consumers the newest wins (R-SY-09). It returns the index into cands and false when no
// candidate is live.
//
// Example:
//
//	i, ok := domain.PickAutocompleteConnection(cands)
func PickAutocompleteConnection(cands []AutocompleteCandidate) (int, bool) {
	best := -1
	for i, c := range cands {
		switch {
		case !c.Live:
		case c.Consumer == defaultConsumerName:
			return i, true
		case best < 0 || c.Attached > cands[best].Attached:
			best = i
		}
	}
	return best, best >= 0
}
