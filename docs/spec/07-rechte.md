# 07 Rechte und Autorisierung

Status: Entwurf M0. Normativ für das Modul `authz`, für jede Route, jede WebSocket-Op, jede
Zustellung und jeden Agenten-Aufruf.
Begriffe: [00-glossar.md](00-glossar.md). Tabellen: [03-datenmodell.md](03-datenmodell.md).
Ereignisse und Nachholen: [04-sync-protokoll.md](04-sync-protokoll.md). Aktionen, Empfangs-Aktionen
und Rate-Limit-Klassen: [05-aktionen.md](05-aktionen.md). Identität, Sitzungen und Step-up:
[08-identitaet.md](08-identitaet.md). Bedrohungen: [../security/threat-model.md](../security/threat-model.md).
Entscheidungen: ADR 0017 (eine Autorisierungsfunktion), ADR 0018 (Freigabe-Modell).

Dieses Kapitel legt fest, wie `authorize(principal, action, resource) -> Decision` entscheidet. Der
Aktionskatalog (welche Aktion welche Stufe braucht) steht in 05 und wird hier nicht wiederholt.
Die beim Entwurf beantragten Ergänzungen stehen jetzt im Kern-Vertrag (Abschnitt 13).

## 1. Grundsätze

1. **Standard ist nichts.** Ohne gültige Freigabe, Owner-Status oder DM-Teilnahme hat ein Prinzipal
   auf einem Ziel keine Stufe. Es gibt keine Verbots-Zeilen; die höchste gültige Stufe gilt.
2. **Eine Wahrheit.** Nur `authz` deutet Freigaben. Andere Module bekommen Entscheidungen oder die
   Menge sichtbarer Kanäle, nie Freigabe-Zeilen (03, Grundsatz 12).
3. **Zeit wird bei jedem Aufruf geprüft.** Ablaufdaten von Freigaben und Gruppenmitgliedschaften
   wirken ab der Sekunde des Ablaufs, ohne dass ein Job laufen muss. Jobs melden den Ablauf nur.
4. **Rechte gelten bei der Zustellung.** Was ein Empfänger bekommt, entscheidet `authorize` im
   Moment der Zustellung, nicht im Moment des Ereignisses (04, Abschnitt 8).
5. **Fail closed.** Jeder Fehler in `authz` (Datenbank nicht erreichbar, unbekannte Aktion,
   unbekannter Ressourcentyp) endet in Ablehnung, nie in Erlaubnis.
6. **Schutzregeln stehen über Freigaben.** Abschnitt 5 nennt Regeln, die keine Freigabe aufhebt.
7. **Wer einen Agenten auslösen darf, erreicht über ihn alles, was der Agent lesen kann.** Darum
   bindet `authz` Agenten-Aufrufe an die Rechte des auslösenden Menschen (Schutzregel SR9) und zeigt
   den Lesehorizont jedes Agenten offen an (Abschnitt 6).

## 2. Freigabe-Modell

### 2.1 Subjekte

| Subjekt | Code | Darf Freigaben empfangen | Höchste Stufe | Zusatzrechte |
|---|---|---|---|---|
| Mensch | `user` | ja | `trigger` | alle |
| Gruppe | `group` | ja (wirkt für ihre gültigen Mitglieder) | `trigger` | alle |
| Agent | `agent` | ja | `write` (R-DM-08) | nur `upload` (R-DM-20) |
| App | `app` | ja | `write` (R-DM-08) | nur `upload` (R-DM-20) |

Gruppen enthalten nur Menschen (R-DM-07). Gruppen sind keine handelnden Prinzipale.

Begründung für "Agenten und Apps nur `upload`": `approve_actions` an einem Agenten hieße, dass ein
Agent seine eigenen Freigabe-Karten bestätigen könnte. `manage` hieße, dass ein Agent Freigaben
vergibt und Kanal-Prompts liest. `export` hieße, dass ein Agent Verläufe gesammelt abzieht. Keines
davon braucht ein Agent für seine Arbeit. Die Tabelle `grants` hält das mit einem Check (03,
R-DM-20).

### 2.2 Ziele und Vererbung

Ziele sind `instance`, `space` und `channel` (03, `grants.target_kind`). Threads und alles darin
haben keine eigenen Freigaben; sie erben vom Kanal. Rechner (`host`) sind ein viertes Ziel
außerhalb dieses Baums (Abschnitt 2.12). Der Pfad einer Ressource steht in 05,
Abschnitt 2.2.

```text
instance
  └── space (Bereich)
        └── channel (Kanal: text oder forum)
              └── thread
                    └── message, run, run_step, component, interaction, agent_session
DM-Kanal: außerhalb des Baums, nur dm_participants
```

Eine Freigabe auf einem Ziel gilt für das Ziel und alles darunter. Auf einem Kanal gilt die höchste
Stufe aller gültigen Freigaben auf Kanal, Bereich und Instanz. Ein Kanal kann damit nie weniger
erlauben als sein Bereich. Wer "Bereich Support außer Kanal Eskalation" braucht, gibt Freigaben auf
die einzelnen Kanäle statt auf den Bereich (ADR 0018).

### 2.3 Stufen

Stufen sind aufsteigend und schließen die niedrigeren ein: `read` < `write` < `trigger`.

| Stufe | Bedeutung (Kurzfassung; maßgeblich ist die Spalte "Recht" in 05) |
|---|---|
| `read` | Bereich, Kanal, Threads, Nachrichten, Dateien, Läufe sehen; suchen; eigenen Gelesen-Stand, Benachrichtigungen pflegen |
| `write` | dazu Nachrichten senden, bearbeiten und löschen (eigene), Threads anlegen, schließen, öffnen, Reaktionen, Entwürfe, Tippen |
| `trigger` | dazu Agenten auslösen, Komponenten bedienen, Befehle ausführen, eigene Läufe abbrechen, Agenten-Sitzung neu beginnen oder binden |

`write` löst nie einen Agenten aus. Nur `trigger` tut das (Studie C, übernommen in Baseline
Abschnitt 3).

### 2.4 Zusatzrechte

| Zusatzrecht | Wofür | Braucht zusätzlich |
|---|---|---|
| `upload` | Dateien hochladen und anhängen (`file.upload`, Anhänge an `message.create`) | `write` |
| `approve_actions` | Komponenten mit `required_capability = 'approve_actions'` bedienen (Freigabe-Karten) | `trigger` |
| `export` | Kanal-Verlauf exportieren (`export.channel`) | `read` |
| `manage` | Verwaltung des Ziels: Kanäle anlegen und ändern, Verhalten, Tags, Anheften, fremde Nachrichten löschen, Threads verschieben, Eingangs-Webhooks, "Wer hat Zugriff?", Freigaben im eigenen Rahmen (Abschnitt 2.6) | Stufe beliebig |
| `terminal` | Terminal auf einem Rechner öffnen (`terminal.open`, `terminal.attach`); nur auf dem Ziel `host`, nur für Menschen und Gruppen (Abschnitt 2.12) | keine Stufe; SR10 und SR11 |

Zusatzrechte sind die Vereinigung über alle gültigen Freigaben des Prinzipals und seiner gültigen
Gruppen auf dem Pfad (05, Abschnitt 2.3). Ein Zusatzrecht ersetzt nie die nötige Stufe.

### 2.5 Gültigkeit einer Freigabe

Eine Freigabe `g` gilt zum Zeitpunkt `t` genau dann, wenn:

- `g.revoked_at IS NULL`,
- `g.expires_at IS NULL OR g.expires_at > t`,
- das Subjekt aktiv ist (`principals.status = 'active'`), bei Gruppen: `groups.deleted_at IS NULL`,
- bei Gruppen-Freigaben für Mensch `u`: es gibt eine Mitgliedschaft `m` mit `m.removed_at IS NULL`
  und `m.expires_at IS NULL OR m.expires_at > t`,
- bei App-Subjekten: die Installation ist `active`.

Grenze: Bei `expires_at = t` ist die Freigabe schon ungültig. `t` ist die Server-Uhr, einmal je
`authorize`-Aufruf gelesen, damit alle Prüfungen eines Aufrufs dieselbe Zeit sehen.

Freigaben werden nie geändert, nur widerrufen und neu angelegt (R-DM-09). Eine Verlängerung ist
also Widerruf plus neue Freigabe mit neuem Grund.

### 2.6 Delegation mit `manage`

Wer `manage` auf einem Ziel `Z` hat, darf auf `Z` und darunter Freigaben anlegen und widerrufen, mit
diesen Grenzen (R-AZ-05, präzisiert):

1. Stufe der neuen Freigabe höchstens die eigene wirksame Stufe auf dem Ziel der neuen Freigabe.
2. Zusatzrechte der neuen Freigabe höchstens die eigenen wirksamen Zusatzrechte dort (`manage`
   eingeschlossen; wer `manage` hat, darf es weitergeben).
3. Widerruf nur für Freigaben, die man nach 1 und 2 selbst hätte anlegen dürfen. Sonst könnte eine
   Person mit `write` und `manage` die stärkere Freigabe einer anderen Person entfernen, die der
   Owner gesetzt hat (05 `grant.revoke`, R-AZ-05).
4. Subjekte nur Menschen und Gruppen. Freigaben an Agenten und Apps legt nur der Owner an.
   Grund: Eine Freigabe an einen Agenten erweitert, was dessen Laufzeit liest und an einen
   LLM-Anbieter sendet. Das ist ein Datenfluss zu einem Auftragsverarbeiter und eine Änderung des
   Lesehorizonts ([20-datenschutz.md](20-datenschutz.md), Abschnitt 6). (05 `grant.create`.)
5. Freigaben auf der Instanz legt nur der Owner an.
6. Lesen (`grant.read`): Owner sehen alle Freigaben. Wer nur `manage` hat, sieht die Freigaben auf
   Zielen, auf denen er `manage` hat, und darunter; Filter nach Ziel oder Subjekt verkleinern
   diese Menge, sie vergrößern sie nie. Ein Filter auf ein Subjekt zeigt also nicht dessen
   Freigaben außerhalb des eigenen Bereichs.

Mitgliedschaften in Gruppen mit `source = 'oidc_claim'` bestimmt allein der Gruppen-Claim des
Anbieters ([08-identitaet.md](08-identitaet.md), Abschnitt 3.4); `group.member.add` und
`group.member.remove` auf solchen Gruppen enden mit `409 conflict` und `reason:
claim_managed_group`, weil die nächste Anmeldung eine Handänderung sonst still überschriebe.

### 2.7 Owner

- Owner haben auf jedem Ziel außer DM-Kanälen die Stufe `trigger` und alle Zusatzrechte (05,
  Abschnitt 2.3), unabhängig von Freigaben.
- Owner-Aktionen (Spalte "Owner" in 05) dürfen nur aktive Owner.
- Mindestens ein aktiver Owner bleibt immer (Schutzregel SR8).
- Der erste Owner entsteht nur per Server-Befehl ([08-identitaet.md](08-identitaet.md),
  Abschnitt 4). Weitere Owner nur per `owner.add` durch einen Owner mit Step-up.
- Owner lesen keine fremden Direktnachrichten (R-AZ-02, Schutzregel SR6).

### 2.8 Direktnachrichten

DM-Kanäle haben keinen Bereich und keine Freigaben (R-DM-10). Teilnehmer mit `left_at IS NULL`
haben `write` und `upload`. Niemand sonst sieht den Kanal, auch kein Owner. In DM-Kanälen gibt es
keine Agenten.

### 2.9 Agenten als Prinzipale

1. Agenten bekommen Freigaben wie Menschen, höchstens `write` und nur `upload` als Zusatzrecht.
2. **Bediener-Regel.** Ein Agent hat auf jedem Kanal, dessen wirksamer Bediener er ist, und auf
   jedem Thread mit `threads.operator_agent_id` gleich ihm mindestens `write`, ohne Freigabe-Zeile.
   Grund: Ohne diese Regel müsste jeder Bediener-Wechsel eine zweite, leicht vergessene Freigabe
   nach sich ziehen; beim Wechsel bliebe die alte Freigabe stehen. Die Regel folgt dem Bediener
   automatisch (05, Abschnitt 2.3 Punkt 4; R-AZ-13).
