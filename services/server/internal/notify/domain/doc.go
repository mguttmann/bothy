// SPDX-License-Identifier: Apache-2.0

// Package domain is the domain layer of the notify module: the decision whether, how and where
// one person gets a hint (docs/spec/13-push-benachrichtigungen.md section 4 "Entscheidung je
// Empfänger", 13 section 12 point 3, R-PN-01).
//
// The decision is split into side-effect-free functions: [ResolveLevel] (13 3.1 "Wirksame
// Stufe"), [InQuietHours] (13 3.4 "Ruhezeiten mit Zeitzone"), [Decide] (the flow of 13 4.1 for
// one hint and one recipient), [Flush] (the one hint at the end of a bundling window, 13 4.5)
// and [ReleaseDelayed] (the read check of a delayed mobile push, 13 4.4).
//
// The package imports only the standard library without I/O and kernel-core (02 section 5
// "Schichten im Modul"). It reads no database, calls no authorize, reads no clock, loads no time
// zone and reads no configuration: the caller (the push consumer of M3-19, the settings of M3-18
// and the gateway delivery of M3-23) gathers the facts through ports and passes them in, together
// with the time "now", the values of the configuration keys and each quiet-hour rule's loaded
// *time.Location. So the same facts always give the same decision, and the rules are proven by
// tables (docs/DECISIONS.md D-M3-17-1).
//
// Example:
//
//	d, err := domain.Decide(domain.RecipientFacts{Kind: domain.KindMention, Now: now, ThreadID: thread,
//		Allowed: true, Level: domain.LevelFacts{InstanceDefault: domain.LevelMentions},
//		Devices: devices, Policy: domain.Policy{CoalesceWindow: coalesce, MobileDelay: delay}})
//	if err == nil && d.Outcome == domain.OutcomeSend {
//		// one job per route of d.Routes
//	}
package domain
