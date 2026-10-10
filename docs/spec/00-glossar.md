# 00 Glossar

Status: Entwurf M0, nach den Rückmeldungen aller Kapitel integriert. Verbindlich für alle
Spezifikationen. Die Begriffe aus der Planungsgrundlage (Abschnitt 3) sind übernommen und um alle
weiteren Begriffe der Kapitel 03, 04 und 05 erweitert. Sortiert nach dem deutschen Begriff.
Code-Namen sind Englisch und gelten in API, Datenbank und Code.

| Begriff | Code-Name | Definition |
|---|---|---|
| Abruf-Ticket | `file ticket` | Kurzlebige, signierte URL für Medien-Player, die keinen DPoP-Beweis senden können; jeder Abruf prüft `file.read` neu ([ADR 0048](../adr/0048-datei-abruf-nur-ueber-den-server-mit-kurzlebigen-tickets.md)). |
| Agent | `agent` | Prinzipal für ein Bot-Konto, das ein Adapter außerhalb des Servers betreibt; höchstens Stufe `write` und als Zusatzrecht höchstens `upload`; löst nie selbst Agenten aus. |
| Agenten-Adapter | `adapter` | Programm außerhalb des Servers, das einen Agenten über die öffentliche API mit seiner Laufzeit verbindet (z. B. der Referenz-Adapter für Hermes Agent). |
| Agenten-Laufzeit | `runtime` | Software, die einen Agenten-Turn wirklich ausführt; läuft nie im Server-Prozess (B6, B10). |
| Agenten-Sitzung | `agent_session` | Zuordnung eines Threads zu einer externen Sitzung eines Agenten; ältere Zeilen desselben Threads sind die früheren Sitzungs-Referenzen. |
| Aktion | `action` | Benannte Operation in Punkt-Schreibweise (z. B. `message.create`), die `authorize` prüft. |
| Aktionskatalog | `action catalogue` | Geschlossene, normative Liste aller Aktionen in docs/spec/05-aktionen.md. |
| Anhang | `message_attachment` | Zuordnung einer Datei zu einer Nachricht in fester Reihenfolge. |
| Anheften | `pin` | Markierung einer Nachricht, damit sie oben im Thread auffindbar ist. |
| Anmeldevorgang | `auth_login_flow` | Zustand einer OIDC-Anmeldung oder Verknüpfung zwischen Start, Rückleitung und Einlösen des Server-Codes. |
| Antwort mit Zitat | `reply_to_message_id` | Verweis einer Nachricht auf eine frühere Nachricht desselben Threads. |
| API-Token | `api_token` | Langlebiges Zugangstoken für Agenten, Apps oder persönliche Werkzeuge im Format `bothy_<art>_<zufall>_<prüfsumme>` ([ADR 0061](../adr/0061-token-format-mit-festem-praefix-und-pruefsumme.md)); gespeichert nur als Hash. |
| App | `app` | Prinzipal einer installierten Integration mit Manifest; handelt nur über die öffentliche API, höchstens Stufe `write` und Zusatzrecht `upload`. |
| App-Installation | `app_installation` | Installation einer App in der Instanz mit bewilligten Scopes und Ereignis-Abos. |
| App-Manifest | `manifest` | Versionierte Beschreibung einer App: Scopes, Ereignis-Abos, Slash-Befehle, Komponenten, Webhook-Ziele. |
| Audit-Log | `audit_event` | Nur anhängendes, hash-verkettetes Protokoll sicherheitsrelevanter Entscheidungen, getrennt von Telemetrie. |
| Audit-Prüfpunkt | `audit_checkpoint` | Signierter Stand der Hash-Kette des Audit-Logs, zusätzlich außerhalb der Datenbank abgelegt. |
| Aufbewahrungsklasse | `retention class` | Name einer Löschregel (z. B. `content`, `security`); Dauer und Weg stehen in docs/spec/20-datenschutz.md. |
| Aufruf-Marke | `invocation token` | Kurzlebige, vom Server je Zustellung mit EdDSA signierte JWS (`typ = agent-invocation+jwt`) für genau einen Agenten-Aufruf mit Auslöser, Ziel, Stufe, Lauf, Backend-Richtlinie und Werkzeugklasse (I3). |
| Auftragsbindung | `acting_for` | Regel, dass ein Agent im Rahmen eines Laufs höchstens die Rechte des auslösenden Menschen hat (SR9). |
| Ausgangsfach | `outbox` | Lokale Warteschlange eines Clients für Schreibvorgänge, jeder mit eigener Client-ID, je Thread streng in Reihenfolge. |
| Auslöse-Art | `trigger_kind` | Herkunft eines Laufs oder einer Marke: `message`, `followup`, `interaction`, `command`, `autocomplete`. |
| Auslöser-Status | `trigger_state` | Zustand an der auslösenden Nachricht: `received`, `queued`, `working`, `done`, `failed`, `cancelled`, `followup_delivered`; ersetzt Status-Emoji; der Grund steht in `trigger_state_reason`. |
| authorize | `authorize` | Die einzige Autorisierungsfunktion `authorize(principal, action, resource) -> Decision` im Modul `authz` (B7). |
| Backend-Richtlinie | `billing_mode` | Festlegung je Agent, worauf er läuft: `personal` (Abo genau einer Person) oder `organization` (Organisations-API-Key). |
| Bediener | `operator` | Genau ein Agent oder keiner je Kanal (Erbe vom Bereich, überschreibbar); ohne Bediener antwortet kein Agent. Ein Thread hält seinen Bediener ab der Anlage fest, der Hauptverlauf nutzt den wirksamen Bediener. |
| Bediener-Regel | `operator rule` | Der wirksame Bediener eines Kanals und der feste Bediener eines Threads haben dort mindestens `write`, ohne Freigabe-Zeile. |
| Benachrichtigungs-Einstellung | `notification_preference` | Stufe `all`, `mentions` oder `mute` eines Menschen je Bereich, Kanal oder Thread; ohne eigene Einstellung gelten Kanal-Voreinstellung, persönlicher und Instanz-Standard. |
| Bereich | `space` | Oberste Gruppe der Seitenleiste mit globaler Reihenfolge und optionalem Standard-Bediener. |
| Bestätigung | `ack` | Meldung eines Agenten-Adapters, bis zu welcher globalen Sequenz er Ereignisse dauerhaft verarbeitet hat. |
| Branding | `branding` | Anzeigename, Logo und Farben der Instanz als Einstellung (`branding.*`). |
| Client-Interaktions-ID | `client_interaction_id` | UUIDv7 des Clients je Interaktion; eindeutig je Prinzipal, macht Klicks wiederholbar. |
| Client-Nachrichten-ID | `client_msg_id` | UUIDv7 des Clients je Nachricht; eindeutig je Autor, eine Wiederholung liefert das Original. |
| Commit-Reihenfolge | `commit order` | Reihenfolge, in der Transaktionen sichtbar werden; die globale Sequenz folgt ihr genau. |
| Cursor | `cursor` | Höchste globale Sequenz, die der Server für einen Empfänger geprüft hat; Startpunkt für das Nachholen. |
| Datei | `file` | Hochgeladenes Objekt im Objektspeicher mit Zweck, Prüfsumme, Virenscan, Vorschaubildern, optionalem Transkript und Dateitext. |
| Dateitext | `file_text_extract` | Aus einer Datei gewonnener Text mit eigenem Suchvektor. |
| Dateizweck | `files.purpose` | Art einer Datei: `attachment`, `avatar`, `branding`, `gdpr_export`; bestimmt den Lese-Pfad. |
| Dev-Umgebung | `dev_environment` | Abgeschottete Arbeitsumgebung einer Person für einen Runner-Agenten (Etappe Mehrbenutzer-Dev-Umgebung). |
| Direktnachricht | `dm` | Kanal-Art für Gespräche zwischen Menschen ohne Bereich, ohne Agent und ohne Freigaben; Zugriff nur für Teilnehmer. |
| DPoP | `dpop` | Proof-of-Possession nach RFC 9449: Tokens sind an den Geräteschlüssel gebunden und ohne ihn wertlos. |
| DSGVO-Auftrag | `gdpr_request` | Job für Export, Löschung oder Berichtigung personenbezogener Daten einer Person. |
| Eigene Messung | `own measurement` | Zählung des Servers selbst über `kernel/measure` für Live-Ansichten und SLO-Ampel, als Minuten-Aggregate in `stats_minutes`; unabhängig von der Telemetrie, mit SLI-Grenzen aus den Sloth-Dateien (docs/spec/25-visualisierung.md). |
| Eingangs-Webhook | `incoming_webhook` | Geheime URL, über die ein externes System als App in einen Kanal schreibt; löst nie Agenten aus. |
| Einladungs-Code | `enrollment_code` | Einmaliger Code des Owners für das erste Gerät, das Verknüpfen einer Identität oder die Wiederherstellung. |
| Empfangs-Aktion | `receive_action` | Aktion, die ein Empfänger auf die Ressource eines Ereignisses braucht, um es zu bekommen. |
| Entwurf | `draft` | Noch nicht gesendeter Text eines Menschen je Thread oder Kanal, versioniert und geräteübergreifend. |
| Ereignis | `event` | Mitteilung über eine Änderung mit einheitlichem Umschlag; dauerhaft oder kurzlebig. |
| Ereignis, dauerhaftes | `durable event` | Ereignis im Ereignis-Log mit globaler Sequenz; nachholbar bis `sync.event_retention`. |
| Ereignis, kurzlebiges | `ephemeral event` | Ereignis ohne Speicherung und ohne Sequenz (Tippen, Live-Box-Patch, Präsenz, Hinweis, Anfrage zur Autovervollständigung); höchstens einmal zugestellt. |
| Ereignis-Log | `event_log` | Tabelle der dauerhaften Ereignisse, geordnet nach globaler, commit-geordneter Sequenz (B8). |
| Ereignis-Umschlag | `envelope` | Gemeinsame Felder jedes Ereignisses (`id`, `seq`, `type`, `scope`, `resource`, `payload` und weitere), gleich für Gateway, REST-Nachholen und Webhooks; ohne JSON-`null`. |
| Erinnerung vor Ablauf | `access.expiring` | Persönliches Ereignis, bevor eine Freigabe oder Mitgliedschaft abläuft. |
| Erledigt | `closed` | Thread-Zustand nach Abschluss; eine neue Nachricht öffnet den Thread wieder. |
| Erwähnung | `mention` | Nennung eines Menschen oder Agenten in einer Nachricht; keine Rollen, kein @everyone, kein @here. |
| Feature-Flag | `feature_flag` | Schalter für eine Funktion, global oder für Personen und Gruppen. |
| Fingerabdruck des Rechners | `key_fingerprint` | Prüfwert über Noise- und Anmeldeschlüssel eines Rechners; der Owner tippt seinen Anfang bei der Kopplung ein ([24-terminal.md](24-terminal.md), Abschnitt 4.3). |
| Fokus | `focus` | Menge der Threads, die ein Client gerade anzeigt; nur sie bekommen Live-Box-Patches und Tippen. |
| Forum | `forum` | Kanal-Art als Liste von Beiträgen; jeder Beitrag ist ein Thread; mit Tags und Erledigt-Bereich. |
| Forum-Tag | `forum_tag` | Schlagwort eines Forums zum Filtern von Beiträgen. |
| Freigabe | `grant` | Subjekt x Ziel x Stufe x Zusatzrechte x optionales Ablaufdatum x Grund; Ziele `instance`, `space`, `channel` und `host` (dort ohne Stufe, nur `terminal`); Widerruf setzt `revoked_at`, die Zeile bleibt. |
| Frischer Step-up | `terminal.step_up_max_age` | Step-up, der jünger als `terminal.step_up_max_age` ist und für genau eine Terminal-Öffnung verbraucht wird ([08-identitaet.md](08-identitaet.md), Abschnitt 6.3). |
| Gateway | `gateway` | WebSocket-Schnittstelle des Servers unter `/api/v1/gateway` für Ereignisse und kurzlebige Ops; eigenes Modul ohne Tabellen. |
| Gateway-Sitzung | `gateway session` | Server-seitiger Zustand einer WebSocket-Verbindung; innerhalb von `sync.resume_window` fortsetzbar. |
| Gelesen-Stand | `read_state` | Höchste gelesene Thread-Sequenz eines Menschen je Thread; steigt nur, außer durch ausdrückliches Zurücksetzen. |
| Gerät | `device` | Installierter Client eines Menschen; ein neues Gerät wartet (`pending`) auf Bestätigung und darf bis dahin nur sich selbst lesen. |
| Geräteschlüssel | `device_key` | Schlüsselpaar im sicheren Speicher eines Geräts mit Zweck `dpop` (bindet Tokens) oder `step_up` (signiert nur nach Biometrie oder Geräte-PIN). |
| Gruppe | `group` | Benannte Menge von Menschen als Subjekt von Freigaben; kein handelnder Prinzipal. |
| Hauptverlauf | `main thread` | Thread mit `kind = main`, der den durchgehenden Verlauf eines Text- oder DM-Kanals trägt; ohne festen Bediener. |
| Heartbeat | `heartbeat` | Regelmäßiges Lebenszeichen eines Clients oder Agenten; bei Agenten mit Zustand und Ist-Modus. |
| Herkunft | `origin` | Quelle einer Nachricht: `human`, `agent`, `app`, `import`, `system`; nur `human` löst Agenten aus. |
| Hinweis | `notification` | Push oder kurzlebiges `notification.created` an einen Menschen; trägt nie Inhalt im Push. |
| Host-Dienst | `bothy-host` | Eigenes Programm auf einem Rechner, das als Dienstbenutzer ohne Administratorrechte die Schlüssel des Rechners hält und nur ausgehend mit dem Server spricht; die PTYs startet sein PTY-Helfer als Sitzungsbenutzer. |
| Idempotenz-Schlüssel | `Idempotency-Key` | UUIDv7 in der Kopfzeile schreibender Anfragen ohne eigene Client-ID; Wiederholung liefert die gespeicherte Antwort. |
| Identität | `user_identity` | Verknüpfung eines Menschen mit einem Subjekt bei einem Identitätsanbieter. |
| Identitätsanbieter | `identity_provider` | Anmelde-Treiber: OIDC, vertrauenswürdiger Proxy-Header oder lokales Konto (Passkey, Einladungs-Code, Geräte-Kopplung, Passwort mit TOTP als Rückfall) (B9). |
| Import-Quelle | `import_source` | Konfigurierte Quelle eines Imports (z. B. ein Discord-Server) mit Zustand und Fortschritt. |
| Importiertes Objekt | `import_item` | Ein übernommenes Objekt mit eindeutigem Herkunftsschlüssel `origin_key` je Quelle. |
| Instanz | `instance` | Eine Installation; eine Organisation; ein Server im Discord-Sinn (B13). |
| Interaktion | `interaction` | Bedienung einer Komponente durch einen Prinzipal; autorisiert wie ein Agenten-Aufruf und höchstens einmal wirksam. |
| Invariante I1 | `I1` | Bei `billing_mode = personal` hat genau der `credential_holder` wirksam `trigger` auf den Zielen des Agenten; transaktional geprüft. |
| Invariante I2 | `I2` | Prüfung unmittelbar vor jedem Agenten-Aufruf, dass bei `personal` der Auslöser der `credential_holder` ist; fail closed. |
| Invariante I3 | `I3` | Signierte Aufruf-Marke; der Adapter startet nur, wenn sein Ist-Modus dem Soll der Marke entspricht. |
| Invariante I6 | `I6` | Bei `hosts.llm_credentials = personal` öffnet nur der Inhaber der Anmeldung ein Terminal auf dem Rechner, auch kein anderer Owner; transaktional und bei jedem Öffnen geprüft. |
| Ist-Modus | `reported_billing_mode` | Vom Adapter gemeldete tatsächliche Backend-Richtlinie, aus Heartbeat und je Lauf; fehlt bei Adaptern, die je Lauf dem Soll folgen (dann `billing_modes_supported`). |
| Kanal | `channel` | Ort für Gespräche in genau einem Bereich; Arten `text`, `forum`, `dm`; optional schreibgeschützt (`read_only`). |
| Kanal-Prompt | `prompt` | Anweisungstext des Kanals für den Bediener; nur mit `manage` lesbar, nie in Ereignissen. |
| Komponente | `component` | Interaktives Element an einer Nachricht: Knopf, Auswahl oder Formular. |
| Konfigurationswert | `config key` | Benannter Wert für eine Grenze, ein Fenster oder ein Intervall; Standard steht in genau einer Tabelle. |
| Kopplungs-Code des Rechners | `host pairing code` | Einmaliger Code `bothy_hpc_...` des Owners, mit dem der Host-Dienst seine öffentlichen Schlüssel beim Server anmeldet. |
| Kosteneintrag | `cost_entry` | Buchung der Kosten eines Laufs, als Schätzung oder abgeglichen, mit Ist-Modus. |
| Lauf | `run` | Ein Agenten-Turn mit Zustand, Schritten, Kennzahlen und Kosten; zugleich die Live-Box. |
| Lauf-Dokument | `run_document` | JSON-Darstellung eines Laufs, auf die Live-Box-Patches angewendet werden. |
| Lauf-Kontext | `run_context` | Vom Server gebauter Kontext eines Laufs mit Vertrauensmarke je Nachricht; der Adapter holt ihn mit `run.context.read`. |
| Lauf-Schritt | `run_step` | Geordneter Teil eines Laufs: Werkzeugaufruf, Statuszeile, Unteragent, Workflow oder Hintergrundjob. |
| Läuft gerade | `run overview` | Übersicht aller aktiven Läufe, die ein Mensch sehen darf. |
| Live-Box | `live box` | Live aktualisierte Darstellung eines Laufs im Thread; klappt am Ende zu einer Zeile zusammen. |
| Live-Rahmen | `stats.live` | Kurzlebiges Gateway-Ereignis mit dem aktuellen Stand der Läufe, Agenten und Raten, je `stats.live_interval` an Verbindungen mit Abo `stats.subscribe`; gefiltert auf die sichtbaren Kanäle, ohne Inhalte und ohne Personen. |
| LLM-Anmeldung des Rechners | `llm_credentials` | Erklärung des Owners je Rechner: `none`, `personal` (mit Inhaber) oder `organization`; Grundlage von I6. |
| Lokale Anmeldedaten | `local_credential` | Passwort und TOTP eines lokalen Kontos als abschaltbarer Rückfall. |
| Löschmarke | `tombstone` | Erhaltene Zeile einer gelöschten Nachricht ohne Inhalt; hält die Thread-Sequenz lückenlos. |
| Mensch | `user` | Prinzipal einer Person. |
| Minuten-Aggregat | `stats_minutes` | Zeile der eigenen Messung: Anzahl, Summe und gute Ereignisse einer Metrik je Minute, Kanal, Agent und Code; Aufbewahrungsklasse `stats`. |
| Momentaufnahme | `snapshot` | Zustand, den der Server bei `ready` oder bei `resync_required` liefert, ohne Thread-Inhalte. |
| Nachholen | `catch-up` | Abruf aller sichtbaren dauerhaften Ereignisse nach einem Cursor, gefiltert durch `authorize`. |
| Nachricht | `message` | Beitrag in einem Thread mit Thread-Sequenz, Herkunft, Art und Markdown-Inhalt. |
| Nachrichten-Revision | `message_revision` | Frühere Fassung einer bearbeiteten Nachricht. |
| Nachtrag | `followup` | Nachricht in einem Thread mit aktivem Lauf; wird dem Agenten als Nachtrag zum Lauf zugestellt. |
| Neu laden erforderlich | `resync_required` | Antwort des Servers, wenn Nachholen nicht möglich ist; der Client lädt eine Momentaufnahme. |
| Noise-Strom | `noise stream` | Ende-zu-Ende verschlüsselter Terminal-Strom nach `Noise_NK_25519_ChaChaPoly_SHA256` zwischen Gerät und Host-Dienst; der Server leitet nur weiter. |
| Owner | `owner` | Instanz-Inhaber; darf alles außer fremde Direktnachrichten lesen; Owner-Rechte nur in einer Sitzung, nie über ein persönliches API-Token; mindestens einer. |
| Passkey | `passkey_credential` | WebAuthn-Anmeldedaten eines Menschen für lokale Anmeldung und Step-up. |
| Preistabelle | `price_table` | Preise je Anbieter, Modell und Einheit mit Quelle und Abrufzeit. |
| Prinzipal | `principal` | Wer handelt: Mensch (`user`), Agent (`agent`) oder App (`app`); `anonymous` nur für öffentliche Aktionen; ein Rechner (`host`) nur mit `host.connect` über seinen Anmeldeschlüssel, ohne Zeile in `principals`. |
| Projektbindung | `project_binding` | Bindung eines Forums an ein Dev-Projekt, das ein Runner-Agent als Bediener bearbeitet. |
| PTY-Helfer | `bothy-host pty-helper` | Teil des Host-Dienstes, der als Sitzungsbenutzer läuft, PTYs startet und beendet und keine Schlüssel kennt. |
| Push-Registrierung | `push_registration` | Push-Ziel eines Geräts bei einem Anbieter; Push trägt nie Inhalt. |
| Rate-Limit-Klasse | `rate-limit class` | Benannte Gruppe von Aktionen mit gemeinsamem Token-Bucket (z. B. `rl.message`). |
| Reaktion | `reaction` | Freies Unicode-Zeichen an einer Nachricht; kein Status. |
| Rechner | `host` | Angebundener Rechner (macOS, Linux, Windows, auch eine Dev-VM) mit Host-Dienst; Ziel von Freigaben mit dem Zusatzrecht `terminal`. |
| Rechner-Inhaber | `hosts.assigned_user_id` | Mensch, dem ein Rechner zugewiesen ist; nur er bekommt dort als Nicht-Owner ein Terminal (SR10). |
| Rechte-Version | `permission_version` | Zähler, der bei jeder Rechte-Änderung steigt und Caches und offene Verbindungen neu bewerten lässt. |
| Refresh-Token-Familie | `refresh_token_family` | Kette rotierender, an den DPoP-Schlüssel gebundener Refresh-Tokens einer Sitzung; eine Wiederverwendung außerhalb der Nachsicht `auth.refresh_reuse_grace` sperrt die ganze Familie. |
| Relais | `relay` | Teil des Moduls `terminal`, das Client- und Host-Datenverbindung einer Sitzung im Speicher eines Knotens verbindet, ohne Inhalte zu lesen. |
| Ruhezeiten | `quiet_hours` | Zeitfenster ohne Push für einen Menschen. |
| Schließ-Code | `close code` | Code, mit dem der Server eine WebSocket-Verbindung beendet, mit festgelegter Client-Reaktion. |
| Schlüssel-Pinning | `key pinning` | Ein Gerät merkt sich den Noise-Schlüssel eines Rechners beim ersten Öffnen und warnt hart bei einem Wechsel. |
| Schreibschutz | `read_only` | Zustand eines Kanals, in dem kein Prinzipal, auch kein Owner, schreibt; nur ein laufender Import schreibt (Lese-Spiegel). |
| Schwärzen | `redaction` | Entfernen der Nutzlast eines Ereignisses nach Löschung; der Umschlag bleibt. |
| Scope | `scope` | Bewilligter Zugriffsbereich einer App zusätzlich zu ihren Freigaben. |
| Seitenleisten-Zustand | `user_space_state` | Persönlich eingeklappte Bereiche und eigene Reihenfolge, geräteübergreifend. |
| Sequenz je Thread | `thread_seq` | Lückenlose Nummer jeder Nachricht in ihrem Thread; bestimmt die Anzeige-Reihenfolge. |
| Sequenz, globale | `seq` | Lückenlose, commit-geordnete Nummer jedes dauerhaften Ereignisses der Instanz. |
| Sicherungslauf | `backup_run` | Lauf einer Sicherung oder eines automatischen Wiederherstellungstests mit Ergebnis je Prüfung. |
| Signierschlüssel | `server_signing_key` | Schlüssel des Servers für Zugriffstokens, Aufruf-Marken, Sitzungs-Marken des Terminals, ausgehende Webhooks und Audit-Prüfpunkte; privater Teil nur im Secret-Provider. |
| Sitzung | `auth_session` | Anmeldesitzung eines Menschen auf einem Gerät (App, Erweiterung als Kind-Sitzung oder Konsole); ihr Widerruf wirkt sofort auf alle Tokens. |
| Sitzungs-Marke | `terminal session token` | Kurzlebige, vom Server mit EdDSA signierte JWS (`typ = terminal-session+jwt`), die genau einmal genau eine Terminal-Sitzung auf einem Rechner erlaubt. |
| Sitzungsbenutzer | `host_service.session_user` | Benutzer des Betriebssystems auf einem Rechner, unter dem alle Terminal-Sitzungen laufen; ohne Leserecht auf die Schlüssel des Host-Dienstes. |
| Skill-Bindung | `channel_skill_binding` | Geordneter Skill-Verweis, den der Bediener in einem Kanal lädt. |
| Slash-Befehl | `command` | Von Agenten oder Apps registrierter Befehl mit Optionen und Autovervollständigung. |
| Statusseite | `status page` | Anzeige des Betriebszustands mit Komponenten und Störungen. |
| Step-up | `step_up` | Frische Bestätigung durch den Menschen (Passkey, Step-up-Geräteschlüssel mit Biometrie oder PIN, Passwort mit TOTP oder erneute OIDC-Anmeldung), die Verwaltungsaktionen verlangen. |
| Steuerbefehl | `control command` | Befehl wie Stopp oder Status, der nie als Prompt an das Modell geht und kein Nachtrag wird. |
| Störung | `status_incident` | Eintrag der Statusseite zu einem Ausfall oder einer Wartung. |
| Stufe | `level` | Aufsteigende Rechtestufe einer Freigabe: `read` < `write` < `trigger`. |
| Stufe trigger | `trigger` | Höchste Stufe: darf Agenten auslösen und Komponenten bedienen. |
| Sync-Epoche | `sync_epoch` | Kennung der Zeitlinie des Ereignis-Logs; neu nach jeder Wiederherstellung, Clients speichern sie mit dem Cursor. |
| Systemaktion | `system action` | Aktion im Katalog, die nur der Server selbst ausführt (Job, Server-Befehl), z. B. `grant.expire`. |
| Terminal-Profil | `terminal profile` | In der Konfiguration des Host-Dienstes festgelegter Start einer Sitzung (`shell`, `claude`, feste Befehle); der Server kennt nur Name und Art. |
| Terminal-Sitzung | `terminal_session` | Eine PTY auf einem Rechner für einen Menschen, mit Zustand `opening`, `active`, `detached` oder `closed`; Inhalte kennt nur das Gerät und der Rechner. |
| Thread | `thread` | Einheit eines Gesprächs und einer Agenten-Sitzung mit fester ID, Titel, Titelquelle und Zustand. |
| Tippen-Anzeige | `typing` | Kurzlebiges Ereignis, dass ein Mensch im Thread schreibt. |
| Titelquelle | `title_source` | `auto` oder `manual`; ein manuell gesetzter Titel wird nie automatisch überschrieben. |
| Transkript | `transcript` | Text einer Sprachnachricht, erstellt vom Agenten oder einem Transkriptions-Dienst. |
| TV-Modus | `tv mode` | Vollbild-Darstellung der Visualisierungen ohne Bedienelemente, in der Sitzung eines Menschen, Kosten standardmäßig verborgen. |
| Upload-Sitzung | `file_upload_session` | Fortsetzbarer Upload einer Datei in Teilen, gleich für beide Speicher-Treiber. |
| Verbraucher | `consumer` | Benannter Leser des Ereignis-Logs mit dauerhaftem Cursor: je Agent in `agent_consumer_cursors`, im Worker (Push, Webhooks) in `event_consumer_cursors`. |
| Vorlage | `message_template` | Gespeicherter Text für häufige Eingaben, persönlich oder je Kanal. |
| Vorschau einer Rechte-Änderung | `access.preview` | Trockenlauf einer Rechte-Aktion mit Unterschieden, Invarianten und Hinweisen in Klartext; ohne Wirkung. |
| Vorschaubild | `file_thumbnail` | Verkleinerte Darstellung einer Datei in fester Variante. |
| Webhook, ausgehender | `webhook_subscription` | Abo einer App oder eines Agenten, Ereignisse signiert (`v1a`, optional HMAC `v1`) an eine freigegebene HTTPS-Adresse zu erhalten. |
| Werkzeugklasse | `tool_class` | Was die Laufzeit eines Agenten tun darf: `chat` < `scoped` < `host`; Soll am Agenten, Ist im Heartbeat. |
| Wirksame Stufe | `effective level` | Höchste Stufe aller gültigen Freigaben eines Prinzipals und seiner Gruppen auf dem Pfad vom Ziel nach oben. |
| Wirksamer Bediener | `effective operator` | Bediener eines Kanals nach Auflösung des Erbes vom Bereich. |
| Zähler-Zeile | `sequence_counters` | Tabellenzeile, deren Sperre in der schreibenden Transaktion die Commit-Reihenfolge der Nummern erzwingt. |
| Zugangsdaten-Inhaber | `credential_holder_id` | Mensch, auf dessen persönlichem Abo ein Agent mit `billing_mode = personal` läuft. |
| Zugriffstoken | `access token` | Kurzlebiges, DPoP-gebundenes Token eines Menschen mit Sitzungs-ID `sid`. |
| Zusatzrecht | `capability` | Recht neben der Stufe: `approve_actions`, `upload`, `export`, `manage`; auf Rechnern nur `terminal`. |
| Zustellfeld | `delivery field` | Feld im Umschlag, das je Zustellung entsteht und nie im Log steht: `invocation_token`, `redelivered`. |
| Zustellung | `delivery` | Übergabe eines Ereignisses an eine Verbindung, einen Push-Dienst oder einen Webhook; jedes Mal durch `authorize` geprüft. |