3. Agenten lösen nie selbst aus. Ihre Nachrichten haben `origin = 'agent'` (Schutzregel SR4).
4. Agenten führen keine Step-up-Aktionen aus (R-AZ-03).
5. Agenten haben implizit alle Scopes außer `users:read` und `costs:reconcile` (05, Abschnitt
   2.6); ihre Grenze sind die Freigaben und die Bediener-Regel.
6. Handelt ein Agent im Rahmen eines Laufs, gilt zusätzlich die Auftragsbindung (SR9).

### 2.10 Apps

Eine App braucht für jede Aktion beides:

1. eine wirksame Stufe aus Freigaben (höchstens `write`) und
2. den Scope der Aktion in `app_installations.granted_scopes` (05, Abschnitt 2.6).

Sichtbarkeit folgt der Stufe: Hat eine App eine Freigabe auf einen Kanal, ist der Kanal für sie
sichtbar. Fehlt ihr nur der Scope, ist die Antwort `403` mit Grund `scope_missing`, nicht `404`;
die App kennt den Kanal ohnehin aus ihrer Freigabe. Eingangs-Webhooks schreiben als ihre App, mit
`origin = 'app'`, und lösen nie aus.

### 2.11 Werkzeugklasse eines Agenten

Rechte regeln, wer einen Agenten erreicht. Was der Agent dann tun kann, bestimmt seine Laufzeit.
Damit der Server dafür eine Grenze ziehen kann, hat jeder Agent eine Werkzeugklasse:

| Klasse | Code | Was die Laufzeit darf | Schlimmster Missbrauch |
|---|---|---|---|
| Gespräch | `chat` | antworten, Thread-Kontext lesen, keine Wirkung außerhalb des Threads | falscher oder ungewollter Text im Thread |
| Begrenzt | `scoped` | benannte Werkzeuge mit eigenen, minimalen Zugangsdaten; Wirkung nach außen nur nach Freigabe-Karte | eine benannte, begrenzte Wirkung |
| Host | `host` | Shell, freies Dateisystem oder freier Netzzugang mit den Rechten des Laufzeit-Benutzers (z. B. ein Coding-Agent mit abgeschalteten Werkzeug-Rückfragen) | Übernahme des Rechners, auf dem die Laufzeit läuft, und aller dort erreichbaren Geheimnisse |

Datenmodell (03, `agents`): `tool_class text NOT NULL DEFAULT 'chat' CHECK (tool_class IN
('chat','scoped','host'))` und `reported_tool_class text NULL`. Setzen ist die Aktion
`agent.tool_class.set` (Owner, Step-up, Audit, Rechte `ja`).

Soll und Ist wie bei der Backend-Richtlinie: Die Aufruf-Marke trägt `tool_class` (Soll). Der
Adapter meldet seine tatsächliche Klasse im Heartbeat und startet keinen Lauf, wenn die tatsächliche
Klasse höher ist als das Soll (Grund `tool_class_mismatch`). Die Reihenfolge `chat` < `scoped` <
`host` ist dabei maßgeblich. Das Protokoll legt [09-agenten.md](09-agenten.md) fest; der Claim
`tool_class` steht in 05, Abschnitt 5.

### 2.12 Rechner und Zusatzrecht `terminal`

Normativ zusammen mit [24-terminal.md](24-terminal.md), Abschnitt 8.

1. Ein Rechner ist ein eigenes Ziel. Er erbt nichts von Instanz, Bereich oder Kanal; eine Freigabe
   auf die Instanz gibt kein Terminal (R-AZ-24).
2. Auf einem Rechner gibt es keine Stufe und genau ein Zusatzrecht: `terminal`. Empfänger sind nur
   Menschen und Gruppen (03, R-DM-35). Freigaben dort legt nur der Owner an.
3. Wirksames `terminal(P, H)` für einen Menschen `P` auf dem Rechner `H`:
   - `P` ist aktiver Owner in einer Sitzung, und nicht (`H.llm_credentials = 'personal'` und
     `P <> H.credential_holder_id`); oder
   - eine gültige Freigabe (2.5) mit Ziel `H` und `terminal` an `P` oder eine seiner gültigen
     Gruppen.

   Die Ausnahme des ersten Punkts (Owner auf einem Rechner mit `personal`) entscheidet die
   Schutzregel SR11 mit dem Grund `i6_holder_only` (Abschnitt 4.2, Schritt 9; M2-40); das Atom
   `+terminal` der Anforderung wiederholt sie nicht (docs/DECISIONS.md D-M2-39-3).
4. Wirksames `terminal` reicht nicht: SR10 und SR11 (Abschnitt 5) sperren danach. Die Vorschau
   einer Freigabe, die wegen SR10 nicht wirkt, sagt das (Hinweis `terminal_not_effective`).
5. Lesen eines Rechners (`host.read`): Owner, der Rechner-Inhaber (`hosts.assigned_user_id`) und
   jeder mit einer gültigen Freigabe `terminal` auf ihn, auch wenn SR10 sie sperrt (damit die
   Person den Grund sieht). Sonst `404`.

## 3. Prinzipal-Kontext

`authorize` bekommt nicht nur eine ID, sondern den geprüften Kontext der Anfrage. Die
Authentisierung ([08-identitaet.md](08-identitaet.md)) baut ihn; `authz` vertraut nur ihm.

| Feld | Inhalt | Quelle |
|---|---|---|
| `id`, `kind` | Prinzipal und Art (`user`, `agent`, `app`, `anonymous`, `host`); `host` ist ein Rechner, angemeldet über seinen Anmeldeschlüssel ([24-terminal.md](24-terminal.md), Abschnitt 4.1), ohne Zeile in `principals` | Token bzw. Anmeldung des Rechners |
| `session_id` | `auth_sessions.id` (nur Menschen) | `sid` im Zugriffstoken oder Konsole-Cookie |
| `device_id` | Gerät (nur Menschen) | Token, DPoP-Schlüssel |
| `step_up_at` | letzte frische Bestätigung der Sitzung | `auth_sessions.step_up_at` |
| `token_id`, `token_scopes` | API-Token und seine Scopes (Agenten, Apps, persönliche Tokens) | `api_tokens` |
| `installation_id`, `granted_scopes` | nur Apps | `app_installations` |
| `acting_for` | auslösender Mensch, Lauf, Thread und `jti`, wenn ein Agent im Rahmen eines Laufs handelt | geprüfte Aufruf-Marke in der Kopfzeile der Anfrage (Schutzregel SR9) |
| `view_as` | Mensch, dessen Sicht der Owner gerade ansieht | Sitzung mit aktivem "Als Person ansehen" (Abschnitt 9.2) |
| `request_time` | eine Server-Zeit je Aufruf | Uhr |

Ein persönliches API-Token (Schalter `tokens.personal_enabled`) handelt als sein Mensch, aber ohne
Sitzung. Es kann darum nie Step-up erbringen.

## 4. Die Funktion `authorize`

### 4.1 Entscheidungen und Gründe

`authorize` liefert eine Entscheidung aus 05, Abschnitt 2.5, und bei Ablehnung einen Grund-Code.
Der Code steht im Audit-Log (`audit_events.deny_reason`) und in Fehlerantworten nur, wo er nichts
verrät (bei `404` nie).

| Grund | Entscheidung | Bedeutung |
|---|---|---|
| `unauthenticated` | `unauthenticated` | kein gültiger Prinzipal |
| `principal_inactive` | `unauthenticated` | Prinzipal `disabled` oder `deleted` |
| `principal_pending` | `forbidden` | bereitgestellter Mensch im Zustand `pending` ([08-identitaet.md](08-identitaet.md), Abschnitt 3.1); Problem-Code `device_pending`; erlaubt sind dieselben Aktionen wie für ein wartendes Gerät |
| `session_revoked` | `unauthenticated` | Sitzung widerrufen oder abgelaufen |
| `device_inactive` | `unauthenticated` | Gerät `revoked` oder Schlüssel widerrufen |
| `device_pending` | `forbidden` | Problem-Code `device_pending`; Gerät wartet auf Freigabe; erlaubt sind nur `self.read`, `device.list` für das eigene Gerät, `auth.token.refresh`, `auth.logout` (05, Abschnitt 2.5) |
| `token_revoked` | `unauthenticated` | API-Token widerrufen oder abgelaufen |
| `app_not_installed` | `unauthenticated` | App-Installation nicht `active` |
| `no_grant` | `not_found` | kein Leserecht oder Ressource fehlt |
| `level_insufficient` | `forbidden` | Stufe zu niedrig |
| `capability_missing` | `forbidden` | Zusatzrecht fehlt |
| `scope_missing` | `forbidden` | App ohne Scope |
| `owner_only` | `forbidden` | Aktion nur für Owner |
| `not_self` | `forbidden` bzw. `not_found` | Ressource gehört einem anderen (Gerät, Einstellung); `not_found`, wenn der Prinzipal sie nicht sehen darf |
| `subject_kind_not_allowed` | `forbidden` | Agent oder App als Ausführender oder Empfänger nicht erlaubt |
| `delegation_exceeds_own_level` | `forbidden` | Delegation über die eigene Stufe |
| `delegation_exceeds_own_capabilities` | `forbidden` | Delegation über die eigenen Zusatzrechte |
| `step_up_required` | `step_up_required` | frische Bestätigung fehlt |
| `step_up_not_possible` | `forbidden` | Token ohne Sitzung (Agent, App, persönliches Token) |
| `view_as_read_only` | `forbidden` | schreibende Aktion während "Als Person ansehen" |
| `i1_violation` | `invariant_violation` | Änderung würde I1 verletzen |
| `last_owner` | `invariant_violation` | Änderung ließe keinen aktiven Owner übrig |
| `origin_not_human` | `forbidden` | Auslösung durch Agent, App, Import oder System |
| `agent_not_reachable` | `forbidden` | Agent ist nicht Bediener, nicht erwähnt oder Erwähnung nicht erlaubt |
| `billing_mode_personal_not_holder` | `forbidden` | I2 |
| `tool_class_owner_only` | `forbidden` | SR2 |
| `dev_environment_not_ready` | `forbidden` | SR3 |
| `dev_environment_foreign_thread` | `forbidden` | SR3, Thread läuft in einer fremden Umgebung |
| `project_not_ready` | `forbidden` | Projektbindung nicht bereit (05, Abschnitt 5, Punkt 7) |
| `channel_read_only` | `forbidden` | Kanal schreibgeschützt; schreibende und auslösende Aktionen gesperrt, auch für Owner (05, Regel 1.8); Problem-Code `channel_read_only` |
| `channel_archived` | `forbidden` | Kanal oder sein Bereich archiviert; schreibende und auslösende Aktionen gesperrt, auch für Owner (Abschnitt 4.5); Problem-Code `channel_read_only` mit `reason` |
| `run_active_other_agent` | `forbidden` | Thread hat einen aktiven Lauf eines anderen Agenten (05, Abschnitt 5, Punkt 8) |
| `run_active` | `forbidden` | Modell-Befehl bei aktivem Lauf (05 `command.invoke`) |
| `billing_mode_mismatch` | `invariant_violation` | `run.start` mit Ist-Modus ungleich Soll (I3); Problem-Code `billing_mode_mismatch` (`409`) |
| `tool_class_mismatch` | `invariant_violation` | Adapter meldet eine höhere Werkzeugklasse als die Marke; Problem-Code `tool_class_mismatch` (`409`) |
| `invocation_token_invalid` | `forbidden` | Aufruf-Marke ungültig, abgelaufen oder nicht die aktuelle (`run.start`) |
| `import_not_running` | `forbidden` | `import.apply` ohne laufende Quelle oder mit Ziel außerhalb ihrer Kanäle |
| `malware_detected` | kein `authorize`-Ergebnis | Grund des Audit-Eintrags zu `file.upload` bei einem Malware-Fund ([12-dateien.md](12-dateien.md), Abschnitt 5.2) |
| `component_inactive` | `forbidden` | Komponente benutzt, abgelaufen oder deaktiviert |
| `component_restricted` | `forbidden` | `restricted_to = 'run_starter'` und Prinzipal ist weder Auslöser noch Owner |
| `acting_for_insufficient` | `not_found` bzw. `forbidden` | Auftragsbindung: der auslösende Mensch hat das Recht nicht |
| `terminal_not_host_assignee` | `forbidden` | SR10: Nicht-Owner auf einem Rechner, dessen Inhaber er nicht ist; Problem-Code `terminal_denied` mit `reason: not_host_assignee` |
| `terminal_server_host_owner_only` | `forbidden` | SR10: Server-Host nur für Owner; Problem-Code `terminal_denied` mit `reason: server_host_owner_only` |
| `i6_holder_only` | `forbidden` | SR11: Rechner mit persönlicher LLM-Anmeldung, Prinzipal ist nicht ihr Inhaber; Problem-Code `terminal_denied` mit `reason: i6_holder_only` |
| `i6_violation` | `invariant_violation` | Änderung würde I6 verletzen; Problem-Code `i6_violation` (`409`) |
| `terminal_profile_unknown` | `forbidden` | Profil nicht in `hosts.reported_profiles` oder Index des Arbeitsordners ungültig; Problem-Code `terminal_denied` mit `reason: profile_unknown` |
| `terminal_session_limit` | `forbidden` | `terminal.max_sessions_per_person` oder `terminal.max_sessions_per_host` erreicht; Problem-Code `terminal_denied` mit `reason: session_limit` |
| `terminal_step_up_not_fresh` | `step_up_required` | Step-up älter als `terminal.step_up_max_age`, schon verbraucht oder (Konsole) nicht per Passkey; in der Konsole gilt das schnelle Wiederanhängen ohne Step-up nie |
| `host_not_active` | `forbidden` | Rechner nicht `active` oder Host-Dienst nicht verbunden; Problem-Code `host_unavailable` (`409`) |
| `host_auth_failed` | `unauthenticated` | Anmeldung eines Rechners mit unbekanntem Schlüssel oder falscher Signatur |
| `unknown_action` | Fehler `500` | Programmfehler, Alarm |
| `trusted_header_requires_loopback` | no `authorize` result | Reason of the audit entry of `identity_provider.manage` when enabling the driver `trusted_header` is refused because the public listener is not bound to loopback; problem code `conflict` (`409`) with `reason: trusted_header_requires_loopback` ([08-identitaet.md](08-identitaet.md), section 2.4, PLAN W4) |

