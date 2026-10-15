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
	chapter04 = "04-sync-protokoll.md"
	chapter05 = "05-aktionen.md"
	chapter06 = "06-api.md"
	chapter07 = "07-rechte.md"
	chapter08 = "08-identitaet.md"
	chapter09 = "09-agenten.md"
	chapter11 = "11-erweiterungen.md"
	chapter12 = "12-dateien.md"
	chapter14 = "14-suche.md"
	chapter16 = "16-betrieb.md"
	chapter17 = "17-observability.md"
	chapter18 = "18-release.md"
	chapter19 = "19-sicherheit.md"
	chapter20 = "20-datenschutz.md"
	chapter24 = "24-terminal.md"
)

// Defaults that several keys share; goconst asks for one constant each.
const (
	// defaultTrue is the default of a switch that is on.
	defaultTrue = "true"
	// defaultQuarterHour is a default of 15 minutes.
	defaultQuarterHour = "15m"
	// defaultTenSeconds is a default of 10 seconds.
	defaultTenSeconds = "10s"
	// defaultDay is a default of 24 hours.
	defaultDay = "24h"
)
