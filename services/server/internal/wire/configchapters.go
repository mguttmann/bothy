// SPDX-License-Identifier: Apache-2.0

package wire

// The chapters of docs/spec whose tables "Configuration values of this chapter" hold the keys of
// the schema; config.Key.Chapter names one of them. Each chapter has one constant however many
// keys it holds (quality gate rule go:S1192, goconst in make lint). Names carry the chapter
// number, so a constant added by one ticket has the same name and text as the same constant
// added by a parallel one.
const (
	chapter02 = "02-architecture.md"
	chapter03 = "03-data-model.md"
	chapter04 = "04-sync-protocol.md"
	chapter05 = "05-actions.md"
	chapter06 = "06-api.md"
	chapter07 = "07-permissions.md"
	chapter08 = "08-identity.md"
	chapter09 = "09-agents.md"
	chapter11 = "11-extensions.md"
	chapter12 = "12-files.md"
	chapter16 = "16-operations.md"
	chapter17 = "17-observability.md"
	chapter19 = "19-security.md"
	chapter20 = "20-privacy.md"
)