### 4.2 Ablauf

```text
authorize(p, action, ref) -> Decision:
  t    := now()                                      # eine Zeit je Aufruf
  spec := catalog[action]
  if spec fehlt:            alarm(); return ERROR(unknown_action)

  # 1. Wer handelt? (Sperren)
  if p.kind == anonymous:
      return spec.right == public ? ALLOW : DENY(unauthenticated)
  d := check_active(p, t)                            # Status, Sitzung, Gerät, Token, Installation
  if d: return d                                     # unauthenticated mit Grund
  if p.status == pending and action nicht in PENDING_ALLOWED:
      return DENY(forbidden, principal_pending)      # bereitgestellt, 05 Abschnitt 2.5
  if p.device_state == pending and action nicht in PENDING_ALLOWED:
      return DENY(forbidden, device_pending)         # 05, Abschnitt 2.5

  # 2. Rechte-Stand (Abschnitt 7)
  snap := snapshot(p, t)                             # Cache je (p, permission_version), gültig bis snap.valid_until

  # 3. Ressource und Weg zum Ziel
  res := resolve(ref)                                # Pfad Thread -> Kanal -> Bereich -> Instanz, DM-Flag, Eigentümer
  if res fehlt: return DENY(not_found, no_grant)

  # 4. Wirksame Rechte auf dem Pfad
  eff := effective(snap, res.path)                   # Owner, DM, Freigaben, Gruppen, Bediener-Regel,
                                                     # höchste Stufe, Vereinigung der Zusatzrechte,
                                                     # Begrenzung für Agenten und Apps
  if p.acting_for:   eff := eff ∩ effective(snapshot(p.acting_for.user, t), res.path)   # SR9
  if p.view_as:      eff := eff ∩ effective(snapshot(p.view_as, t), res.path)            # Abschnitt 9.2

  # 5. Sichtbarkeit: kein Leserecht ergibt 404
  if not visible(spec, p, eff, res): return DENY(not_found, no_grant | acting_for_insufficient)

  # 6. Anforderung aus dem Katalog (Spalte "Recht", "Owner")
  d := requirement(spec, p, eff, res)                # Stufe, Zusatzrecht, Scope, self, owner, participant
  if d: return d                                     # forbidden mit Grund

  # 7. Schreibverbot in "Als Person ansehen"
  if p.view_as and spec.writes: return DENY(forbidden, view_as_read_only)

  # 8. Step-up
  if spec.step_up == ja or (spec.step_up == bedingt and hat_anmeldedaten(p, action)):
      if p.kind != user or p.session_id fehlt: return DENY(forbidden, step_up_not_possible)
      if p.step_up_at fehlt or t - p.step_up_at > auth.step_up_max_age:
          return DENY(step_up_required)
  if spec.step_up == bedingt and (p.kind != user or p.session_id fehlt):
      return DENY(forbidden, step_up_not_possible)   # erste Einrichtung nur in einer Sitzung

  # 9. Schutzregeln der Aktion (Abschnitt 5)
  d := guards[action](p, res, eff, snap, t)           # z. B. agent.invoke: SR1 (I2), SR2, SR3, SR4
  if d: return d

  return ALLOW
```

Danach, nicht in `authorize`, folgen Audit und Rate-Limit-Zählung nach den Spalten in 05.
Rate-Limits prüft eine eigene Schicht vor `authorize` (05, Abschnitt 2.5).

Reihenfolge und Grund:

- Sperren vor allem anderen: Ein gesperrter Prinzipal bekommt nie eine Auskunft über Ressourcen.
- Sichtbarkeit vor Anforderung: Wer nicht lesen darf, bekommt `404`, auch wenn ihm zusätzlich
  eine Stufe fehlt.
- Anforderung vor Step-up: Wer das Recht ohnehin nicht hat, wird nicht zur Bestätigung
  aufgefordert. Sonst würde die Step-up-Aufforderung verraten, dass die Aktion möglich wäre.
- Schutzregeln zuletzt: Sie brauchen den vollen Kontext (Stufe, Lauf, Thread) und gelten auch für
  Owner, wo sie das sagen.

Ausnahme in der Reihenfolge: Ist die Anforderung einer Aktion `All(HumanSession, ...)` (`host.read`,
`host.manage`, ab M2-40 auch `terminal.*`), prüft `authorize` direkt nach Schritt 1 und vor
Rechte-Stand, Auflösung und Sichtbarkeit, ob ein Mensch in einer Sitzung handelt; Agenten, Apps und
persönliche Tokens bekommen `forbidden` mit `subject_kind_not_allowed`. Grund: 05 Abschnitt 4.14
lehnt sie mit diesem Grund ab, und die Sichtbarkeit davor würde daraus `404` machen. Die Antwort
hängt nur vom Prinzipal ab, nie von der Ressource, sie verrät also nichts über einen Rechner
(AZ-P4; docs/DECISIONS.md D-M2-39-4).

Anforderungen, die Fakten einer späteren Etappe brauchen (Läufe, Sitzungen und Befehle der
Agenten, Komponenten, Dateien, Webhooks, Import), stehen bis dahin als zurückgestellte Einträge im
Katalog des Codes: Sie verhalten sich wie eine unbekannte Aktion (`500`, Alarm), und die
Start-Prüfung lehnt jede Route, Op oder jeden Ereignistyp mit einer solchen Aktion ab. Das Ticket,
das die Fakten liefert, ersetzt den Eintrag in derselben Änderung. Grund: Der Katalog ist ab M1
geschlossen (AZ-P3), und eine noch nicht prüfbare Anforderung darf nie erlauben. `bedingt` in der
Spalte "Step-up" (05, Abschnitt 2.4) gilt bis zum Ticket, das die Bedingung liefert, wie `ja`.
`hat_anmeldedaten(p, action)` ist eine Frage an `PrincipalSource`: bei `passkey.register` ein
Passkey der Person mit `revoked_at IS NULL`, bei `local_credential.set` eine Zeile in
`local_credentials` mit `totp_enabled_at IS NOT NULL` (ein eingerichteter Passwortweg). Die Regel
selbst bleibt in `authz` (B7).

Aktionen mit zwei Zielen (`thread.move`: Thread und Zielkanal, Recht "manage auf Quelle und Ziel")
tragen das zweite Ziel im Ressourcen-Verweis (`ref.target`). `resolve` löst beide Pfade auf;
Sichtbarkeit und Anforderung gelten auf beiden. Ohne Leserecht auf das Ziel ist die Antwort `404`,
gleich wie bei einer unbekannten ID. Grund: Eine zweite Prüfung im Anwendungsfall wäre Rechte-Logik
außerhalb von `authz` (05, Regel 1.1).

Audit nach der Spalte "Audit" in 05: `authorize` übergibt jede Ablehnung und jeden Fehler an
`kernel/auditing`; sie werden nach dem Rollback in eigener Transaktion geschrieben und je
`audit.deny_aggregate_window` zusammengefasst (AZ-P13). Ausgenommen sind Filter: die
Empfangsprüfung je Empfänger bei Zustellung und Nachholen (Gateway, `GET /api/v1/events`, Push,
Webhooks) und die Prüfung je Element einer Liste, Suche oder Momentaufnahme (7.4, Regel 3). Sie
laufen im Filtermodus (`authorize.Filter(ctx)`, 02 Abschnitt 4.5) und schreiben keine Ablehnung.
Grund: Sonst schriebe jedes Ereignis für jeden Nicht-Empfänger eine Zeile; geprüft werden
Anfragen und Ops. Erlaubnisse einer Aktion mit Audit `ja`
schreibt ihr Anwendungsfall in seiner Transaktion mit seinen Details, weil nur er die Details
kennt; ein Test des Anwendungsfalls findet genau einen Eintrag.

### 4.3 Sichtbarkeit

`visible(spec, p, eff, res)` ist wahr, wenn eines gilt:

- Ressource ist `instance` und `p` ist angemeldet (`auth`), oder die Aktion ist `public`.
- `eff.level >= read` auf dem Pfad der Ressource.
- Ressource ist ein Bereich und `p` hat `read` auf mindestens einem seiner Kanäle (05, `space.read`).
- Ressource ist ein Agent und `p` ist Owner oder hat `read` auf einem Kanal, den der Agent bedient
  (05, `agent.read`).
- Ressource gehört `p` selbst (eigenes Gerät, eigene Sitzung, eigene Einstellung, eigener
  DSGVO-Auftrag), oder `p` ist Owner und die Aktion erlaubt `owner`.
- DM-Kanal oder Inhalt darin: `p` ist Teilnehmer.
- Ressource ist ein Rechner und 2.12, Punkt 5 gilt; bei einer Terminal-Sitzung: `p` ist ihr Mensch
  oder Owner.

### 4.4 Invarianten in der Transaktion

`authorize` prüft, ob jemand eine Änderung versuchen darf. Ob das Ergebnis erlaubt ist, prüft
`authz.VerifyInvariants(tx, change)` in derselben Transaktion nach der Änderung und vor dem Commit:

