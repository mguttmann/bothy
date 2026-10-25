// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// SchemaVersion is the version of the export format (manifest.json schema_version); the JSON
// Schemas of docs/api/gdpr-export/v1 describe it. A change that is not additive raises it.
const SchemaVersion = 1

// The two files privacy writes itself (20 section 5.2).
const (
	// FileReadme explains the files in the language of the person.
	FileReadme = "README.txt"
	// FileManifest names the schema version, the instance, the creation time and the SHA-256 of
	// every other file; it is written last.
	FileManifest = "manifest.json"
)

// ManifestFile is one entry of manifest.json.
type ManifestFile struct {
	// Path is the path in the archive.
	Path string `json:"path"`
	// Size is the length in bytes.
	Size int64 `json:"size"`
	// SHA256 is the lower-case hex SHA-256 of the bytes.
	SHA256 string `json:"sha256"`
}

// Manifest is manifest.json.
type Manifest struct {
	SchemaVersion int            `json:"schema_version"`
	InstanceID    ids.ID         `json:"instance_id"`
	SubjectUserID ids.ID         `json:"subject_user_id"`
	RequestID     ids.ID         `json:"request_id"`
	CreatedAt     time.Time      `json:"created_at"`
	Files         []ManifestFile `json:"files"`
}

// PackageName returns the name of the archive, export-<user_id>-<date>.zip with the UTC date of
// at (20 section 5.2).
func PackageName(subject ids.ID, at time.Time) string {
	return "export-" + subject.String() + "-" + at.UTC().Format(time.DateOnly) + ".zip"
}

// ReadmeLanguage returns the language of README.txt for the locale of the person (users.locale,
// a BCP 47 tag): de for a German tag, en for every other or no tag.
func ReadmeLanguage(locale string) string {
	l := strings.ToLower(locale)
	if l == "de" || strings.HasPrefix(l, "de-") {
		return "de"
	}
	return "en"
}

// Readme returns README.txt in the language lang (de or en), explaining every file of the
// package. It names no person and no value of the instance.
func Readme(lang string) string {
	if lang == "de" {
		return readmeDE
	}
	return readmeEN
}

// readmeEN is README.txt in English.
const readmeEN = `Export of your personal data

This archive holds your own data and metadata of this instance, as JSON (Art. 15 and 20 GDPR).
Messages and other contents of other people are not included. Channel and thread names appear
only where you may still read the channel; otherwise only their IDs.

manifest.json       schema version, instance, creation time and the SHA-256 of every file
profile.json        your profile (principal and user)
identities.json     linked identities, passkeys and whether a password exists (no secrets)
devices.json        devices, sessions with login method and last IP, tokens, push targets,
                    terminal hosts and sessions
access.json         your grants, groups, owner status, agents you hold, apps you installed and
                    your privacy requests
messages.jsonl      your messages with their earlier versions, one per line
reactions.jsonl     your reactions, one per line
mentions.jsonl      where you were mentioned (only message ID and time), one per line
files/files.json    your uploads (name, type, size, checksum); the bytes are in files/<id>
runs.jsonl          agent runs you started (state, times, costs), one per line
interactions.jsonl  your answers to forms and buttons, one per line
personal.json       read state, drafts, settings, notification settings, quiet hours, sidebar,
                    templates
audit.jsonl         audit entries with you as actor or as resource, one per line

The format is described by JSON Schemas (schema_version in manifest.json).
`

// readmeDE is README.txt in German, a product string of the de locale (D-W-94).
//
// language-check: off
const readmeDE = `Export Ihrer personenbezogenen Daten

Dieses Archiv enthält Ihre eigenen Daten und Metadaten dieser Instanz als JSON (Art. 15 und 20
DSGVO). Nachrichten und andere Inhalte anderer Personen sind nicht enthalten. Namen von Kanälen
und Threads stehen nur dort, wo Sie den Kanal noch lesen dürfen; sonst nur ihre IDs.

manifest.json       Schemaversion, Instanz, Erstellungszeit und SHA-256 jeder Datei
profile.json        Ihr Profil (Principal und Benutzer)
identities.json     verknüpfte Identitäten, Passkeys und ob ein Passwort existiert (keine Geheimnisse)
devices.json        Geräte, Sitzungen mit Anmeldeart und letzter IP, Tokens, Push-Ziele,
                    Terminal-Hosts und -Sitzungen
access.json         Ihre Freigaben, Gruppen, Owner-Status, gehaltene Agenten, installierte Apps und
                    Ihre Datenschutzanfragen
messages.jsonl      Ihre Nachrichten mit früheren Fassungen, eine pro Zeile
reactions.jsonl     Ihre Reaktionen, eine pro Zeile
mentions.jsonl      wo Sie erwähnt wurden (nur Nachrichten-ID und Zeit), eine pro Zeile
files/files.json    Ihre Uploads (Name, Typ, Größe, Prüfsumme); die Bytes liegen unter files/<id>
runs.jsonl          von Ihnen gestartete Agenten-Läufe (Zustand, Zeiten, Kosten), einer pro Zeile
interactions.jsonl  Ihre Antworten auf Formulare und Schaltflächen, eine pro Zeile
personal.json       Lesestand, Entwürfe, Einstellungen, Benachrichtigungen, Ruhezeiten, Seitenleiste,
                    Vorlagen
audit.jsonl         Audit-Einträge mit Ihnen als Handelnde oder als Ressource, einer pro Zeile

Das Format beschreiben JSON-Schemas (schema_version in manifest.json).
`

// language-check: on
