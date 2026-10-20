// SPDX-License-Identifier: Apache-2.0

// Package domain is the domain layer of the runs module. It holds the rules of
// docs/spec/09-agenten.md section 3 "Operator and trigger rules" as pure functions:
//
//   - Decide applies steps 1 to 6 of 09 3.4 and the voice rule of 09 13 to one new message and
//     returns exactly one outcome: no trigger, a rejection at acceptance with a reason code of
//     09 3.5, an invocation of one candidate (operator or guest, in the thread of the message or
//     in a new auto thread) or a follow-up to the active run of the thread.
//   - DecideCommand applies 09 10.4 to a command owned by an agent.
//   - Settle applies step 7: it turns the verdict of the agent.invoke (or run.followup.create)
//     check into trigger_state and trigger_state_reason.
//
// It also holds the rules of the run itself (09 section 5 and 6):
//
//   - State and CanTransition are the run state machine of 03 runs: the allowed transitions,
//     the active states of the one-active-run-per-thread index and the end states.
//   - ReasonTable is the reason table of 09 5.2 plus user_disabled, with the state and the setter
//     of every code; ValidateServerReason checks a reason the server sets, NormalizeAdapterReason
//     turns a reason an adapter reports into the stored one, and OriginalReasonDetail keeps a
//     replaced adapter reason for the run steps.
//   - TriggerOnOpen and TriggerOnTransition map the opening and every transition of a run to the
//     trigger_state of its triggering message (09 6).
//
// And it holds the rules of the invocation token (09 4.5, 05 section 5, M2-06):
//
//   - NewInvocationClaims builds the claims of an InvocationGrant after the table of trigger
//     kinds and refuses an incomplete grant; Payload encodes them.
//   - ParseInvocationToken checks the header (alg EdDSA, typ agent-invocation+jwt byte for byte,
//     a kid) with kernel/jose, Claims decodes and checks the payload, and CheckInvocationClaims
//     compares iss, aud, run, iat and exp with the expectation of the server.
//
// The package imports only the standard library and kernel-core (kernel/ids, kernel/errs,
// kernel/textcheck, kernel/jose); it has
// no I/O, no clock, no randomness, no context and never calls authorize (D-B7, ADR 0017). Every
// permission fact is an input that the application layer gathers from authz, so the authz guard of
// agent.invoke stays the only place that decides permissions and these functions only choose the
// candidate and the reason a person sees. The functions never mutate their input.
//
// Example:
//
//	d, err := domain.Decide(in)
//	if err != nil {
//		return err // broken input: fail closed, nothing is invoked
//	}
//	if d.Check != domain.CheckNone {
//		v := domain.Verdict{Allowed: allowed, Reason: denyReason} // from authorize
//		s, err := domain.Settle(d, v)
//		// s.State and s.Reason go to messages.trigger_state and trigger_state_reason
//	}
package domain