1. Sperrreihenfolge nach 03, Abschnitt 6.3 (`permission_version` zuerst).
2. Änderung schreiben.
3. Betroffene Agenten und Owner bestimmen (03, Abschnitt 6.4).
4. I1 (SR1) und SR8 (letzter Owner) berechnen.
5. Verletzt: Rollback, Antwort `409 invariant_violation` mit Code und Hinweis, Audit-Eintrag mit
   `decision = 'deny'` in eigener Transaktion.
6. Sonst: `access.changed` je betroffenem Prinzipal, Audit, Commit.

### 4.5 Archivierte Bereiche und Kanäle

`space.archive` und `channel.archive` sind endgültig; 05 hat keine Gegenaktion. Die Daten bleiben
für Aufbewahrung und Export.

1. In einem archivierten Kanal und in jedem Kanal eines archivierten Bereichs ist die wirksame
   Stufe jedes Prinzipals, Owner eingeschlossen, höchstens `read`. Aktionen, deren Recht `write`
   oder `trigger` auf ein Ziel darin braucht, enden mit `forbidden` und Grund `channel_archived`
   (HTTP `403 channel_read_only` mit `reason`). Lesen bleibt für alle, die vorher lesen durften.
2. Verwaltende Änderungen an einem archivierten Bereich oder Kanal (Umbenennen, Verschieben,
   Bediener, Tags) passieren `authorize`, aber `chat` lehnt sie mit `409 conflict` ab.
3. Archivieren ist eine Rechte-Änderung (Spalte "Rechte" in 05): Rechte-Version erhöhen und
   `access.changed` mit einem Eintrag in `changed` (Stufe `read`) für jeden, dessen Stufe darüber
   lag.
4. Die Listen der Seitenleiste (`listSpaces`, `listSpaceChannels`) zeigen Archiviertes nicht;
   Einzelabruf, Verlauf und Suche liefern es weiter an Berechtigte.
5. I1 betrachtet nur nicht archivierte Ziele ([10-backend-richtlinie.md](10-backend-richtlinie.md),
   Abschnitt 5.3), weil dort niemand `trigger` hat.

Grund: Archivieren soll einen Verlauf einfrieren, nicht verstecken; Lesen ohne Schreiben ist das
Verhalten, das Menschen von archivierten Kanälen kennen, und ein Owner verliert dabei nichts.

### 4.6 Erwähnungen auflösen

Ein Erwähnungs-Token im Inhalt ([03-datenmodell.md](03-datenmodell.md), `messages`,
Inhaltsformat) wird beim Senden und Bearbeiten nur dann zur Erwähnung, wenn gilt:

| Erwähnter | Bedingung |
|---|---|
| Mensch | `authorize(autor, user.read, mensch)` erlaubt, oder der Mensch darf den Thread der Nachricht lesen (`authorize(mensch, thread.read, thread)`) |
| Agent | `authorize(autor, agent.read, agent)` erlaubt |

Sonst bleibt das Token Text. Grund für die zweite Bedingung bei Menschen: Agenten haben keinen
Scope `users:read` und sehen das Verzeichnis nicht (05, Abschnitt 2.6); ihre Erwähnung soll trotzdem
den Menschen erreichen, der den Thread liest ([13-push-benachrichtigungen.md](13-push-benachrichtigungen.md),
Abschnitt 2.1). Der Autor erfährt so nur, ob ein Handle den Thread lesen darf, in den er selbst
schreibt; das Verzeichnis bleibt verborgen. Ein Mensch ohne Leserecht auf den Kanal, den ein Mensch
erwähnt, bekommt eine Zeile in `mentions`, aber nie einen Hinweis (8.1, Zeile Push).

## 5. Schutzregeln, die keine Freigabe aufhebt

Jede Schutzregel hat eine Nummer, einen Prüfort und einen Test (Abschnitt 11).

### SR1 Backend-Richtlinie (I1, I2)

- **I1, transaktional.** Für jeden Agenten mit `billing_mode = 'personal'` ist die Menge der
  Prinzipale mit wirksamem `trigger` auf irgendeinem Ziel, das der Agent bedient, genau
  `{credential_holder_id}` (Baseline Abschnitt 5; Prüfung 03, Abschnitt 6.4). Geprüft in der
  Transaktion jeder Aktion aus 05, Regel 1.6.
- **Folgerung.** Owner haben überall `trigger`. Darum geht `personal` nur, wenn der
  `credential_holder` der einzige aktive Owner ist und niemand sonst `trigger` auf den bedienten
  Zielen hat. Wird ein zweiter Owner angelegt, scheitert `owner.add` mit `i1_violation`, solange ein
  Agent auf `personal` steht. Unter dieser Folgerung sind "genau" und "höchstens" dieselbe Bedingung,
  weil der einzige Owner immer selbst `trigger` hat.
- **I2, vor jedem Aufruf.** `agent.invoke` prüft bei Annahme und unmittelbar vor jeder Zustellung:
  bei `personal` ist der Auslöser der `credential_holder`, sonst `billing_mode_personal_not_holder`,
  fail closed, Audit (05, Abschnitt 5, Punkt 4; 04, Abschnitt 15.3).

### SR2 Werkzeugklasse

Ein Agent mit `tool_class = 'host'` darf nur ausgelöst werden von:

- einem aktiven Owner, oder
- einem Menschen, für den SR3 eine bereite, eigene Dev-Umgebung nachweist, in der der Lauf
  ausgeführt wird.

Sonst `tool_class_owner_only`. Geprüft in `agent.invoke` bei Annahme und vor jeder Zustellung.
Eine Freigabe mit `trigger` auf einen Kanal mit `host`-Bediener wird nicht abgelehnt; die Vorschau
(Abschnitt 9.3) sagt in Klartext, dass sie keine Auslösung ermöglicht. Grund für die Prüfung beim
Aufruf statt beim Anlegen: Bereitschaft einer Umgebung ändert sich ohne Rechte-Transaktion, und
Gruppenmitgliedschaften ändern sich häufig. Der Aufruf ist der einzige Ort, an dem alle Fakten
aktuell sind.

### SR3 Dev-Abschottung

Für Kanäle mit Projektbindung (`project_bindings`) gilt zusätzlich zu 05, Abschnitt 5, Punkt 6:

1. Ein Lauf eines Nicht-Owners läuft nur in einer Dev-Umgebung dieses Menschen, die bereit ist
   (`dev_environments`, [21-dev-isolation.md](21-dev-isolation.md), Abschnitt 6). Sonst
   `dev_environment_not_ready`.
2. Die Agenten-Sitzung eines Threads ist an die Umgebung gebunden, in der ihr erster Lauf lief
   (`agent_sessions.dev_environment_id`, 03). Löst ein anderer Mensch in diesem Thread aus,
   ist das `dev_environment_foreign_thread`. Grund: Eine Sitzung liest und schreibt die Dateien
   ihrer Umgebung; ein fremder Auslöser würde in der Umgebung einer anderen Person arbeiten.
3. Owner-Läufe laufen in der Umgebung des Owners. Das ist der heutige Ein-Personen-Betrieb.
4. Die Aufruf-Marke trägt `dev_environment_id`; der Runner startet nur in genau dieser Umgebung.

Solange es keine bereite Umgebung gibt, ist jedes Dev-Projekt damit nur für Owner auslösbar, egal
welche Freigaben bestehen.

### SR4 Nur Menschen lösen aus

- Nur Nachrichten mit `origin = 'human'` lösen Agenten aus; Agenten, Apps, Importe, Systemnachrichten
  und Eingangs-Webhooks nie (05, Abschnitt 5, Punkt 1).
- Agenten und Apps haben höchstens `write` und als Zusatzrecht nur `upload` (Abschnitt 2.1).
- Grund: Bot-Schleifen sind ausgeschlossen, und die Trigger-Menge aus I1 bleibt menschlich.

### SR5 Verwaltung nur durch Menschen mit Bestätigung

- Step-up-Aktionen nur mit Sitzung eines Menschen und frischer Bestätigung (R-AZ-03).
- Owner-Aktionen nur durch aktive Owner, auch wenn eine Freigabe `manage` auf der Instanz besteht.
- Owner-Status wirkt nur in einer Sitzung. Ein persönliches API-Token eines Owners handelt mit den
  Freigaben dieses Menschen, ohne Owner-Rechte. Grund: Ein gestohlenes Token ohne Gerätebindung darf
  nie die ganze Instanz öffnen ([08-identitaet.md](08-identitaet.md), Abschnitt 12; 05,
  Abschnitt 2.3; R-AZ-14).

### SR6 Direktnachrichten nur für Teilnehmer

Kein Owner-Status, keine Freigabe und kein "Als Person ansehen" öffnet einen DM-Kanal (Ruling
L21). Der einzige Weg im Produkt ist der DSGVO-Export einer Person: `gdpr.request.create` für eine
andere Person, Freigabe und Download, jeweils durch einen Owner mit Step-up und Audit; das Paket
enthält nur die eigenen Nachrichten dieser Person ([20-datenschutz.md](20-datenschutz.md),
Abschnitt 5.2). Weitere Wege, etwa eine behördliche Anordnung, liegen außerhalb des Produkts
(R-AZ-02).

### SR7 Gesperrt heißt ohne Rechte

Ein Prinzipal mit Status `disabled` oder `deleted`, eine widerrufene Sitzung, ein widerrufenes
Gerät, ein widerrufenes Token oder eine nicht aktive App-Installation ergibt `unauthenticated`,
bevor Freigaben gelesen werden. Ein Mensch im Zustand `pending` und ein wartendes Gerät ergeben
`forbidden` außer für die Selbstauskunft (4.1). Freigaben gesperrter oder wartender Prinzipale
bleiben stehen, wirken aber nicht.

### SR8 Letzter Owner

Keine Transaktion darf den letzten aktiven Owner entfernen: weder `owner.remove` noch `user.disable`
auf einen Owner, wenn danach kein Owner mit `principals.status = 'active'` bleibt. Code
`last_owner` (03 `owners`).

### SR9 Auftragsbindung (Confused Deputy)

Ein Agent, der im Rahmen eines Laufs die API benutzt, legt die geprüfte Aufruf-Marke des Laufs bei
(Kopfzeile, Name legt [06-api.md](06-api.md) fest). Dann gilt für jede Aktion, die Inhalte liest oder
in einen Kanal schreibt:

```text
eff(agent im Lauf, Ziel) = eff(agent, Ziel) ∩ eff(auslösender Mensch, Ziel)
```

`∩` heißt: niedrigere Stufe, Schnittmenge der Zusatzrechte. Ausnahmen, für die nur die Rechte des
Agenten gelten: Antworten in den Thread des Laufs, Patches am eigenen Lauf, `run.finish`,
`interaction.respond`, `agent_session.report`. Die Marke wird bei jeder Anfrage geprüft (Signatur,
`aud`, Lauf aktiv); die Rechte des Menschen werden aktuell berechnet, nicht aus der Marke gelesen.
Ein Entzug wirkt also sofort auch auf Werkzeugaufrufe eines laufenden Agenten.

Grund: Ohne diese Regel kann ein Mensch mit `trigger` auf Kanal A einen Agenten, der zusätzlich
Kanal B lesen darf, bitten, B zusammenzufassen. Der Agent ist dann ein "confused deputy". Die
SDKs ([09-agenten.md](09-agenten.md)) binden die Marke während eines Laufs automatisch an jeden
API-Aufruf. Was die Laufzeit außerhalb der Server-API weiß (eigener Speicher, frühere Sitzungen),
deckt die Regel nicht; dafür gibt es den Lesehorizont-Hinweis (Abschnitt 6) und die Bedrohung
T-AI-02 im Bedrohungsmodell.

### SR10 Terminal nur auf eigenem Rechner

Ein Mensch, der nicht aktiver Owner ist, bekommt `terminal.open` und `terminal.attach` nur auf einem
Rechner, der

- ihm zugewiesen ist (`hosts.assigned_user_id = P`), oder
- seine eigene bereite Dev-Umgebung ist (`hosts.dev_environment_id` mit `dev_environments.user_id =
  P` und Bereitschaft nach [21-dev-isolation.md](21-dev-isolation.md), Abschnitt 6.2; Etappe
  "Mehrbenutzer-Dev-Umgebung").

