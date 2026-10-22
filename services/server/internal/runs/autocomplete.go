// SPDX-License-Identifier: Apache-2.0

package runs

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// AutocompleteQuery is the input of CommandExec.Autocomplete (schema AutocompleteQuery and the
// path of autocompleteCommand): the partial value of one option of a command (09 section 10.5).
type AutocompleteQuery struct {
	ChannelID, CommandID ids.ID
	// ThreadID is the thread of the request; zero when the request names none.
	ThreadID ids.ID
	// Option names the option being typed; Value is its partial text.
	Option, Value string
	// Options are the values of the other options entered so far, in the order of the request.
	Options []OptionValue
}

// Choice is one suggestion (schema CommandChoice): a display name and the canonical value.
type Choice struct {
	Name, Value string
}

// AutocompleteResult is the answer of CommandExec.Autocomplete (schema AutocompleteResult).
// Choices is never nil: an empty list encodes as [].
type AutocompleteResult struct {
	Choices []Choice
}

// AutocompleteAnswer is the owner's answer to an open request (schema AutocompleteAnswer with the
// request of the path, or the op command.autocomplete_result).
type AutocompleteAnswer struct {
	RequestID ids.ID
	Choices   []Choice
}

// AutocompleteRelay is the source port of runs that hands one autocomplete request to exactly one
// live gateway connection of the command's owner in this process (04 section 15.7 point 2; brief
// M2-17 design note 1); the gateway implements it. It never blocks on a connection.
//
// Example:
//
//	sent, err := relay.Ask(ctx, ownerID, ev, token)
type AutocompleteRelay interface {
	// Ask offers ev with the delivery field invocation_token (the revealed token; empty for an app
	// owner) to one live connection of ownerID. sent is false when no such connection exists or
	// its outbound queue dropped the frame. Errors only report programming faults (an
	// unregistered type, a payload that cannot be encoded or is too large).
	Ask(ctx context.Context, ownerID ids.ID, ev eventlog.EphemeralEvent, invocationToken string) (sent bool, err error)
}
