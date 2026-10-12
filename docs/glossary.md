# Glossary: German to English

All repository content is English (ruling D-W-94 in the [ruling log](DECISIONS.md)). Until
2026-10-11 the planning documents were German. This page maps the German terms of those documents
to the one English term that the repository uses for each concept. Use it to write new text, to
read German material from before the switch (notes, older branches, ticket reports) and to
review translations.

The product glossary with definitions is [00 Glossary](spec/00-glossary.md). Its code names
(tables, fields, actions, enum values) are authoritative: where a code name exists, the English
term matches it.

Rules:

1. One English term per concept. Where one German word has two meanings, the table names both,
   each with its own English term.
2. Spelling is US English. The established project term "catalogue" (code name `action catalogue`,
   job catalogue, signal catalogue) keeps its spelling.
3. Terms of the product interface follow the German and English term table of
   [01 Product](spec/01-product.md), section 9. That table also keeps the German wording of the
   `de` locale.
4. Section 3 lists words that tools read. Documents and tools use exactly the English word given
   there.
5. German stays only where section 4 says so. The gate `make docs` fails on German prose
   ([tools/docs/language_check.py](../tools/docs/language_check.py)); this page is excluded from
   that check.

## 1. Product terms

| German | English | Code name | Note |
|---|---|---|---|
| Abruf-Ticket | file ticket | `file ticket` | short-lived signed URL for media players |
| Agent | agent | `agent` | |
| Agenten-Adapter | agent adapter | `adapter` | |
| Agenten-Aufruf | agent invocation | `agent.invoke` | |
| Agenten-Laufzeit | agent runtime | `runtime` | |
| Agenten-Sitzung | agent session | `agent_session` | |
| Aktion | action | `action` | |
| Aktionskatalog | action catalogue | `action catalogue` | |
| Anhang | attachment | `message_attachment` | |
| Anheften | pin | `pin` | |
| Anmeldevorgang | login flow | `auth_login_flow` | |
| Anmeldung, Abmeldung | login, logout | `auth.login.*`, `auth.logout` | "LLM-Anmeldung": LLM login |
| Antwort mit Zitat | reply with quote | `reply_to_message_id` | |
| App-Installation | app installation | `app_installation` | |
| App-Manifest | app manifest | `manifest` | |
| Audit-Log | audit log | `audit_event` | |
| Audit-Prüfpunkt | audit checkpoint | `audit_checkpoint` | |
| Aufbewahrung, Aufbewahrungsklasse | retention, retention class | `retention class` | |
| Aufruf-Marke | invocation token | `invocation token` | |
| Auftragsbindung | acting-for binding | `acting_for` | |
| Ausgangsfach | outbox | `outbox` | |
| auslösen | trigger | `trigger` | "Agenten auslösen": trigger agents |
| Auslöse-Art | trigger kind | `trigger_kind` | |
| Auslöser | trigger; triggering person | | the message or the person that starts a run |
| Auslöser-Status | trigger state | `trigger_state` | |
| Backend-Richtlinie | backend policy | `billing_mode` | the concept and chapter 10; values `personal`, `organization` |
| Abrechnungsart | billing mode | `billing_mode` | |
| Abo (KI-Abo) | subscription | | |
| Backend-Schalter | backend switch | | |
| Bediener | operator | `operator` | the agent that answers in a channel |
| Bediener-Regel | operator rule | `operator rule` | |
| Benachrichtigung | notification | `notification` | |
| Benachrichtigungs-Einstellung | notification preference | `notification_preference` | |
| Bereich | space | `space` | |
| Bestätigung | ack; confirmation | `ack` | `ack` for the adapter acknowledgement |
| Betreiber | instance operator | | the person or team that runs a Bothy installation; "operator" alone always means the channel operator agent |
| Betrieb | operations | | |
| Bot-Konto | bot account | | |
| Datei | file | `file` | |
| Dateitext | file text extract | `file_text_extract` | |
| Dateizweck | file purpose | `files.purpose` | |
| Dev-Abschottung | dev isolation | | chapter 21 |
| Dev-Umgebung | dev environment | `dev_environment` | |
| Direktnachricht | direct message (DM) | `dm` | |
| DSGVO | GDPR | `gdpr` | |
| DSGVO-Auftrag | privacy request | `gdpr_request` | |
| Eigene Messung | own measurement | `stats_minutes` | |
| Eingangs-Webhook | incoming webhook | `incoming_webhook` | |
| Einladungs-Code | enrollment code | `enrollment_code` | |
| Einrichtungsassistent | setup wizard | | |
| Empfangs-Aktion | receive action | `receive_action` | |
| Entwurf (einer Nachricht) | draft | `draft` | |
| Ereignis | event | `event` | |
| Ereignis, dauerhaftes | durable event | `durable event` | |
| Ereignis, kurzlebiges | ephemeral event | `ephemeral event` | |
| Ereignis-Log | event log | `event_log` | |
| Ereignis-Umschlag | event envelope | `envelope` | |
| Erinnerung vor Ablauf | expiry reminder | `access.expiring` | |
| Erledigt | done | `closed` | thread state; interface label "Done" |
| Erwähnung | mention | `mention` | |
| Fingerabdruck | fingerprint | `key_fingerprint` | |
| Fokus | focus | `focus` | |
| Forum, Beitrag | forum, post | `forum`, `thread` | |
| Forum-Tag | forum tag | `forum_tag` | |
| Freigabe (Zugriff) | grant | `grant` | interface label "Access grant" |
| Freigabe (einer Aktion) | approval | `approve_actions` | "Freigabe-Karte": approval card; "Freigabe-Knöpfe": approval buttons; "Freigabe nötig": approval needed |
| Freigabe (eines Geräts) | device approval | `device.approve` | "Gerät freigeben": approve a device |
| Freigabe (eines DSGVO-Auftrags) | approval of a privacy request | `gdpr.request.decide` | |
| freigegebene Adresse | allowed address | `webhook.allowed_hosts` | allow-list |
| freigegeben (für eine Person sichtbar) | shared with, granted to | | "was ihr freigegeben ist": what has been granted to them |
| Freischaltung | enabling, rollout | | not a grant |
| freigeben (Aktionen) | approve | `approve_actions` | "Aktionen freigeben": approve actions |
| Frischer Step-up | fresh step-up | `terminal.step_up_max_age` | |
| Gateway-Sitzung | gateway session | `gateway session` | |
| Gelesen-Stand | read state | `read_state` | |
| Gerät | device | `device` | |
| Gerätebindung | device binding | | |
| Geräteschlüssel | device key | `device_key` | |
| Gruppe | group | `group` | |
| Hauptverlauf | main thread | `main thread` | |
| Herkunft | origin | `origin` | |
| Herkunftsschlüssel | origin key | `origin_key` | |
| Hinweis (an einen Menschen) | notification | `notification` | |
| Hinweis (der Vorschau) | hint | `hints` | |
| Hinweis (im Text) | note | | "Hinweis:" in a template: "Note:" |
| Host-Dienst | host service | `bothy-host` | |
| Idempotenz-Schlüssel | idempotency key | `Idempotency-Key` | |
| Identität | identity | `user_identity` | |
| Identitätsanbieter | identity provider | `identity_provider` | |
| Import-Quelle | import source | `import_source` | |
| Importiertes Objekt | imported item | `import_item` | |
| Instanz | instance | `instance` | |
| Interaktion | interaction | `interaction` | |
| Invariante | invariant | | I1 to I6 |
| Ist-Modus | reported mode | `reported_billing_mode` | the mode the adapter reports |
| Soll (Modus, Werkzeugklasse) | configured mode, configured tool class | `billing_mode`, `tool_class` | never "target", which is the grant target |
| Kanal | channel | `channel` | |
| Kanal-Prompt | channel prompt | `prompt` | |
| Komponente | component | `component` | |
| Konfigurationswert | configuration value | `config key` | |
| Konsole | console | | the web admin and statistics console |
| Kopplung, Kopplungs-Code | pairing, pairing code | `host.pair` | |
| Kosteneintrag | cost entry | `cost_entry` | |
| Kostenabgleich | cost reconciliation | | |
| Lauf | run | `run` | |
| Lauf-Dokument | run document | `run_document` | |
| Lauf-Kontext | run context | `run_context` | |
| Lauf-Schritt | run step | `run_step` | |
| Läuft gerade | running now | `run overview` | the overview of active runs |
| Lesehorizont | read horizon | | hint H1 |
| Live-Box | live box | `run` | |
| Live-Rahmen | live frame | `stats.live` | |
| Live-Topologie | live topology | | |
| LLM-Anmeldung des Rechners | LLM login of the host | `llm_credentials` | |
| Lokale Anmeldedaten | local credentials | `local_credential` | |
| Löschmarke | tombstone | `tombstone` | |
| Mensch, Menschen | person, people | `user`, origin `human` | `user` and `human` only as code names |
| Minuten-Aggregat | minute aggregate | `stats_minutes` | |
| Mitglied, Mitgliedschaft | member, membership | | |
| Momentaufnahme | snapshot | `snapshot` | |
| Nachholen | catch-up | `catch-up` | |
| Nachricht | message | `message` | |
| Nachrichten-Revision | message revision | `message_revision` | |
| Nachtrag | follow-up | `followup` | |
| Neu laden erforderlich | resync required | `resync_required` | |
| Noise-Strom | Noise stream | `noise stream` | |
| Owner | owner | `owner` | the instance owner role; also the project owner who gives requirements |
| Passkey | passkey | `passkey_credential` | |
| Persönlich, Organisation | personal, organization | `personal`, `organization` | |
| Preistabelle | price table | `price_table` | |
| Prinzipal | principal | `principal` | |
| Projektbindung | project binding | `project_binding` | |
| PTY-Helfer | PTY helper | `bothy-host pty-helper` | |
| Push-Registrierung | push registration | `push_registration` | |
| Rate-Limit-Klasse | rate-limit class | `rate-limit class` | |
| Reaktion | reaction | `reaction` | |
| Rechner | host | `host` | a connected computer with the host service |
| Rechner-Inhaber | host assignee | `hosts.assigned_user_id` | |
| Recht, Rechte | permission, permissions | `permission_version`, `access.*` | "Rechte-Änderung": permission change; "Rechte-Stand": permission snapshot; "Rechte-Cache": permission cache; "Leserecht": read permission; planned identifiers such as `RightsChange` keep their names |
| Rechte-Version | permission version | `permission_version` | |
| Relais | relay | `relay` | |
| Rückfrage | question | `waiting_input`, `input_needed` | an agent asks a person and waits for the answer; "Werkzeug-Rückfrage": tool permission prompt; "ohne Rückfrage": without a confirmation prompt |
| Ruhezeiten | quiet hours | `quiet_hours` | |
| Schätzung | estimate | `cost_is_estimate` | |
| Schließ-Code | close code | `close code` | |
| Schlüssel-Pinning | key pinning | `key pinning` | |
| Schreibschutz, schreibgeschützt | read-only | `read_only` | |
| Schutzregel | guard rule | `guard_blocked` | SR1 to SR12 |
| Schwärzen | redaction | `redaction` | |
| Seitenleiste, Seitenleisten-Zustand | sidebar, sidebar state | `user_space_state` | |
| Sequenz, globale | global sequence | `seq` | |
| Sequenz je Thread | thread sequence | `thread_seq` | |
| Sicherung | backup | | |
| Sicherungslauf | backup run | `backup_run` | |
| Signierschlüssel | signing key | `server_signing_key` | |
| Sitzung | session | `auth_session` | a login session; see also terminal session |
| Sitzungs-Marke | terminal session token | `terminal session token` | |
| Sitzungsbenutzer | session user | `host_session_users` | |
| Skill-Bindung | skill binding | `channel_skill_binding` | |
| Slash-Befehl | slash command | `command` | |
| Sondertasten-Leiste | extra keys bar | | |
| sperren | disable (person, agent, host); revoke (device); suspend (app); lock (database row) | `user.disable`, `agent.disable`, `device.revoke`, `app.suspend` | "Sperrreihenfolge": lock order; "gesperrt" after failed logins: locked out |
| Statusseite | status page | `status page` | |
| Steuerbefehl | control command | `control command` | |
| Störung | incident | `status_incident` | |
| Stufe | level | `level` | `read` < `write` < `trigger` |
| Sync-Epoche | sync epoch | `sync_epoch` | |
| Systemaktion | system action | `system action` | |
| Terminal-Profil | terminal profile | `terminal profile` | |
| Terminal-Sitzung | terminal session | `terminal_session` | |
| Tippen-Anzeige | typing indicator | `typing` | |
| Titelquelle | title source | `title_source` | |
| Transkript | transcript | `transcript` | |
| Trennen, Wiederanhängen | detach, reattach | `detached`, `terminal.attach` | |
| TV-Modus | TV mode | `tv mode` | |
| Upload-Sitzung | upload session | `file_upload_session` | |
| Verbraucher | consumer | `consumer` | |
| Vererbung | inheritance | | |
| Verbot | deny rule | | "kein Verbot": no deny rules |
| Visualisierung | visualization | | |
| Vorlage (Nachricht) | template | `message_template` | |
| Vorschau einer Rechte-Änderung | permission change preview | `access.preview` | |
| Vorschaubild | thumbnail | `file_thumbnail` | in docs/design: preview image |
| Wartungsmodus | maintenance mode | `read_only` | |
| Webhook, ausgehender | outgoing webhook | `webhook_subscription` | |
| Werkzeugklasse | tool class | `tool_class` | |
| Widerruf, widerrufen | revocation, revoke | `revoked_at` | |
| Wiederherstellung | restore; recovery | | restore of a backup; recovery of an account |
| Wirksame Stufe | effective level | `effective level` | |
| Wirksamer Bediener | effective operator | `effective operator` | |
| Zähler-Zeile | counter row | `sequence_counters` | |
| Zugangsdaten-Inhaber | credential holder | `credential_holder_id` | |
| Zugriffstoken | access token | `access token` | |
| Zusatzrecht | capability | `capability` | `approve_actions`, `upload`, `export`, `manage`, `terminal` ("terminal capability") |
| Fähigkeit (eines Adapters) | adapter feature | heartbeat `capabilities` | kept apart from capability |
| Prüfstelle | single decision point; enforcement point | `authorize` | the one authorization function; the places that check an invariant |
| Zustellfeld | delivery field | `delivery field` | |
| Zustellung | delivery | `delivery` | |