Sonst `terminal_not_host_assignee`. Auf einem Server-Host (`hosts.declared_server_host` oder
`hosts.reported_is_server_host`) nur Owner (`terminal_server_host_owner_only`); der Host-Dienst
prüft das zusätzlich am Claim `own` der Sitzungs-Marke. Geprüft bei jedem Öffnen und Anhängen und
bei jeder Änderung von `assigned_user_id`, `dev_environment_id` oder des Owner-Status: Fällt die
Bedingung weg, enden die Sitzungen (`access_revoked`). Grund: [21-dev-isolation.md](21-dev-isolation.md)
und ADR 0046 verwerfen den OS-Benutzer als alleinige Grenze zwischen Personen; ein gemeinsames
Home teilt Geheimnisse und LLM-Anmeldungen (Ruling T1).

### SR11 Terminal und LLM-Anmeldung (I6)

Ist `hosts.llm_credentials = 'personal'`, erlauben `terminal.open` und `terminal.attach` nur
`hosts.credential_holder_id`, auch keinem anderen Owner (`i6_holder_only`). Freigaben auf solche
Rechner an andere Subjekte als den Inhaber, auch an Gruppen, lehnt die Transaktion ab
(`i6_violation`, 03 Abschnitt 6.8). Abgrenzung zu SR1: I1 hat keine Owner-Ausnahme, I6 hat eine,
weil ein Terminal Zugriff auf einen bestimmten Rechner ist und keine vererbte Stufe
([24-terminal.md](24-terminal.md), Abschnitt 7.3; ADR 0068).

## 6. Lesehorizont von Agenten

Der Lesehorizont `H(A)` eines Agenten ist die Menge der Kanäle, die A lesen darf (Freigaben plus
Bediener-Regel). Die Trigger-Menge `T(A)` sind alle Menschen mit wirksamem `trigger` auf einem von
A bedienten Ziel, für die keine Schutzregel die Auslösung sperrt.

Hinweis H1: Für jeden Menschen `P` in `T(A)` und jeden Kanal `K` in `H(A)`, den `P` nicht lesen darf,
gilt: `P` kann über A Inhalte aus `K` erfahren, wenn die Laufzeit von A Wissen über Läufe hinweg
behält. SR9 verhindert das nur für Lesezugriffe über die Server-API.

H1 ist ein Hinweis, keine Sperre. Er erscheint:

- in der Vorschau jeder Rechte-Änderung (Abschnitt 9.3),
- in "Wer hat Zugriff?" als Herkunft "indirekt über Agent A" (Abschnitt 9.1),
- in der Liste "Rechte-Hinweise" der Admin-Konsole.

Grund für Hinweis statt Sperre: Eine harte Regel müsste auch bei `channel.create` greifen (ein neuer
Kanal in einem Bereich mit Agenten-Freigabe vergrößert `H(A)`) und würde Anlegen von Kanälen
unerwartet scheitern lassen. Ob eine Laufzeit Wissen behält, weiß nur der Betreiber. Die
empfohlene Konfiguration ist ein Agent je Publikum: Die Menschen in `T(A)` dürfen mindestens alles
lesen, was A liest. Dann ist `H1` leer.

## 7. Filterung in SQL: Rechte-Cache mit Versionszähler

### 7.1 Entscheidung

Listen, Zähler, Suche, Vorschläge und Nachholen filtern in der Datenbankabfrage mit der Menge der
sichtbaren Kanäle aus einem Rechte-Cache in `authz`. Es gibt keine materialisierte Sicht wirksamer
Rechte in der Datenbank (R-DM-19). Abgelehnte Alternative und Gründe stehen in ADR 0018.

Kurz: Eine materialisierte Sicht wäre eine zweite Wahrheit, die Ablaufzeiten trotzdem in jeder
Abfrage prüfen müsste (`valid_until > now()`), bei jeder Gruppenänderung viele Zeilen schreiben
würde und bei einem Fehler in der Pflege still falsche Ergebnisse liefert. Ein Cache, der aus
derselben Funktion wie `authorize` entsteht, kann nicht abweichen, ohne dass der Differenztest
(AZ-P8) es findet.

### 7.2 Rechte-Stand (Snapshot)

```text
RightsSnapshot {
  principal_id, permission_version, computed_at, valid_until
  owner: bool
  instance_level, instance_caps                  # aus Freigaben auf der Instanz
  level_by_space:   map[space_id]level           # aus Freigaben auf Bereichen
  caps_by_space:    map[space_id]set
  level_by_channel: map[channel_id]level         # aus Freigaben auf Kanälen; DM-Kanäle der Teilnahme
  caps_by_channel:  map[channel_id]set
  instance_wide: bool                            # Owner oder read auf der Instanz
  fully_visible_space_ids: sortierte Liste       # Bereiche mit read auf dem Bereich
  visible_channel_ids:     sortierte Liste       # Kanäle mit read auf dem Kanal, DM der Teilnahme
  visible_space_ids:       sortierte Liste       # Bereiche mit mindestens einem sichtbaren Kanal
  provenance (nur für access.explain, nicht im Cache)
}
```

Aufbau:

1. Gültige Freigaben des Prinzipals, bei Menschen zusätzlich die seiner gültigen Gruppen, alle mit
   der Zeit `t` aus Abschnitt 2.5, je Ziel (Instanz, Bereich, Kanal): höchste Stufe, Vereinigung
   der Zusatzrechte.
2. Owner: `trigger` und alle Zusatzrechte auf der Instanz (gilt nie für DM-Kanäle).
3. DM-Kanäle der Teilnahme mit `write` und `upload`.
4. `valid_until` = kleinstes `expires_at` aller benutzten Freigaben und Mitgliedschaften (oder
   unendlich).

Der Stand speichert Stufen je Ziel, nicht je Kanal ausgebreitet. Die wirksame Stufe auf einer
Ressource bildet `authorize` erst beim Aufruf aus dem Pfad, den `resolve` liefert (Kanal, Bereich,
Instanz): höchste Stufe entlang des Pfads, dazu die Bediener-Regel aus dem aufgelösten wirksamen
Bediener (Abschnitt 2.9), die Begrenzung für Agenten und Apps (2.1) und die Grenze archivierter
Ziele (4.5). Ebenso bildet `Scope` die Listen für Abfragen je Anfrage (7.4). Grund: Ein neuer Kanal
oder Bereich ändert keine Rechte-Version (05, `channel.create` und `space.create` haben "Rechte"
`nein`); mit ausgebreiteten Kanallisten im Cache wäre er bis zur nächsten Rechte-Änderung für jeden
mit Freigabe auf Bereich oder Instanz unsichtbar. Das Nachholen nutzt dieselben Namen direkt in
seiner Abfrage, weil `event_log` die Spalte `space_id` trägt (04, Abschnitt 7.2).

Grenze: Der Rechte-Stand kennt Ziele, keine Threads. Die Bediener-Regel auf Thread-Ebene
(Abschnitt 2.9, Punkt 2: `threads.operator_agent_id`) gilt darum bei `authorize` für den einzelnen
Thread, aber nicht in Listenfiltern (7.4). Ein Agent, der einen Thread in einem
sonst nicht lesbaren Kanal bedient, sieht ihn in Listen nicht. Das schließt zu wenig ein und
verrät nichts. Wie Agenten Threads lesen, entscheidet die Etappe M2 (Agenten-Plattform).

### 7.3 Gültigkeit des Caches

Ein Cache-Eintrag gilt, solange beides zutrifft:

- `permission_version` des Eintrags gleich der aktuellen Rechte-Version und
- `request_time < valid_until`.

Die aktuelle Rechte-Version liest jede REST-Anfrage in ihrer Lese-Transaktion
(`SELECT value FROM sequence_counters WHERE name = 'permission_version'`, eine Zeile über den
Primärschlüssel). Gateway-Prozesse benutzen die Version aus dem Ereignis-Log in Log-Reihenfolge
(04, Abschnitt 8.1). Damit sieht jede Anfrage nach dem Commit einer Rechte-Änderung den neuen
Stand, auch in einem anderen Prozess. Ein Lesen ohne aktuelle Version gibt es nicht.

Der Versionszähler ist global. Jede Rechte-Änderung macht alle Einträge ungültig; neu berechnet
wird nur, wer danach eine Anfrage stellt. Rechte-Änderungen sind selten, Anfragen häufig; ein Zähler
je Prinzipal würde den Fall "Gruppe mit vielen Mitgliedern" teuer machen und ist darum nicht nötig.

### 7.4 Abfrage-Muster

```sql
-- Nachrichtensuche, $1 = sichtbare Kanäle aus Scope (je Anfrage gebildet, 7.2),
-- $2 = Suchausdruck, gebaut vom Modul Suche (14-suche.md)
SELECT m.id, m.thread_id, m.channel_id, ts_rank(m.search_vector, $2::tsquery) AS rank
  FROM messages m
 WHERE m.channel_id = ANY($1::uuid[])
   AND m.deleted_at IS NULL
   AND m.search_vector @@ $2::tsquery
 ORDER BY rank DESC, m.id DESC
 LIMIT $3;

-- Ungelesen-Zähler je Kanal, $1 = angefragte Kanäle, geschnitten mit Scope
SELECT t.channel_id,
       sum(t.last_message_seq - greatest(coalesce(r.last_read_seq, 0), t.unread_floor_seq)) AS unread
  FROM threads t
  LEFT JOIN read_states r ON r.thread_id = t.id AND r.user_id = $2
 WHERE t.channel_id = ANY($1::uuid[])
 GROUP BY t.channel_id;
```

Regeln:

1. Jede Abfrage über Inhalte, Threads, Kanäle, Bereiche, Läufe, Dateien oder Erwähnungen hat das
   Prädikat `channel_id = ANY($visible)` (bzw. `space_id = ANY($visible_spaces)`). Die Listen
   bildet `Authorizer.Scope` je Anfrage aus dem Rechte-Stand und der aktuellen Struktur
   (`StructureSource`): Kanäle mit eigener Freigabe und DM-Kanäle der Teilnahme, alle Kanäle der
   Bereiche in `fully_visible_space_ids`, bei `instance_wide` alle Kanäle außer DM, bei Agenten die
   nach der Bediener-Regel bedienten. Neue Kanäle und Bereiche erscheinen so ohne neue
   Rechte-Version. Ein Lint im Repository-Layer lehnt lesende sqlc-Abfragen auf diesen Tabellen
   ohne dieses Prädikat ab, außer sie stehen auf einer geprüften Ausnahmeliste (Auflösung für
   `authorize`, Laden einer schon geprüften Ressource, Jobs mit Systemrecht wie die Aufbewahrung,
   Log-Tail). Schreibende Abfragen treffen nur Zeilen einer Ressource, die `authorize` vorher
   geprüft hat (z. B. die Kopien von `channel_id` beim Verschieben); der Lint prüft sie nicht.
2. Gesamtzahlen (`total`), Facetten und "weitere Treffer" zählen nur innerhalb dieses Prädikats.
3. Nach dem SQL-Filter läuft für jedes Ergebnis zusätzlich `authorize` mit der Lese-Aktion (z. B.
   `message.read`). Der SQL-Filter ist die Leistung, `authorize` die Wahrheit. Ein Unterschied
   zwischen beiden verwirft das Element, schreibt ein Log ohne IDs und Inhalte und erhöht
   `bothy.authz.errors` mit `bothy.reason = filter_mismatch`; daran hängt der Alarm
   `BothyAuthzError` ([signals.md](../observability/signals.md)).
4. Vorschläge (Erwähnungen, Befehle, Kanal-Schnellwechsel) folgen denselben Regeln: ein Mensch, den
   der Fragende nicht sehen darf, erscheint nicht, und die Anzahl verrät nichts.
