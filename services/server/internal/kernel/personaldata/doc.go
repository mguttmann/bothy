// SPDX-License-Identifier: Apache-2.0

// Package personaldata declares the Participant that exports and erases the data of one person per
// module (docs/spec/20-privacy.md sections 4.4 and 5.2, brief M7-06). Every module with tables of
// personal data of 20 section 2 implements it for exactly those tables; internal/wire registers
// every participant in one Registry, and the module privacy runs them for the jobs
// privacy.export and privacy.erase. A participant reads and writes only the tables of its own
// module (03 principle 12): privacy never runs SQL on them.
//
// Export: privacy calls Export of every participant with a Writer that builds the package of 20
// section 5.2. A participant writes the sections of the JSON files it owns (Writer.Section), the
// lines of the JSON Lines files (Writer.Line) and the bytes of own uploads (Writer.Content). Only
// data and metadata of the person: never a secret, a hash, a key or the content of another person.
//
// Erasure: privacy calls Erase of every participant once per step of 20 section 4.4 (StepDisable
// to StepPseudonymize), in that order, and records the last finished step, so a crashed job
// resumes at the next one. A participant does what the step asks for its own tables and returns
// nil for a step it has nothing to do in. Every call is idempotent: run again after a crash it
// finishes the step without repeating an effect that is already there.
//
// Example:
//
//	reg := personaldata.NewRegistry()
//	_ = reg.Register(identityParticipant)
//	for _, p := range reg.All() {
//		if err := p.Export(ctx, personaldata.Export{Subject: alex}, w); err != nil {
//			return err
//		}
//	}
package personaldata
