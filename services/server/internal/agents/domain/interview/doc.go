// SPDX-License-Identifier: Apache-2.0

// Package interview is the pure model of the interview of "New agent"
// (docs/spec/26-builtin-agents.md 6, A1-07): the rules of the interview so far, the payload of the
// ephemeral event agent.interview_requested, the strict decoding of the builder's answer with the
// draft check of 26 4.3 (package definition) and the catalogue check of 26 7.4, the cost estimate
// and the state of one open request. It does no I/O; the app layer relays the request, waits and
// audits.
//
// A builder answer is model output and never trusted: a member outside the answer schema, a
// surface of 26 5.5 in the draft (frontmatter, inline shell, an @ import outside the agent folder)
// or a text that is no display text refuses the whole answer (InvalidError, the fallback
// builder_invalid); any other finding of a draft goes back to the person as a field error.
//
// Example:
//
//	a, err := interview.ParseAnswer(raw, limits, catalogue)
//	var inv *interview.InvalidError
//	if errors.As(err, &inv) {
//		// fallback builder_invalid
//	}
package interview
