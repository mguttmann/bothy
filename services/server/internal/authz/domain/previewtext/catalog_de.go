// SPDX-License-Identifier: Apache-2.0

package previewtext

// catalogDE is the German catalogue of the preview text (brief M1-49 "Catalog"; 07 section 9.3).
// Placeholders stand in braces; quotes are ASCII.
var catalogDE = catalog{
	"level.read":                          "Lesen",
	"level.write":                         "Schreiben",
	"level.trigger":                       "Auslösen",
	"subject.group":                       "Gruppe {group}",
	"where.instance":                      "in der ganzen Instanz",
	"where.space":                         "im Bereich {space}",
	"where.channel":                       "im Kanal {space}/{channel}",
	"summary":                             `{subject} erhält{until} die Stufe "{level}" {where}.`,
	"until":                               " bis {date}",
	"abilities.single":                    "{subject} kann damit: {abilities}.",
	"abilities.group":                     "Mitglieder von {group} können damit: {abilities}.",
	"ability.read":                        "Nachrichten lesen",
	"ability.write":                       "Nachrichten lesen und schreiben, Threads anlegen",
	"ability.trigger_agent":               "Agent {agent} auslösen",
	"caps":                                "Dateien hochladen: {upload}. Freigabe-Karten bestätigen: {approve}.",
	"caps.export":                         "Exportieren: ja.",
	"caps.manage":                         "Verwalten: ja.",
	keyYes:                                "ja",
	keyNo:                                 "nein",
	"denial.delegation_exceeds_own_level": "Abgelehnt: Du kannst höchstens deine eigene Stufe weitergeben.",
	"denial.delegation_exceeds_own_capabilities": "Abgelehnt: Du kannst nur eigene Zusatzrechte weitergeben.",
	"denial.subject_kind_not_allowed":            "Abgelehnt: Agenten und Apps bekommen Freigaben nur vom Owner, höchstens Schreiben und nur Dateien hochladen.",
	"denial.owner_only":                          "Abgelehnt: Freigaben auf der ganzen Instanz legt nur ein Owner an.",
	"hint.i1":                                    "Abgelehnt: {agent} läuft auf dem persönlichen Abo von {holder}. Zuerst auf Organisation umstellen.",
	"hint.tool_class":                            "Hinweis: {agent} hat die Werkzeugklasse Host; {subject} kann {agent} damit nicht auslösen (nur Owner).",
	"hint.read_horizon":                          "Hinweis: {agent} liest auch {space}/{channel}. {subject} darf {space}/{channel} nicht lesen, kann aber über {agent} Inhalte daraus erfahren.",
	"hint.read_horizon.group":                    "Hinweis: {agent} liest auch {space}/{channel}. {count} Mitglieder von {group} dürfen {space}/{channel} nicht lesen, können aber über {agent} Inhalte daraus erfahren.",
	"hint.read_horizon.group_one":                "Hinweis: {agent} liest auch {space}/{channel}. 1 Mitglied von {group} darf {space}/{channel} nicht lesen, kann aber über {agent} Inhalte daraus erfahren.",
	"hint.group_reach":                           "Hinweis: Die Gruppe {group} erreicht {count} Menschen.",
	"hint.group_reach.one":                       "Hinweis: Die Gruppe {group} erreicht einen Menschen.",
	"hint.expiry_missing":                        "Hinweis: Die Freigabe für {subject} hat kein Ablaufdatum.",
	"date":                                       "02.01.2006",
	// The guided flow resolve_i1 (M2-21, 10 section 11 point 2).
	"switch.organization": "Zuerst wird {agent} auf Organisation umgestellt; die Kosten trägt dann der Organisations-Key.",
	"switch.not_ready":    "Der Adapter von {agent} meldet noch keinen Organisations-Key. Aufrufe scheitern, bis er umgestellt ist.",
	// The grant on a host (M2-40, 07 section 9.3).
	"summary.host": `{subject} erhält{until} das Zusatzrecht "Terminal" auf dem Rechner {host}.`,
	"hint.i6":      "Abgelehnt: Auf {host} gilt die persönliche LLM-Anmeldung von {holder}. Dort darf nur {holder} eine Freigabe haben.",
	"hint.terminal_not_effective.server_host":     "Hinweis: Die Freigabe wirkt für {subject} nicht. {host} betreibt den Server; dort öffnen nur Owner ein Terminal.",
	"hint.terminal_not_effective.not_assignee":    "Hinweis: Die Freigabe wirkt für {subject} nicht. Auf {host} öffnen nur Owner und die Person, der der Rechner zugewiesen ist, ein Terminal.",
	"hint.terminal_not_effective.no_session_user": "Hinweis: Die Freigabe wirkt für {subject} erst, wenn {subject} auf {host} einen eigenen Sitzungsbenutzer hat.",
	"hint.terminal_not_effective.group":           "Hinweis: Die Freigabe wirkt für {count} Mitglieder von {group} auf {host} nicht.",
	"hint.terminal_not_effective.group_one":       "Hinweis: Die Freigabe wirkt für 1 Mitglied von {group} auf {host} nicht.",
}