5. Große Instanzen: Wird die Liste so lang, dass der Planer kippt, ist die Abhilfe eine temporäre
   Tabelle aus demselben Snapshot in derselben Transaktion, nie eine dauerhafte Rechte-Tabelle.
   Der Lasttest `perf/catchup` (`perf.catchup_page_p99`, [02-architektur.md](02-architektur.md),
   Abschnitt 17) läuft auch mit einer großen Menge sichtbarer Kanäle und hält die Grenze fest.
6. Die Beispiele oben zeigen das Ergebnis, nicht den Ort der Abfrage: Liegen die Tabellen in
   verschiedenen Modulen (hier `threads` in `chat`, `read_states` in `sync`), setzt der Code die
   Teile über Ports zusammen (02, Abschnitt 4.1); kein Modul liest fremde Tabellen.

## 8. Zustellfilter je Empfänger und sofortige Wirkung

### 8.1 Zustellfilter

| Weg | Prüfung je Empfänger | Quelle |
|---|---|---|
| Dauerhafte Ereignisse (Gateway, `GET /api/v1/events`) | `authorize(empfänger, receive_action, resource)`; persönliche Ereignisse nur an `audience_principal_id` | 05, Abschnitt 6; 04, Abschnitt 7 |
| Kurzlebige Ereignisse (Tippen, `run.updated`, Präsenz) | Empfangs-Aktion und Fokus | 05, Abschnitt 6; 04, Abschnitte 11, 12 |
| Agenten-Verbraucher | wie Menschen nach eigenen Freigaben, plus persönliche Ereignisse an den Agenten; vor `agent.invocation`, `agent.followup`, `interaction.created`, `command.invoked` zusätzlich `agent.invoke` für den Auslöser | 04, Abschnitt 15 |
| Push | `push.deliver` (Leserecht auf den Thread), Benachrichtigungs-Einstellung, Ruhezeiten, nicht der Autor; Erwähnungen ohne Leserecht erzeugen nichts | 05; [13-push-benachrichtigungen.md](13-push-benachrichtigungen.md) |
| Ausgehende Webhooks | `webhook_out.deliver`: Empfangs-Aktion des Ereignisses für die App nach Freigaben und Scope, Abo des Typs | 05 |
| Datei-Abruf | `file.read` bei jedem Abruf über den Server, auch mit Abruf-Ticket; keine dauerhaften Links an Clients | [12-dateien.md](12-dateien.md), [ADR 0048](../adr/0048-datei-abruf-nur-ueber-den-server-mit-kurzlebigen-tickets.md) |
| Suche, Zähler, Listen | SQL-Filter plus `authorize` je Element (Abschnitt 7.4) | dieses Kapitel |

### 8.2 Sofortige Wirkung bei Entzug

Ein Entzug (Freigabe widerrufen, Mitgliedschaft entfernt, Owner entfernt, Prinzipal gesperrt,
Gerät oder Sitzung widerrufen, Bediener geändert) wirkt in dieser Reihenfolge:

1. **Commit.** Die Transaktion erhöht `permission_version` als ersten Schritt (03, Abschnitt 6.3)
   und schreibt `access.changed` je betroffenem Prinzipal sowie den Audit-Eintrag.
2. **REST.** Jede Anfrage nach dem Commit liest die neue Version und berechnet den Snapshot neu
   (Abschnitt 7.3). Es gibt kein Zeitfenster mit altem Stand.
3. **Offene Verbindungen.** Jeder Gateway-Prozess bewertet die Verbindungen des Betroffenen neu,
   wenn er im Log an `access.changed` ankommt: Fokus auf entfernte Ziele endet, gepufferte
   Ereignisse dieser Ziele werden verworfen, das Ereignis geht sofort hinaus (04, Abschnitt 8.3).
   Bei Sitzungs-, Geräte- oder Prinzipal-Widerruf schließt er mit `4012` (04, Abschnitt 8.5).
4. **Laufende Läufe.** Verliert ein Mensch `trigger` auf dem Thread eines von ihm ausgelösten
   aktiven Laufs, schreibt dieselbe Transaktion `run.cancel_requested` mit Grund `access_revoked`
   (R-SY-11). Nach `agent.cancel_grace` setzt der Server den Lauf selbst auf `cancelled`. Werkzeug-
   aufrufe des Agenten scheitern schon vorher an SR9.
5. **Aufrufe in der Warteschlange.** I2 vor jeder Zustellung lässt den Lauf `failed` werden
   (04, Abschnitt 15.3).
6. **Komponenten.** Eine Interaktion prüft `trigger` beim Klick; offene Karten werden nicht
   umgeschrieben.
7. **Terminal-Sitzungen.** Verliert ein Mensch das wirksame `terminal` auf einem Rechner oder
   greift SR10 oder SR11, beendet dieselbe Transaktion seine Sitzungen dort (`access_revoked` bzw.
   `i6_violation`); der Server schließt die Verbindungen und weist den Host-Dienst an, die PTY zu
   beenden. Sperre der Person, Widerruf von Gerät oder Anmeldesitzung und Abmeldung beenden die
   Sitzungen, die daran hängen ([24-terminal.md](24-terminal.md), Abschnitt 8.3).
8. **Tokens.** Zugriffstokens bleiben bis zu ihrem Ablauf formal gültig, aber `authorize` prüft
   Sitzung und Rechte bei jeder Anfrage. Aufruf-Marken laufen nach `agent.invocation_token_ttl` ab
   und werden bei `run.start` erneut geprüft.

### 8.3 Ablauf durch Zeit

Ein Ablauf ist kein Schreibvorgang. Die Entscheidung ist trotzdem sofort richtig, weil jeder Aufruf
die Zeit prüft und `valid_until` den Cache ungültig macht. Damit Clients ihren Cache räumen und Läufe
enden, läuft der Job `authz.expiry_sweep` alle `authz.expiry_sweep_interval`. Er findet Freigaben und
Mitgliedschaften mit `expires_at` bis jetzt, deren Ablauf noch nicht gemeldet ist
(`expiry_reported_at IS NULL`, 03), höchstens `authz.expiry_sweep_batch_max` Zeilen je Transaktion,
und schreibt je Stapel in einer Transaktion: Rechte-Version erhöhen, `expiry_reported_at` setzen,
`access.changed`, `grant.changed` mit `change = expired` bzw. `group.changed` mit
`change = member_expired`, `run.cancel_requested` für betroffene Läufe, Audit-Eintrag mit
Systemaktion `grant.expire` bzw. `group.member.expire` (05, R-AZ-15). Weil die Auswahl am
gespeicherten Merker hängt und nicht an der Zeit des letzten Laufs, wird jeder Ablauf genau einmal
gemeldet, auch nach einem Ausfall des worker. Die größte Verzögerung beim Räumen des Client-Caches
ist `authz.expiry_sweep_interval`; die Zustellung neuer Inhalte endet sofort.

## 9. Anforderungen an die Admin-Oberfläche

### 9.1 "Wer hat Zugriff?" (`access.explain`)

Für einen Bereich oder Kanal liefert der Server je Prinzipal: wirksame Stufe, Zusatzrechte und die
Herkunft als Kette. Herkunftsarten:

| Herkunft | Beispiel in Klartext |
|---|---|
| direkt | "Freigabe für Alex auf Bereich Support, bis 31.12., Grund: Vertretung" |
| Gruppe | "über Gruppe Support-Team (Mitglied bis 30.11.)" |
| geerbt | "vom Bereich Support geerbt" |
| Owner | "Owner" |
| Bediener | "Bediener dieses Kanals" (Agent) |
| DM | "Teilnehmer" |
| indirekt über Agent | "kann über Agent Lotse Inhalte dieses Kanals erfahren" (Hinweis H1) |
| gesperrt durch Schutzregel | "trigger, aber Auslösen gesperrt: Dev-Umgebung nicht bereit (SR3)" |

In der API heißen die Herkunftsarten `owner`, `direct`, `group`, `inherited`, `operator`,
`dm_participant`, `via_agent` und `guard_blocked` (Schema `AccessSource`, mit Agent, Regel,
Ablauf und Grund der Freigabe); ein Eintrag nur aus `via_agent` hat die Stufe `none`.
Gesperrte Prinzipale erscheinen durchgestrichen (`inactive`) mit der Stufe, die ihre Freigaben
gäben. Die Ansicht braucht `owner` oder `manage` (05).

### 9.2 "Als Person ansehen" (`access.view_as`)

- Nur Owner, mit Step-up, Audit beim Start und Ende.
- Der Server setzt `view_as` im Prinzipal-Kontext für höchstens `authz.view_as_max_duration`.
- Wirksame Rechte sind die Schnittmenge aus Owner und Person. DM-Kanäle der Person bleiben
  unsichtbar (SR6). Alle schreibenden Aktionen enden mit `view_as_read_only`; nach Ablauf endet
  jede Entscheidung mit `step_up_required`.
- Die Konsole zeigt durchgehend einen Hinweisbalken mit Name und Restzeit.
- Zweck: prüfen, was eine Person sieht und auslösen kann, ohne ihre Zugangsdaten zu kennen.
- In M1 ist "Als Person ansehen" eine Momentaufnahme: `viewAsUser` liefert die Bereiche und
  Kanäle, die die Person sieht, mit Stufe und `expires_at`; `view_as` lebt nur innerhalb dieser
  Anfrage. Das Durchblättern von Inhalten als andere Person in der Konsole braucht einen Zustand an
  der Konsolen-Sitzung (Ziel-Person, Beginn) und Operationen zum Starten und Beenden; beides
  ergänzt die Konsolen-Sitzung (PLAN M6-01) im Kern-Vertrag, bevor sie gebaut wird.

### 9.3 Vorschau in Klartext

Jede Aktion mit Spalte "Rechte" = `ja` hat eine Vorschau ohne Wirkung: die Aktion
`access.preview` (05; Recht wie die Zielaktion, ohne Step-up, ohne Audit, `rl.admin`). Für
Freigaben ist der Weg `POST /api/v1/grants/preview`; Vorschauen weiterer Aktionen kommen mit
ihren Routen. Der Server führt
die Änderung in einer Transaktion aus, berechnet Unterschiede und Invarianten und rollt zurück. Die
Antwort enthält:

- je betroffenem Prinzipal: hinzugekommene, entfallene und geänderte Stufen und Zusatzrechte,
- das Ergebnis von I1 und SR8 (würde die Änderung abgelehnt, mit Hinweis "zuerst auf
  organization umstellen"),
- Hinweise: SR2 und SR3 sperren die Auslösung, SR10 sperrt ein Terminal
  (`terminal_not_effective`), H1 Lesehorizont, Freigabe an eine Gruppe erreicht
  n Menschen, Ablaufdatum fehlt bei Personen außerhalb von Gruppen (API: `hints`),
- einen Text aus Vorlagen (kein Modell erzeugt ihn), in der Sprache des Aufrufers (Hauptteil von
  `users.locale`, `de` oder `en`, sonst `de`, wie 01 "Sprachen"; API: `text`, `text_locale`).
  Daten stehen in der Zeitzone des Aufrufers (`users.time_zone`). Die Vorlagen liegen als Katalog
  je Sprache im Code von `authz`; ein Golden-Test hält das Beispiel unten Byte für Byte.

Recht der Zielaktion heißt bei Freigaben: Leserecht auf das Ziel (sonst `404`) und `owner` oder
`manage` darauf (sonst `403`). Die Grenzen der Delegation (2.6: eigene Stufe, eigene Zusatzrechte,
Agenten und Apps nur durch Owner, Instanz nur durch Owner) lassen die Vorschau gelingen: `200` mit
`allowed = false` und `denial_reason` mit dem Grund aus 4.1. Grund: Die Vorschau soll zeigen,
warum eine Freigabe scheitern würde.

Beispiel:

```text
Sam erhält bis 31.12.2026 die Stufe "Auslösen" im Kanal Marketing/Kampagnen.
Sam kann damit: Nachrichten lesen und schreiben, Threads anlegen, Agent Texter auslösen.
Dateien hochladen: nein. Freigabe-Karten bestätigen: nein.
Hinweis: Texter liest auch Marketing/Intern. Sam darf Marketing/Intern nicht lesen, kann aber über Texter Inhalte daraus erfahren.
```

Die Konsole zeigt die Vorschau vor jeder Bestätigung; der Step-up folgt danach.

### 9.4 Erinnerung vor Ablauf

Der Job `authz.expiry_reminder` läuft mit `authz.expiry_sweep_interval` und findet Freigaben und
Mitgliedschaften, die innerhalb von `authz.expiry_reminder_lead` ablaufen und noch nicht erinnert
sind (Spalte `expiry_reminded_at` an `grants` und `group_memberships`, 03), höchstens
`authz.expiry_sweep_batch_max` je Transaktion. Er schreibt das persönliche Ereignis
`access.expiring` (05) an aktive Empfänger, ohne Doppel: bei Freigaben an einen Menschen, Agenten
oder eine App an das Subjekt und an `granted_by`; bei Freigaben an eine Gruppe an alle aktiven
Owner und an `granted_by` (eine Gruppe ist kein Prinzipal, der persönliche Ereignisse empfängt);
bei Mitgliedschaften an das Mitglied und an `added_by`. Bei Freigaben auf die Instanz fehlt
`target`, weil ein Ziel nur Bereich oder Kanal sein kann. Die Konsole zeigt die Liste "Läuft bald ab" mit
"Verlängern" (Widerruf plus neue Freigabe mit neuem Grund). Es gibt keine E-Mails (Baseline
Abschnitt 4).

### 9.5 Weitere Pflichten

- Entziehen wirkt sofort und sagt das in der Bestätigung.
- Every grant and every revocation needs a reason (03, `grants.reason` and `grants.revoke_reason`); deleting a group revokes its grants with the reason `group_deleted` (M1-48).
- Liste "Rechte-Hinweise": H1, Freigaben an gesperrte Prinzipale, `trigger` ohne mögliche
  Auslösung (SR2, SR3), Agenten mit `personal` und ihre Trigger-Menge.
- Filterbares Protokoll aus dem Audit-Log (`audit.read`), [ADR 0021](../adr/0021-audit-log-hash-verkettet-und-getrennt-von-telemetrie.md).

## 10. Beispiele

Ausgangslage für alle Beispiele (fiktiv): Bereiche Support, Marketing, Engineering, Research. Ein
Owner. Agenten: Lotse (Bediener von Support, `organization`, `chat`), Texter (Bediener von
Marketing, `organization`, `scoped`), Runner (Bediener des Forums Engineering/Projekt-A mit
Projektbindung, `organization`, `host`), Assistent (Bediener von Research, `personal`,
`credential_holder` = Owner, `scoped`).

### 10.1 Support nur für Alex

| Freigabe | Subjekt | Ziel | Stufe | Zusatzrechte | Ablauf | Grund |
|---|---|---|---|---|---|---|
| G-1 | Alex | Bereich Support | `trigger` | `upload`, `approve_actions` | keiner | Support-Leitung |

Ergebnis: Alex sieht nur Support, kann Lotse auslösen und dessen Freigabe-Karten bestätigen.
Marketing, Engineering und Research sind für Alex `404`; Suche und Zähler zeigen dort nichts.
Lotse ist `organization`: das ist Pflicht, sobald Alex `trigger` hat (I1).

### 10.2 Marketing nur für Sam

| Freigabe | Subjekt | Ziel | Stufe | Zusatzrechte | Ablauf | Grund |
|---|---|---|---|---|---|---|
| G-2 | Sam | Bereich Marketing | `trigger` | `upload` | keiner | Marketing |

Ergebnis: Sam löst Texter aus. Freigabe-Karten von Texter mit `approve_actions` kann Sam nicht
bestätigen; das bleibt beim Owner. Liest Texter zusätzlich einen Kanal, den Sam nicht lesen darf,
zeigt die Vorschau Hinweis H1.

### 10.3 Ein Dev-Projekt für Robin mit Ablauf

| Freigabe | Subjekt | Ziel | Stufe | Zusatzrechte | Ablauf | Grund |
|---|---|---|---|---|---|---|
| G-3 | Robin | Kanal Engineering/Projekt-A | `trigger` | `upload` | 31.12.2026 | Projektmitarbeit |

Ergebnis bis zur Dev-Abschottung: Robin liest und schreibt im Forum, aber jede Auslösung endet mit
`dev_environment_not_ready` (SR3) bzw. `tool_class_owner_only` (SR2). Die Vorschau sagt das vor
dem Anlegen. Sobald eine bereite Umgebung für Robin eingetragen ist, laufen Robins Threads dort.
Threads des Owners kann Robin lesen, aber nicht auslösen (`dev_environment_foreign_thread`). Ab dem
31.12.2026 ist der Kanal für Robin `404`, ohne dass ein Job laufen muss.

### 10.4 Owner hat alles

Der Owner sieht alle Bereiche und Kanäle, löst alle Agenten aus, auch Assistent (`personal`, er ist
der `credential_holder`). DMs zwischen Alex und Sam sieht er nicht. Würde er Alex `trigger` auf
Research geben, lehnt der Server mit `i1_violation` ab: "zuerst auf organization umstellen".

## 11. Tests

### 11.1 Testmatrix

Welt der Matrix: wie Abschnitt 10, dazu Gruppe Leser (Mitglied Sam, Mitgliedschaft mit Ablauf) mit
Freigabe G-4 (`read` auf Support/Allgemein), Freigabe G-5 (Lotse, `write`, `upload`, Bereich
Support), Freigabe G-6 (App Tracker, `write`, Kanal Support/Eskalation, Scope nur
`messages:write`), Kanal Engineering/Werkzeuge (Text, Bediener Runner, ohne Projektbindung), ein
DM zwischen Alex und Sam. Die Uhr ist in Tests steuerbar.

| ID | Prinzipal | Ziel | Aktion | Bedingung | Erwartet |
|---|---|---|---|---|---|
| AZ-M01 | Owner | Research/Notizen | `message.read` | | `allow` |
| AZ-M02 | Owner | DM Alex-Sam | `message.read` | | `not_found` (SR6) |
| AZ-M03 | Alex | Support/Allgemein | `message.create` | G-1 auf Bereich | `allow` (geerbt) |
| AZ-M04 | Alex | Support/Allgemein | `agent.invoke` (Lotse) | | `allow` |
| AZ-M05 | Alex | Marketing/Kampagnen | `message.read` | | `not_found` |
| AZ-M06 | Alex | Instanz | `search.query` "Kampagne" | 3 Treffer nur in Marketing | 0 Treffer, `total = 0` |
| AZ-M07 | Sam | Support/Allgemein | `message.read` | Mitgliedschaft Leser gültig | `allow` (Gruppe) |
| AZ-M08 | Sam | Support/Allgemein | `message.create` | nur G-4 | `forbidden` `level_insufficient` |
| AZ-M09 | Sam | Support/Allgemein | `message.read` | Uhr = Ablauf der Mitgliedschaft | `not_found` (Grenze, ohne Job) |
| AZ-M10 | Sam | Support/Allgemein | `message.read` | Mitgliedschaft entfernt | `not_found` |
| AZ-M11 | Alex | Support/Eskalation | `agent.invoke` | G-1 `trigger` auf Bereich und zusätzlich `read` auf Kanal | `allow` (höchste gilt, überlappend) |
| AZ-M12 | Alex | Support/Eskalation | `message.create` | G-1 widerrufen, `read` auf Kanal bleibt | `forbidden` `level_insufficient` |
| AZ-M13 | Alex | Support/Allgemein | `message.read` | G-1 widerrufen | `not_found` |
| AZ-M14 | Robin | Engineering/Projekt-A | `message.create` | G-3 gültig | `allow` |
| AZ-M15 | Robin | Engineering/Projekt-A | `agent.invoke` | keine Dev-Umgebung | `forbidden` `dev_environment_not_ready` |
| AZ-M16 | Robin | Engineering/Projekt-A | `agent.invoke` | eigene Umgebung bereit | `allow`, Marke mit `dev_environment_id` |
| AZ-M17 | Robin | Thread des Owners in Projekt-A | `agent.invoke` | eigene Umgebung bereit | `forbidden` `dev_environment_foreign_thread` |
| AZ-M18 | Robin | Engineering/Projekt-A | `thread.read` | Uhr nach 31.12.2026 | `not_found` |
| AZ-M19 | Owner | Engineering/Projekt-A | `agent.invoke` | | `allow` |
| AZ-M20 | Alex | Engineering/Werkzeuge | `agent.invoke` (Runner, `host`) | `trigger`-Freigabe auf Kanal | `forbidden` `tool_class_owner_only` |
| AZ-M21 | Owner | Research/Notizen | `grant.create` Sam `trigger` | Assistent `personal` | `invariant_violation` `i1_violation` |
| AZ-M22 | Owner | Assistent | `agent.billing_mode.set` auf `personal` | Alex hat `trigger` auf einem bedienten Kanal | `invariant_violation` `i1_violation` |
| AZ-M23 | Owner | Alex | `owner.add` | Assistent `personal` | `invariant_violation` `i1_violation` |
| AZ-M24 | Owner | Support/Allgemein | `channel.operator.set` Assistent | Alex hat `trigger` | `invariant_violation` `i1_violation` |
| AZ-M25 | Owner | Gruppe mit `trigger` auf Research | `group.member.add` Sam | Assistent `personal` | `invariant_violation` `i1_violation` |
| AZ-M26 | Owner | Owner selbst | `user.disable` | einziger Owner | `invariant_violation` `last_owner` |
| AZ-M27 | Lotse | Support/Allgemein | `message.create` | G-5 | `allow` |
| AZ-M28 | Lotse | Marketing/Kampagnen | `message.create` | Agent im fremden Bereich | `not_found` |
| AZ-M29 | Texter | Marketing/Kampagnen | `message.create` | keine Freigabe, Bediener | `allow` (Bediener-Regel) |
| AZ-M30 | Lotse | Support/Allgemein | Nachricht erwähnt Texter | `origin = agent` | kein Lauf (SR4) |
| AZ-M31 | Owner | Lotse | `grant.create` `trigger` | Subjekt Agent | `forbidden` `subject_kind_not_allowed` |
| AZ-M32 | Tracker | Support/Eskalation | `message.create` | G-6, Scope `messages:write` | `allow` |
| AZ-M33 | Tracker | Support/Eskalation | `message.read` | Scope `messages:read` fehlt | `forbidden` `scope_missing` |
| AZ-M34 | Tracker | Support/Allgemein | `message.create` | keine Freigabe | `not_found` |
| AZ-M35 | Sam mit `trigger` + `manage` auf Marketing, persönliches Token | Marketing | `grant.create` Robin `read` | Token ohne Sitzung | `forbidden` `step_up_not_possible` |
| AZ-M36 | Owner | Marketing | `grant.create` | Step-up älter als `auth.step_up_max_age` | `step_up_required` |
| AZ-M37 | Alex mit `write` + `manage` auf Support | Support/Allgemein | `grant.create` Sam `trigger` | | `forbidden` `delegation_exceeds_own_level` |
| AZ-M38 | Alex mit `trigger` + `manage` auf Support | Support/Allgemein | `grant.create` Sam `read` + `export` | Alex ohne `export` | `forbidden` `delegation_exceeds_own_capabilities` |
| AZ-M39 | Alex mit `write` + `manage` auf Support | Freigabe des Owners für Sam (`trigger`) | `grant.revoke` | | `forbidden` `delegation_exceeds_own_level` |
| AZ-M40 | Alex | beliebig | beliebig | Alex gesperrt | `unauthenticated` `principal_inactive` |
| AZ-M41 | Alex | beliebig | beliebig | Sitzung widerrufen, Token noch nicht abgelaufen | `unauthenticated` `session_revoked` |
| AZ-M42 | Alex | Karte mit `restricted_to = run_starter`, Lauf vom Owner | `interaction.create` | | `forbidden` `component_restricted` |
| AZ-M43 | Sam | Karte mit `approve_actions` in Marketing | `interaction.create` | Sam ohne `approve_actions` | `forbidden` `capability_missing` |
| AZ-M44 | Lotse im Lauf von Alex | Engineering/Projekt-A | `message.read` | Lotse hat zusätzlich `read` auf Engineering | `not_found` `acting_for_insufficient` (SR9) |
| AZ-M45 | Lotse ohne Laufbezug | Engineering/Projekt-A | `message.read` | wie M44 | `allow`, Hinweis H1 in der Konsole |
| AZ-M46 | Owner als Alex | Marketing/Kampagnen | `message.read` | "Als Person ansehen" | `not_found` |
| AZ-M47 | Owner als Alex | Support/Allgemein | `message.create` | "Als Person ansehen" | `forbidden` `view_as_read_only` |
| AZ-M48 | Alex | Marketing-Nachricht mit Erwähnung von Alex | `push.deliver` | | kein Push, Erwähnung still gespeichert |
| AZ-M49 | Tracker | Ereignis aus Support/Allgemein | `webhook_out.deliver` | Abo auf `message.created` | nicht zugestellt |
| AZ-M50 | Sam | Support/Allgemein | `message.read` | Gruppe Leser gelöscht | `not_found` |
| AZ-M51 | Alex | Support/Allgemein | `agent.invoke` | Entzug zwischen Annahme und Zustellung | Lauf `failed` `access_revoked`, keine Zustellung (I2) |
| AZ-M52 | Alex | Marketing/Kampagnen | `GET` Nachricht mit fremder ID | | gleich wie zufällige ID (Status, Körper, Kopfzeilen) |
| AZ-M53 | Eingangs-Webhook von Tracker | Support/Eskalation | `webhook_in.post` mit Erwähnung von Lotse | Kanal mit `free_response` | Nachricht `origin = app`, kein Lauf (SR4) |
| AZ-M54 | Alex | Rechner R1 (Inhaber Alex, `organization`) | `terminal.open` | Freigabe `terminal` an Alex, frischer Step-up | `allow` |
| AZ-M55 | Sam | Rechner R1 | `terminal.open` | Freigabe `terminal` an Sam | `forbidden` `terminal_not_host_assignee` (SR10) |
| AZ-M56 | Owner | Rechner R2 (`personal`, Inhaber der Anmeldung Robin, Robin ist zweiter Owner) | `terminal.open` | | `forbidden` `i6_holder_only` (SR11) |
| AZ-M57 | Owner | Rechner R2 | `grant.create` Gruppe Leser `terminal` | | `invariant_violation` `i6_violation` |
| AZ-M58 | Alex | Rechner R1 | `terminal.open` | Step-up schon für eine andere Öffnung verbraucht | `step_up_required` `terminal_step_up_not_fresh` |
| AZ-M59 | Lotse | Rechner R1 | `terminal.open` | Agent-Token | `forbidden` `subject_kind_not_allowed` |
| AZ-M60 | Alex | Rechner R3 (ohne Inhaber) | `terminal.open` | Freigabe an die Instanz mit `trigger` und allen Zusatzrechten, keine Freigabe auf R3 | `not_found` (Rechner nicht sichtbar) |

### 11.2 Pflicht-Tests

| ID | Test | Schlägt fehl, wenn |
|---|---|---|
| AZ-P1 | Routen-Test: zählt alle registrierten HTTP-Routen | eine Route keine bekannte `x-authz-action` hat (05, Regel 3) |
| AZ-P2 | Op- und Ereignis-Test | eine WebSocket-Op oder ein Ereignistyp ohne Eintrag in 05, Abschnitt 6 bzw. 7 registriert ist |
| AZ-P3 | Katalog geschlossen | eine unbekannte Aktion nicht mit `500` und Alarm endet |
| AZ-P4 | 404-Gleichheit | Antwort für fremde ID und unbekannte ID sich in Status, Körper oder Kopfzeilen unterscheidet, oder der Median der Antwortzeit mehr als `authz.not_found_timing_tolerance` abweicht. Routen, deren Ressource der Anfragende immer sehen darf (z. B. `user.read` für Menschen), haben keine fremde unsichtbare Instanz und stehen mit Grund als "nicht anwendbar" in der Liste des Tests |
| AZ-P5 | Zähler und Suche verraten keine fremden Kanäle | in zufällig erzeugten Welten (Eigenschaftstest) Ungelesen-Zähler, Erwähnungszähler, Such-Treffer, `total`, Facetten, Vorschläge, "Läuft gerade" oder Snapshot einen Kanal ohne Leserecht berücksichtigen |
| AZ-P6 | Adapter ohne gültige Marke tut nichts | der Referenz-Adapter oder ein SDK-Adapter bei fehlender Marke, falscher Signatur, falschem `aud`, abgelaufener Marke, wiederholter `jti`, abweichendem `billing_mode` oder abweichender `tool_class` die Laufzeit startet (Konformitätstest der SDKs, [09-agenten.md](09-agenten.md)) |
| AZ-P7 | I1 bei jeder Änderungsart | für eine der Aktionen aus 05, Regel 1.6, eine Änderung, die I1 verletzt, committet wird; je Aktion ein Fall |
| AZ-P8 | Differenztest Cache gegen Referenz | `authorize` mit Cache in zufälligen Welten anders entscheidet als eine langsame Referenz ohne Cache |
| AZ-P9 | Entzug sofort | nach dem Commit eines Widerrufs in Prozess A eine Anfrage an Prozess B noch erlaubt wird. Prozess heißt eine eigene Server-Instanz mit eigenem Pool, Log-Tail, Rechte-Cache und Hub an derselben Datenbank; zwei solche Instanzen in einem Test-Binary genügen, weil sie keinen Speicher teilen |
| AZ-P10 | Ablauf ohne Job | bei abgeschaltetem `authz.expiry_sweep` eine abgelaufene Freigabe noch wirkt |
| AZ-P11 | Auftragsbindung | ein Agent im Lauf eines Menschen Inhalte liest oder schreibt, die der Mensch nicht lesen bzw. schreiben darf |
| AZ-P12 | SQL-Prädikat | eine lesende sqlc-Abfrage auf Inhaltstabellen ohne `= ANY($visible)` und ohne Eintrag in der Ausnahmeliste existiert (Abschnitt 7.4, Regel 1) |
| AZ-P13 | Ablehnungen im Audit | eine Ablehnung einer Anfrage oder Op nicht oder nicht zusammengefasst (`audit.deny_aggregate_window`) im Audit-Log steht, oder ein Filter (Abschnitt 4.2) eine Ablehnung schreibt |

## 12. Entscheidungen dieses Kapitels

| Nr | Entscheidung | Grund |
|---|---|---|
| R-RE-01 | Rechte-Cache mit globalem Versionszähler und `valid_until`; keine materialisierte Sicht. | Eine Wahrheit (R-DM-19); Ablauf durch Zeit ohne Schreibvorgang korrekt. |
| R-RE-02 | Jede REST-Anfrage liest die aktuelle Rechte-Version. | Entzug wirkt prozessübergreifend ohne Zeitfenster; Kosten eine Zeile je Anfrage. |
| R-RE-03 | Agenten und Apps bekommen als Zusatzrecht nur `upload`. | Keine Selbstfreigabe, keine Verwaltung, kein Massenexport durch Bots. |
| R-RE-04 | Bediener-Regel: wirksamer Bediener hat implizit `write`. | Folgt dem Bediener ohne zweite Freigabe; kein Rest nach Wechsel. |
| R-RE-05 | Werkzeugklasse `host` nur für Owner oder eigene bereite Dev-Umgebung, geprüft beim Aufruf. | Studie C P6(b); Bereitschaft ändert sich ohne Rechte-Transaktion. |
| R-RE-06 | Auftragsbindung: Agent im Lauf hat höchstens die Rechte des Auslösers. | Schließt den Confused Deputy über die Server-API. |
| R-RE-07 | Lesehorizont als Hinweis, nicht als Sperre. | Eine Sperre ließe `channel.create` unerwartet scheitern; Laufzeit-Gedächtnis kennt nur der Betreiber. |
| R-RE-08 | `manage` widerruft nur, was es selbst hätte anlegen dürfen; Freigaben an Agenten und Apps nur durch den Owner. | Keine Rechte-Ausweitung über Delegation; Agenten-Freigaben sind Datenflüsse zu Auftragsverarbeitern. |
| R-RE-09 | Ablauf-Job meldet nur; die Entscheidung hängt nie an ihm. | Studie C: Ablauf je Aufruf geprüft; Clients räumen trotzdem auf. |
| R-RE-10 | Vorschau als Trockenlauf in der echten Transaktion mit Rollback. | Dieselbe Logik wie der echte Schreibvorgang; Klartext aus Vorlagen, nicht aus einem Modell. |
| R-RE-11 | Rechner als eigenes Ziel mit nur `terminal`; SR10 und SR11 als Schutzregeln. | Ein Terminal ist Fernzugriff auf einen Rechner; weder Vererbung noch Delegation dürfen ihn öffnen (Ruling T1, T2). |

## 13. Abgleich mit dem Kern-Vertrag

Die beim Entwurf beantragten Ergänzungen sind entschieden (Integration vom 2026-10-10):

| Nr | Ergänzung | Entscheidung, Ort |
|---|---|---|
| K-RE-1 | Check: Agenten und Apps als Zusatzrecht nur `upload` | übernommen, 03 `grants`, R-DM-20 |
| K-RE-2 | Bediener-Regel in "Wirksame Stufe" | übernommen, 05 Abschnitt 2.3 Punkt 4, R-AZ-13 |
| K-RE-3 | `agents.tool_class`, `agents.reported_tool_class`, `agent.tool_class.set`, Claim `tool_class` | übernommen, 03, 05 |
| K-RE-4 | `dev_environments`, `dev_environment_id` an Sitzungen und Läufen, Claim, Aktionen | übernommen als Etappe "Mehrbenutzer-Dev-Umgebung", 03, 05 Abschnitt 4.13 |
| K-RE-5 | SR8 auch für `user.disable` | übernommen, 03 `owners`, 05 |
| K-RE-6 | `grant.revoke` im Rahmen von R-AZ-05; Freigaben an Agenten und Apps nur Owner | übernommen, 05 Abschnitt 4.4, R-AZ-05 |
| K-RE-7 | Aktion `access.preview` | übernommen, 05 Abschnitt 4.2 |
| K-RE-8 | Systemaktionen `grant.expire`, `group.member.expire`, Ereignis `access.expiring`, `expiry_reminded_at` | übernommen, 03, 05 |
| K-RE-9 | Grund-Codes aus 4.1 als Werte von `audit_events.deny_reason` | übernommen ohne Datenbank-Check: Liste geschlossen per Test (R-DM-28) |
| K-RE-10 | Owner-Rechte nur mit Sitzung | übernommen, 05 Abschnitt 2.3, R-AZ-14 |

## 14. Konfigurationswerte dieses Kapitels

| Name | Standard | Bedeutung |
|---|---|---|
| `authz.expiry_sweep_interval` | `1m` | Abstand des Jobs, der Abläufe meldet und Erinnerungen schreibt |
| `authz.expiry_reminder_lead` | `48h` | so lange vor Ablauf kommt die Erinnerung (Studie C: zwei Tage) |
| `authz.expiry_sweep_batch_max` | `500` | Zeilen je Transaktion der Jobs `authz.expiry_sweep` und `authz.expiry_reminder` |
| `authz.view_as_max_duration` | `15m` | Höchstdauer von "Als Person ansehen" |
| `authz.snapshot_cache_max_entries` | `10000` | Rechte-Stände im Speicher je Prozess (LRU) |
| `authz.not_found_timing_tolerance` | `20ms` | erlaubte Abweichung des Medians in AZ-P4 |