## 2. Planning and process terms

| German | English | Note |
|---|---|---|
| Abhängig von | Depends on | plan table column |
| Abschnitt | section | |
| Akzeptanztests | acceptance tests | |
| Arbeitsname | working name | |
| Arbeitsweise je Ticket | way of working per ticket | |
| Auftrag (eines Tickets) | brief | "Bau-Brief": build brief |
| Auftragsverarbeiter | processor | GDPR |
| Bau-Agent, Bau-Sitzung, Bau-Ticket | build agent, build session, build ticket | |
| Befund | finding | |
| Bekannte Widersprüche | known contradictions | |
| Bedrohung, Bedrohungsmodell | threat, threat model | |
| Begründung, Grund | rationale, reason | |
| Entscheidung | decision | |
| Entscheidungen dieses Kapitels | Decisions of this chapter | section title in every chapter |
| Entwurf (Status) | draft | "Status: Entwurf M0": "Status: draft M0" |
| Etappe | milestone | M0 to M8; "stages" in the fixed build rules means milestones |
| Exit-Kriterien | exit criteria | |
| Festlegung | ruling | |
| Gegenmaßnahme | mitigation | |
| Kapitel | chapter | |
| Kern-Vertrag | core contract | chapters 00, 03, 04, 05 |
| Leitung, Planungsleitung | lead, planning lead | |
| Lücke | gap | |
| Nicht-Umfang | out of scope | |
| Owner-Vorgabe | owner requirement | |
| Planungsgrundlage | planning basis | |
| Prüfung | check | |
| Regel | rule | |
| Restrisiko | residual risk | |
| Risiko | risk | |
| Schaufenster | showcase | |
| Schreiber | writer | |
| Spezifikation | specification | |
| Teststrategie | test strategy | |
| Umfang | scope | |
| Voraussetzungen | prerequisites | |
| Welle | wave | |
| Werkzeug, Werkzeugkette | tool, toolchain | |
| Ziel | goal; target | goal of a milestone or ticket; target of a grant or an SLO |

