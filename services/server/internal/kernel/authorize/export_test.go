// SPDX-License-Identifier: Apache-2.0

package authorize

// DecisionReasons returns every reason that decision.go names as a literal: the constants and
// the keys of its tables, which become audit reasons of 07 section 4.1.
func DecisionReasons() []Reason {
	out := []Reason{
		reasonScopeMissing, reasonDevicePending, reasonPrincipalPend, reasonChannelReadOnly,
		reasonChannelArchived, reasonPrincipalInact, reasonSessionRevoked, reasonDeviceInactive,
		reasonI1Violation, reasonLastOwner, reasonBillingMismatch, reasonToolMismatch,
		reasonI6Violation, reasonHostNotActive, reasonTokenInvalid, reasonComponentInactive,
	}
	for _, table := range []map[Reason]bool{keysOf(terminalReasons), keysOf(forbiddenCodes), revokedReasons, keysOf(invariantCodes)} {
		for r := range table {
			out = append(out, r)
		}
	}
	return out
}

func keysOf[V any](m map[Reason]V) map[Reason]bool {
	out := make(map[Reason]bool, len(m))
	for k := range m {
		out[k] = true
	}
	return out
}
