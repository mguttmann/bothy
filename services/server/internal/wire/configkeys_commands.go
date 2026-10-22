// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// The keys of autocompletion (M2-17), also named by the cross-check autocomplete_below_request_timeout.
const (
	keyAutocompleteTimeout    = "command.autocomplete_timeout"
	keyAutocompleteMaxChoices = "command.autocomplete_max_choices"
	// autocompleteChoicesBound is the maxItems of GatewayAutocompleteResult.choices,
	// AutocompleteAnswer.choices and AutocompleteResult.choices in docs/api/openapi.yaml: a value
	// above it could never be used over the gateway (docs/DECISIONS.md D-M2-17).
	autocompleteChoicesBound = "25"
)

// commandsConfigKeys returns the keys of the command register of docs/spec/09-agents.md section
// 21 that no earlier ticket registered (M2-16, M2-17): the most static choices of one command
// option, the same bound as the suggestions of an answer (09 section 10.5 point 3), capped by the
// schema bound of the answer, and the wait for suggestions. command.max_per_principal and
// command.description_max_chars are registered by appsConfigKeys (M2-24).
func commandsConfigKeys() []config.Key {
	return []config.Key{
		{
			Name: keyAutocompleteMaxChoices, Kind: config.KindInt, Default: autocompleteChoicesBound, Min: "1", Max: autocompleteChoicesBound,
			Doc:     "Most suggestions of one autocomplete answer and most static choices of one command option.",
			Chapter: chapter09,
		},
		{
			Name: keyAutocompleteTimeout, Kind: config.KindDuration, Default: "2500ms", Min: minPositiveDuration,
			Doc:     "Longest wait of autocompleteCommand for the suggestions of the command owner; below http.request_timeout.",
			Chapter: chapter09,
		},
	}
}
