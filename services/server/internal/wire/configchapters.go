// SPDX-License-Identifier: Apache-2.0

package wire

// The chapters of docs/spec whose tables "Konfigurationswerte dieses Kapitels" hold the keys of
// the schema; config.Key.Chapter names one of them. Each chapter has one constant however many
// keys it holds (quality gate rule go:S1192, goconst in make lint). Names carry the chapter
// number, so a constant added by one ticket has the same name and text as the same constant
// added by a parallel one.
const (
	chapter02 = "02-architektur.md"
	chapter03 = "03-datenmodell.md"
	chapter16 = "16-betrieb.md"
)
