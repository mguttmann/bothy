# Risiken

Stand: Ende M0 (Planung). Diese Seite führt die Produktrisiken: was den Bau, den Betrieb oder die
Zusagen des Produkts gefährdet, wie wahrscheinlich das ist, was es anrichtet und womit der Plan
gegensteuert. Einzelne Bedrohungen mit Angreifer, STRIDE-Kategorie und Tests stehen im
[Bedrohungsmodell](security/threat-model.md); hier steht nur, was daraus als Risiko für das Produkt
bleibt. Risiken einer einzelnen Instanz (eigene Rechner, eigene Konten, eigener Betrieb) stehen im
privaten Betriebs-Repository dieser Instanz.

**Herkunft.** Die Risiken der Team-Berichte, des Kern-Vertrags und des Nachtrags zu Terminal und
Visualisierung, zusammengeführt: Gleiche Ursachen stehen in einer Zeile. Risiken, die eine spätere
Entscheidung beseitigt hat, stehen unter [Erledigt](#erledigt).

**Bewertung.** Wahrscheinlichkeit und Auswirkung je `niedrig`, `mittel` oder `hoch`, so wie im
Bedrohungsmodell (Abschnitt 6.1). Haben zwei Berichte dasselbe Risiko verschieden bewertet, gilt die
höhere Bewertung, außer eine spätere Entscheidung hat die Ursache verkleinert; dann nennt die Zeile
diese Entscheidung.

**Pflege.** Am Ende jeder Etappe zieht das Abschluss-Gate diese Seite nach ([Plan](PLAN.md),
Abschnitt 4 Punkt 6): Risiken der Etappe prüfen, neu bewerten, erledigte verschieben, neue
aufnehmen. Die Spalte "Prüfung" nennt die Etappe, in der das Risiko zuerst mit dem genannten Test
oder Ticket belegt oder widerlegt wird. IDs werden nie neu vergeben.

## Die fünf größten Risiken

| Rang | ID | Risiko | Wahrscheinlichkeit | Auswirkung |
|---|---|---|---|---|
| 1 | RK-01 | Prompt-Injection steuert Agenten mit Werkzeugen | hoch | hoch |
| 2 | RK-77 | Mit dem ersten Push wird die Historie sichtbar, samt alter Beispielwerte und interner Kennungen | hoch | hoch |
| 3 | RK-13 | Falscher Beleg des Ist-Modus beim Referenz-Adapter führt Organisationsverkehr über ein persönliches Abo | mittel | hoch |
| 4 | RK-23 | Der Host-Dienst auf fremden Rechnern gibt mehr als die erlaubte Shell | mittel | hoch |
| 5 | RK-07 | Das Gedächtnis einer Laufzeit trägt Wissen zwischen Publikumsgruppen | mittel | hoch |

**Begründung der Reihenfolge.** Die Reihenfolge folgt drei Kriterien, in dieser Rangfolge:

1. **Bewertung:** Wahrscheinlichkeit mal Auswirkung aus dem Register (`niedrig` 1, `mittel` 2,
   `hoch` 3).
2. **Art der Folge:** Die Folge verlässt die Instanz oder ist nicht umkehrbar, das heißt, ein
   späterer Fix nimmt sie nicht zurück: Sperre beim Anbieter, Veröffentlichung, Abfluss von Daten,
   Inhalte, Wissen oder eine Shell bei jemandem ohne Recht darauf, Rechtsverstoß. Ausfälle, Brüche
   an Schnittstellen, Aufwand und Einschränkungen behebt ein späterer Fix; sie erfüllen das
   Kriterium nicht.
3. **Rest nach der Gegenmaßnahme:** was kein Test im Repository ganz abdecken kann. Führt das
   [Bedrohungsmodell](security/threat-model.md) die Bedrohung, gilt seine Stufe "Restrisiko" (bei
   mehreren die höchste). Führt es sie nicht, nennt die Begründung unten eine Stufe auf derselben
   Skala mit Grund; sie zählt wie eine Stufe des Bedrohungsmodells. Bei gleicher Stufe zählt, wie
   weit der Rest reicht.

`make docs` prüft das erste Kriterium mit `tools/docs/risk_ranking.py` (D-W-78, D-W-79): Die
Tabelle nennt die Bewertung des Registers, sie steigt nach unten nie an, kein Risiko außerhalb der
Tabelle ist höher bewertet als ihr letzter Rang, und die Liste "Hinter Rang 5" nennt genau die
Risiken außerhalb der Tabelle mit derselben Bewertung wie Rang 5. Die Kriterien 2 und 3 sind
Abwägungen; sie prüft die Pflege am Ende jeder Etappe.

1. **RK-01** und **RK-77** sind die einzigen Risiken mit `hoch` und `hoch`. Bei beiden verlässt
   die Folge die Instanz und ist nicht umkehrbar: Ein Werkzeug wirkt nach außen, eine
   veröffentlichte Historie lässt sich nicht zurückholen. RK-01 steht vorn, weil mehr bleibt:
   Kein Modell erkennt eingeschleuste Anweisungen sicher (T-AI-01 bis T-AI-03, Restrisiko
   `mittel`). Das Produkt setzt darum auf Grenzen statt Erkennung, und jeder neue Agent und jedes
   neue Werkzeug erweitert die Fläche.
2. **RK-77** hat eine endliche Ursache, die vorhandene Historie. Zwei Historien-Scans
   (`make denylist-history`, gitleaks über alle Refs) finden die Stellen, veröffentlicht wird nur
   ein Commit ohne Eltern (D-W-93), und ein Red Team prüft das Ergebnis. Es
   bleiben nur Kennungen, die weder die Deny-Liste mit ihren Regeln noch gitleaks erkennt. RK-77
   hat im Bedrohungsmodell keinen Eintrag; nach Kriterium 3 ist dieser Rest `niedrig`: Scans und
   Red Team prüfen eine Ursache, die nicht mehr wächst, denn nach dem Umschalten prüft
   `denylist-main.yml` jeden Push auf `main`.
3. **RK-13** führt die Risiken der nächsten Bewertung an (`mittel` und `hoch` oder `hoch` und
   `mittel`). Die Folge verlässt die Instanz: Der Anbieter kann den Zugang der ganzen Organisation
   sperren, und das ist nicht umkehrbar. Ein unbekannter Ist-Modus bricht den Lauf ab (fail closed).
   Es bleibt der Fall, dass der Adapter einen falschen Modus als sicher meldet, weil er an einer
   nicht öffentlichen Oberfläche von Hermes Agent hängt. Das deckt kein Test im Repository ganz ab
   (T-AI-08, Restrisiko `mittel` im Adapter), und dieser Rest trifft die ganze Organisation.
   RK-22 stand bis D-W-57 vor RK-13 mit derselben Folge. Seit Ruling T12 hat jede Person auf einem
   Rechner ihren eigenen Sitzungsbenutzer, und seit Ruling T13 (D-W-62) verbindet kein
   konfigurierter Ort die Konten wieder: Arbeitsordner und `CLAUDE_CONFIG_DIR` gehören je einem
   Sitzungsbenutzer (bei `personal` liegt er im Home, D-W-82), und HD11 prüft jeden konfigurierten
   Pfad auf allen drei Plattformen (D-W-80). Eine Abo-Anmeldung erreicht dann
   keine Sitzung eines anderen Menschen, auch wenn `doctor` sie nicht findet. Die Begründung von
   D-W-57 allein trug das nicht: Gemeinsame Arbeitsordner waren ein zweiter Weg (jetzt
   RK-88). Was bleibt, hängt an einer lokalen Rechteausweitung, an Fehlern des
   Betreibers (früher vergebener Name, geteilter Ordner außerhalb der Konfiguration) oder an der
   eigenen Handlung einer Person in einem solchen Ordner. Darum steht RK-22 jetzt auf `niedrig`.
4. **RK-23:** Eine Shell oder Rechte auf root-Niveau bei jemandem ohne Recht darauf nimmt kein Fix
   zurück. Nach der Gegenmaßnahme bleiben Lücken im Betriebssystem des Rechners (T-TM-04, Restrisiko
   `mittel`); dazu gibt es eine harte Sperre: Ohne Beleg für eine Plattform gibt es dort kein
   Terminal (M2-42). Der Rest bleibt auf einem Rechner; darum steht RK-23 hinter RK-13. Dort reicht
   er aber über jedes vergebene Recht hinaus, bis zu Sitzungen und Anmeldungen anderer Personen.
5. **RK-07:** Wissen, das eine andere Publikumsgruppe erreicht hat, nimmt kein Fix zurück. Der
   Server sieht das Gedächtnis der Laufzeit nicht; die Gegenmaßnahme ist eine Empfehlung und der
   Hinweis H1, und den Rest erreicht kein Test im Repository (T-AI-01, Restrisiko `mittel`). Er
   reicht über die Grenze zwischen Publikumsgruppen, aber nur als Wissen. Darum steht RK-07
   hinter RK-23, dessen Rest Handeln erlaubt.

**Hinter Rang 5** stehen alle übrigen Risiken mit der Bewertung `mittel` und `hoch` oder `hoch`
und `mittel`:

- **RK-27** (offene Terminal-Sitzungen in der Konsole) erfüllt die Kriterien 2 und 3 wie RK-07
  (T-TM-07, Restrisiko `mittel`), aber sein Rest reicht weniger weit. Er setzt einen schon
  unterwanderten Browser einer Person voraus, die das Terminal öffnen darf, und bleibt in deren
  Rechten; das ist derselbe Rest wie bei Schadcode auf jedem Gerät (T-TK-01). Der Rest von RK-07
  geht dagegen über die Grenze zwischen Publikumsgruppen.
- **RK-02, RK-04, RK-33, RK-49 und RK-50** erfüllen Kriterium 2: Inhalte bei Menschen ohne
  Leserecht, Anfragen in interne Netze, gelöschte Daten kommen zurück (Rechtsverstoß), Inhalte in
  der Telemetrie, Datenverlust nach einer fehlerhaften Sicherung. Nach Kriterium 3 bleibt
  weniger: Ihre Ursache liegt in Code und Konfiguration des Repositorys, und Tests decken sie ab.
  RK-02 prüfen der Differenztest AZ-P8 und die Fall-Tabellen je Aktion (T-AZ-03), RK-04 der
  Egress-Client mit Adressprüfung nach der Auflösung und SSRF-Lint (T-NW-01), RK-33
  Lösch-Journal, Startprüfung und die Tests SEC-T15 und DP-T07 (T-BK-02), RK-49 die Allowlist im
  Gate, der Linter für Lognachrichten und der Kanarien-Test (T-LG-01), RK-50 der Integrationstest
  von Archivierung, Sicherung und Wiederherstellung (ADR 0058). Die genannten Bedrohungen stehen
  im Bedrohungsmodell mit dem Restrisiko `niedrig`. RK-50 hat dort keinen Eintrag; nach
  Kriterium 3 ist sein Rest aus demselben Grund `niedrig`.
- **RK-11, RK-12, RK-14, RK-15, RK-31, RK-39, RK-44, RK-56 und RK-85** erfüllen Kriterium 2 nicht.
  Ihre Folge ist eine Einschränkung (RK-11, RK-39, RK-44), ein Bruch an einer Schnittstelle (RK-12,
  RK-14, RK-15), eine Grenze des Durchsatzes (RK-31), Aufwand (RK-56) oder ein ausgelassener
  Prüfschritt (RK-85); ein späterer Fix behebt sie.

## Register

### Agenten, Sicherheit und Rechte

| ID | Risiko | Wahrscheinlichkeit | Auswirkung | Gegenmaßnahme | Verweis | Prüfung |
|---|---|---|---|---|---|---|
| RK-01 | Prompt-Injection über Inhalte unberechtigter Autoren, andere Agenten, Dateien oder Importe bringt einen Agenten mit Werkzeugen zu ungewollten Wirkungen. | hoch | hoch | Grenzen statt Erkennung: Herkunfts- und Vertrauensmarke je Kontext-Nachricht, Werkzeugklassen (SR2), Auftragsbindung (SR9), Freigabe-Karten mit Kopf aus Server-Feldern, Agenten lösen nie Agenten aus; Red Team in jeder Etappe. | [09](spec/09-agenten.md), [07](spec/07-rechte.md), [ADR 0054](adr/0054-herkunfts-und-vertrauensmarkierung-im-agenten-kontext.md), T-AI-01 bis T-AI-03, AG-20, M2-13, M2-14 | M2 |
| RK-02 | Rechte zeigen zu viel: Der Rechte-Stand veraltet nach einer Strukturänderung oder dem Ablauf einer Freigabe, oder eine Rechte-Änderung nennt nicht alle Betroffenen für `access.changed`. Ereignisse erreichen dann Menschen ohne Leserecht, oder Inhalte bleiben auf ihren Geräten. | mittel | hoch | `authorize` je Ereignis mit Ressourcenpfad; Nachhol-Filter über sichtbare Bereiche; Job `authz.expiry_sweep`; Betroffene bestimmt `authz` selbst aus dem Ziel; Cache an der Rechte-Version; Differenztest AZ-P8 und Fall-Tabellen-Test je Aktion. | [07](spec/07-rechte.md), [04](spec/04-sync-protokoll.md), M1-28, M1-29, M1-30, M1-53 | M1 |
| RK-03 | Ein Client wendet die Lösch-Regel nach Entzug oder Widerruf nicht richtig an; Inhalte bleiben auf dem Gerät. | mittel | mittel | Konformitätstest im Client-Testkit; Entzug in der Sync-Engine mit Integrationstest. | [04](spec/04-sync-protokoll.md), [15](spec/15-client-architektur.md), M4-14 | M4 |
| RK-04 | SSRF oder DNS-Rebinding über ausgehende Webhooks und Adressen für Interaktionen. | mittel | hoch | Nur HTTPS, Host-Erlaubnisliste `webhook.allowed_hosts`, IP-Sperrliste nach der Auflösung, Verbindung an die geprüfte Adresse gebunden, keine Weiterleitungen; ein Egress-Client mit SSRF-Lint für jeden ausgehenden Aufruf. | [19](spec/19-sicherheit.md), [ADR 0016](adr/0016-webhooks-nach-standard-webhooks.md), M1-22, M2-26 | M1 |
| RK-05 | Wer Datenbank und Ziel der Prüfpunkte kontrolliert, kann das Audit-Log fälschen. | niedrig | hoch | Prüfpunkte in einem getrennten Konto mit Aufbewahrungssperre; die Konsole warnt, solange kein Ziel gesetzt ist. | [ADR 0021](adr/0021-audit-log-hash-verkettet-und-getrennt-von-telemetrie.md), M7-22 | M7 |
| RK-06 | Eine Sperre im Identitätsanbieter wirkt erst nach `auth.idp_recheck_interval`. | mittel | mittel | `user.disable` wirkt sofort; Sperren per SCIM oder Shared Signals als Ausbau. | [08](spec/08-identitaet.md), M1-45, V3-08 | M1 |
| RK-07 | Das Gedächtnis einer Laufzeit trägt Wissen zwischen Publikumsgruppen; der Server sieht es nicht (Hinweis H1). | mittel | hoch | Empfehlung "ein Agent je Publikum"; H1 sichtbar in der Vorschau einer Freigabe und in "Wer hat Zugriff?". | [07](spec/07-rechte.md), [Bedrohungsmodell](security/threat-model.md) T-AI-01 | M2 |
| RK-08 | Windows Hello liefert den Schlüssel eventuell nicht als ES256; Windows-Geräte ohne TPM 2.0 oder ohne P-256 im TPM. | mittel | mittel | Messung im Windows-Ticket; Laufzeitprüfung; Rückfall auf einen Software-Schlüssel mit Kennzeichnung "nicht hardwaregebunden", sichtbar bei der Freigabe des Geräts; weitere Algorithmen nur per ADR. | [08](spec/08-identitaet.md), [03](spec/03-datenmodell.md) `device_keys`, M4-11 | M4 |
| RK-09 | DPoP, Aufruf-Marken und Auftragsbindung erhöhen den Aufwand in Adaptern und SDKs; eine fehlerhafte Umsetzung schwächt die Prüfung. | mittel | mittel | Die SDKs kapseln DPoP und Auftragsbindung; Konformitätstest AZ-P6 und Konformitätssuite mit Fake-Agent für jeden Adapter. | [ADR 0026](adr/0026-sdks-generierter-kern-plus-handschicht.md), M2-29, M2-31 | M2 |
| RK-10 | Zeitlicher Seitenkanal in der Suche: Die Dauer hängt auch von Treffern in fremden Kanälen ab. | niedrig | niedrig | Kandidaten-Grenze, `statement_timeout`, Rate-Limit `rl.search`. | [14](spec/14-suche.md), M3-14 | M3 |

### Anbieter, Adapter und API

| ID | Risiko | Wahrscheinlichkeit | Auswirkung | Gegenmaßnahme | Verweis | Prüfung |
|---|---|---|---|---|---|---|
| RK-11 | Strenge I1 verhindert einen zweiten Owner, solange persönliche Agenten existieren. | hoch | mittel | Geführter Ablauf `resolve_i1` auch für `owner.add`; Owner-Frage [OQ-01](OPEN-QUESTIONS.md). | [10](spec/10-backend-richtlinie.md), [ADR 0024](adr/0024-backend-richtlinie-personal-organization-mit-invariante.md), M2-21 | M2 |
| RK-12 | Interna von Hermes Agent (Aufrufmuster der Fortschritts-Hooks, `steer`, Erkennung des Anmeldetyps) sind keine zugesagte Plugin-API und ändern sich. | hoch | mittel | Versionsbereich pinnen; Mitschnitt vor dem Bau; Kompatibilitätstest gegen die gepinnte und die neueste Fassung; Kompatibilitätsschicht. | [09](spec/09-agenten.md), M2-32, M2-33 | M2 |
| RK-13 | Der Beleg des Ist-Modus bei Hermes Agent hängt an einer nicht öffentlichen Oberfläche. Ein falscher Beleg führt Organisationsverkehr über ein persönliches Abo (dort hat OAuth Vorrang vor dem API-Schlüssel). | mittel | hoch | Prüfung vor jedem Lauf; `unknown` bricht ab (`billing_mode_mismatch`, fail closed); der Installer leert die OAuth-Variablen im Modus `organization` und prüft die Konfiguration; Mitschnitt in M2-32 vor dem Bau. | [10](spec/10-backend-richtlinie.md), [ADR 0023](adr/0023-signierte-aufruf-marke-und-pruefung-im-adapter.md), [Bedrohungsmodell](security/threat-model.md) T-AI-08, M2-32, M2-33 | M2 |
| RK-14 | Die Claude-Code-CLI ändert Stream-Nachrichten, Fehlertexte, Flags oder die Ausgabe von `claude auth status`. | hoch | mittel | CLI-Version pinnen; Fixtures aus echten Mitschnitten; Test gegen die installierte CLI; Fähigkeiten aus dem init-Ereignis; Fake-`claude` für `doctor`, das bei Unbekanntem fail closed bleibt. | [09](spec/09-agenten.md), [24](spec/24-terminal.md), M2-34, M2-35, M2-42 | M2 |
| RK-15 | Der generierte Python-Client wirft bei unbekannten Enum-Werten und Ereignistypen; additive Änderungen brechen SDK und Adapter. | hoch | mittel | Handschicht oder eigene Vorlagen; Toleranz-Fixture G5 im Gate. | [ADR 0026](adr/0026-sdks-generierter-kern-plus-handschicht.md), [ADR 0014](adr/0014-api-versionierung-und-deprecation-policy.md), M2-23, M2-29 | M2 |
| RK-16 | Die Kostenschätzung weicht von der Rechnung des Anbieters ab (Cache-Dauern, Regionen, Modi, Preisstufen). | hoch | niedrig | Kennzeichnung "Schätzung"; eigene Einheit für Cache-Schreiben mit langer Dauer; optionaler Abgleich als eigene App. | [10](spec/10-backend-richtlinie.md), [ADR 0055](adr/0055-kostenabgleich-als-eigene-app-admin-key-nie-im-server.md), M2-22, M2-28 | M2 |
| RK-17 | Missbrauch eines Admin-Schlüssels des Anbieters bei einem Einbruch. | niedrig | hoch | Der Schlüssel liegt nie im Server-Prozess, nur in der Abgleich-App; der Abgleich ist standardmäßig aus. | [ADR 0055](adr/0055-kostenabgleich-als-eigene-app-admin-key-nie-im-server.md), M2-28 | M2 |
| RK-18 | Ein Agenten-Aufruf läuft nach Neustart oder Doppelzustellung zweimal. | niedrig | hoch | Lauf-Start als Compare-and-swap mit `jti`; die Laufzeit startet erst nach der Zusage; keine automatische Wiederholung; Tests AG-09 und AG-10. | [04](spec/04-sync-protokoll.md), [ADR 0023](adr/0023-signierte-aufruf-marke-und-pruefung-im-adapter.md), M2-09 | M2 |
| RK-19 | Hohe Ereignislast für Agenten, weil jeder Agent alle sichtbaren Kanalereignisse bekommt. | mittel | niedrig | Kontext per `run.context.read` statt aus dem Strom; Leistungsziele des Agenten-Pfads; ein Ereignis-Filter beim Verbinden bleibt als Ergänzung möglich. | [09](spec/09-agenten.md), M2-36 | M2 |
| RK-20 | Webhooks: Das Token eines Eingangs-Webhooks in der URL landet in Proxy-Logs; Empfänger können Ed25519 (`v1a`) mangels Bibliothek nicht prüfen; die RateLimit-Kopfzeilen ändern sich bis zum RFC. | mittel | mittel | Schwärzungsregel für den Pfad in Server und Proxy-Doku, Rotation durch Neuanlage, Rate-Limit `rl.webhook_in`; HMAC `v1` je Abo zuschaltbar mit Testvektoren; neue Kopfzeilen zusätzlich senden. | [06](spec/06-api.md), [Webhooks](api/webhooks.md), M2-26, M2-27 | M2 |
| RK-21 | OpenAPI 3.1 ist in den Generatoren neu oder Beta; die Bruch-Prüfung fehlt bis M7-11, weil `oasdiff` 3.1 nicht sicher kann; die große, von Hand gepflegte Spec und erzeugte Tabellen driften. | mittel | mittel | Versionen gepinnt; das Gate erzeugt und kompiliert alle Clients; Marker für erzeugte Blöcke mit Abgleich im Gate; Bruch-Prüfung mit Mutations-Fixtures in M7-11, bis dahin Review gegen 06 Abschnitt 13. | [06](spec/06-api.md), [ADR 0013](adr/0013-spec-first-openapi-31-und-asyncapi-30-normativ.md), M1-17, M2-23, M7-11 | M1 |
| RK-83 | GitHub oder Jira ändern Format oder Signatur ihrer Webhooks (Jira kündigt das für die Signatur-Methode an); die Referenz-Apps lehnen dann gültige Ereignisse ab. | mittel | niedrig | Apps lesen die Methode aus der Kopfzeile und lehnen Unbekanntes ab statt zu raten; Fixtures mit Quelle und Abrufdatum; eigene Version und Changelog je App, außerhalb des Servers. | [11 10](spec/11-erweiterungen.md#10-fertige-integrationen-als-beispiele), M2-49, M2-50 | M2 |

### Terminal

| ID | Risiko | Wahrscheinlichkeit | Auswirkung | Gegenmaßnahme | Verweis | Prüfung |
|---|---|---|---|---|---|---|
| RK-22 | I6 beruht auf einer Erklärung des Owners. Das Produkt kann die LLM-Anmeldung auf einem Rechner nicht prüfen; eine Abo-Anmeldung an einem unbekannten Ort oder eine, die jemand während einer Sitzung anlegt, bleibt unentdeckt. Andere Menschen nutzen sie nur, wenn die Trennung der Sitzungsbenutzer fällt (lokale Rechteausweitung), wenn ein Ort die Konten wieder verbindet (ein Ordner außerhalb der konfigurierten Pfade, den zwei Sitzungsbenutzer schreiben können, RK-88) oder wenn der Betreiber einen früher vergebenen Sitzungsbenutzer nach Zurückziehen und neuem Koppeln des Rechners oder nach Verlust des Zustandsordners einer anderen Person gibt; das prüfen dann weder Server noch Host-Dienst (24 Abschnitt 2.2). | niedrig | hoch | Ein Sitzungsbenutzer je Person und Rechner, nie geteilt, ohne Zuordnung kein Terminal (Ruling T12, SR12); keine geteilten Orte, Arbeitsordner und `CLAUDE_CONFIG_DIR` je Sitzungsbenutzer, HD11 für jeden konfigurierten Pfad (Ruling T13); Erklärung mit Step-up und Audit; als Zusatzprüfung `doctor` (HD6) fragt `claude auth status` und bekannte Anmeldeorte beim Start, vor jeder PTY und periodisch ab; bei Widerspruch kein Start und Ende aller Sitzungen; Profil `claude` mit kontrollierter Umgebung; Nicht-Owner nur auf eigenen Rechnern (SR10); bei `organization` eigener Schlüssel je Rechner mit Ausgabengrenze, neu nach einem Entzug. Restrisiko T-TM-08. | [24](spec/24-terminal.md) Abschnitte 2.2 und 7, [ADR 0068](adr/0068-terminal-ueber-host-dienst-mit-noise-strom-und-invariante-i6.md), T-TM-08, T-TM-13, TM-T29, M2-40, M2-42, D-W-62 | M2 |
| RK-23 | Der Host-Dienst läuft auf fremden Rechnern. Eine Lücke in Host-Dienst, Kopplung oder Rechteprüfung gibt eine Shell oder Rechte auf root-Niveau. Der Start der PTY als zweiter Benutzer ohne Admin-Rechte und ohne setuid ist nicht für jede Plattform belegt (Dienste unter macOS und Windows), und ConPTY verhält sich anders als eine Unix-PTY. | mittel | hoch | `doctor` verweigert den Start mit Admin-Rechten oder Gruppen auf root-Niveau; Sitzungsbenutzer ohne Leserecht auf die Schlüssel; nur ausgehende Verbindung; frischer Step-up je Öffnung; Einmal-Marke; ohne Beleg für eine Plattform kein Terminal dort; eigene Tests auf Windows; Konformitäts- und Chaos-Suite; Red Team für das Terminal in M2, M4, M5 und M6. | [24](spec/24-terminal.md) Abschnitt 2, [Bedrohungsmodell](security/threat-model.md) T-TM-03, T-TM-04, TM-T22, M2-42, M2-45, M2-48 | M2 |
| RK-24 | Ein bösartiger Server-Betreiber verteilt einen eigenen Rechner-Schlüssel und erlaubt Sitzungen; der Ende-zu-Ende-Strom schützt nicht gegen ihn. | niedrig | hoch | Pinning des Rechner-Schlüssels je Gerät mit harter Warnung; Audit und Meldung an alle Owner bei Neukopplung; Option `host_service.require_local_confirm`. Restrisiko T-TM-01. | [24](spec/24-terminal.md) Abschnitt 5.1, [ADR 0068](adr/0068-terminal-ueber-host-dienst-mit-noise-strom-und-invariante-i6.md) | M2 |
| RK-25 | Gestohlene Schlüssel des Host-Dienstes bleiben unbemerkt, solange der echte Rechner aus ist. | niedrig | hoch | Sperre bei einer zweiten lebenden Steuerverbindung; lokale Bestätigung für bemannte Rechner; Neukopplung. | [24](spec/24-terminal.md) Abschnitt 3.4, T-TM-05 | M2 |
| RK-26 | Dauerläufer einer Person (Hintergrundprozesse, Zeitpläne, Dienste) überleben ihren Entzug und laufen in ihrem Sitzungsbenutzer weiter; über eine lokale Rechteausweitung könnten sie Sitzungen anderer Sitzungsbenutzer erreichen, auch die eines Owners auf dem Rechner eines anderen Menschen. | niedrig | mittel | Ein Sitzungsbenutzer je Person, nie neu vergeben (T12), Hinweis der Konsole, ihn nach dem Entzug zu sperren; Nicht-Owner nur auf eigenen Rechnern (SR10); Prozessgruppe, cgroup oder Job Object je Sitzung; Helfer nicht ersetzbar (HD10); `kernel.yama.ptrace_scope` mindestens `1` unter Linux; Warnung vor dem Step-up für Owner auf dem Rechner eines anderen Menschen; Hinweis der Konsole nach dem Entzug; Wegwerf-Umgebungen ab V2. | [24](spec/24-terminal.md) Abschnitte 2.2, 6.5 und 8.4, T-TM-11, T-TM-12 | M2 |
| RK-27 | Die Konsole ist Angriffsfläche für offene Sitzungen: Browser-Erweiterung, XSS oder Cross-Site WebSocket Hijacking des Strom-Pfads. | mittel | hoch | Passkey mit Nutzerprüfung je Öffnung; CSP mit Trusted Types; kein HTML aus Ausgaben; Origin-Prüfung beim Upgrade, `SameSite=Strict`; ohne Sitzungs-Marke und Noise-Schlüssel keine PTY; Empfehlung eigenes Browser-Profil. Restrisiko T-TM-07. | [ADR 0070](adr/0070-terminal-in-der-web-konsole.md), [Bedrohungsmodell](security/threat-model.md) Abschnitt 5.16, M6-18 | M6 |
| RK-28 | Wenig gepflegte Bibliotheken: die Go-Bibliothek für Noise und der Terminal-Renderer `xterm`; dessen Semantik für Screenreader und das Verhalten unter Flutter Web sind nicht belegt. | mittel | mittel | Offizielle Testvektoren; Escape-Sperren vor dem Renderer mit eigenen Tests (TM-T14); Lizenz-Gate; Fork möglich; eigene Semantik; M4-35 prüft Stand und Sicherheitsmeldungen, M6-18 misst das Web. | [24](spec/24-terminal.md) Abschnitt 12, M2-38, M4-35, M6-18 | M2 |
| RK-29 | Ein Proxy ohne Hash-Routing lässt Terminal-Sitzungen bei mehreren api-Knoten scheitern. | mittel | mittel | Betriebsanforderung in 16; sichtbarer Fehler und Kennzahl für ungepaarte Relais; Test TM-T15. | [24](spec/24-terminal.md) Abschnitt 9, [16](spec/16-betrieb.md), M2-47 | M2 |
| RK-30 | Die eigene langlebige Abo-Anmeldung des Inhabers im Konto des Sitzungsbenutzers (Token aus `claude setup-token`) ist für interaktive Sitzungen nicht ausdrücklich belegt und könnte den Anbieterbedingungen widersprechen. Das Produkt selbst fasst das Token nicht an (D-W-17). | mittel | mittel | M2-45 prüft die Anbieter-Doku vor dem Bau; Owner-Frage [OQ-03](OPEN-QUESTIONS.md); Rückfall `/login` in einer Sitzung oder `organization`. | [24](spec/24-terminal.md) Abschnitt 7.4, M2-45 | M2 |
| RK-84 | Der Schutz des PTY-Helfers gegen Prozesse desselben Benutzers (Speicherschutz, Prüfung der Gegenstelle über die Prozess-ID des Dienstverwalters) ist nur für Linux belegt; unter macOS und Windows könnte ein Sitzungsbenutzer seinen Helfer anzapfen oder ersetzen und damit Sitzungen derselben Person mitlesen. Dieselbe Benutzerkennung kann außerdem die Daten ändern, die der Coding-Agent für HD6 liest (Anmeldung nur zwischen zwei Prüfungen), und ohne `host_service.claude_bin` bleibt der Schlüsselbund unter macOS ungeprüft; das schwächt seit Ruling T12 nur die Zusatzprüfung HD6, nicht die Grenze zwischen Personen, weil jede Person ihren eigenen Sitzungsbenutzer hat. | mittel | mittel | Anforderungen in 24 Abschnitt 2.2 und HD10; ein Sitzungsbenutzer je Person, nie geteilt (SR12); HD6 nur über `host_service.claude_bin` mit fester Suchliste, nie über den `PATH` des Sitzungsbenutzers, HD10 auch für Ordnerkette und Interpreter (24 Abschnitt 7.4); TM-T26, TM-T27 und TM-T28 je Plattform in M2-42 und M2-45. | [24](spec/24-terminal.md) Abschnitt 2.2 und 7.4, T-TM-08, T-TM-12, M2-42, M2-45, D-W-57 | M2 |
| RK-88 | Ein Ort, den zwei Sitzungsbenutzer erreichen, verbindet ihre Konten wieder: Einstellungen, Hooks oder `apiKeyHelper` des Coding-Agenten, `CLAUDE.md` oder ein Programm, das eine Person schreibt und die Sitzung einer anderen liest oder ausführt. Der Code der einen läuft dann im Konto der anderen und erreicht deren Dateien und Anmeldung. HD11 prüft nur konfigurierte Pfade und das Home samt Ordnerkette und Kettendateien, nicht jeden Ordner des Rechners; einen Eintrag aus einer Zeit, in der ein Ort offen war, findet es erst beim nächsten Start des Dienstes oder Aufruf von `doctor`, eine damals geänderte vorhandene Datei gar nicht; eine Datei, die eine spätere Fassung des Coding-Agenten zusätzlich aus der Ordnerkette lädt, erst nach Ergänzung der Liste der Kettendateien. | niedrig | hoch | Ruling T13: Arbeitsordner und `CLAUDE_CONFIG_DIR` je Sitzungsbenutzer (bei `personal` nur im Home), Index des Arbeitsordners nur in der eigenen, gemeldeten Liste, Programme samt Ordnerkette im Besitz eines vertrauenswürdigen Besitzers (unter Windows `SYSTEM`, `Administrators` oder `TrustedInstaller`), das Profil `claude` bei `organization` nur mit `host_service.claude_bin`; HD11 als eine Regel für jeden konfigurierten Pfad nach vollständiger Auflösung samt Ordnerkette und Kettendateien vor jeder PTY und periodisch, "schreibbar" mit dem Anlegen von Einträgen, unter Windows mit der Laufwerkswurzel nach `bothy-host install`; HD7 streicht einen Arbeitsordner bei zwei Sitzungsbenutzern aus beiden Listen; Installation ohne gemeinsame Ordner, Austausch über git; Betreiberschritt nach einem Treffer (16 Abschnitt 4.4); M2-42 vergleicht die Liste der Kettendateien mit der Doku des Coding-Agenten. | [24](spec/24-terminal.md) Abschnitt 2.2, T-TM-13, TM-T29, M2-42, M2-45, D-W-62, D-W-68, D-W-80 bis D-W-83 | M2 |
| RK-89 | Eine Person mit Terminal legt einen Rechner mit `none` oder `organization` für alle still, indem sie sich in ihrem eigenen Konto per Abo anmeldet (HD6 beendet alle Sitzungen, Ruling T2); ohne Hinweis auf das Konto sucht der Betreiber lange. Zweiter Weg: Eine Kettendatei dieser Person in einem Ordner auf dem Weg zu den Orten anderer (etwa `C:\.git` aus der Zeit vor `bothy-host install`) lässt HD11 für alle anderen dort scheitern. | mittel | mittel | Die Meldung von HD6 nennt jeden betroffenen Sitzungsbenutzer mit eigenem Eintrag, die Konsole die Person seiner Zuordnung, auch bei einem Treffer beim Start (der Host-Dienst meldet vor dem Beenden, `bothy-host doctor` nennt die Namen auf dem Rechner); ein Home, das eine Person öffnet, sperrt nur sie (HD11), nicht den Rechner; Betreiberschritt in 16 Abschnitt 4.4: Anmeldung im genannten Konto entfernen, oder Zuordnung beenden und den Sitzungsbenutzer mit `bothy-host session-user remove` aus dem Host-Dienst nehmen (Sperren allein reicht nicht, weil HD6 in jedem Namen aus `host_service.session_users` läuft); Nicht-Owner nur auf eigenen Rechnern (SR10); gegen den zweiten Weg: Ordner auf dem Weg für keinen Sitzungsbenutzer schreibbar (HD11), unter Windows schließt `bothy-host install` die Laufwerkswurzel, die Meldung von HD11 nennt jeden gesperrten Sitzungsbenutzer, der Betreiber entfernt den Eintrag (16 Abschnitt 4.4). Ehrlich offen: Der Stillstand selbst bleibt, bis der Betreiber handelt. | [24](spec/24-terminal.md) Abschnitte 2.2 und 7.4, T-TM-14, TM-T21, TM-T24, TM-T29 (i), M2-42, M2-45, D-W-64, D-W-67, D-W-80, D-W-88, D-W-91 | M2 |
| RK-90 | Die Prüfung der Einträge von HD11 geht durch das ganze Home eines Sitzungsbenutzers. Bei einem großen Home dauert sie länger als `host_service.entry_scan_timeout`; dann bekommt diese Person nach jedem Start des Host-Dienstes keine Sitzungen, bis der Betreiber den Wert erhöht oder das Home kleiner wird. Der Standardwert ist geschätzt, nicht gemessen. | mittel | niedrig | Die Prüfung läuft je Sitzungsbenutzer, und nur er wartet auf sie; die Meldung nennt ihn mit dem Code `entry_scan_timeout`, und 16 Abschnitt 4.4 nennt den Betreiberschritt; M2-42 misst die Dauer auf einem großen Home je Plattform und passt den Standardwert an. Ehrlich offen: Bis dahin kann der Wert für einzelne Personen zu klein sein. | [24](spec/24-terminal.md) Abschnitte 2.2 und 17, TM-T29 (e), M2-42, D-W-89 | M2 |

### Daten, Sync und Leistung

| ID | Risiko | Wahrscheinlichkeit | Auswirkung | Gegenmaßnahme | Verweis | Prüfung |
|---|---|---|---|---|---|---|
| RK-31 | Alle Schreibvorgänge mit Ereignissen reihen sich am Zähler `event_seq` auf; WAL-Flush und `NOTIFY` begrenzen den Durchsatz der ganzen Instanz. Kapazitätszahlen fehlen noch. | mittel | hoch | Zähler-Schritt zuletzt in der Transaktion; keine externen Aufrufe in Transaktionen; Lauf-Patches ohne Ereignis-Log; Leistungs-Gate `perf.event_commit_min_per_s`; Kennzahl für Sperr-Wartezeit; bei Bedarf neue ADR. | [ADR 0009](adr/0009-commit-geordnetes-ereignis-log-mit-globaler-sequenz.md), [ADR 0063](adr/0063-leistungs-gate-fuer-das-ereignis-log.md), M1-16, M1-72, M7-18 | M1 |
| RK-32 | Weitere Reihungen: Die Audit-Hash-Kette serialisiert auditpflichtige Aktionen; viele Zuschauer einer Live-Box erzeugen Last. | niedrig | niedrig | Nur auditpflichtige Aktionen schreiben, gleiche Ablehnungen zusammengefasst; Patches nur an fokussierte Threads und gedrosselt. | [05](spec/05-aktionen.md), [04](spec/04-sync-protokoll.md), M1-24, M2-10 | M1 |
| RK-33 | Eine Wiederherstellung holt Gelöschtes zurück; Löschungen nach DSGVO nach dem Zielzeitpunkt wären rückgängig, wenn Lösch-Journal und Sicherungsfrist nicht zusammenpassen. | mittel | hoch | Lösch-Journal; `bothy restore finalize` führt erledigte Löschaufträge erneut aus; Startprüfung `privacy.deletion_journal_retention` gegen die Sicherungsfrist; gemeinsame Tests SEC-T15 und DP-T07. | [20](spec/20-datenschutz.md), [16](spec/16-betrieb.md), M7-04, M7-07 | M7 |
| RK-34 | Nach einer Wiederherstellung überholt die neue Zeitlinie alte Cursor, und Clients überspringen still Ereignisse. Verkleinert durch die Sync-Epoche. | niedrig | hoch | `sync_epoch` mit `resync_required`; Test im Wiederherstellungs- und im Portabilitätstest. | [ADR 0059](adr/0059-sync-epoche-nach-wiederherstellung.md), [04](spec/04-sync-protokoll.md), M7-04 | M7 |
| RK-35 | In-Transaktions-Ereignisse machen den Kontrollfluss implizit; Abonnenten verlängern Transaktionen oder erzeugen Rückwärtskanten. | mittel | mittel | Registrierung nur an einer Stelle; Phasen-Wächter mit statischem Test; Richtungsprüfung; eigener Span je Abonnent. | [02](spec/02-architektur.md), M1-13 | M1 |
| RK-36 | River ist 0.x (Brüche möglich) und legt eigene Tabellen an. | mittel | mittel | Nur `platform/jobs` importiert River; Version gepinnt; eigenes Schema; Updates über Renovate mit Integrationstest. | [02](spec/02-architektur.md), M1-14 | M1 |
| RK-37 | Mehrere api-Knoten: Rate-Limits gelten je Knoten, der Präsenz-Stand für Benachrichtigungen liegt je Knoten, kurzlebige Ereignisse müssen alle Knoten erreichen. Folgen sind höhere Last, doppelte Pushes oder fehlende Live-Anzeigen. | mittel | mittel | Werte durch Knotenzahl teilen oder Affinität am Proxy; Port für einen gemeinsamen Präsenz-Stand; `NOTIFY` für kurzlebige Ereignisse; Fencing des Agenten-Verbrauchers über `holder_session_id`; Mehrknoten-Betrieb als eigenes Ticket. | [02](spec/02-architektur.md) Abschnitt 16, [13](spec/13-push-benachrichtigungen.md), M7-17 | M7 |
| RK-38 | Eine Änderung der Ausdrücke der Suchvektoren schreibt die Tabelle `messages` neu, mit Ausfallzeit bei großen Instanzen. | mittel | mittel | Wartungsfenster im Upgrade-Leitfaden mit Dauer aus dem Such-Lasttest. | [14](spec/14-suche.md), [Upgrade](operations/upgrade.md), M3-16 | M3 |

### Plattformdienste und Clients

| ID | Risiko | Wahrscheinlichkeit | Auswirkung | Gegenmaßnahme | Verweis | Prüfung |
|---|---|---|---|---|---|---|
| RK-39 | Erweiterungen der Apps (Notification Service Extension, Android-Push) erreichen einen Server im privaten Netz nicht; Mitteilungen zeigen dann nur den Platzhalter. | hoch | mittel | Platzhalter mit Kanalname aus lokalem Wissen; Bestätigungsquote je Plattform als Signal; Doku für Betreiber; Ende-zu-Ende-verschlüsselter Push-Inhalt als spätere Option. | [13](spec/13-push-benachrichtigungen.md), [ADR 0031](adr/0031-push-ohne-inhalt-inhalt-holt-die-app-selbst.md), [ADR 0056](adr/0056-native-erweiterungen-ohne-flutter-engine.md), M5-04, M5-05 | M5 |
| RK-40 | App und Erweiterungen erneuern Tokens gleichzeitig, oder ein Absturz fällt zwischen Rotation und Speichern; die Wiederverwendungs-Erkennung meldet dann ab. Verkleinert durch Kind-Sitzungen und Nachsicht. | niedrig | mittel | Kind-Sitzungen für Erweiterungen; Nachsicht `auth.refresh_reuse_grace` nur mit demselben Schlüssel; Test ID-T16. | [08](spec/08-identitaet.md), M1-34, M5-04 | M1 |
| RK-41 | Das Neukodieren der Vorschauen verliert Qualität oder Farbtreue (Farbprofil, Orientierung, CMYK, große GIFs). | mittel | mittel | Das Original bleibt mit verlustfrei entfernten Metadaten; Farbprofil und Orientierung Pflicht mit Tests; `file.image.max_pixels` und Zeitlimit. | [12](spec/12-dateien.md), [ADR 0047](adr/0047-bild-metadaten-verlustfrei-entfernen.md), M3-06 | M3 |
| RK-42 | Vorsignierte Upload-Adressen binden die Länge eines Teils eventuell nicht; ein Client legt mehr ab als angemeldet. | mittel | mittel | Modus `direct` ist Standard; der vorsignierte Modus erst nach Integrationstest je S3-Dienst; Größe und Prüfsumme immer am Ende. | [12](spec/12-dateien.md), M3-02, M3-03 | M3 |
| RK-43 | Ein langer Ausfall des Virenscanners hält Anhänge und Agenten-Aufrufe mit Anhängen zurück. | mittel | mittel | Statuskomponente `degraded`; Alarm nach `file.scan.pending_alert_after`; nach `file.invocation_hold_max` geht der Aufruf mit Anhangzustand, ohne Inhalt. | [12](spec/12-dateien.md), [ADR 0033](adr/0033-malware-scan-der-uploads-als-getrennter-dienst.md), M3-04, M3-07 | M3 |
| RK-44 | Die Barrierefreiheit von Flutter auf macOS reicht nicht (Vorstudie, nicht selbst geprüft). | mittel | hoch | VoiceOver-Prüfung vor der ersten Auslieferung für macOS; Semantik-Tests je Komponente. | [01](spec/01-produkt.md), M4-32 | M4 |
| RK-45 | Lizenz und Volltextsuche (FTS5) der Verschlüsselungs-Bibliothek der lokalen Datenbank sind nicht geprüft. | niedrig | mittel | Prüfung im Ticket der lokalen Datenbank; Lizenz-Gate; Rückfall auf eine andere Quelle. | [ADR 0029](adr/0029-client-zustand-lokale-datenbank-und-offline-warteschlange.md), M4-12 | M4 |
| RK-46 | Ende-zu-Ende-Tests unter Windows decken Systemdialoge nicht ab (Mitteilungen, Teilen). | hoch | niedrig | Integrationstests plus manuelle Prüfliste je Release. | [15](spec/15-client-architektur.md), M4-34 | M4 |
| RK-47 | Die breiten Schriftformen kürzen lange deutsche Wörter in dichten Listen. | mittel | niedrig | Golden-Tests auf Deutsch mit großer Textskalierung; Kürzung mit Tooltip; die kompakte Dichte behält die Schriftgröße. | [Typografie](design/typography.md), M4-07 | M4 |
| RK-48 | Die erzeugte Dart-Datei der Tokens ist nur gegen Stellvertreter der Flutter-Typen geprüft; Akzent-Varianten je Instanz sind noch nicht erzeugt. | hoch | niedrig | M4-03 erzeugt und prüft die Varianten, M4-04 kompiliert die Datei mit dem echten SDK; bis dahin gilt nur der Standard-Akzent. | [ADR 0030](adr/0030-design-tokens-als-eine-quelle-mit-generator.md), M4-03, M4-04 | M4 |

### Betrieb, Release und Werkzeuge

| ID | Risiko | Wahrscheinlichkeit | Auswirkung | Gegenmaßnahme | Verweis | Prüfung |
|---|---|---|---|---|---|---|
| RK-49 | Neue Attribute oder Logtexte bringen Inhalte in die Telemetrie. | mittel | hoch | Allowlist im Collector, im Gate gegen den Signalkatalog geprüft; konstante Lognachrichten per Linter; Kanarien-Test über alle Ausgänge. | [17](spec/17-observability.md), [ADR 0034](adr/0034-opentelemetry-ueber-collector-lokaler-standard-stack.md), M1-04, M1-69 | M1 |
| RK-50 | pgBackRest mit TLS zwischen Datenbank und Sicherungsdienst und mit eigenem Image ist aufwendig und fehleranfällig. | mittel | hoch | Integrationstest (Archivierung, Sicherung, Wiederherstellung, Prüfung) auf amd64 und arm64; Rückfall: Repository auf gemeinsamem Volume plus Spiegel an das Off-site-Ziel. | [ADR 0058](adr/0058-sicherung-mit-pgbackrest-objektsicherung-und-restore-test.md), M7-01, M7-02, M7-03 | M7 |
| RK-51 | Ein Host trägt alles: Fällt er aus, steht die Instanz bis zur Wiederherstellung; die Dauer hängt an der Geschwindigkeit des Off-site-Ziels. | niedrig | hoch | Der Wiederherstellungstest misst gegen `dr.rto_target`; lokales Repository für schnelle Wiederherstellung auf einen Zeitpunkt; Notfall-Runbook. | [Disaster Recovery](operations/disaster-recovery.md), T-DS-01, M7-03 | M7 |
| RK-52 | Die Container-VM auf einem Mac hat zu wenig Speicher für den Virenscanner oder kein `/dev/net/tun` für den Tailscale-Begleiter; Docker oder Compose sind zu alt für die genutzten Funktionen. | mittel | mittel | Mindestwerte und Versionsprüfung im Installationsschritt (`docker compose config`); Profil `scan` abschaltbar; Portabilitätstest; Rückfall Reverse-Proxy mit OIDC. | [Installation](operations/install-compose.md), M7-14 | M7 |
| RK-53 | WebSocket über `tailscale serve` ist nicht dokumentiert. | niedrig | hoch | Der Portabilitätstest mit Profil `tailscale` prüft das Gateway ausdrücklich. | [Tailscale](operations/tailscale.md), M7-14 | M7 |
| RK-54 | Namen und Übersetzungsregeln ändern sich zwischen Versionen von Collector und Prometheus; Alarm-Schwellen driften von den Konfigurationswerten. | mittel | mittel | Versionen gepinnt; `otelcol validate`, `promtool` und Namens-Test im Gate; Check "Schwelle gleich Konfigurationswert". | [Alarme](observability/alerting.md), M1-04 | M1 |
| RK-55 | Azure Monitor: Ob die native OTLP-Aufnahme allgemein verfügbar ist und wie sie explizite Histogramme darstellt, ist offen. | mittel | niedrig | Overlay als Option markiert; Prüfung gegen eine echte Umgebung nur mit Freigabe der Kosten. | [Backends](observability/backends.md) | M7 |
| RK-56 | Herausgeber-Modell: Jede Organisation braucht Konten, Signatur und Build-Kette für die Apps; SmartScreen warnt anfangs. | hoch | mittel | Vorlage `distribution.yaml`; Release-Runbook; Owner-Frage [OQ-08](OPEN-QUESTIONS.md). | [ADR 0037](adr/0037-app-verteilung-und-auto-update-kanaele.md), [18](spec/18-release.md), M7-12, M7-13 | M7 |
| RK-57 | Die Release-Kette (Build, Signatur, Attestation, Multi-Arch) ist ungetestet, solange es keine Komponenten und kein Remote gibt; die Form der Versions-Tags für das Go-Modul unter `services/server` weicht von der Begründung in 18 ab. | mittel | mittel | Probe-Release auf einem Test-Repository; der Komponenten-Vertrag macht fehlende Teile zu Fehlern; M7-10 klärt die Tag-Form und korrigiert die Begründung. | [18](spec/18-release.md), [ADR 0060](adr/0060-release-automatisierung-im-monorepo.md), M7-10 | M7 |
| RK-58 | Ergänzungen des Kern-Vertrags fehlen oder werden anders gebaut; Briefs, die Kapitel byte-genau prüfen, verschieben sich bei weiteren Nachträgen. Verkleinert: Alle Anträge der Teams sind entschieden. | niedrig | mittel | Fehlt etwas, endet das Ticket mit `BLOCKED` (P-11); M1-02 friert den Aktionskatalog erst am Basis-Commit ein; Angleichung nach P-24. | [Plan](PLAN.md) Abschnitt 19, M1-02, M1-03 | M1 |
| RK-59 | Der Pre-Push mit vollem `make check` dauert lange; Entwickler umgehen ihn mit `--no-verify`. | mittel | mittel | CI prüft identisch; Umgehung ist ein dokumentierter Prozessfehler; Komponenten-Gates überspringen ohne Code. | [Push-Disziplin](quality/push-discipline.md), [ADR 0038](adr/0038-lokale-gates-zuerst-ein-befehl-identisch-zu-ci.md) | M1 |
| RK-60 | Doku-Werkzeuge: Material for MkDocs ist im Wartungsmodus, die Sicherheitszusage endet frühestens am 2026-11-11, und Zensical ist noch vor 1.0; Mermaid lädt in der Site aus einem CDN; das ELK-Layout rendert dort eventuell nicht. | mittel | niedrig | Versionen im Lock gepinnt; gerenderte SVGs mit dem Design-Thema im Repository (`make diagrams`); Wechsel nach den drei Auslösern aus ADR 0045 (D-W-42). | [ADR 0045](adr/0045-dokumentations-site-generator.md), V3-14 | M1 |
| RK-61 | Akzeptierte Befunde des OSV-Scanners für die Node-Werkzeuge laufen zu einem festen Datum ab (`tools/node/osv-scanner.toml`) und machen `make osv` rot. | hoch | niedrig | Gewollt: Der Ablauf erzwingt eine Neubewertung; Renovate schlägt Korrekturen vor. | [Gates](quality/gates.md) | M1 |
| RK-62 | Die eingebauten Profilnamen von SonarQube ändern sich; der Abgleich des Profils bricht ab. | mittel | niedrig | Eltern-Profil konfigurierbar; der Abgleich meldet den fehlenden Namen, statt still zu wählen; Prüfung nach jedem Update. | [ADR 0040](adr/0040-sonarqube-lokal-und-cloud-mit-gleichem-profil-und-gate.md), M0-07 | M1 |
| RK-64 | `go-arch-lint` wird nicht mehr gepflegt. | niedrig | niedrig | Die Konfiguration wird aus dem Modulregister erzeugt; der Generator kann stattdessen depguard-Regeln erzeugen. | [ADR 0050](adr/0050-modulregister-als-quelle-fuer-architektur-gate-und-tabellenbesitz.md) | M1 |
| RK-65 | Fehlt in einer Ereignis-Nutzlast die Sichtbarkeit des Repositorys, laufen die öffentlichen Jobs (CodeQL, Scorecard, Pages) nicht. | niedrig | niedrig | Fail-safe: kein Lauf; Probelauf in der Checkliste Veröffentlichen. | [Veröffentlichen](quality/public-switch.md), [ADR 0039](adr/0039-nur-github-gehostete-runner-oeffentlich-ab-dem-ersten-push.md) | V1 |
| RK-82 | Der Einrichtungsassistent wird mit `curl` geladen und direkt an die Shell weitergereicht; ein manipuliertes Skript oder Bündel liefe mit den Rechten des Betreibers. | niedrig | hoch | Nur HTTPS, Version und SHA-256 des Bündels im Skript eingebaut, Skript signiert und in `SHA256SUMS` wie jedes Artefakt; Prüfweg vor dem Ausführen dokumentiert; kein `sudo`, keine Paketinstallation; Handweg gleichwertig. | [16 3.1](spec/16-betrieb.md#31-installation-in-einem-befehl-einrichtungsassistent), M7-29 | M7 |
| RK-85 | Der Review-Nachweis vor dem Push ist eine lokale Datei; eine Bau-Sitzung kann sie selbst schreiben, statt einen unabhängigen Prüfer zu starten. | mittel | hoch | Der Hook prüft Datei und Urteil, nicht die Unabhängigkeit; der Bericht steht im Text des Pull Requests (Abschnitt "Review report") und wird vom Gesamt-Review der Etappe gesehen; der Plan verlangt eine frische Sitzung ohne Bau-Kontext (PLAN 3.9). Ehrlich offen: Unabhängigkeit ist nicht maschinell belegbar. | [red-team-gate.md](quality/red-team-gate.md), R-QA-13 | M1 |

### Statistik, Visualisierung und Schaufenster

| ID | Risiko | Wahrscheinlichkeit | Auswirkung | Gegenmaßnahme | Verweis | Prüfung |
|---|---|---|---|---|---|---|
| RK-66 | Rückschluss auf einzelne Menschen über Läufe in Kanälen mit nur einem Menschen (Leistungskontrolle). | mittel | mittel | Keine Auslöser und keine Personen-Zähler in Statistik und Live-Strom; Schalter `stats.live_enabled`; Eintrag in der Mitbestimmungs-Tabelle. | [25](spec/25-visualisierung.md) Abschnitt 4, [20](spec/20-datenschutz.md), T-VZ-03 | M6 |
| RK-67 | Die Minuten-Aggregate der eigenen Messung wachsen bei vielen Kanälen und Agenten stark. | mittel | mittel | Nur Zeilen mit Werten; keine Werkzeugnamen; Tagespartitionen mit Abhängen nach `retention.stats_minutes`; Lasttest misst Zeilen je Tag und Flush-Dauer. | [25](spec/25-visualisierung.md) Abschnitt 3.5, [03](spec/03-datenmodell.md), M6-05 | M6 |
| RK-68 | Eigene Messung und Prometheus zeigen verschiedene SLO-Werte (Fehler am Proxy fehlen im Prozess, verlorene Zähler bei Absturz); Betrachter misstrauen der Ampel. | mittel | niedrig | Etikett "eigene Messung"; gleiche Schwellen aus den Sloth-Dateien per Generator und Check (VZ-T03); Verluste gezählt; Alarme bleiben in Prometheus. | [25](spec/25-visualisierung.md) Abschnitt 3, M6-11 | M6 |
| RK-69 | Die optionale Grafana-Einbettung in der Konsole scheitert im Browser oder öffnet Clickjacking, wenn der Betreiber den Proxy falsch konfiguriert. | mittel | mittel | Standard aus; `bothy config check` warnt bei fremder Site; Test SEC-T19 mit Test-Proxy; Doku der Kopfzeilen; Apps betten nie ein. | [25](spec/25-visualisierung.md) Abschnitt 9, [ADR 0069](adr/0069-visualisierung-auf-zwei-ebenen-ohne-iframes-in-apps.md), T-VZ-01 | M6 |
| RK-70 | Zwei Umsetzungen derselben Ansichten (Dart in den Apps, TypeScript im Plugin) laufen optisch und inhaltlich auseinander; eigene Grundformen kosten mehr als eine Bibliothek. | mittel | niedrig | Tokens für beide aus einer Quelle erzeugt; Plugin bewusst auf Metrik-Ansichten beschränkt; Review-Liste der Komponenten; R-VZ-10. | [25](spec/25-visualisierung.md) Abschnitt 8, [Komponenten](design/components.md), M6-12, M7-25 | M6 |
| RK-71 | Das unsignierte Grafana-Plugin erzeugt Warnungen und läuft nicht in Grafana Cloud; der Kiosk-Modus von Grafana ist nicht gegen die Doku belegt. | hoch | niedrig | Lokal provisioniert, freigegeben nur mit dieser Plugin-ID; Owner-Frage [OQ-20](OPEN-QUESTIONS.md); M7-25 prüft den Kiosk-Modus gegen die eingesetzte Grafana-Version. | [25](spec/25-visualisierung.md) Abschnitt 8, M7-25, M7-26, V3-17 | M7 |
| RK-72 | Bildschirmfotos und Video der Apps für macOS und iOS entstehen nur auf einem Mac; ohne erreichbaren Mac verzögert sich das Schaufenster. | mittel | niedrig | Die Aufnahmen sind gegen die Demo-Instanz geskriptet und mit Manifest reproduzierbar; nie gegen echte Daten. | [Plan](PLAN.md) Abschnitt 13, M7-27, M7-28 | M7 |

### Migration, Veröffentlichung und Dev-Umgebung

| ID | Risiko | Wahrscheinlichkeit | Auswirkung | Gegenmaßnahme | Verweis | Prüfung |
|---|---|---|---|---|---|---|
| RK-73 | Fehlt beim Import-Bot die Berechtigung für Nachrichteninhalte, entsteht ein Import voller leerer Nachrichten. | mittel | mittel | Die Vorabprüfung bricht in diesem Fall ab. | [22](spec/22-import-migration.md), M8-04 | M8 |
| RK-74 | Halbfertige Agenten-Ausgaben werden während des Parallelbetriebs kopiert. | mittel | niedrig | Ruhe-Regel mit Rückblick; Stichproben. | [22](spec/22-import-migration.md), M8-05 | M8 |
| RK-75 | Kosten übernommener Sitzungen werden doppelt gebucht, weil die Laufzeit kumulierte Summen meldet. | hoch | niedrig | Kosten-Basis je Sitzung (`cost_baseline_micros`); im persönlichen Modus sind Beträge nur Information. | [22](spec/22-import-migration.md), [03](spec/03-datenmodell.md), M8-06 | M8 |
| RK-76 | Viele Fehlerantworten der Quelle führen zu einer Sperre der Adresse, die auch andere Bots auf demselben Rechner trifft. | niedrig | hoch | Keine Wiederholung bei 401 und 403; Kanal nach 403 meiden; Wartezeiten bei Rate-Limits gedeckelt; eigener Import-Bot. | [22](spec/22-import-migration.md), [ADR 0043](adr/0043-discord-import-nur-lesen-trockenlauf-herkunftsschluessel.md), M8-01 | M8 |
| RK-77 | Mit dem ersten Push wird die Historie sichtbar: alte Beispielwerte, die gitleaks meldet, und interne Kennungen in alten Commits. | hoch | hoch | Historien-Scans `make denylist-history` und `make gitleaks GITLEAKS_LOG_OPTS=--all`; veröffentlicht wird nur ein Commit ohne Eltern, die Planungshistorie bleibt privat (Schritt 2 der Checkliste, D-W-93), und die Scans laufen in einem frischen Klon dieses Commits (Schritt 3); `git filter-repo` (M0-08, D-W-48) nur, wenn doch eine Historie veröffentlicht wird; Red Team vor dem Umschalten; nach dem Umschalten prüft `denylist-main.yml` jeden Push auf `main`. Die Commit-Identität ist entschieden ([Entschieden](OPEN-QUESTIONS.md#entschieden)). | [Öffentlich-Schalten](quality/public-switch.md), [ADR 0042](adr/0042-deny-list-scanner-mit-liste-aus-dem-ops-repo.md), M0-08, V1-04 | M0 |
| RK-78 | Der Arbeitsname kollidiert nach der Veröffentlichung mit einer Marke. | niedrig | hoch | Name nur an den Stellen aus ADR 0044, per Test gelistet; Fachrecherche vor der Veröffentlichung ([OQ-16](OPEN-QUESTIONS.md)). | [ADR 0044](adr/0044-arbeitsname-und-branding-als-konfiguration.md), V1-06 | V1 |
| RK-79 | Platzhalter `OWNER` bleiben nach dem Umschalten in Dateien stehen. | mittel | niedrig | V1-03 ersetzt sie; ein Test findet jeden übrigen Treffer. | [Plan](PLAN.md) Abschnitt 15, V1-03 | V1 |
| RK-80 | Die Dev-Abschottung wird auf einen eigenen Benutzer im selben Betriebssystem verkürzt; gemeinsamer Kernel und gemeinsames HOME teilen Rechte und LLM-Anmeldungen. | niedrig | hoch | Abschottung nur mit Hypervisor-Grenze (Ruling L20); Schutzregel SR3; Red Team P3 je Umgebungsart. | [21](spec/21-dev-isolation.md), [ADR 0046](adr/0046-mehrbenutzer-dev-umgebung-nur-mit-abschottung.md), V2-05 | V2 |
| RK-81 | Apple begrenzt laufende macOS-VMs je Mac; abgeschottete macOS-Umgebungen reichen auf einem Mac-Host nur für wenige Personen. | hoch | niedrig | Linux-VM je Person als Standard, macOS-VM nur bei Bedarf, Cloud-VM gleichwertig (R-DV-02). | [21](spec/21-dev-isolation.md) | V2 |
| RK-87 | Gerenderte Diagramm-SVGs passen nicht mehr zu ihrer Quelle, weil jemand eine `.mmd` ändert und nicht neu rendert; ein Byte-Vergleich ist wegen der Browser-Version nicht möglich. | niedrig | niedrig | `render_diagrams.py --write` stempelt jede SVG mit dem SHA-256 von `.mmd` und Thema; `make diagrams` meldet jede SVG ohne passenden Stempel als veraltet (D-W-56), dazu Thema, Seitenverhältnis und ERDs gegen 03. Rest: eine von Hand bearbeitete SVG mit altem Stempel; Review. | [quality/gates.md](quality/gates.md), D-W-39, D-W-56 | M0 |

## Erledigt

Risiken aus den Berichten, die eine spätere Entscheidung beseitigt hat.

| Risiko | Erledigt durch |
|---|---|
| Die Notification Service Extension braucht den DPoP-Schlüssel ohne Biometrie. | Zwei Geräteschlüssel: DPoP ohne Bestätigung je Signatur, Step-up mit Bestätigung ([08](spec/08-identitaet.md), Ruling L10). |
| Der Owner empfindet die Palette als zu dunkel oder zu einfarbig. | Gestaltung vom Owner abgenommen ([Offene Fragen](OPEN-QUESTIONS.md#entschieden)). |
| Konfigurationsnamen überschneiden sich zwischen Kapiteln. | Eine Quelle je Name; die [Konfigurationsreferenz](operations/configuration.md) wird erzeugt und lehnt doppelte Namen ab. |
| Beispiel-Secrets in `openapi.yaml` und veralteter ADR-Index lassen die Gates scheitern. | Beispielwerte im Token-Format ohne geheimnisähnliche Werte; ADR-Index neu erzeugt und im Gate geprüft. |
| Die Deny-Liste blockiert jeden Commit wegen der Commit-Identität. | Die gewählte Identität ist nur in den Autor- und Committer-Feldern erlaubt (Owner-Entscheidung, siehe [Entschieden](OPEN-QUESTIONS.md#entschieden)). |
| Neue Diagramme sind nicht im Repository und werden von `make diagrams` nicht erfasst. | Committet; `make diagrams` rendert sie. |
| Beantragte Kern-Vertrag-Ergänzungen (Werkzeugklasse, Dev-Umgebungen, Auftragsbindung, Vorschau der Rechte) werden nicht übernommen. | Übernommen; Rest siehe RK-58. |
| RK-63 und RK-86: Risiken aus dem Betrieb eigener Runner und aus Läufen im privaten Zustand. | Nur GitHub-gehostete Runner, das Repository ist ab dem ersten Push öffentlich ([ADR 0039](adr/0039-nur-github-gehostete-runner-oeffentlich-ab-dem-ersten-push.md), D-W-92). |