## 3. Words that tools read

Documents and tools use exactly these words. A change to one of them changes the document, the
tool and its tests together.

| German (before 2026-10-11) | English | Read by |
|---|---|---|
| `## N. Konfigurationswerte dieses Kapitels` | `## N. Configuration values of this chapter` | tools/configdoc/configdoc.py, tools/docs/tests/test_config_values.py |
| column `Ort` | column `Place` | tools/configdoc/configdoc.py |
| columns `Name \| Standard \| Bedeutung` | columns `Name \| Default \| Meaning` | tools/configdoc/configdoc.py |
| `\| Schlüssel \| Umgebung \| Ort \| Typ \| Standard \| Bedeutung \| Stand \|` | `\| Key \| Environment \| Place \| Type \| Default \| Meaning \| Status \|` | generated by tools/configdoc/configdoc.py |
| `Quelle: [...], Abschnitt N.` | `Source: [...], section N.` | generated by tools/configdoc/configdoc.py |
| status `aktiv`, `geplant (<Etappe>)` | status `active`, `planned (<milestone>)` | generated by tools/configdoc/configdoc.py |
| place `Werkzeug` | place `Tool` | tools/configdoc/configdoc.py |
| places `Lasttest`, `Host-Dienst`, `Abgleich-App`, `Hermes-Adapter` | places `Load test`, `Host service`, `Reconciliation app`, `Hermes adapter` | tools/configdoc/configdoc.py (`PLACES`) |
| `leer` (default value) | `empty` | config tables, tools/configdoc/configdoc.py (D5) |
| `keiner, Pflicht` (default value) | `no default, required` | config tables, tools/configdoc/configdoc.py (D5) |
| field `etappe` | field `milestone` | tools/configdoc/planned.yaml |
| `\| Nr \| Titel \| Status \| Datum \|` | `\| No. \| Title \| Status \| Date \|` | generated by tools/docs/adr_index.py |
| `Rang`, `Wahrscheinlichkeit`, `Auswirkung` | `Rank`, `Likelihood`, `Impact` | tools/docs/risk_ranking.py |
| `niedrig`, `mittel`, `hoch` | `low`, `medium`, `high` | tools/docs/risk_ranking.py |
| `**Hinter Rang N**` | `**Behind rank N**` | tools/docs/risk_ranking.py |
| FK clause words `Indexe`, `Aufbewahrung`, `Rechte`, `Partitionierung`, `Hinweis` | `Indexes`, `Retention`, `Permissions`, `Partitioning`, `Note` | tools/docs/erd_check.py |
| `Etappe: M1` (brief header) | `Milestone: M1` | tools/docs/brief_deps.py (third header field) |
| `## Gate-Werkzeuge` | `## Gate tools` | tools/docs/tests/test_toolchain_doc.py |
| `## Erlaubte Lizenzen` | `## Allowed licenses` | tools/ci/tests/test_license_check.py |
| `## Semantische Rollen` | `## Semantic roles` | generated by tools/design-tokens/tokenkit/emit_docs.py |

## 4. German that stays

- Proper names, listed in [tools/docs/language-allow.txt](../tools/docs/language-allow.txt).
- The verdict tokens of the review gate, `Urteil: JA` and `Urteil: NEIN <reason>` ("verdict:
  yes", "verdict: no"). The reviewer tooling writes them and
  [tools/hooks/review_gate.py](../tools/hooks/review_gate.py) compares the last line of the
  report exactly (D-W-32).
- Product strings of the `de` locale that a test holds verbatim, such as the German golden texts
  of the permission change preview (07 section 9.3, brief M1-49) and the German column of the
  term table in 01 section 9. They stand between `<!-- language-check: off -->` and
  `<!-- language-check: on -->`.
- This page.
