---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
consulted: Builder-Check (frischer Prüfer)
informed: alle Teams
---

# Terminal über einen Host-Dienst mit Noise-Ende-zu-Ende-Strom und Invariante I6

## Context and Problem Statement

Der Owner verlangt ein Terminal als Pflichtbestandteil (Nachtrag vom 2026-10-10): Shell und
Sitzungen eines Coding-Agenten auf angebundenen Rechnern, in Desktop, Handy und Web-Konsole, mit
eigenem Recht je Rechner, erneuter Passkey-Bestätigung beim Öffnen, Zugang nur über ein privates
Netz oder einen Ende-zu-Ende-Tunnel, Audit, Leerlauf-Abbruch und einem eigenen Benutzer ohne
Administratorrechte auf dem Rechner. Das Produkt darf kein Tailscale voraussetzen (Anforderung 21),
der Server ist Autorisierungspunkt (B7), Agenten-Inhalte und Zugangsdaten bleiben aus dem Server
heraus (B10, I5). Ein Terminal erlaubt jede Nutzung der LLM-Anmeldung des Rechners; Anthropic
erlaubt Abo-Anmeldungen nur für die eigene Nutzung
(https://code.claude.com/docs/en/legal-and-compliance, abgerufen 2026-10-10 [belegt]). Wie kommt
ein Terminal sicher vom Rechner auf das Gerät, und wer darf es öffnen?

## Decision Drivers

* Kein eingehender Port auf dem Rechner; funktioniert hinter NAT, im privaten Netz und in der Cloud.
* Der Server entscheidet jede Öffnung, liest aber keinen Inhalt.
* Geprüfter, kleiner Standard mit öffentlichen Testvektoren statt eigener Kryptografie.
* Keine Hintertür am Rechte-Modell vorbei; Agenten und Apps öffnen nie ein Terminal.
* Persönliche Abos nur für ihren Inhaber, auch auf Rechnern mit mehreren Ownern.
* Ehrliches Schutzziel: was die Verschlüsselung leistet und was nicht.

## Considered Options

* Host-Dienst mit ausgehender WebSocket-Verbindung, Server als Relais, Noise `NK` Ende-zu-Ende
* SSH zum Rechner über das private Netz (Tailscale oder VPN), Server gibt nur Zugangsdaten aus
* Host-Dienst mit TLS-Strom, den der Server terminiert und weiterleitet (kein Ende-zu-Ende)
* WebRTC-Datenkanal zwischen Gerät und Rechner mit Signalisierung über den Server

## Decision Outcome

Gewählt: "Host-Dienst mit ausgehender WebSocket-Verbindung, Server als Relais, Noise `NK`", weil
nur diese Option ohne eingehenden Port und ohne Tailscale-Pflicht auskommt, den Server zur
einzigen Prüfstelle macht und den Inhalt trotzdem vor Relais, Logs und Datenbank schützt.
Normativ: [24-terminal.md](../spec/24-terminal.md).

1. **Host-Dienst** `bothy-host` (eigenes Go-Binary, `services/host`) läuft als Benutzer ohne
   Administratorrechte, prüft das selbst (`doctor`) und spricht nur ausgehend mit dem Server. Die
   Sitzungen laufen als Sitzungsbenutzer ihrer Person (Punkt 7), der die Schlüssel des Rechners
   nicht lesen und seinen PTY-Helfer (Systemdienst, HD10) nicht ersetzen kann; eine zweite lebende Steuerverbindung
   mit denselben Schlüsseln sperrt den Rechner. Bei `personal` fasst der Host-Dienst keine
   Abo-Anmeldung an (D-W-17, D-W-18).
2. **Noise** `Noise_NK_25519_ChaChaPoly_SHA256` (Revision 34, https://noiseprotocol.org/noise.html,
   abgerufen 2026-10-10 [belegt]). Das Gerät kennt den statischen Schlüssel des Rechners aus der
   Kopplung. Prolog mit Protokollversion, Sitzungs-ID und Rechner-ID.
3. **Sitzungs-Marke** im ersten Handshake-Paket: EdDSA-JWS des Servers, genau einmal gültig, an den
   ephemeren Schlüssel des Geräts gebunden (`cek`). Die PTY startet erst nach der ersten gültigen
   Transportnachricht. Grund: Die erste Nachricht von `NK` ist laut Spezifikation wiederholbar
   (Abschnitt 7.7 [belegt]).
4. **Rechte**: eigenes Ziel `host` mit nur dem Zusatzrecht `terminal`, nur für Menschen und
   Gruppen; Owner implizit. Schutzregel SR10: Nicht-Owner nur auf dem eigenen Rechner, der
   Server-Host nur für Owner.
5. **Invariante I6**: Bei `llm_credentials = personal` öffnet nur der Inhaber der Anmeldung ein
   Terminal, auch kein anderer Owner; nur eine Einzel-Freigabe an ihn. Transaktional geprüft und bei
   jedem Öffnen.
6. **Step-up** je Öffnung frisch und verbraucht.
7. **Ein Sitzungsbenutzer je Person** (Ruling T12, siehe unten): Jede Person hat auf jedem Rechner
   ihren eigenen Sitzungsbenutzer, nie geteilt; ohne Zuordnung kein Terminal (SR12).

Abgrenzung zu [ADR 0024](0024-backend-richtlinie-personal-organization-mit-invariante.md) und
Ruling L22: I1 kennt keine Owner-Ausnahme, weil Owner auf jedem Ziel `trigger` erben und ein Agent
über viele Ziele erreichbar ist. Ein Terminal ist Zugriff auf genau einen Rechner, keine vererbte
Stufe. Ohne Owner-Ausnahme könnte eine Instanz mit zwei Ownern keinen persönlichen Rechner haben.
Die Ausnahme schränkt Owner ein und erweitert nichts; L22 bleibt für I1 unverändert.

### Ergänzung 2026-10-11: ein Sitzungsbenutzer je Person und Rechner (Ruling T12)

Bis zu dieser Ergänzung liefen alle Sitzungen eines Rechners unter einem gemeinsamen
Sitzungsbenutzer. "Keine fremde Abo-Anmeldung" hing dann an HD6, einer Prüfung im Konto genau
dieses Benutzers. Drei Kritik-Runden fanden dieselbe Fehlerklasse: Das Ergebnis ließ sich aus dem
Konto fälschen (Programm im `PATH`, K2-19), war ohne `host_service.claude_bin` unter macOS
abgeschaltet (K3-01) oder über einen Interpreter wieder fälschbar (K3-06). Eine Anmeldung, die eine
Person mit `/login` anlegte, nutzte dann jede spätere Sitzung anderer Menschen.

Entscheidung: Die Fehlerklasse wird an einer Stelle behoben, nicht je Fund.

* Der Server hält je Rechner die Zuordnung Person zu Sitzungsbenutzer (`host_session_users`); der
  Owner legt sie bei der Bestätigung oder mit `host.manage` an. Ohne gültige Zuordnung bekommt eine
  Person kein Terminal, auch ein Owner nicht (SR12, `terminal_denied` mit `reason: no_session_user`).
* Ein Sitzungsbenutzer gehört höchstens einer Person, auch nach dem Ende der Zuordnung; der Name
  bleibt für den Rechner belegt (`409 conflict` mit `reason: session_user_taken`).
* Die Sitzungs-Marke nennt den Sitzungsbenutzer (`osu`); der Host-Dienst startet die PTY nur im
  Helfer dieses Benutzers und bindet jeden Sitzungsbenutzer an die erste Person, die ihn bekam.
* HD6 bleibt als Zusatzprüfung und Hinweis (Ruling T2 gilt weiter: Widerspruch zur Erklärung
  verhindert den Start). Ihr Ergebnis ist nicht mehr die Grenze; ein leerer `host_service.claude_bin`
  schwächt die Durchsetzung nicht.
* I6, SR10 (Ruling T1) und die Owner-Ausnahme (Ruling T2) bleiben unverändert und gelten zusätzlich.
* Der Host-Dienst meldet die Namen seiner Sitzungsbenutzer (`hosts.reported_session_users`); der
  Owner ordnet nur gemeldete Namen zu (sonst `400 validation_failed`), und eine Zuordnung wirkt
  nur, solange der Rechner ihren Namen meldet (D-W-63). So fällt ein Tippfehler bei der Zuordnung
  auf und nicht als Manipulationswarnung `4104` beim Öffnen.

Verworfen: HD6 bei leerem `host_service.claude_bin` unter macOS als Fehler werten (Vorschlag aus
K3-01). Grund: Das schließt nur diese eine Instanz; jede weitere Lücke im Orakel (Interpreter,
Module, Daten derselben Benutzerkennung) bliebe eine Lücke in der Grenze. Getrennte Konten machen
die Regel unabhängig von jeder Prüfung wahr. Die Trennung über Benutzerkennungen ist keine
vollständige Grenze zwischen Personen (ADR 0046); darum gilt SR10 weiter.

### Ergänzung 2026-10-11: keine geteilten Orte (Ruling T13)

Getrennte Sitzungsbenutzer trennen Personen nur, wenn kein Ort ihre Konten wieder verbindet. Kritik
Runde 4 fand zwei solche Orte: Arbeitsordner des Profils `claude` als eine Liste für alle
Sitzungsbenutzer (Projekt-Einstellungen mit Hooks und `apiKeyHelper`, `CLAUDE.md` in der
Ordnerkette laufen dann im Konto einer anderen Person) und einen `CLAUDE_CONFIG_DIR` ohne festen
Besitzer (ein `/login` darin erreichte alle). Das ist dieselbe Fehlerklasse wie vor T12.

Entscheidung: eine Regel und eine Prüfung statt eines Flickens je Ort.

* Auf einem Rechner gibt es keinen Ort, den ein Sitzungsbenutzer schreiben und ein anderer lesen
  oder ausführen kann, wenn er Code, Einstellungen oder Anmeldungen trägt.
* Arbeitsordner stehen je Sitzungsbenutzer in `host_service.claude_workdirs` und gehören ihm; ein
  Ordner bei zwei Sitzungsbenutzern fällt aus beiden Listen (HD7); der Index des Arbeitsordners
  zählt nur in seiner gemeldeten Liste (Claim `wdi`); der `CLAUDE_CONFIG_DIR` liegt bei
  `organization` fest in seinem Home und bei `personal` nur im Home; Programme gehören samt
  Ordnerkette einem vertrauenswürdigen Besitzer (`root`, unter Windows `SYSTEM`, `Administrators`
  oder `NT SERVICE\TrustedInstaller`); das Profil `claude` startet bei `organization` genau
  `host_service.claude_bin`.
* Die doctor-Prüfung HD11 prüft das mit einer Regel für jeden konfigurierten Pfad und das Home
  samt Ordnerkette und den Kettendateien darin (`CLAUDE.md`, `CLAUDE.local.md`, `AGENTS.md`,
  `.mcp.json`, `.claude`, `.git`: was Coding-Agent und git aus den Ordnern über dem Arbeitsordner
  laden), vor jeder PTY und periodisch; ein Verstoß sperrt nur den betroffenen Sitzungsbenutzer,
  an einem Ort für alle den ganzen Host-Dienst. Jeder Pfad wird vorher vollständig aufgelöst, die
  Orte eines Sitzungsbenutzers von seinem PTY-Helfer; ein Pfad, der sich nicht auflösen lässt,
  gilt als nicht vorhanden. "Schreibbar" schließt bei Ordnern das Anlegen von Einträgen ein, bei
  Dateien das Anhängen. Unter Windows entfernt `bothy-host install` nach Bestätigung das Recht
  aller angemeldeten Benutzer, in der Wurzel eines Laufwerks Ordner anzulegen, ohne die Rechte
  der Ordner darunter zu ändern (24 Abschnitt 2.2 Punkt 6, D-W-80, D-W-83, D-W-84, D-W-86,
  D-W-87).
* Gemeinsamer Code wird über git ausgetauscht, nie über einen gemeinsamen Ordner.

Verworfen: je Ort eine eigene Prüfung (HD7 für Besitz der Arbeitsordner, HD9 für fremde
Arbeitsordner, wie die Kritik vorschlug). Grund: Jeder neue Ort bräuchte wieder eine eigene
Prüfung und würde wieder einzeln gefunden. Grenze, ehrlich: HD11 sieht Ordner außerhalb der
Konfiguration nicht, etwa einen gemeinsamen Ordner, in dem eine Person in ihrer Shell selbst den
Coding-Agenten startet; den richtet der Betreiber nach 16 Abschnitt 4.4 nicht ein. Eine Datei,
die eine spätere Fassung des Coding-Agenten zusätzlich aus der Ordnerkette lädt, prüft HD11 erst,
wenn die Liste der Kettendateien ergänzt ist (D-W-62, D-W-68, Bedrohungsmodell T-TM-13).

### Consequences

* Gut, weil der Rechner keinen Port öffnet und das Produkt ohne privates Netz funktioniert.
* Gut, weil Relais, Proxy, Logs, Datenbank-Abzug und ein mitlesender api-Prozess keinen Inhalt sehen.
* Gut, weil jede Öffnung durch `authorize` läuft und im Audit steht.
* Schlecht, weil ein bösartiger Server-Betreiber einen eigenen Schlüssel verteilen und Sitzungen
  erlauben kann. Gegenmittel: Schlüssel-Pinning je Gerät mit harter Warnung, Audit jeder
  Neukopplung, Option `host_service.require_local_confirm`; im Bedrohungsmodell als Restrisiko
  geführt (Ruling T3).
* Schlecht, weil mehrere Server-Knoten Hash-Routing im Proxy brauchen, damit beide Enden einer
  Sitzung auf demselben Knoten landen (Ruling T6).
* Schlecht, weil das Produkt den Inhalt einer LLM-Anmeldung nicht prüfen kann; I6 hängt an der
  Erklärung des Owners. Der Host-Dienst prüft das Vorhandensein einer Abo-Anmeldung und verweigert
  den Start bei Widerspruch; tragend ist aber der eigene Sitzungsbenutzer je Person (Ruling T12).
* Schlecht, weil der Betreiber je Person einen Benutzer des Betriebssystems und einen PTY-Helfer
  einrichtet und der Owner jede Person zuordnen muss, bevor sie ein Terminal bekommt.
* Schlecht, weil Personen keinen gemeinsamen Arbeitsordner auf einem Rechner nutzen können; jede
  arbeitet in ihrem eigenen Klon und tauscht über git aus (Ruling T13).
* Schlecht, weil eine einzelne Person mit einer Abo-Anmeldung in ihrem Konto einen Rechner mit
  `none` oder `organization` für alle stilllegen kann (Ruling T2); die Meldung nennt dafür jeden
  betroffenen Sitzungsbenutzer, auch bei einem Treffer beim Start (D-W-91), und der Betreiber
  beendet den Stillstand, indem er die Anmeldung
  entfernt oder den Sitzungsbenutzer mit `bothy-host session-user remove` herausnimmt (D-W-64,
  D-W-67, Bedrohungsmodell T-TM-14). Dasselbe gilt für eine Kettendatei einer Person in einem
  gemeinsamen Ordner auf dem Weg zu den Orten anderer; die gibt es nur aus einer Zeit, in der
  dieser Ordner schreibbar war, und der Betreiber entfernt sie (D-W-80). Ein Home, das eine Person
  für andere öffnet, sperrt dagegen nur ihren eigenen Sitzungsbenutzer (HD11, D-W-88).
* Schlecht, weil jeder Neustart des Host-Dienstes alle Sitzungen des Rechners beendet, auch
  `bothy-host session-user add` und `remove`, die ihn neu starten, damit die geänderte Liste der
  Sitzungsbenutzer gilt (D-W-90); der Betreiber wählt dafür eine ruhige Zeit.

### Confirmation

* Konformität mit öffentlichen Noise-Testvektoren (cacophony, noise-c; 24 Abschnitt 5.2) in Go
  und Dart (TM-T01) und Kreuztest Go gegen Dart (TM-T02).
* Trennung der Benutzer und Sperre bei zweiter Steuerverbindung (TM-T22, TM-T23).
* Wiedereinspiel- und Marken-Tests im Host-Dienst (TM-T03 bis TM-T05).
* Kanarien-Test über Relais, Logs, Datenbank und Telemetrie (TM-T06).
* Rechte-Testmatrix AZ-M54 bis AZ-M61 und I6-Tests (TM-T09) in `make test`.
* Ein Sitzungsbenutzer je Person: TM-T28 (ohne Zuordnung, vergebener Name, falsches `osu`, Abo im
  Konto einer anderen Person, nicht gemeldeter Name) und fremde Home-Ordner (TM-T24: HD9 gegen
  Benutzer, die keine Sitzungsbenutzer sind; ein offenes Home eines Sitzungsbenutzers sperrt nach
  HD11 nur ihn).
* Keine geteilten Orte: TM-T29 (fremd schreibbarer Arbeitsordner mit Hook und `apiKeyHelper`,
  Ordnerkette, doppelter Arbeitsordner, lesbarer `CLAUDE_CONFIG_DIR`, fremder Eintrag,
  schreibbares oder fremdes Programm, fremder Index, fremde Kettendatei).
* Stillstand durch HD6 und sein Ende, auch mit einem Treffer beim Start: TM-T21.
* Red Team Terminal am Ende von M2, M4, M5, M6 (Bedrohungsmodell, Abschnitt 6.7).

## Pros and Cons of the Options

### Host-Dienst, Relais, Noise `NK`

* Gut, weil ausgehend, ohne Netz-Voraussetzung, Ende-zu-Ende verschlüsselt.
* Gut, weil `github.com/flynn/noise` (BSD-3-Clause, https://pkg.go.dev/github.com/flynn/noise,
  abgerufen 2026-10-10 [belegt]) `HandshakeNK` und `Prologue` mitbringt.
* Schlecht, weil ein eigenes Paket für den Dart-Handshake nötig ist.

### SSH über das private Netz

* Gut, weil bewährt und mit vorhandenen Werkzeugen nutzbar.
* Schlecht, weil der Rechner einen Port öffnet und das Produkt ein privates Netz voraussetzt
  (Anforderung 21).
* Schlecht, weil der Server die Öffnung nicht mehr selbst entscheidet, sondern nur Schlüssel ausgibt;
  Entzug wirkt erst, wenn der Schlüssel abläuft.

### TLS bis zum Server, Server leitet weiter

* Gut, weil einfach.
* Schlecht, weil der Server jeden Tastendruck und jede Ausgabe im Klartext sieht; ein Dump eines
  api-Prozesses oder ein Log-Fehler legt Shell-Inhalte offen.

### WebRTC-Datenkanal

* Gut, weil direkte Verbindungen den Server entlasten.
* Schlecht, weil STUN und TURN als weitere Dienste nötig sind und das Relais trotzdem gebraucht wird.
* Schlecht, weil der Browser-Stack und die Bibliotheken der Plattformen eine große Angriffsfläche
  haben.

## More Information

* Verwandt: ADR 0020 (Step-up), ADR 0024 (I1), ADR 0046 (Dev-Abschottung), ADR 0067 (JOSE-Profil),
  [ADR 0070](0070-terminal-in-der-web-konsole.md) (Terminal in der Web-Konsole).
* Rulings T1 bis T6, T9, T10, T12 und T13 der Leitung (DECISIONS, D-W-57, D-W-62).
* Überprüfung: vor der Veröffentlichung (externer Test P2) und beim Wechsel der Noise-Bibliothek.
