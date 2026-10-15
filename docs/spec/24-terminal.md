# 24 Terminal

Status: Entwurf M0, Nachtrag vom 2026-10-10. Normativ für das Modul `terminal`, den Host-Dienst
`bothy-host`, die Terminal-Ansicht der Apps und der Web-Konsole und für jede Prüfung, die eine
Terminal-Sitzung erlaubt.
Begriffe: [00-glossar.md](00-glossar.md). Tabellen: [03-datenmodell.md](03-datenmodell.md)
(`hosts`, `host_session_users`, `terminal_sessions`, `grants`). Ereignisse: [04-sync-protokoll.md](04-sync-protokoll.md).
Aktionen: [05-aktionen.md](05-aktionen.md), Abschnitt 4.14. Rechte und Schutzregeln:
[07-rechte.md](07-rechte.md) (SR10, SR11, SR12). Step-up: [08-identitaet.md](08-identitaet.md),
Abschnitt 6.3. Invariante I6: [10-backend-richtlinie.md](10-backend-richtlinie.md), Abschnitt 3.
Bedrohungen: [../security/threat-model.md](../security/threat-model.md), Abschnitt 6.7.
Entscheidungen: [ADR 0068](../adr/0068-terminal-ueber-host-dienst-mit-noise-strom-und-invariante-i6.md),
[ADR 0070](../adr/0070-terminal-in-der-web-konsole.md).

Alle Grenzen, Fristen und Intervalle sind benannte Konfigurationswerte. Ihre Standardwerte stehen
nur in Abschnitt 17.

## 1. Zweck und Grenzen

Das Terminal ist Pflichtbestandteil des Produkts. Eine berechtigte Person öffnet in der Desktop-App,
in der mobilen App oder in der Web-Konsole eine echte Terminal-Sitzung auf einem angebundenen
Rechner: eine Login-Shell oder eine interaktive Sitzung eines Coding-Agenten (z. B. Claude Code).

Sicherheit ist eingebaut, nicht abschaltbar:

1. Nur Menschen mit dem Zusatzrecht `terminal` auf dem Rechner öffnen ein Terminal, und nur mit
   einem eigenen Sitzungsbenutzer dort (Abschnitt 2.2). Owner haben das Recht implizit, außer I6
   sperrt (Abschnitt 7).
2. Jede Öffnung verlangt einen frischen Step-up, der genau für diese Öffnung gilt (Abschnitt 8).
3. Der Inhalt läuft Ende-zu-Ende verschlüsselt zwischen Gerät und Rechner; der Server vermittelt und
   entscheidet, liest aber nichts (Abschnitt 5).
4. Jede Sitzung steht im Audit-Log, mit Leerlauf-Abbruch und Höchstdauer (Abschnitte 10 und 12).
5. Auf dem Rechner laufen Host-Dienst und Sitzungen als getrennte Benutzer ohne
   Administratorrechte. Jede Person hat auf jedem Rechner ihren eigenen Sitzungsbenutzer, den nie
   eine andere Person bekommt (Ruling T12), und kein Ort mit Code, Einstellungen oder Anmeldungen
   ist für einen Sitzungsbenutzer schreibbar und für einen anderen lesbar (Ruling T13). Die
   Sitzungen erreichen die Schlüssel des Rechners nicht und können den PTY-Helfer nicht ersetzen
   (Abschnitt 2.2).

Nicht in dieser Fassung: Dateiübertragung und Zwischenablage über den Kanal, Aufzeichnung von
Inhalten, gemeinsame Sitzungen mehrerer Menschen, Terminal für Agenten oder Apps, Übernahme eines
Threads des Runners ins Terminal (Abschnitt 11.3).

Anti-Ziele: ein Weg am Rechte-Modell vorbei; Terminal-Inhalte im Server, in der Telemetrie, im
Audit, in der Suche, im Push oder im Kontext eines Agenten; ein Weg, über das Terminal das
persönliche Abo eines anderen Menschen zu nutzen.

## 2. Bausteine

### 2.1 Überblick

| Baustein | Ort | Aufgabe |
|---|---|---|
| Modul `terminal` | Server, Rolle `api` | Rechner, Kopplung, Sitzungen, Sitzungs-Marken, Relais der Ströme |
| Host-Dienst `bothy-host` | angebundener Rechner (macOS, Linux, Windows; auch eine Dev-VM aus [21-dev-isolation.md](21-dev-isolation.md)), Dienstbenutzer | Schlüssel, Steuerverbindung zum Server, Noise-Gegenstelle, Selbstprüfung |
| PTY-Helfer `bothy-host pty-helper` | derselbe Rechner, je Sitzungsbenutzer ein Helfer unter diesem Benutzer; gestartet vom Dienstverwalter des Betriebssystems aus einer Binärdatei, die kein Sitzungsbenutzer ändern kann | PTY je Sitzung seiner Person, Profile, Prüfungen im Konto der Sitzungen (HD6, HD8, HD9, HD11); kennt keine Schlüssel |
| Terminal-Ansicht | Desktop-App, mobile App, Web-Konsole | Darstellung, Eingabe, Noise-Gegenstelle, Schlüssel-Pinning |
| Paket `packages/bothy_noise` | Dart | Noise-Handshake `NK`, geprüft gegen öffentliche Testvektoren (5.2) |

```mermaid
flowchart LR
    C["Gerät der Person<br/>Terminal-Ansicht"] -- "WSS /terminal/sessions/{id}/stream<br/>Noise-Rahmen" --> P["Rand<br/>Proxy mit Hash-Routing"]
    P --> S["Server<br/>Modul terminal, Relais"]
    H["Host-Dienst bothy-host<br/>Dienstbenutzer, Schlüssel"] -- "WSS /hosts/connect (Steuerung)<br/>WSS /hosts/sessions/{id}/stream (Daten)" --> P
    H -- "lokaler Kanal je Helfer<br/>Klartext-Rahmen" --> X["PTY-Helfer je Person<br/>eigener Sitzungsbenutzer"]
    X --> PTY["PTY: Shell oder<br/>Coding-Agent"]
```

### 2.2 Host-Dienst `bothy-host`

- Eigenes kleines Go-Binary im Ordner `services/host`, nicht der Server (Ruling L1 gilt für den
  Server). Eigene Version und eigener Release-Kanal ([18-release.md](18-release.md)).
- Benutzer des Betriebssystems, alle ohne Administratorrechte, je als Dienst des
  Betriebssystems (launchd, systemd oder Windows-Dienst; [16-betrieb.md](16-betrieb.md),
  Abschnitt 4.4):
    - der **Dienstbenutzer** `bothy-host`: besitzt den Zustandsordner mit den Schlüsseln (4.2), hält
      Steuer- und Datenverbindungen und spricht Noise;
    - je Person ein **Sitzungsbenutzer** aus `host_service.session_users`: Unter ihm läuft der
      PTY-Helfer dieser Person und damit jede ihrer Shells und jeder ihrer Coding-Agenten. Kein
      Sitzungsbenutzer hat Leserecht auf den Zustandsordner.
- Den Dienstbenutzer und jeden PTY-Helfer verbindet ein eigener lokaler Kanal (Unix-Socket bzw.
  Named Pipe), dessen Zugriffsrechte nur diese zwei Benutzer zulassen. Über den Kanal laufen nur
  die entschlüsselten Rahmen aus 5.4, Steuerbefehle (Sitzung starten, beenden, Größe) und die
  Ergebnisse der Prüfungen, die der Helfer ausführt (dazu die Identitäten seiner Arbeitsordner für
  HD7, Punkt 6), nie die Schlüssel des Rechners. Grund: Eine Shell läuft mit allen Rechten ihres Benutzers. Liefe sie als
  Dienstbenutzer, könnte jede Person mit einem Terminal die Schlüssel kopieren, sich von anderswo
  als Rechner anmelden und jede spätere Sitzung mitlesen, auch die von Ownern.
- **Ein Sitzungsbenutzer je Person und Rechner, nie geteilt** (Ruling T12):
    1. Der Server hält je Rechner die Zuordnung Person zu Sitzungsbenutzer (`host_session_users`,
       [03-datenmodell.md](03-datenmodell.md)). Der Owner legt sie bei der Bestätigung (3.2) oder
       später mit `host.manage` an (Step-up, Grund, Audit); wer den Rechner lesen darf
       (`host.read`), sieht die gültigen Zuordnungen. Ohne gültige Zuordnung bekommt eine Person
       auf dem Rechner kein Terminal, auch ein Owner nicht (SR12 in [07-rechte.md](07-rechte.md);
       `403 terminal_denied` mit `reason: no_session_user`).
    2. Eine Person hat je Rechner höchstens eine gültige Zuordnung. Ein Sitzungsbenutzer gehört
       höchstens einer Person, auch über die Zeit: Endet eine Zuordnung, bleibt sein Name für diesen
       Rechner belegt, und keine andere Person bekommt ihn (`409 conflict` mit
       `reason: session_user_taken`). Eine Person bekommt nach dem Ende ihrer Zuordnung einen neuen
       Sitzungsbenutzer, nie einen gebrauchten.
    3. Die Sitzungs-Marke nennt den Sitzungsbenutzer (`osu`, 5.3). Der Host-Dienst startet die PTY
       nur im Helfer dieses Benutzers und nur, wenn er in `host_service.session_users` steht. Er
       merkt sich im Zustandsordner, welcher Person (`sub`) er jeden Sitzungsbenutzer zuerst gegeben
       hat, und lehnt eine Marke mit einer anderen Person für denselben Sitzungsbenutzer ab
       (`session_user_mismatch`). So hält der Rechner die Regel auch dann, wenn die Zuordnung im
       Server falsch ist. Geht diese Datei verloren (neuer Zustandsordner), lernt er die Bindungen
       neu; die Zuordnung im Server gilt weiter. `bothy-host session-user remove` nimmt einen
       Sitzungsbenutzer aus `host_service.session_users` und `host_service.claude_workdirs`,
       entfernt seinen PTY-Helfer, schließt sein Home für alle anderen Benutzer (Punkt 4) und
       startet den Host-Dienst neu ([16-betrieb.md](16-betrieb.md), Abschnitt 4.4); seine Bindung
       bleibt im Zustandsordner, und `bothy-host session-user add` lehnt einen Namen ab, der dort
       schon an eine Person gebunden ist.

        Grenze, ehrlich: "Nie geteilt, auch über die Zeit" gilt je Rechner-Eintrag (`hosts.id`) und je
        Zustandsordner. Wird ein Rechner zurückgezogen und als neuer Eintrag gekoppelt, sind seine
        früheren Namen im Server wieder frei; geht der Zustandsordner verloren, kennt der Host-Dienst
        die früheren Bindungen nicht mehr. Ein Sitzungsbenutzer mit den Anmeldedaten einer früheren
        Person kann dann einer anderen Person zugeordnet werden, ohne dass Server oder Host-Dienst es
        merken [abgeleitet]. Davor schützt in diesen Fällen nur der Betreiber: Er vergibt einen früher
        benutzten Namen nie neu, sondern legt mit `bothy-host session-user add` einen neuen an, nimmt
        den alten mit `bothy-host session-user remove` heraus und löscht ihn danach samt seinen
        Dauerläufern ([16-betrieb.md](16-betrieb.md), Abschnitt 4.4 Punkt 1;
        Bedrohungsmodell T-TM-08 und T-TM-11).

    4. Die Home-Ordner der Sitzungsbenutzer sind gegeneinander verschlossen. Das Home eines
       Sitzungsbenutzers ist ein Ort dieses Sitzungsbenutzers (Punkt 6) und gibt keinem anderen
       Lese- oder Ausführrecht; das prüft HD11, und ein Verstoß sperrt nur diesen
       Sitzungsbenutzer, mit seinem Namen. Die Home-Ordner aller anderen Benutzer des Rechners,
       auch das des Dienstbenutzers und das eines früheren Sitzungsbenutzers, kann kein
       Sitzungsbenutzer auflisten (HD9). Darum schließt `bothy-host session-user remove` das Home
       des Benutzers, den es herausnimmt (Punkt 3). Ein Dauerläufer dieses Benutzers (6.5) kann
       es danach wieder öffnen; darum löscht der Betreiber den Benutzer nach `remove` ("Grund für
       HD9" unter der Tabelle der Prüfungen).
    5. Der Host-Dienst meldet die Namen aus `host_service.session_users` beim Einlösen des
       Kopplungs-Codes (`host.pair`) und mit jedem `report` (`hosts.reported_session_users`, nur Namen
       und die Anzeigenamen der Arbeitsordner nach HD7, nie Pfade). Er liest seine Konfiguration,
       also auch `host_service.session_users` und `host_service.claude_workdirs`, nur beim Start;
       eine Änderung wirkt erst nach einem Neustart. `bothy-host session-user add` und `remove`
       starten ihn dafür selbst neu; jede andere Änderung der Konfiguration (etwa Arbeitsordner,
       Profile, `host_service.claude_bin`) wirkt nach `bothy-host restart`
       ([16-betrieb.md](16-betrieb.md), Abschnitt 4.4). Jeder Neustart beendet die laufenden
       Sitzungen des Rechners (6.2). Nach jeder Anmeldung der Steuerverbindung schickt er `report`,
       bevor er ein `session.open` annimmt, und der Server schickt `session.open` erst nach diesem
       `report`. Der Server ordnet nur einen gemeldeten Namen zu (sonst `400 validation_failed` mit
       `errors[].code = session_user_not_on_host`), und eine Zuordnung wirkt für SR12 nur, solange der
       Rechner ihren Namen meldet. Grund: Ein Tippfehler oder ein noch nicht angelegter Benutzer soll
       bei der Zuordnung auffallen, nicht erst beim Öffnen als Manipulationswarnung `4104` (R-TM-16).
       Ein Name muss einem gemeldeten Namen genau gleichen, auch in Groß- und Kleinschreibung, weil
       der Host-Dienst `osu` genau mit `host_service.session_users` vergleicht; die Eindeutigkeit je
       Rechner gilt dagegen ohne Groß- und Kleinschreibung (03, `host_session_users`;
       docs/DECISIONS.md D-M2-39-9).
    6. **Keine geteilten Orte** (Ruling T13): Auf dem Rechner gibt es keinen Ort, den ein
       Sitzungsbenutzer schreiben und ein anderer lesen oder ausführen kann, wenn er Code,
       Einstellungen oder Anmeldungen trägt. Für die Orte, die der Host-Dienst kennt, heißt das:
        - Orte eines Sitzungsbenutzers sind sein Home, der `CLAUDE_CONFIG_DIR` des Profils `claude`
          (bei `organization` ein fester Ordner in seinem Home, bei `personal` der Wert aus seiner
          Login-Shell, der in seinem Home liegen muss; 7.4) und seine Arbeitsordner.
          `host_service.claude_workdirs` nennt die Arbeitsordner je Sitzungsbenutzer; jeder
          Arbeitsordner gehört genau einem Sitzungsbenutzer. Steht ein Ordner dort bei zwei
          Sitzungsbenutzern, streicht HD7 ihn aus beiden Listen. Das Profil `claude` bietet einer
          Person nur die Arbeitsordner ihres eigenen Sitzungsbenutzers an, und zwar genau seine Liste
          nach HD7, die der Host-Dienst meldet (Punkt 5); in dieser Liste zählt der Index der
          Sitzungs-Marke (5.3). Diese Orte gehören ihrem Sitzungsbenutzer oder einem
          vertrauenswürdigen Besitzer (unten) und sind nur für ihn zugänglich (Modus `0700` bzw. eine
          ACL ohne Eintrag für andere Sitzungsbenutzer).
        - Jeder Ordner auf dem Weg zu einem Ort eines Sitzungsbenutzers ist für keinen anderen
          Sitzungsbenutzer schreibbar und gehört keinem anderen. Lesen und durchqueren dürfen andere
          diese Ordner (etwa `/home`), den Ort selbst nicht.
        - In jedem dieser Ordner gehören die Einträge, die der Coding-Agent oder git aus den Ordnern
          über einem Arbeitsordner laden, keinem anderen Sitzungsbenutzer und sind für keinen anderen
          schreibbar. Diese **Kettendateien** sind `CLAUDE.md`, `CLAUDE.local.md`, `AGENTS.md`,
          `.mcp.json` und die Ordner `.claude` und `.git` mit allem darin.
        - Kein Eintrag in einem Ort eines Sitzungsbenutzers gehört einem anderen Sitzungsbenutzer.
        - Orte für alle sind die Programme: die Binärdatei des Helfers, `host_service.claude_bin`
          (bei `organization` zugleich das Programm des Profils `claude`, 11.1) und das Programm jedes
          Profils `command`. Sie und jeder Ordner auf dem Weg zu ihnen gehören einem
          vertrauenswürdigen Besitzer und sind für keinen Sitzungsbenutzer schreibbar (dieselbe Regel
          wie HD10). Grund: Ein solches Programm läuft in jedem Sitzungsbenutzer. Ein anderes Konto
          als Besitzer reicht nicht, auch wenn es kein Sitzungsbenutzer ist, etwa das Konto einer
          Person mit Administratorrechten, dem ein Ordner eines Paketverwalters gehört: Wer an dieses
          Konto kommt, ändert ein Programm aller Personen, und HD11 kann nicht prüfen, wer das ist
          [abgeleitet].
        - **Vertrauenswürdiger Besitzer** ist unter Linux und macOS `root`, unter Windows `SYSTEM`,
          `Administrators` oder `NT SERVICE\TrustedInstaller`. Grund für Windows: Nur
          `TrustedInstaller` darf die geschützten Dateien und Ordner des Systems ändern ("Permission
          for full access to modify WRP-protected resources is restricted to TrustedInstaller",
          https://learn.microsoft.com/en-us/windows/win32/wfp/protected-file-list, abgerufen
          2026-10-11 [belegt]), und Microsoft nennt Dateien mit "TrustedInstaller as its owner"
          und das Konto `NT SERVICE\TrustedInstaller`
          (https://learn.microsoft.com/en-us/archive/blogs/irenak/sysk-277-how-to-bring-back-the-trustedinstaller,
          abgerufen 2026-10-11 [belegt]). Dass `C:\Program Files` und `C:\Windows\System32` mit
          Standardrechten diesem Konto gehören und `C:\` ihm oder `SYSTEM`, ist [vermutet: Berichte
          von Nutzern]; M2-42 misst es. Alle drei Konten haben auf dem Rechner ohnehin volle Rechte;
          ihnen zu trauen fügt keine Partei hinzu [abgeleitet].
        - **Schreibbar** heißt bei einer Datei: Inhalt, Attribute, Rechte oder Besitz ändern oder
          sie löschen; bei einem Ordner zusätzlich: einen Eintrag anlegen, löschen oder umbenennen.
          Unter Linux und macOS zählt das Recht `w` aus dem Modus, unter Linux auch aus der ACL, für
          den Sitzungsbenutzer, eine seiner Gruppen oder alle anderen. Unter macOS zählen dazu diese
          Rechte aus der ACL eines Eintrags für den Sitzungsbenutzer, eine seiner Gruppen oder
          `everyone`: bei Dateien `write` ("Open for writing") und `append` ("Open for writing, but
          in a fashion that only allows writes into areas of the file not previously written"), bei
          Ordnern `add_file`, `add_subdirectory` und `delete_child`, bei beiden `delete`,
          `writeattr`, `writeextattr`, `writesecurity` und `chown` (chmod(1) von macOS,
          wiedergegeben unter https://ss64.com/mac/chmod.html, abgerufen 2026-10-11 [belegt]).
          Unter Windows zählt jedes Recht, das die ACL dem Sitzungsbenutzer oder einer
          Gruppe in seinem Token gibt (auch `Everyone`, `Authenticated Users` und `Users`): bei
          Ordnern `FILE_ADD_FILE` ("For a directory, the right to create a file in the
          directory"), `FILE_ADD_SUBDIRECTORY` ("the right to create a subdirectory") und
          `FILE_DELETE_CHILD`, bei Dateien `FILE_WRITE_DATA` und `FILE_APPEND_DATA`, bei beiden
          `FILE_WRITE_ATTRIBUTES`, `FILE_WRITE_EA`, `DELETE`, `WRITE_DAC` und `WRITE_OWNER`
          (https://learn.microsoft.com/en-us/windows/win32/fileio/file-access-rights-constants,
          abgerufen 2026-10-11 [belegt]); `GENERIC_WRITE` und `GENERIC_ALL` zählen mit den Rechten,
          die sie umfassen. Ein Eintrag, der nur vererbt wird (`IO`), zählt für den Ordner selbst
          nicht. Grund: Wer in einem Ordner Einträge anlegen kann, legt dort eine Kettendatei oder
          einen Ordner mit dem Namen eines Ortes an; wer an eine Datei anhängen kann, ergänzt etwa
          eine `CLAUDE.md` um eigene Anweisungen; wer Rechte oder Besitz ändern kann, gibt sich
          jedes andere Recht [abgeleitet]. Attribute und erweiterte Attribute zählen mit: Keine der
          Quellen sagt, welche davon für das Laden und Starten folgenlos sind, und ein Recht zu viel
          in der Liste sperrt nur, eines zu wenig öffnet einen Ort [abgeleitet].
        - **Pfadauflösung:** HD7, HD10 und HD11 lösen jeden Pfad vor der Prüfung vollständig auf,
          unter Linux und macOS wie `realpath` ("an absolute pathname that resolves to the same
          directory entry, whose resolution does not involve '.', '..', or symbolic links",
          https://pubs.opengroup.org/onlinepubs/9799919799/functions/realpath.html, abgerufen
          2026-10-11 [belegt]), unter Windows mit `GetFinalPathNameByHandle` ("A final path is the
          path that is returned when a path is fully resolved",
          https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getfinalpathnamebyhandlew,
          abgerufen 2026-10-11 [belegt]). HD10 und HD11 prüfen danach jeden Ordner der aufgelösten
          Kette, jede Komponente des Pfads, wie er in der Konfiguration steht, und jeden Verweis
          (symbolischer Link, unter Windows auch Verzeichnisverbindung), dem die Auflösung folgt,
          samt dem Ordner, in dem er liegt. Ein Verweis zählt dabei wie ein Eintrag seines Ordners
          und wie sein Ziel: Ein Verweis von `root` auf dem Weg zu einem Ort von B, der in einen
          Ordner von A zeigt, besteht nicht.
          Dasselbe gilt für eine Kettendatei, die ein Verweis ist, und für Verweise in `.claude` und
          `.git`. Auflösen kann nur, wer den Pfad öffnen darf: Die Orte eines Sitzungsbenutzers
          (Home, `CLAUDE_CONFIG_DIR`, Arbeitsordner) löst sein PTY-Helfer als dieser
          Sitzungsbenutzer auf, denn sie sind nur für ihn zugänglich (oben), mit Modus `0700` auch
          nicht für den Dienstbenutzer [abgeleitet]; die Orte für alle löst der Dienstbenutzer auf.
          Lässt sich ein Pfad nicht auflösen, weil er fehlt, ein Verweis ins Leere zeigt oder ein
          Ordner auf dem Weg für den auflösenden Benutzer verschlossen ist (etwa das Home eines
          anderen Sitzungsbenutzers), gilt er als nicht vorhanden: Ein Arbeitsordner fällt aus der
          Liste seines Sitzungsbenutzers, ein Profil `command` mit diesem Programm fällt weg (HD7,
          Warnung); `host_service.claude_bin` gilt für HD6 und das Profil `claude` als leer, und ein
          `CLAUDE_CONFIG_DIR` bei `personal` zählt wie einer außerhalb des Home (7.4); lässt sich
          das Home selbst nicht auflösen, besteht HD11 für diesen Sitzungsbenutzer nicht. Grund: Was nicht aufgelöst ist,
          ist nicht geprüft. Ein Arbeitsordner oder Programm, das nie angeboten wird, verbindet keine
          Konten; den ganzen Sitzungsbenutzer wegen eines fehlenden Ordners zu sperren, wäre mehr als
          nötig. Einen Verweis ins Leere als vorhanden zu werten, wäre gefährlich: Sein Ziel kann ein
          anderer später anlegen [abgeleitet].
          HD7 vergleicht Ordner nach ihrer Identität im Dateisystem, nicht nach der
          Schreibweise: unter Linux und macOS Gerät und Inode ("A file identity is uniquely
          determined by the combination of st_dev and st_ino",
          https://pubs.opengroup.org/onlinepubs/9799919799/basedefs/sys_stat.h.html, abgerufen
          2026-10-11 [belegt]), unter Windows Seriennummer des Volumes und die 128-Bit-Datei-ID aus
          `FILE_ID_INFO` ("The file identifier and the volume serial number uniquely identify a
          file on a single computer",
          https://learn.microsoft.com/en-us/windows/win32/api/winbase/ns-winbase-file_id_info,
          abgerufen 2026-10-11 [belegt]), abgefragt mit `GetFileInformationByHandleEx` und
          `FileIdInfo` (vorhanden ab Windows 8: "This value is not supported before Windows 8",
          https://learn.microsoft.com/en-us/windows/win32/api/minwinbase/ne-minwinbase-file_info_by_handle_class,
          abgerufen 2026-10-11 [belegt]). Die 64-Bit-Datei-ID aus `BY_HANDLE_FILE_INFORMATION`
          reicht nicht: "The 64-bit identifier in this structure is not guaranteed to be unique on
          ReFS"
          (https://learn.microsoft.com/en-us/windows/win32/api/fileapi/ns-fileapi-by_handle_file_information,
          abgerufen 2026-10-11 [belegt]). Jeder PTY-Helfer meldet dem Host-Dienst über den lokalen
          Kanal je Arbeitsordner seiner Liste nur diese Identität, nie einen Inhalt; der Host-Dienst
          vergleicht die Werte aller Helfer und bildet daraus beim Start die Listen nach HD7. So
          erkennt HD7 denselben Ordner auch hinter einem Verweis oder in anderer Groß- und
          Kleinschreibung. Der PTY-Helfer startet das Profil `claude` im
          aufgelösten Arbeitsordner, und HD6 und das Profil rufen `host_service.claude_bin` über
          den aufgelösten Pfad auf, also genau das, was HD10 und HD11 geprüft haben. Grund: Eine
          Prüfung nur des angegebenen Pfads ohne Auflösung ließe einen Verweis durch, der einem
          vertrauenswürdigen Besitzer gehört und in einen Ordner einer anderen Person zeigt
          [abgeleitet].
        - **Laufwerkswurzel unter Windows:** Die Wurzel eines Laufwerks liegt auf dem Weg zu jedem Ort
          darauf. Mit Standardrechten darf dort jeder angemeldete Benutzer Ordner anlegen: Die
          DISA-Prüfliste für Windows 11 nennt für `C:\` die Einträge
          `NT AUTHORITY\Authenticated Users:(AD)` ("Create folders / append data - This folder
          only") und `NT AUTHORITY\Authenticated Users:(OI)(CI)(IO)(M)` ("Modify - Subfolders and
          files only") (https://www.stigaview.com/products/win11/v2r3/WN11-00-000095/, abgerufen
          2026-10-11 [belegt]). Eine Person könnte so `C:\.git` oder `C:\.claude` anlegen, also
          Kettendateien über jedem Ort; dasselbe Muster beschreibt Git für Windows: "Those untrusted
          parties could create the folder `C:\.git`"
          (https://github.com/git-for-windows/git/security/advisories/GHSA-vw2c-22j4-2fh2,
          abgerufen 2026-10-11 [belegt]). Darum entfernt `bothy-host install` nach Bestätigung des
          Betreibers den Eintrag `(AD)` an der Wurzel des Systemlaufwerks und jedes Laufwerks, auf
          dem ein Ort liegt ([16-betrieb.md](16-betrieb.md), Abschnitt 4.4). Der Eintrag gilt nur
          für die Wurzel selbst. Der Befehl schreibt dafür die DACL genau dieses Ordners neu, mit
          allen anderen Einträgen in derselben Reihenfolge und denselben Steuerbits, und zwar mit
          `SetFileSecurityW` ("Note that security applied to a directory is not inherited by its
          children",
          https://learn.microsoft.com/en-us/windows/win32/api/securitybaseapi/nf-securitybaseapi-setfilesecurityw,
          abgerufen 2026-10-11 [belegt]), nie mit `SetNamedSecurityInfo` oder `SetSecurityInfo`.
          Diese beiden wenden bei jedem Setzen einer DACL die Vererbung neu auf alles darunter an
          ("If you are setting the discretionary access control list (DACL) [...] the system
          automatically propagates any inheritable access control entries (ACEs) to existing child
          objects",
          https://learn.microsoft.com/en-us/windows/win32/api/aclapi/nf-aclapi-setnamedsecurityinfow;
          "the system imposes the current inheritance model on the ACLs of all objects in the
          hierarchy below the target object",
          https://learn.microsoft.com/en-us/windows/win32/secauthz/automatic-propagation-of-inheritable-aces;
          beide abgerufen 2026-10-11 [belegt]), an der Wurzel also auf das ganze Laufwerk;
          `SetNamedSecurityInfo` ordnet dabei in den Ordnern darunter geerbte Einträge hinter die
          eigenen (Quelle zu `SetNamedSecurityInfo` oben [belegt]). Microsoft nennt
          `SetFileSecurityW` veraltet ("This function is obsolete", Quelle zu `SetFileSecurityW`
          oben [belegt]); gewählt ist es trotzdem, weil seine Doku als einzige der genannten
          ausdrücklich sagt, dass nichts an die Kinder geht. Weil die
          vererbbaren Einträge der Wurzel gleich bleiben, ändert sich darunter kein Recht
          [abgeleitet]. Den Eintrag `(OI)(CI)(IO)(M)` lässt der Befehl stehen: Er gibt auf der
          Wurzel selbst kein Recht, und seine Entfernung nähme nach den Regeln der Vererbung jedem
          bestehenden Ordner darunter seine geerbte Kopie ("If you remove an inheritable ACE from a
          parent object, automatic inheritance removes any copies of the ACE that were inherited by
          child objects", Quelle zur automatischen Vererbung oben [belegt]); bestehende Programme
          verlören so ihre Schreibrechte.
          Ordner, die diesen Eintrag erben, sind für jeden Sitzungsbenutzer schreibbar; HD11 findet
          sie, wenn sie auf dem Weg zu einem Ort liegen. Ist die Wurzel eines Laufwerks mit einem
          geprüften Ort für einen Sitzungsbenutzer schreibbar, meldet HD11 dafür den eigenen Code
          `drive_root_writable`, damit der Betreiber den Schritt aus 16 Abschnitt 4.4 findet; die
          lokale Ausgabe von `bothy-host doctor` nennt das Laufwerk, an den Server geht nur der Code.
          Für ein Laufwerk, auf dem erst später ein Ort dazukommt, macht
          `bothy-host install --drive-root <Laufwerk>` nur diesen Schritt. Die
          Folge ist dieselbe wie bei jedem Verstoß an diesen Orten; an der Wurzel des
          Systemlaufwerks heißt das immer "Start verweigert", weil die Binärdatei des Helfers dort
          liegt, also ein Ort für alle.

        HD11 prüft das mit einer Regel für jeden konfigurierten Pfad, statt jeden Ort einzeln zu
        regeln; HD10 prüft für den Helfer und `host_service.claude_bin` zusätzlich Gegenstelle und
        Interpreter. Die vier Begriffe oben (vertrauenswürdiger Besitzer, schreibbar, Pfadauflösung,
        Laufwerkswurzel) gelten für diesen Punkt und für HD7, HD10 und HD11 gleich. Gemeinsamen Code tauschen Personen über git aus, nie über einen gemeinsamen
        Ordner (Bedrohungsmodell T-TM-13).

    Grund: Die Anmeldedaten eines Sitzungsbenutzers (Dateien im Home, Schlüsselbund, ein Token in
    seinen Startdateien) gehören so genau einer Person. Eine Abo-Anmeldung, die jemand in seiner
    Sitzung anlegt, erreicht keine Sitzung eines anderen Menschen, solange kein Ort die Konten wieder
    verbindet (Punkt 6). Das ist die tragende Durchsetzung
    von "keine fremde Abo-Anmeldung" (7.4); HD6 ist nur eine Zusatzprüfung. Ein geteilter
    Sitzungsbenutzer machte jede Anmeldung für alle Terminal-Nutzer des Rechners nutzbar, und eine
    Prüfung im Konto dieses Benutzers ließe sich mit seinen eigenen Rechten fälschen oder umgehen.

    Grund für T13: Getrennte Konten helfen nur, wenn kein Ort sie wieder verbindet. Ein gemeinsamer
    Arbeitsordner war so ein Ort. Claude Code liest die Projekt-Einstellungen
    `.claude/settings.json` und `.claude/settings.local.json` des Arbeitsordners; Einstellungsdateien
    tragen unter anderem "`permissions`, `hooks`, and credential helpers such as `apiKeyHelper`",
    "The reload covers user, project, local, and managed settings", und "most `env` values apply
    only after each teammate trusts the folder" (https://code.claude.com/docs/en/settings,
    abgerufen 2026-10-11 [belegt]). Außerdem gilt: "CLAUDE.md and CLAUDE.local.md files in the
    directory hierarchy above the working directory are loaded at launch"
    (https://code.claude.com/docs/en/memory, abgerufen 2026-10-11 [belegt]). Schreibt Person A dort,
    laufen ihre Hooks und ihr `apiKeyHelper` als Sitzungsbenutzer von B, ihre Anweisungen steuern
    Bs Agenten, und nach der Vertrauensabfrage setzt sie auch Umgebungsvariablen wie
    `CLAUDE_CODE_OAUTH_TOKEN` in Bs Sitzung [abgeleitet]. Das ist Codeausführung über die Grenze
    zwischen Personen und ein zweiter Weg für fremde Anmeldungen. Ein gemeinsamer
    `CLAUDE_CONFIG_DIR` wäre derselbe Ort für Anmeldungen: Ein `/login` darin erreichte alle
    Personen. Statt jeden Fund einzeln zu schließen, gilt eine Regel mit einer Prüfung für alle
    Pfade.

    Grund für die Kettendateien: Was der Coding-Agent aus den Ordnern über
    dem Arbeitsordner lädt, wirkt wie ein Ort von B, auch wenn der Ordner selbst niemandem sonst
    gehört. Neben `CLAUDE.md` und `CLAUDE.local.md` lädt Claude Code ohne solche Dateien "every
    `AGENTS.md` and `.claude/AGENTS.md` in your working directory and the directories above it"
    (https://code.claude.com/docs/en/memory, abgerufen 2026-10-11 [belegt]). Startet es in einem
    Unterordner eines git-Repositorys, liest es `.claude/settings.local.json` "at the repository
    root" (https://code.claude.com/docs/en/settings, abgerufen 2026-10-11 [belegt]); `.mcp.json`
    liegt "in project root" (https://code.claude.com/docs/en/mcp, abgerufen 2026-10-11 [belegt]),
    und dieser kann über dem Arbeitsordner liegen [abgeleitet]. git sucht sein Repository ebenfalls
    in den Ordnern darüber; die Doku nennt "directories that Git should not chdir up into while
    looking for a repository directory" (https://git-scm.com/docs/git, abgerufen 2026-10-11
    [belegt]). Hooks eines solchen Repositorys liefen dann in der Sitzung von B [abgeleitet]. Eine
    Regel für alle Einträge dieser Ordner ginge nicht: In `/home` liegen die Home-Ordner der anderen
    Sitzungsbenutzer, und sie gehören ihnen.

    Grenze, ehrlich: HD11 prüft die Orte, die die Konfiguration nennt, und das Home jedes
    Sitzungsbenutzers, nicht jeden Ordner des Rechners. Den `CLAUDE_CONFIG_DIR` bei `personal`, den
    die Login-Shell des Inhabers setzt, deckt das Home nur, weil der PTY-Helfer das Profil `claude`
    allein mit einem Wert im Home startet (7.4). Allgemein schreibbare Ordner wie `/tmp` sind
    nach T13 kein Ort für Code, Einstellungen oder Anmeldungen; ein Programm, das trotzdem von dort
    lädt, deckt HD11 nicht ab [abgeleitet]. Wechselt eine Person in ihrer Shell selbst in einen
    Ordner außerhalb dieser Orte, den ein anderer Sitzungsbenutzer schreiben kann, und startet dort
    den Coding-Agenten, sieht HD11 das nicht; solche Ordner richtet der Betreiber nach T13 nicht ein
    ([16-betrieb.md](16-betrieb.md), Abschnitt 4.4). Unter Windows ist das auch jeder Ordner, den
    ein Administrator direkt in der Wurzel eines Laufwerks anlegt, solange er den vererbten Eintrag
    `Authenticated Users:(OI)(CI)(IO)(M)` trägt (Punkt 6). Die Kettendateien sind die Namen, die die
    genannten Quellen am 2026-10-11 nennen; lädt eine spätere Fassung des Coding-Agenten weitere
    Dateien aus der Ordnerkette, prüft HD11 sie erst, wenn die Liste ergänzt ist. Öffnet ein
    Sitzungsbenutzer seine eigenen Orte für andere, sieht HD11 das bei der nächsten Prüfung, und gar
    nicht, wenn er sie vorher wieder schließt. Einen Eintrag, den ein anderer Sitzungsbenutzer in
    dieser Zeit dort anlegt, findet HD11 erst beim nächsten Start des Dienstes oder Aufruf von
    `bothy-host doctor`, weil nur diese Prüfung jeden Eintrag eines Ortes durchläuft. Was er in eine
    schon vorhandene, für ihn schreibbare Datei schreibt, erkennt HD11 gar nicht an Besitz oder
    Modus. Nach einem Treffer prüft der Betreiber den Ort ([16-betrieb.md](16-betrieb.md), Abschnitt
    4.4).

    Folge, ehrlich: Liegt eine Kettendatei, die einem Sitzungsbenutzer A gehört oder die er
    schreiben kann, in einem Ordner auf dem Weg zu den Orten mehrerer Sitzungsbenutzer (etwa der
    Wurzel eines Laufwerks oder einem gemeinsamen Elternordner der Arbeitsordner), besteht HD11 für
    jeden anderen dieser Sitzungsbenutzer nicht. Eine Person kann so den Rechner für die anderen
    stilllegen. Anlegen kann A einen solchen Eintrag nur, solange der
    Ordner für A schreibbar ist; das meldet HD11 selbst als Verstoß an jedem dieser Orte, und unter
    Windows schließt `bothy-host install` die Wurzel (Punkt 6). Es bleibt ein Eintrag aus der Zeit
    davor, etwa ein `C:\.git`, das A vor der Installation angelegt hat. Der Stillstand endet, wenn
    der Betreiber den Eintrag entfernt ([16-betrieb.md](16-betrieb.md), Abschnitt 4.4;
    Bedrohungsmodell T-TM-14).

- Jeder PTY-Helfer läuft als sein Sitzungsbenutzer, aber dieser steuert ihn nicht. Sonst
  könnte eine Shell den Helfer beenden, einen eigenen an seine Stelle setzen, die Ergebnisse von
  HD6, HD8, HD9 und HD11 fälschen und den Klartext jeder späteren Sitzung lesen. Darum gilt:
    1. `bothy-host install` (einmal mit Administratorrechten, [16-betrieb.md](16-betrieb.md),
       Abschnitt 4.4) legt je Sitzungsbenutzer einen Helfer als Dienst des Systems an, der unter
       diesem Sitzungsbenutzer läuft: systemd-Systemdienst mit `User=` und Socket-Aktivierung, launchd-LaunchDaemon mit
       `UserName` und `Sockets`, Windows-Dienst mit dem Konto des Sitzungsbenutzers. Binärdatei,
       Dienst-Definition und der Ordner mit dem Endpunkt des Kanals gehören einem
       vertrauenswürdigen Besitzer (2.2 Punkt 6) und sind für keinen Sitzungsbenutzer schreibbar;
       stoppen oder neu konfigurieren kann er den Dienst nicht (HD10).
    2. Den Endpunkt des Kanals legt unter Linux und macOS der Dienstverwalter an und reicht ihn an
       den Helfer weiter; ein Prozess des Sitzungsbenutzers kann ihn nicht neu anlegen. Unter Windows
       legt der Helfer die Named Pipe an.
    3. `bothy-host` prüft beim Verbinden die Gegenstelle doppelt: Benutzer über die Anmeldedaten des
       Prozesses, die das Betriebssystem liefert, und Prozess-ID gleich der Prozess-ID, die der
       Dienstverwalter für den Helfer-Dienst dieses Sitzungsbenutzers meldet. Passt eins nicht, schickt er keinen Rahmen und
       meldet HD10 nicht bestanden, mit dem Namen dieses Sitzungsbenutzers (Regel für Namen unter
       der Tabelle der Prüfungen).
    4. Der Helfer schützt seinen Speicher vor anderen Prozessen desselben Benutzers: unter Linux
       `PR_SET_DUMPABLE = 0`, unter macOS Hardened Runtime ohne `get-task-allow`, unter Windows eine
       Prozess-DACL ohne Zugriffsrechte für den Sitzungsbenutzer. Für Linux belegt: Ein Prozess
       desselben Benutzers darf sich nur an einen Prozess hängen, der "dumpable" ist
       (https://www.kernel.org/doc/html/latest/admin-guide/LSM/Yama.html, abgerufen 2026-10-10
       [belegt]); für macOS und Windows [vermutet], das Bau-Ticket belegt es mit TM-T26.

    Grenze, ehrlich: Prozesse eines Sitzungsbenutzers erreichen die Shells derselben
    Benutzerkennung, zum Beispiel über das PTY-Gerät, das diesem Sitzungsbenutzer gehört
    [abgeleitet]. Alle Sitzungen einer Person auf einem Rechner teilen sich also einen
    Vertrauensbereich; Sitzungen verschiedener Personen trennt die Benutzerkennung (T12). Diese
    Trennung ist keine vollständige Grenze zwischen Personen (gemeinsamer Kernel, lokale Lücken des
    Betriebssystems; [ADR 0046](../adr/0046-mehrbenutzer-dev-umgebung-nur-mit-abschottung.md)). Darum
    gilt SR10 weiter, und die App warnt einen Owner, der auf dem Rechner eines anderen Menschen ein
    Terminal öffnet (8.4; Bedrohungsmodell T-TM-12).

- `bothy-host doctor` prüft vor jedem Start und auf Befehl (voller Lauf); HD6 und HD11 laufen
  zusätzlich vor jedem Start einer PTY und alle `host_service.credential_check_interval` (7.4),
  HD11 dabei ohne die Prüfung der Einträge:

| Prüfung | Bestanden, wenn | Folge bei Verstoß |
|---|---|---|
| HD1 Benutzer | `host_service.session_users` nennt mindestens einen Benutzer; kein Benutzer des Host-Dienstes ist `root`, `Administrator` oder `SYSTEM`; alle Sitzungsbenutzer sind verschieden, und keiner ist der Dienstbenutzer | Start verweigert |
| HD2 Gruppen | kein Benutzer des Host-Dienstes (Dienstbenutzer und jeder Sitzungsbenutzer) ist in einer gesperrten Gruppe: fest `admin`, `sudo`, `wheel`, `Administrators`, `docker`, `lxd`, `libvirt`, `disk`, dazu `host_service.extra_forbidden_groups`; unter Linux ist zusätzlich der Socket des Docker-Daemons für keinen von ihnen schreibbar | Start verweigert |
| HD3 sudo | kein passwortloses `sudo` für einen von ihnen (`sudo -n true` scheitert) | Start verweigert |
| HD4 Zustandsordner | `host_service.state_dir` gehört dem Dienstbenutzer, Modus `0700`; Schlüsseldatei `0600` (Windows: DPAPI, Abschnitt 4.2) | Start verweigert |
| HD5 Server-Host | erkennt, ob derselbe Rechner den Server betreibt (Abschnitt 2.3) | Warnung; Sitzungen nur für Owner (SR10) |
| HD6 LLM-Anmeldung | Abschnitt 7.4, im Konto jedes Sitzungsbenutzers; Zusatzprüfung, nicht die tragende Grenze (T12) | bei `none` oder `organization` und erkannter Abo-Anmeldung: Start verweigert bzw. Sitzungen beendet (7.4); die Meldung nennt jeden betroffenen Sitzungsbenutzer mit eigenem Eintrag |
| HD7 Profile | jedes Profil verweist auf ein vorhandenes Programm, Profile `command` mit absolutem Pfad, das Profil `claude` bei `organization` auf `host_service.claude_bin` (leer oder als leer geltend nach 7.4: das Profil fällt weg, 11.1); jeder Schlüssel in `host_service.claude_workdirs` ist ein Name aus `host_service.session_users`, jeden Arbeitsordner kann der PTY-Helfer seines Sitzungsbenutzers auflösen, und jeder steht bei genau einem Sitzungsbenutzer, verglichen nach Auflösung und Identität im Dateisystem, die jeder Helfer für seine Liste meldet (2.2 Punkt 6) | Profil fällt weg, Warnung; bei `host_service.claude_workdirs` fällt nur der falsche Teil weg: ein Schlüssel, der kein Name aus `host_service.session_users` ist, mit seiner Liste, ein Ordner, den der Helfer seines Sitzungsbenutzers nicht auflösen kann, aus dessen Liste, ein Ordner bei zwei Sitzungsbenutzern aus beiden Listen. Gemeldet und für `wdi` benutzt werden die Listen danach (2.2 Punkte 5 und 6) |
| HD8 Trennung | der Dienstbenutzer prüft selbst Besitz, Modus und ACL von `host_service.state_dir` und der Schlüsseldatei (kein Eintrag für einen Sitzungsbenutzer oder eine seiner Gruppen); zusätzlich versucht jeder PTY-Helfer beim Start, beide zu lesen, und scheitert | Start verweigert |
| HD9 Fremde Home-Ordner | jeder PTY-Helfer versucht beim Start, den Inhalt jedes Home-Ordners eines anderen lokalen Benutzers aufzulisten, den das Betriebssystem führt und der kein Sitzungsbenutzer ist (auch den des Dienstbenutzers und den eines früheren Sitzungsbenutzers), und scheitert bei jedem; das Home eines Sitzungsbenutzers prüft HD11 als dessen Ort (2.2 Punkt 4) | Start verweigert; die lokale Ausgabe von `bothy-host doctor` nennt jeden Benutzer, dessen Home sich auflisten lässt; die Meldung an den Server nennt davon nur frühere Sitzungsbenutzer, je einen mit eigenem Eintrag, sonst nur den Code (unter der Tabelle) |
| HD10 Helfer-Integrität | der Dienstbenutzer prüft: Binärdatei des Helfers, jede Dienst-Definition, jeder Ordner eines Kanal-Endpunkts und, wenn gesetzt, `host_service.claude_bin` (das Programm für HD6 und bei `organization` des Profils `claude`, 7.4; unter Windows eine `.exe`) mit jedem Ordner auf dem Weg dorthin und, wenn die Datei mit `#!` beginnt, dem Interpreter, den die feste Suchliste aus 7.4 findet, und jedem Ordner dieser Suchliste, gehören nach Auflösung einem vertrauenswürdigen Besitzer und sind für keinen Sitzungsbenutzer schreibbar (beides je Plattform nach 2.2 Punkt 6); die Gegenstelle jedes Kanals ist der Prozess, den der Dienstverwalter für den Helfer dieses Sitzungsbenutzers meldet; unter Linux ist `kernel.yama.ptrace_scope` mindestens `1` | Start verweigert; bei einem Verstoß im Betrieb wie HD6 (7.4): keine neuen Sitzungen, laufende enden mit `host_policy_violation` und `close_detail = helper_untrusted`; bei einer falschen Gegenstelle nennt die Meldung den Sitzungsbenutzer dieses Kanals (unter der Tabelle) |
| HD11 Geteilte Orte | eine Regel für jeden Pfad aus der Konfiguration des Host-Dienstes (Arbeitsordner, `host_service.claude_bin` und Programme der Profile, dazu die Binärdatei des Helfers; den Zustandsordner prüfen HD4 und HD8) und jeden Ort eines Sitzungsbenutzers (2.2 Punkt 6; den `CLAUDE_CONFIG_DIR` bei `personal` deckt das Home, 7.4); jeder Pfad nach Auflösung, "schreibbar" und "vertrauenswürdiger Besitzer" je Plattform nach 2.2 Punkt 6; eine schreibbare Laufwerkswurzel unter Windows mit dem Code `drive_root_writable`. Ort eines Sitzungsbenutzers: Besitz, Modus, ACL und Gruppen des Ortes, jedes Ordners auf dem Weg dorthin und der Kettendateien in diesen Ordnern geben keinem anderen Sitzungsbenutzer Schreibrecht und gehören keinem anderen; der Ort selbst gibt anderen Sitzungsbenutzern auch kein Lese- oder Ausführrecht; das prüft sein PTY-Helfer. Ort für alle: der Ort und jeder Ordner auf dem Weg dorthin gehören einem vertrauenswürdigen Besitzer und sind für keinen Sitzungsbenutzer schreibbar (wie HD10); das prüft der Dienstbenutzer. Beim Start des Dienstes und auf Befehl zusätzlich die Prüfung der Einträge: kein Eintrag in einem Ort eines Sitzungsbenutzers gehört einem anderen Sitzungsbenutzer; sie läuft je Sitzungsbenutzer in seinem PTY-Helfer, begrenzt durch `host_service.entry_scan_timeout` (unter der Tabelle) | Verstoß an einem Ort eines Sitzungsbenutzers: dieser bekommt keine Sitzungen, laufende enden mit `host_policy_violation` und `close_detail = shared_location`, bis HD11 für ihn besteht; die Meldung nennt ihn. Verstoß an einem Ort für alle: Start verweigert; im Betrieb keine neuen Sitzungen, und alle laufenden enden mit `host_policy_violation` und `close_detail = shared_location`, bis HD11 besteht; die Meldung nennt keinen Sitzungsbenutzer. Ein Fund der Prüfung der Einträge gilt, bis ein späterer voller Lauf ihn nicht mehr findet (unter der Tabelle) |

  Prüfung der Einträge (HD11): Sie geht durch jeden Eintrag jedes Ortes eines Sitzungsbenutzers,
  also durch sein ganzes Home, und läuft darum je Sitzungsbenutzer in seinem eigenen PTY-Helfer.
  Beim Start des Dienstes wartet jeder Sitzungsbenutzer nur auf seinen eigenen Lauf: Bis dieser
  fertig ist, besteht HD11 für ihn nicht (Code `entry_scan_running`), und er bekommt keine
  Sitzungen: Eine Öffnung scheitert wie bei einem Fund mit `open_failed` und
  `close_detail = shared_location`, und die Konsole zeigt den Code aus `reported_doctor`. Die
  anderen bekommen ihre Sitzungen, sobald ihr eigener Lauf fertig ist. Auf Befehl läuft die Prüfung
  neben dem Betrieb, und bis zu ihrem Ende gilt das bisherige Ergebnis. Ist ein Lauf nach
  `host_service.entry_scan_timeout` nicht fertig, bricht der Helfer ihn ab; HD11 besteht dann für
  diesen Sitzungsbenutzer nicht (Code `entry_scan_timeout`, mit seinem Namen), mit derselben Folge
  wie bei einem Fund. Einen Fund, auch einen Abbruch, hebt nur ein späterer voller Lauf auf, der
  fertig wird und nichts mehr findet, also ein Start des Dienstes oder `bothy-host doctor`. Die
  Läufe vor jeder PTY und alle `host_service.credential_check_interval` prüfen die Einträge nicht
  und heben einen solchen Fund nie auf. Grund: Die Dauer hängt von der Größe des Home ab, und ein
  großes Home soll nur seinen eigenen Sitzungsbenutzer aufhalten, nicht den Rechner. Ein
  abgebrochener Lauf hat nicht jeden Eintrag gesehen, und nur diese Prüfung sieht, ob ein
  gefundener Eintrag noch da ist.

  Grund für die Sperrliste in HD2: Einige Gruppen geben Rechte auf root-Niveau, ohne Administrator
  zu heißen. Docker schreibt: "The `docker` group grants root-level privileges to the user."
  (https://docs.docker.com/engine/install/linux-postinstall/, abgerufen 2026-10-10 [belegt]). Für
  `lxd` sagt die LXD-Doku, lokaler Zugriff auf den Daemon sei voller Zugriff und nur Benutzern zu
  geben, denen man root anvertrauen würde
  (https://canonical.com/lxd/docs/default/explanation/security/index.html.md, abgerufen 2026-10-10
  [belegt]). `libvirt` (Steuerung von VMs mit Zugriff auf Geräte des Rechners) und `disk` (roher
  Zugriff auf Datenträger) stehen aus demselben Grund auf der Liste [abgeleitet]. Die feste Liste kann der
  Betreiber nicht kürzen; `host_service.extra_forbidden_groups` ergänzt sie, weil Distributionen
  weitere solche Gruppen haben.

  Grund für HD9: Die Shell einer Person darf keine Dateien anderer Benutzer des Rechners lesen.
  Unter macOS sind Benutzer standardmäßig in der Gruppe `staff`, und die CIS-Prüfliste schreibt:
  "By default, macOS allows all valid users into the top level of every other user's home folder"
  (https://www.tenable.com/audits/items/CIS_Apple_macOS_12.0_Monterey_v4.0.0_L1.audit:da3f5b54f52fa5be1eb1f2bbc291c7c0,
  abgerufen 2026-10-10 [belegt]). Ein Sitzungsbenutzer in `staff` besteht HD9 auf einem
  macOS-Rechner mit Standardrechten darum nicht [abgeleitet]; `bothy-host install` legt ihn dort
  ohne `staff` an, den Weg belegt das Bau-Ticket. HD9 prüft das Ergebnis statt der Gruppe, weil
  Gruppen und Rechte der Home-Ordner je Plattform und Rechner verschieden sind; derselbe Lesetest
  gilt unter Linux und Windows. HD9 prüft nur Home-Ordner von Benutzern, die keine
  Sitzungsbenutzer sind. Das Home eines Sitzungsbenutzers prüft HD11 als dessen Ort (2.2 Punkt 4):
  Öffnet eine Person ihr eigenes Home für andere, ist das ein Fehler in ihrem Konto, und HD11
  sperrt nur ihren Sitzungsbenutzer und nennt ihn. Prüfte HD9 auch dieses Home, schlüge es im
  Helfer jedes anderen Sitzungsbenutzers an und verweigerte den Start des ganzen Rechners, ohne
  die Person zu nennen; eine Person könnte den Rechner so mit einer Änderung der Rechte ihres
  Home stilllegen. Die Home-Ordner der übrigen Benutzer kann keine Person mit Terminal ändern,
  mit einer Ausnahme: das Home eines früheren Sitzungsbenutzers. `bothy-host session-user remove`
  schließt es (2.2 Punkt 3), beendet aber keine Prozesse dieses Benutzers. Ein Dauerläufer, den
  die Person als Sitzungsbenutzer eingerichtet hat (6.5), läuft weiter und kann das Home wieder
  öffnen, denn es gehört weiter ihrem Benutzer. Dann verweigert HD9 beim nächsten Start oder beim
  nächsten `bothy-host doctor` den Start des ganzen Rechners. Darum nennt die Meldung an den
  Server diesen früheren Sitzungsbenutzer (unten), die Konsole dazu die Person seiner Zuordnung,
  und der Betreiber löscht den Benutzer gleich nach `remove` samt seinen Dauerläufern
  ([16-betrieb.md](16-betrieb.md), Abschnitt 4.4 Punkt 1); danach führt das Betriebssystem ihn
  nicht mehr, und HD9 prüft sein Home nicht mehr. Ehrlich offen bleibt der Weg zwischen `remove`
  und dem Löschen. Die Folge bleibt auch für dieses Home "Start verweigert": HD9 sieht nicht, wer
  ein Home geöffnet hat, und ein offenes Home zeigt die Dateien einer Person, die kein Terminal
  mehr bekommt.

  Grund für HD10: Ergebnisse aus dem Konto des Sitzungsbenutzers zählen nur, wenn der Helfer echt
  ist. Darum prüft der Dienstbenutzer die Echtheit selbst, und HD8 hat einen Teil, der nicht vom
  Helfer abhängt. Die Linux-Doku zu Yama beschreibt Stufe `1` als "restricted ptrace": Anhängen
  darf nur ein Prozess mit einer festgelegten Beziehung, standardmäßig ein Vorfahre (gleiche
  Quelle [belegt]). Dann kann ein Dauerläufer des Sitzungsbenutzers sich nicht an Shells späterer
  Sitzungen hängen.

  `doctor` meldet nur Ergebnis und Code je Prüfung an den Server (`hosts.reported_doctor`), nie
  Inhalte, Pfade oder Werte. Regel für Namen: Kann eine Person mit Terminal, auch eine
  frühere, einen Verstoß herbeiführen, nennt die Meldung das Konto, in dem der Betreiber suchen
  muss, je Konto mit einem eigenen Eintrag und dessen Namen (`session_user`); Prüfung und Name
  sind zusammen eindeutig. Das sind: bei HD6 das Konto mit der Abo-Anmeldung, bei HD11 an einem
  Ort eines Sitzungsbenutzers dieser Sitzungsbenutzer, bei HD10 mit einer falschen Gegenstelle
  der Sitzungsbenutzer dieses Kanals und bei HD9 jeder frühere Sitzungsbenutzer, dessen Home sich
  auflisten lässt. Früher heißt: Der Host-Dienst hat den Namen im Zustandsordner an eine Person
  gebunden (2.2 Punkt 3), und er steht nicht mehr in `host_service.session_users`. Die übrigen
  Verstöße, die den Start verweigern oder Sitzungen beenden, kann keine Person mit Terminal
  herbeiführen; sie liegen in Konfiguration, Gruppen, `sudo`, Rechten des Zustandsordners und der
  Laufwerkswurzel oder im Besitz der Programme, und ihre Einträge nennen kein Konto. Namen anderer Benutzer des Rechners
  gehen nie an den Server; ein Verstoß bei ihnen steht dort in einem Eintrag ohne Namen, und die
  Ausgabe auf dem Rechner nennt sie (unten). Grund für den Namen: HD6, HD9 und HD10 legen mit
  einem solchen Verstoß den ganzen Rechner still (7.4, "Grund für HD9", HD10 in der Tabelle), und
  HD6 kann in mehreren Konten zugleich anschlagen; der Betreiber muss jedes Konto kennen, in dem
  er suchen muss, und die Konsole nennt dazu die Person der Zuordnung. Eine neue Prüfung folgt
  derselben Regel. Der Name ist kein Geheimnis, der Server kennt ihn aus der Zuordnung; die Namen
  anderer Benutzer braucht er nicht, und deren Home kann keine Person mit Terminal ändern. Grenze: Nach
  dem Verlust des Zustandsordners kennt der Host-Dienst die früheren Bindungen nicht mehr (2.2
  Punkt 3, Grenze); ein früherer Sitzungsbenutzer aus der Zeit davor steht dann ohne Namen in der
  Meldung, und nur die Ausgabe auf dem Rechner nennt ihn.

  Ausgabe auf dem Rechner: `bothy-host doctor` gibt je Prüfung Ergebnis und Code aus, bei HD6,
  HD10 und HD11 die Namen nach der Regel für Namen, bei HD9 jeden Benutzer, dessen Home sich
  auflisten lässt, und bei `drive_root_writable` das Laufwerk; nie Inhalte oder Werte. Der
  Exit-Code ist ungleich 0, sobald eine Prüfung nicht besteht; Warnungen allein ändern ihn nicht.
  Läuft der Host-Dienst, führt der Befehl (mit Administratorrechten) den vollen Lauf im laufenden
  Dienst aus und gibt dessen Ergebnis aus, und der Dienst meldet es mit `report`; den Weg dorthin
  belegt das Bau-Ticket. Findet dieser Lauf einen Verstoß, dessen Folge "Start verweigert" ist,
  außer bei HD6, HD10 und HD11, die im Betrieb ihre eigene Folge haben, beendet sich der
  Host-Dienst wie bei einem verweigerten Start (unten), und seine Sitzungen enden wie bei jedem
  Neustart (6.2). Läuft er nicht, prüft der Befehl allein.

  Meldung bei verweigertem Start: Verweigert `doctor` beim Start des Dienstes den Start, startet
  der Host-Dienst keine PTY und nimmt keine Sitzung an. Ist er gekoppelt, meldet er das Ergebnis
  vorher noch: Er meldet sich über die Steuerverbindung an, schickt `report`, schließt die
  Verbindung, ohne ein `session.open` anzunehmen, und beendet sich. So nennt die Konsole auch bei
  einem Treffer beim Start, etwa nach einem Neustart des Rechners oder nach
  `bothy-host session-user remove`, jeden betroffenen Sitzungsbenutzer; erreicht der Host-Dienst
  den Server nicht oder kann er sich nicht anmelden (etwa weil seine Schlüssel nicht lesbar sind),
  nennt ihn die Ausgabe auf dem Rechner. Grund: Sonst zeigte die Konsole den Rechner nur als
  getrennt, mit dem Ergebnis aus der Zeit davor. Eine Sitzung gibt es dabei nicht; das ist, was
  Ruling T2 verlangt.

  Ein verweigerter Start ist kein Absturz: Der Host-Dienst endet dann mit einem eigenen Exit-Code,
  nach dem ihn der Dienstverwalter nicht von selbst neu startet (so legt es die Dienst-Definition
  aus `bothy-host install` fest; den Weg je Plattform belegt das Bau-Ticket). Wieder gestartet
  wird er erst mit `bothy-host restart`, nachdem der Betreiber den Verstoß behoben hat
  ([16-betrieb.md](16-betrieb.md), Abschnitt 4.4 Punkt 6). Grund: Keiner der Verstöße mit der
  Folge "Start verweigert" behebt sich ohne Eingriff [abgeleitet]; ein Neustart durch den
  Dienstverwalter meldete den Rechner nur in einer Schleife immer wieder an.

- Verbindung nur ausgehend vom Host-Dienst zum Server, über WebSocket mit TLS. Der Rechner öffnet
  keinen Port. Grund: funktioniert hinter NAT, im privaten Netz und in der Cloud gleich.

### 2.3 Server-Host

Läuft der Host-Dienst auf dem Rechner, der den Server betreibt, erreicht eine Shell dort den
Secret-Provider, die Datenbank und die Sicherungen. Darum gilt ein Rechner als Server-Host, wenn
eines zutrifft:

- Der Owner erklärt es bei der Bestätigung (Pflichtfrage, `hosts.declared_server_host`).
- Der Host-Dienst meldet es (`hosts.reported_is_server_host`): Die Adresse aus `server.public_url`
  löst auf eine Adresse einer eigenen Netzschnittstelle oder auf Loopback auf, oder die Gegenstelle
  der Steuerverbindung ist eine lokale Adresse [abgeleitet: eine Heuristik, die einen Server hinter
  einem fremden Rand nicht erkennt].

Dann gilt:

- Warnung in Konsole und Rechner-Karte;
- `bothy-host` verweigert Sitzungen, deren Marke `own = false` trägt (Abschnitt 5.3), und der
  Server prüft dasselbe vorher (SR10).

Erkennt der Host-Dienst den Server-Host nicht, schützt SR10 trotzdem: Nicht-Owner bekommen nur
Rechner, deren Inhaber sie sind, und der Owner weist den Server-Host niemandem zu.

## 3. Rechner und Lebenszyklus

### 3.1 Zustände

```mermaid
stateDiagram-v2
    [*] --> pairing: host.manage (anlegen), Kopplungs-Code
    pairing --> confirming: host.pair (Schlüssel übergeben)
    pairing --> pairing: neuer Kopplungs-Code
    confirming --> active: host.manage (Fingerabdruck bestätigt, LLM-Anmeldung erklärt)
    confirming --> pairing: neuer Kopplungs-Code
    active --> pairing: Neukopplung
    active --> disabled: host.manage (sperren)
    disabled --> active: host.manage (entsperren, nicht bei duplicate_connection)
    disabled --> pairing: Neukopplung
    active --> disabled: zweite Steuerverbindung (3.4)
    active --> retired: host.manage (zurückziehen)
    disabled --> retired: host.manage (zurückziehen)
    retired --> [*]
```

Nur `active` erlaubt Sitzungen. Jeder Übergang aus `active` beendet alle Sitzungen des Rechners
(Grund in Abschnitt 10.3) und erhöht die Rechte-Version, weil er wirksame Rechte ändert. Sperren
durch den Owner setzt `hosts.disabled_reason = 'owner'`; ein Rechner mit `duplicate_connection`
(3.4) lässt sich nicht entsperren, nur neu koppeln (`409 invalid_state_transition`).

### 3.2 Kopplung

1. Der Owner legt den Rechner an (`host.manage`, Step-up, Audit) mit Name, optionalem Inhaber
   (`assigned_user_id`) und Grund. Er bekommt genau einmal einen Kopplungs-Code im Format
   `bothy_hpc_<zufall>_<prüfsumme>` ([ADR 0061](../adr/0061-token-format-mit-festem-praefix-und-pruefsumme.md));
   gespeichert wird nur der HMAC. Gültigkeit `terminal.pairing_code_ttl`, einmalig.
2. Auf dem Rechner ruft der Betreiber `bothy-host pair --server <url> --code <code>` auf. Der
   Host-Dienst erzeugt lokal ein statisches X25519-Schlüsselpaar (Noise) und ein
   Ed25519-Schlüsselpaar (Anmeldung beim Server), legt beide privat im Zustandsordner ab
   (Abschnitt 4.2) und sendet die öffentlichen Schlüssel mit dem Code, seinen Profilen und den
   Namen seiner Sitzungsbenutzer (`host.pair`, öffentlich, `rl.public_auth`). Der Rechner wird
   `confirming`.
3. `bothy-host pair` zeigt lokal den Fingerabdruck (Abschnitt 4.3). Der Owner tippt die ersten
   `terminal.fingerprint_confirm_chars` Zeichen in der App ein, erklärt dabei die
   LLM-Anmeldung des Rechners (Abschnitt 7) und ordnet Personen ihre Sitzungsbenutzer zu (2.2).
   Die App bietet dafür nur die Namen an, die der Rechner gemeldet hat
   (`hosts.reported_session_users`); einen anderen Namen lehnt der Server mit
   `400 validation_failed` ab (`errors[].code = session_user_not_on_host`). Erst dann wird der
   Rechner `active` (`host.manage`, Step-up, Audit). Zuordnungen lassen sich später mit
   `host.manage` ergänzen oder beenden, mit derselben Prüfung gegen die letzte Meldung. Grund für
   das Eintippen: Ein blind bestätigter Rechner wäre eine Einladung an jeden, der den Code
   abgefangen hat (wie R-ID-08 für Geräte).

Ein falscher Fingerabdruck ergibt `409 fingerprint_mismatch`, der Rechner bleibt `confirming`;
nach `terminal.fingerprint_max_attempts` Fehlversuchen fällt er auf `pairing` zurück, und die
Schlüssel verfallen.

### 3.3 Neukopplung

Ein neuer Kopplungs-Code für einen bestehenden Rechner (`host.manage`) setzt ihn auf `pairing`,
verwirft die gespeicherten Schlüssel sofort, schließt seine Verbindungen und beendet seine
Sitzungen (`host_rekeyed`). `hosts.key_generation` steigt. Der Audit-Eintrag nennt alten und neuen
Fingerabdruck; jeder aktive Owner bekommt das Ereignis `host.updated` mit `key_changed = true`.
Clients sehen beim nächsten Öffnen einen anderen Schlüssel und warnen hart (Abschnitt 5.5).

### 3.4 Zweite Steuerverbindung

Je Rechner gilt genau eine Steuerverbindung. Meldet sich eine zweite mit demselben Anmeldeschlüssel
an, während die ältere noch besteht, prüft der Server die ältere mit einem WebSocket-Ping (mit
`cluster.mode = multi` über `NOTIFY ephemeral` an den Knoten der älteren Verbindung, 03, Abschnitt
6.6):

- Antwortet die ältere nicht innerhalb von `terminal.host_duplicate_probe_timeout`, gilt sie als
  tot; der Server schließt sie mit `4009` und nimmt die neue an. Das ist der übliche Fall nach
  einem Netzwechsel.
- Antwortet die ältere, laufen zwei Host-Dienste mit denselben Schlüsseln. Das ist ein Hinweis auf
  kopierte Schlüssel. Der Server schließt beide Verbindungen mit `4112`, setzt den Rechner auf
  `disabled` mit `hosts.disabled_reason = 'duplicate_connection'`, beendet alle Sitzungen
  (`host_disabled`), schreibt einen Audit-Eintrag (`host.connect`, Code `duplicate_connection`) und
  sendet `host.updated` an alle Owner. Weiter geht es nur mit einer Neukopplung (3.3).

Grund: Stilles Verdrängen der älteren Verbindung würde einem Angreifer mit kopierten Schlüsseln
erlauben, den echten Rechner zu verdrängen und jede spätere Sitzung zu empfangen.

## 4. Schlüssel

### 4.1 Zwei Schlüsselpaare je Rechner

| Schlüssel | Algorithmus | Zweck | Server kennt |
|---|---|---|---|
| Noise-Schlüssel | X25519 | statischer Schlüssel des Rechners im Noise-Muster `NK` | öffentlichen Teil (`hosts.noise_public_key`) |
| Anmeldeschlüssel | Ed25519 | Anmeldung der Steuer- und Datenverbindungen | öffentlichen Teil (`hosts.auth_public_jwk`, `hosts.auth_jkt`) |

Grund für zwei Schlüssel: Der Anmeldeschlüssel beweist dem Server den Rechner, der Noise-Schlüssel
beweist ihn dem Gerät. Ein Fehler in einem Pfad gibt den anderen nicht preis.

### 4.2 Ablage auf dem Rechner

Private Schlüssel liegen als Datei im Zustandsordner `host_service.state_dir` mit Modus `0600`
(Ordner `0700`), im Besitz des Dienstbenutzers. Unter Windows verschlüsselt der Host-Dienst die
Datei mit DPAPI im Kontext seines Dienstbenutzers. Einen Schlüsselbund des Betriebssystems nutzt er
nur, wo der Dienstbenutzer eine entsperrte Sitzung hat. Grund: Der Schlüsselbund eines
Dienstbenutzers ohne Anmeldesitzung ist nicht entsperrt (Ruling T10).

Kein Sitzungsbenutzer kann den Zustandsordner lesen (HD8). Wer auf dem Rechner zum
Dienstbenutzer aufsteigt, hat die Schlüssel trotzdem; dann zeigt 3.4 den zweiten Host-Dienst an,
sobald beide gleichzeitig verbunden sind. Ist der echte Rechner in dieser Zeit aus, bleibt der
Diebstahl bis zu seiner Rückkehr unbemerkt (Bedrohungsmodell T-TM-05).

### 4.3 Fingerabdruck

`key_fingerprint = base32_crockford(SHA-256("bothy-host-fp/1" || noise_public_key ||
auth_public_key_raw))`, Anzeige in Gruppen zu vier Zeichen. Beide Schlüssel stehen darin, weil ein
Wechsel jedes der beiden eine Neukopplung ist.

Einzelheiten (docs/DECISIONS.md D-M2-38-11): `noise_public_key` ist der rohe X25519-Schlüssel,
`auth_public_key_raw` der rohe Ed25519-Schlüssel (das dekodierte `x` des JWK), je 32 Byte.
Crockford-Base32 mit dem Alphabet `0123456789ABCDEFGHJKMNPQRSTVWXYZ`: 5-Bit-Gruppen ab dem
höchstwertigen Bit, die letzte Gruppe rechts mit Null-Bits aufgefüllt, Großbuchstaben, ohne
Auffüllzeichen und ohne Prüfzeichen. Die 32 Byte des Hashs ergeben 52 Zeichen, angezeigt als 13
Gruppen zu vier, verbunden mit `-` (die Form von `key_fingerprint` in `openapi.yaml`). Bekannter
Vektor: Noise-Schlüssel mit den Bytes `0x00` bis `0x1f` und Anmeldeschlüssel mit `0x20` bis `0x3f`
ergeben `F0CC-6QS6-RF5D-3BSH-CAZQ-PXMR-J8RA-AHK5-1HF0-9S02-1GW6-BW3J-HK90`. Server, Host-Dienst
und Apps prüfen ihre Berechnung gegen `services/host/testdata/noise/fingerprint.json`.

## 5. Ende-zu-Ende-Strom

### 5.1 Schutzziel

Der Server ist Relais und Autorisierungspunkt. Die Ende-zu-Ende-Verschlüsselung schützt den Inhalt
gegen: Relais und Proxy, Logs, einen Abzug der Datenbank oder der Sicherungen, einen mitlesenden
api-Prozess.

Sie schützt **nicht** gegen einen bösartigen Server-Betreiber. Grund: Das Muster `NK` beweist nur,
dass die Gegenstelle den Schlüssel besitzt, den der Server bei der Kopplung verteilt hat; ein
bösartiger Server kann einen eigenen Schlüssel verteilen und Sitzungen erlauben. Das Produkt setzt
den Server ohnehin als vertrauenswürdig voraus ([19-sicherheit.md](19-sicherheit.md), Abschnitt 1).
Gegenmittel: Schlüssel-Pinning je Gerät (5.5) und die Option `host_service.require_local_confirm`
(Abschnitt 6.4). Ruling T3.

In der Web-Konsole wirkt das Pinning gegen einen bösartigen Server nicht: Der Server liefert den
Code der Konsole selbst und kann eine Fassung ohne Pin-Prüfung schicken. Dort bleibt gegen diesen
Gegner nur `host_service.require_local_confirm`. Das Pinning der Konsole fängt nur einen
Schlüsselwechsel ab, den ein ehrlicher Server meldet (Neukopplung), und darum löscht die Konsole
ihre Pins beim Abmelden ohne Verlust an Schutz ([ADR 0070](../adr/0070-terminal-in-der-web-konsole.md)).

### 5.2 Protokoll

Noise Protocol Framework, Revision 34 (https://noiseprotocol.org/noise.html, abgerufen
2026-10-10 [belegt]), Protokollname `Noise_NK_25519_ChaChaPoly_SHA256`.

| Element | Festlegung | Grund |
|---|---|---|
| Muster | `NK`: `<- s`, `...`, `-> e, es`, `<- e, ee` [belegt, Abschnitt 7.5 der Spezifikation] | Das Gerät kennt den statischen Schlüssel des Rechners aus der Kopplung; der Rechner kennt das Gerät nicht, ihn überzeugt die Sitzungs-Marke |
| Prolog | `"bothy-terminal/1"` \|\| Sitzungs-ID (16 Byte) \|\| Rechner-ID (16 Byte); die IDs in der Oktett-Reihenfolge von RFC 9562, also die Bytes so, wie die Textform sie von links nach rechts nennt (docs/DECISIONS.md D-M2-38-7) | Abweichender Prolog lässt den Handshake scheitern (Abschnitt 6 der Spezifikation [belegt]); bindet Handshake an Protokollversion, Sitzung und Rechner |
| Nachricht 1 (Gerät an Rechner) | `-> e, es`, Nutzlast: Sitzungs-Marke (5.3) | Die Nutzlast ist nur an den Rechner verschlüsselt und laut Spezifikation wiederholbar (Abschnitt 7.7, Ziel-Eigenschaft 2 [belegt]); die Einmal-Marke schließt das |
| Nachricht 2 (Rechner an Gerät) | `<- e, ee`, Nutzlast: `{"v":1,"host_version":...,"profile":...}` | Gerät weiß danach, dass es mit dem Inhaber des statischen Schlüssels spricht |
| Erste Transportnachricht des Geräts | Rahmen `start` mit Spalten und Zeilen | erst danach startet die PTY (Ruling T9) |
| Größe | jede Noise-Nachricht höchstens 65535 Byte [belegt, Abschnitt 3 der Spezifikation] | Grenze der Spezifikation; ein WebSocket-Binärrahmen trägt genau eine Noise-Nachricht |
| Diffie-Hellman | ein Ergebnis aus 32 Null-Bytes (öffentlicher Schlüssel kleiner Ordnung) lässt den Handshake scheitern, in Go und Dart (docs/DECISIONS.md D-M2-38-9) | ein solches gemeinsames Geheimnis hängt nicht vom privaten Schlüssel ab; ein Angreifer hätte es gewählt |
| Nach einem Fehler | nach einem gescheiterten Handshake-Schritt oder einer gescheiterten Entschlüsselung verweigert das Objekt jeden weiteren Aufruf; jede Richtung verweigert die Nonce 2^64 - 1 (Abschnitt 5.1 der Spezifikation [belegt]; docs/DECISIONS.md D-M2-38-8) | eine gescheiterte Authentifizierung ist Manipulation (5.3, `4104`); Weitermachen ließe ein Relais Proben senden |

Bibliotheken:

- Go: `github.com/flynn/noise`, BSD-3-Clause (https://github.com/flynn/noise/blob/master/LICENSE,
  abgerufen 2026-10-10 [belegt]); kennt `HandshakeNK`, `DH25519`, `CipherChaChaPoly`,
  `HashSHA256` und `Config.Prologue` (https://pkg.go.dev/github.com/flynn/noise, abgerufen
  2026-10-10 [belegt], Version v1.1.0 vom 2024-02-02). Der Pflegezustand der Bibliothek ist nicht
  belegt [vermutet: wenig Aktivität]; der Konformitätstest TM-T01 mit den Testvektoren unten hält
  sie, und das Bau-Ticket prüft vor der Aufnahme Lizenz und Stand mit dem Lizenz-Gate. Gepinnt in
  `services/host/go.mod`: `github.com/flynn/noise` v1.1.0 (neueste Version laut Go-Modul-Proxy am
  2026-10-12 [belegt]) mit `golang.org/x/crypto` v0.58.0 und `golang.org/x/sys` v0.49.0;
  `tools/ci/tests/test_bothy_noise_package.py` hält diese Versionen gleich `go.mod` und
  `pubspec.lock`.
- Dart: X25519, ChaCha20-Poly1305, SHA-256 und HMAC-SHA-256 aus dem Paket `cryptography` (die HKDF
  der Noise-Spezifikation baut `bothy_noise` selbst aus HMAC, Abschnitt 4.3 der Spezifikation), Apache-2.0,
  Version 2.9.0 (https://pub.dev/packages/cryptography, abgerufen 2026-10-10 [belegt]; neueste
  Version am 2026-10-12, gepinnt in `pubspec.lock` (root of the Dart workspace, M4-01)); der
  Handshake selbst als eigenes kleines Paket `packages/bothy_noise`, geprüft gegen dieselben
  Testvektoren. In der Web-Konsole nutzt `cryptography` die Web Crypto API, sonst reines Dart
  (gleiche Quelle [belegt]).

Testvektoren: Die Spezifikation selbst enthält keine. Das Noise-Wiki verweist auf Vektor-Dateien
anderer Projekte (https://github.com/noiseprotocol/noise_wiki/wiki/Test-vectors, abgerufen
2026-10-10 [belegt]). TM-T01 nutzt zwei davon, die beide einen Eintrag für
`Noise_NK_25519_ChaChaPoly_SHA256` enthalten:
`vectors/cacophony.txt` aus https://github.com/centromere/cacophony und
`tests/vector/noise-c-basic.txt` aus https://github.com/rweather/noise-c (beide abgerufen
2026-10-10 [belegt]). Das Bau-Ticket legt die Dateien mit Commit-Hash und Lizenz unter
`services/host/testdata/noise/` ab und nutzt sie für Go und Dart. Sie liegen unverändert, aber mit
gzip komprimiert unter `vectors/`, mit Herkunft, Commit, Lizenz und Prüfsumme des Inhalts in
`services/host/testdata/noise/README.md` (docs/DECISIONS.md D-M2-38-15); cacophony liegt heute
unter https://github.com/haskell-cryptography/cacophony (GitHub leitet um). Die Aufzeichnungen
für TM-T02 liegen in `services/host/testdata/noise/recordings/` (`go.json` von Go, `dart.json` von
Dart).

### 5.3 Sitzungs-Marke

Der Server stellt nach `terminal.open` bzw. `terminal.attach` eine Sitzungs-Marke aus: eine
kompakte JWS nach dem JOSE-Profil ([ADR 0067](../adr/0067-jose-profil-mit-der-standardbibliothek.md)),
Kopf `alg = EdDSA`, `kid`, `typ = terminal-session+jwt`, signiert mit einem Server-Schlüssel mit
Zweck `terminal` (03, `server_signing_keys`). Der Host-Dienst holt die öffentlichen Schlüssel über
die Steuerverbindung (`hello` trägt die JWKS des Zwecks).

| Claim | Inhalt |
|---|---|
| `iss` | `instance.instance_id` |
| `aud` | Rechner-ID |
| `sub` | Prinzipal-ID des Menschen |
| `sid` | Sitzungs-ID; gleich der Sitzungs-ID im Prolog |
| `jti`, `iat`, `exp` | UUIDv7 der Marke, Ausstellung, Ablauf (`iat + terminal.session_token_ttl`) |
| `dev` | Geräte-ID |
| `prf` | Name des Profils |
| `cek` | SHA-256 des ephemeren öffentlichen Schlüssels des Geräts (base64url), den das Gerät beim Öffnen meldet |
| `own` | `true`, wenn die Person zur Zeit der Ausstellung aktiver Owner ist |
| `osu` | Name des Sitzungsbenutzers der Person auf diesem Rechner (`host_session_users.os_user`, 2.2) |
| `wdi` | nur beim Profil der Art `claude` und `att = open`: Index des Arbeitsordners in der Liste von `osu` (11.1) |
| `att` | `open` oder `attach` |

Der Host-Dienst prüft in dieser Reihenfolge und bricht sonst ohne PTY ab: Signatur mit einem
Schlüssel aus der JWKS, `typ`, `aud` gleich eigener ID, `sid` gleich Prolog, `exp` nicht
überschritten (Toleranz `auth.clock_leeway`), `jti` nie gesehen (er merkt jede `jti` bis zu ihrem
`exp`, Ruling T9), `cek` gleich dem Hash des `e` aus Nachricht 1, `prf` ist ein eigenes Profil,
`osu` steht in `host_service.session_users` und ist keiner anderen Person (`sub`) gebunden (2.2),
beim Profil `claude` ist `wdi` ein Index in die Liste der Arbeitsordner von `osu` nach HD7, die er
mit dem letzten `report` gemeldet hat (2.2 Punkte 5 und 6; nie in die eines anderen
Sitzungsbenutzers, Ruling T13), und auf
dem Server-Host `own = true`. Er meldet den Grund als `close_detail` (10.3); der Server
schließt den Strom des Geräts danach mit genau einem Code:

| Fall | `close_detail` | Schließ-Code | Verhalten des Geräts |
|---|---|---|---|
| `exp` überschritten | `token_expired` | `4101` | mit neuem Step-up neu öffnen bzw. anhängen |
| `jti` schon gesehen | `token_replayed` | `4101` | wie oben |
| Signatur, `kid`, `typ`, `aud`, `sid` oder `cek` falsch | `token_invalid` | `4104` | harte Warnung, kein neuer Versuch |
| Handshake gescheitert (Prolog, Schlüssel) | `handshake_failed` | `4104` | wie oben |
| Profil unbekannt oder `wdi` kein Arbeitsordner von `osu` | `profile_unknown` | `4104` | wie oben |
| `osu` nicht in `host_service.session_users` | `session_user_unknown` | `4104` | wie oben |
| `osu` schon einer anderen Person gebunden | `session_user_mismatch` | `4104` | wie oben |
| Server-Host ohne `own = true` | `server_host_owner_only` | `4104` | wie oben |

Grund für die Teilung: Eine abgelaufene oder verbrauchte Marke entsteht auch ohne Angriff (langsames
Netz, doppelter Klick); alles andere deutet auf Manipulation. Ein Name, den der Rechner nicht
meldet, fällt schon bei der Zuordnung (`400 validation_failed`) oder beim Öffnen (SR12, `403
terminal_denied` mit `reason: no_session_user`) auf (2.2 Punkt 5). Weil der Host-Dienst nach
jeder Anmeldung zuerst `report` schickt und seine Sitzungsbenutzer und Arbeitsordner erst nach
einem Neustart ändert, deuten `session_user_unknown` (eine Marke mit einem Namen, den der Rechner
in seinem letzten `report` nicht gemeldet hat) und `profile_unknown` bei einem `wdi` außerhalb der
gemeldeten Liste von `osu` nur auf einen Fehler oder eine Manipulation im Server.
Grund für `cek`: Ein Relais kann die Marke nicht in einen eigenen Handshake setzen, weil der Hash
an den ephemeren Schlüssel des Geräts gebunden ist.

### 5.4 Rahmen

Jede WebSocket-Binärnachricht ist genau eine Noise-Nachricht. Nach dem Handshake trägt jede
Transportnachricht einen Rahmen:

| Typ (1 Byte) | Name | Richtung | Inhalt |
|---|---|---|---|
| `0x01` | `start` | Gerät an Rechner | Spalten, Zeilen (je 2 Byte); einmal, als erste Transportnachricht |
| `0x02` | `data` | beide | Bytes der PTY bzw. Eingabe |
| `0x03` | `resize` | Gerät an Rechner | Spalten, Zeilen |
| `0x04` | `close` | Gerät an Rechner | Sitzung beenden |
| `0x05` | `exit` | Rechner an Gerät | Exitcode des Prozesses (4 Byte, vorzeichenbehaftet) |
| `0x06` | `replay_begin`, `0x07` `replay_end` | Rechner an Gerät | umschließen den Rückblick-Puffer beim Wiederanhängen |

Steuerdaten liegen in der verschlüsselten Nutzlast; das Relais sieht nur Länge und Zeitpunkt.
Ein unbekannter Typ beendet die Sitzung (Schließ-Code `4104`). Textrahmen sind auf diesen
Verbindungen verboten (`4002`).

Kodierung (docs/DECISIONS.md D-M2-38-10): Alle Zahlen sind Big-Endian. `start` und `resize`
tragen genau 4 Byte, erst Spalten, dann Zeilen, je 1 bis 65535 (wie `TerminalSessionOpen`).
`data` trägt 1 bis 65518 Byte; ein leeres `data` zählte als Eingabe ohne Eingabe (6.3). `close`,
`replay_begin` und `replay_end` tragen keinen Inhalt. `exit` trägt genau 4 Byte, eine
vorzeichenbehaftete 32-Bit-Zahl im Zweierkomplement. Ein Rahmen ist höchstens 65519 Byte lang,
der größte Klartext einer Transportnachricht. Richtungen wie in der Tabelle; jeder Fehler beim
Lesen eines Rahmens (leer, zu lang, unbekannter Typ, falsche Richtung, falscher Inhalt) beendet
die Sitzung wie ein unbekannter Typ (`4104`).

### 5.5 Schlüssel-Pinning im Gerät

Jedes Gerät speichert beim ersten erfolgreichen Öffnen einer Sitzung auf einem Rechner dessen
`noise_public_key` und `key_generation` im lokalen Speicher. Liefert der Server später einen
anderen Schlüssel, zeigt die App eine harte Warnung mit beiden Fingerabdrücken und verbindet erst
nach ausdrücklicher Bestätigung durch die Person; die Bestätigung steht im Audit
(`terminal.open` mit `pin_changed = true`). Grund: Ein Schlüsselwechsel ohne Neukopplung wäre ein
Hinweis auf einen Angriff des Servers selbst (5.1). In der Web-Konsole schützt das Pinning nicht
gegen den Server selbst (5.1).

## 6. Sitzungen

### 6.1 Öffnen

Diagramm: [seq-8-terminal-oeffnen.svg](../architecture/seq-8-terminal-oeffnen.svg) (Quelle
[seq-8-terminal-oeffnen.mmd](../architecture/seq-8-terminal-oeffnen.mmd)).

1. Das Gerät erbringt einen Step-up (08, Abschnitt 6.3) und ruft
   `POST /api/v1/hosts/{host_id}/terminal/sessions` mit Profil, Größe und dem Hash seines
   ephemeren Schlüssels (`client_ephemeral_key_hash`), beim Profil `claude` dazu mit dem Index
   des Arbeitsordners in der gemeldeten Liste des eigenen Sitzungsbenutzers (`workdir_index`,
   11.1).
2. `authorize(person, terminal.open, host)`: Recht `terminal` (07, Abschnitt 2.12), SR10, SR11
   (I6), SR12 (gültige Zuordnung eines Sitzungsbenutzers, dessen Namen der Rechner meldet),
   frischer und unverbrauchter Step-up, Grenzen (`terminal.max_sessions_per_person`,
   `terminal.max_sessions_per_host`), Rechner `active` und verbunden, Profil bekannt, beim Profil
   `claude` ein Arbeitsordner des eigenen Sitzungsbenutzers.
3. In einer Transaktion: Step-up verbrauchen (08, Abschnitt 6.3), Zeile in `terminal_sessions`
   mit `state = 'opening'` und dem Sitzungsbenutzer der Person (`host_session_user_id`), Audit, Ereignis `terminal_session.changed`. Antwort: Sitzungs-ID,
   Marke, Stream-Pfad, `noise_public_key`, `key_fingerprint`, `key_generation`.
4. Der Server weist den Host-Dienst über die Steuerverbindung an (`session.open` mit Sitzungs-ID
   und Datenpfad). Der Host-Dienst öffnet die Datenverbindung
   `/api/v1/hosts/sessions/{session_id}/stream`.
5. Das Gerät öffnet `/api/v1/terminal/sessions/{session_id}/stream` und sendet Nachricht 1.
6. Das Relais verbindet beide Seiten, sobald beide da sind; kommt eine Seite nicht innerhalb von
   `terminal.open_timeout`, endet die Sitzung mit `open_failed`.
7. Nach Nachricht 2 und dem Rahmen `start` startet der Host-Dienst die PTY; er meldet
   `session.started` über die Steuerverbindung; der Server setzt `state = 'active'`.

### 6.2 Trennen und Wiederanhängen

- Bricht die Verbindung des Geräts ab, bleibt die PTY auf dem Rechner `terminal.detach_grace`
  lang bestehen; der Server setzt `state = 'detached'`. Der Rückblick-Puffer liegt nur im Speicher
  des Host-Dienstes (`host_service.scrollback_bytes`), nie im Server.
- Wiederanhängen (`terminal.attach`) gibt eine neue Marke und erzeugt einen neuen Noise-Handshake
  mit neuem ephemerem Schlüssel; der Host-Dienst spielt den Rückblick-Puffer zwischen
  `replay_begin` und `replay_end` ein. Die alte Marke ist damit wertlos (der Server speichert nur die
  aktuelle `jti`).
- Step-up: Von einem anderen Gerät derselben Person immer frisch. Vom selben Gerät in derselben
  Anmeldesitzung innerhalb von `terminal.quick_reattach_window` nach der Trennung ohne neuen
  Step-up. Grund: Mobilnetze brechen oft kurz ab; eine Passkey-Abfrage je Funkloch würde zum
  blinden Bestätigen erziehen. Die Ausnahme gilt nur für dasselbe Gerät, dessen Schlüssel die
  Anfrage beweist, und darum nie in der Web-Konsole (`client_component = 'console'`): Dort gibt es
  keinen Geräteschlüssel, nur das Cookie der Konsole. Die Konsole braucht für jedes Anhängen einen
  frischen Passkey ([ADR 0070](../adr/0070-terminal-in-der-web-konsole.md)).
- Hängt ein anderes Gerät an, schließt der Server die ältere Verbindung mit `4103`.
- Bricht die Steuer- oder Datenverbindung des Host-Dienstes ab, setzt der Server die Sitzung auf
  `detached`. Der Host-Dienst hält die PTY und baut die Datenverbindung nach dem Wiederverbinden
  neu auf; das Gerät hängt danach mit `terminal.attach` neu an.
- Startet der Host-Dienst neu (`bothy-host restart`, `bothy-host session-user add` oder `remove`,
  ein Absturz, ein Neustart des Rechners), enden alle Sitzungen des Rechners. Jeder PTY-Helfer
  beendet seine PTYs samt Prozessbaum (6.5), sobald sein Kanal zum Host-Dienst abbricht. Bei der
  Anmeldung der Steuerverbindung nennt der Host-Dienst die Sitzungen, die er kennt (`live_sessions`
  in `identify`): deren PTY noch läuft oder deren Ende er noch nicht melden konnte; dieses Ende
  meldet er danach mit `session.ended`. Nach einem Neustart kennt er keine. Der Server beendet jede
  andere Sitzung des Rechners im Zustand `active` oder `detached` mit `host_restarted`. Grund: Der neue
  Prozess liest seine Konfiguration neu und prüft beim Start alles neu (2.2); eine Sitzung aus der
  Zeit davor liefe ungeprüft unter der alten Konfiguration weiter, etwa mit einem entfernten
  Arbeitsordner. Ihr Rückblick-Puffer lag im Speicher des alten Prozesses. Die Liste in `identify`
  braucht keinen Zustand im Server und trennt einen Neustart von einem Netzabbruch, nach dem die
  PTYs weiterlaufen.
- Nach `terminal.detach_grace` ohne Anhängen beendet der Host-Dienst die PTY, der Server schließt
  die Sitzung mit `detach_expired`.

### 6.3 Grenzen

| Grenze | Wert | Wirkung |
|---|---|---|
| Leerlauf | `terminal.idle_timeout` | ohne Eingabe des Geräts (Rahmen `data`, `resize`); Ende `idle_timeout` |
| Höchstdauer | `terminal.max_session_duration` | ab `opened_at`, auch über Wiederanhängen hinweg; Ende `max_duration` |
| Sitzungen je Person | `terminal.max_sessions_per_person` | nicht `closed`; Ablehnung `terminal_session_limit` |
| Sitzungen je Rechner | `terminal.max_sessions_per_host` | dasselbe |
| Puffer im Relais | `terminal.relay_buffer_max_bytes` je Richtung | voll: Verbindung des langsamen Endes mit `4014` schließen, Sitzung `detached` |
| Rate-Limit | `rl.terminal` je Mensch, `rl.host` je Rechner | 05, Abschnitt 3 |

Den Leerlauf misst der Host-Dienst, weil nur er die Rahmentypen sieht; er meldet das Ende über die
Steuerverbindung. Der Server misst zusätzlich die Zeit ohne Bytes vom Gerät im Relais und beendet
nach `terminal.idle_timeout` plus `terminal.idle_grace` selbst. Grund: Ein manipulierter
Host-Dienst darf eine Sitzung nicht beliebig offen halten. Der Job `terminal.sweep`
([02-architektur.md](02-architektur.md), Abschnitt 12.3) prüft dieselben Grenzen in der Datenbank
und fängt Sitzungen eines ausgefallenen Knotens auf.

### 6.4 Lokale Bestätigung

Mit `host_service.require_local_confirm = true` fragt der Host-Dienst vor dem Start jeder PTY auf
dem Rechner nach (Benachrichtigung des Betriebssystems an die angemeldete Person des Rechners) und
startet erst nach Zustimmung innerhalb von `host_service.local_confirm_timeout`. Standard ist aus.
Grund: Rückfall gegen einen bösartigen Server-Betreiber (5.1), nur sinnvoll, wenn jemand am Rechner
sitzt.

### 6.5 Ende einer Sitzung auf dem Rechner

Der PTY-Helfer startet jede Sitzung in einer eigenen Prozessgruppe und Sitzung des Betriebssystems
(`setsid`); unter Linux zusätzlich in einer eigenen cgroup (systemd-Scope des Sitzungsbenutzers),
unter Windows in einem Job Object mit `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`. Endet eine Sitzung,
gleich aus welchem Grund, beendet er den ganzen Baum: unter Linux alle Prozesse der cgroup, unter
Windows das Job Object, unter macOS die Prozessgruppe und danach jeden Prozess des
Sitzungsbenutzers, dessen Elternkette auf die Sitzung zurückgeht. Grund: Ohne das liefe ein mit
`nohup`, `setsid` oder `disown` gestarteter Prozess nach einem Entzug weiter.

Grenze: Dauerläufer, die eine Person als Sitzungsbenutzer einrichtet (crontab, systemd-Dienste des
Benutzers, LaunchAgent, geplante Aufgabe), beendet das nicht; sie laufen außerhalb jedes
Sitzungsbaums. Sie laufen im Sitzungsbenutzer dieser Person, den keine andere Person je bekommt
(2.2, T12), und erreichen über keinen geteilten Ort eine andere Person (T13), darum keine Sitzung
eines anderen Menschen. Nach `bothy-host session-user remove` können sie aber das Home ihres
Benutzers wieder öffnen, das `remove` geschlossen hat; dann verweigert HD9 den Start des ganzen
Rechners, und die Meldung nennt diesen früheren Sitzungsbenutzer (2.2, "Grund für HD9"). Die
Konsole zeigt beim Entzug einer Person, die nicht Owner ist, den Hinweis, ihren Sitzungsbenutzer
mit `bothy-host session-user remove` aus dem Host-Dienst zu nehmen und danach auf dem Rechner
samt seinen Dauerläufern zu löschen; Sperren allein beendet sie nicht
([16-betrieb.md](16-betrieb.md), Abschnitt 4.4 Punkt 1; 2.2 Punkt 3; Bedrohungsmodell T-TM-11).
Die Schlüssel erreichen solche Prozesse nicht (2.2).

### 6.6 Zustand einer Sitzung

| `state` | Bedeutung |
|---|---|
| `opening` | angelegt, Handshake läuft |
| `active` | PTY läuft, Gerät verbunden |
| `detached` | PTY läuft, kein Gerät verbunden; Wiederanhängen möglich |
| `closed` | beendet; `close_reason` gesetzt |

## 7. LLM-Anmeldung und Invariante I6

### 7.1 Warum

Anthropic verbietet, Abo-Anmeldedaten für andere Personen zu nutzen
(https://code.claude.com/docs/en/legal-and-compliance, abgerufen 2026-10-10 [belegt]; Wortlaut in
[10-backend-richtlinie.md](10-backend-richtlinie.md), Abschnitt 2). Ein Terminal erlaubt jede
Nutzung der Anmeldung, die der Sitzungsbenutzer der Person auf dem Rechner hat. Darum bekommt jede
Person ihren eigenen Sitzungsbenutzer (2.2, T12): Eine Anmeldung darin nutzt nur sie. Das Produkt
kann den Inhalt der Anmeldung nicht prüfen. Darum erklärt der Owner zusätzlich je Rechner, welche
Anmeldung dort vorgesehen ist, und der Server erzwingt, wer dann ein Terminal öffnen darf.

### 7.2 Erklärung

`hosts.llm_credentials`, gesetzt bei der Bestätigung (3.2) und später mit `host.manage` (Step-up,
Audit, Rechte `ja`):

| Wert | Bedeutung |
|---|---|
| `none` | Auf dem Rechner gibt es keine LLM-Anmeldung. Das Profil `claude` wird nicht angeboten. |
| `personal` | Die Anmeldung ist das persönliche Abo genau eines Menschen (`hosts.credential_holder_id`). Dieser Mensch meldet sich selbst im Konto seines Sitzungsbenutzers an; der Host-Dienst fasst die Anmeldung nicht an (7.4). |
| `organization` | Die Anmeldung ist ein Organisations-API-Schlüssel aus dem Zustandsordner des Host-Dienstes. |

Der Owner erklärt sie mit `setHostLlmCredentials` (`PUT /api/v1/hosts/{host_id}/llm-credentials`,
M2-40): `credential_holder_id` genau bei `personal`, ein Mensch, der nicht gelöscht ist (fehlt er bei
`personal`: `errors[].code = required`; bei `none` oder `organization` gesendet: `not_allowed`;
unbekannt, kein Mensch oder gelöscht: `unknown_user`), mit Pflichtgrund und `If-Match`. Dieselbe
Erklärung, die der Rechner schon hat, antwortet `200` ohne neue Version, ohne Ereignis und ohne
neue Rechte-Version (Audit mit `no_change`). Ein `retired` Rechner antwortet `409 conflict`, weil
`retired` endgültig ist (3.1); jeder andere Zustand nimmt die Erklärung an, und `confirmHost`
(M2-41) setzt sie bei der Bestätigung erneut (docs/DECISIONS.md D-M2-40-7).

### 7.3 Invariante I6

**I6:** Ist `llm_credentials = personal`, darf nur `credential_holder_id` ein Terminal auf dem
Rechner öffnen oder anhängen. Das gilt auch für andere Owner (Owner-Ausnahme, Ruling T2). Auf
einem solchen Rechner gibt es nur eine Einzel-Freigabe `terminal` an den Inhaber der Anmeldung,
keine Gruppen-Freigaben und keine Freigaben an andere Menschen.

Prüfstellen:

1. **Transaktional** in `grant.create` mit Ziel `host` und in `host.manage` (Bestätigung,
   Wechsel der Erklärung, Wechsel des Inhabers der Anmeldung): Würde danach ein anderer Prinzipal
   als `credential_holder_id` eine gültige Freigabe `terminal` auf dem Rechner haben, rollt die
   Transaktion zurück: `409 i6_violation` (Entscheidung `invariant_violation`), Audit `deny` (03,
   Abschnitt 6.8). `authz` prüft I6 bei jeder Rechte-Änderung wie I1 (M2-40), für jeden Rechner
   mit `personal`: So fängt die Prüfung auch `user.enable` einer Person, deren ruhende Freigabe auf
   einem Rechner, der inzwischen `personal` ist, wieder gültig würde (docs/DECISIONS.md D-M2-40-5).
2. **Bei jedem `terminal.open` und `terminal.attach`**: SR11 in 07, fail closed.
3. **Beim Wechsel** auf `personal` oder des Inhabers beendet dieselbe Transaktion alle Sitzungen
   anderer Menschen auf dem Rechner (`i6_violation`).

Abgrenzung zu I1 ([ADR 0024](../adr/0024-backend-richtlinie-personal-organization-mit-invariante.md),
Ruling L22): I1 kennt keine Owner-Ausnahme, weil Owner auf allen Zielen `trigger` erben und ein
Agent auf vielen Zielen ausgelöst wird. Ein Terminal ist Zugriff auf einen bestimmten Rechner, keine
vererbte Stufe. Ohne Owner-Ausnahme wäre ein persönlicher Rechner bei zwei Ownern unmöglich. Die
Ausnahme schränkt Owner ein; sie erweitert nichts. [ADR 0068](../adr/0068-terminal-ueber-host-dienst-mit-noise-strom-und-invariante-i6.md)
hält das fest.

### 7.4 Durchsetzung auf dem Rechner

- Tragend ist der eigene Sitzungsbenutzer je Person (2.2, Ruling T12): Eine Abo-Anmeldung, die
  in einem Sitzungsbenutzer entsteht, nutzt nur die Person, der er zugeordnet ist; keine Sitzung
  eines anderen Menschen läuft dort. Das hängt an keinem Ergebnis von HD6 und gilt auch dann, wenn
  HD6 gefälscht oder abgeschaltet ist. Es setzt voraus, dass kein Ort die Konten wieder verbindet
  (Ruling T13, 2.2 Punkt 6); das hält die Einrichtung, und HD11 prüft es für jeden konfigurierten
  Pfad. HD6 ist eine Zusatzprüfung: Sie deckt eine
  Erklärung des Owners auf, die nicht zum Rechner passt, und meldet sie. Ein leerer
  `host_service.claude_bin` oder ein gefälschtes Ergebnis von HD6 schwächt die Durchsetzung nicht.
- `bothy-host doctor` (HD6) fragt den Coding-Agenten nach seiner aktiven Anmeldung, nie nach Werten:
  bei Claude Code `claude auth status` mit dem Feld `authMethod` (`none`, `claude.ai`,
  `oauth_token`, `api_key`, `api_key_helper`, `third_party`;
  https://code.claude.com/docs/en/cli-reference, abgerufen 2026-10-10 [belegt]). Zusätzlich meldet
  er nur das Vorhandensein bekannter Anmeldeorte: `~/.claude/.credentials.json` unter Linux,
  `%USERPROFILE%\.claude\.credentials.json` unter Windows, unter macOS der Schlüsselbund mit
  Rückfall auf dieselbe Datei, `CLAUDE_CONFIG_DIR`, `CLAUDE_CODE_OAUTH_TOKEN`
  (https://code.claude.com/docs/en/authentication, abgerufen 2026-10-10 [belegt]). Den Namen des
  Schlüsselbund-Eintrags nennt die Quelle nicht; der Host-Dienst prüft ihn darum nur über
  `claude auth status` [vermutet: kein dokumentierter Weg ohne den Agenten].
- HD6 ruft den Coding-Agenten nie über den `PATH` des Sitzungsbenutzers auf, sondern nur über den
  absoluten Pfad `host_service.claude_bin`, mit einer Umgebung, die der PTY-Helfer selbst setzt:
  `HOME` bzw. `USERPROFILE` und `CLAUDE_CONFIG_DIR` der jeweiligen Prüfung, keine Startdateien
  des Sitzungsbenutzers und eine feste Suchliste als `PATH`, nämlich der Ordner, in dem
  `host_service.claude_bin` laut Konfiguration steht (aufgelöst nach 2.2 Punkt 6, nicht der
  Ordner, auf den ein Verweis `claude` zeigt), dann `/usr/bin` und `/bin` (Windows:
  `%SystemRoot%\System32`). Grund für den konfigurierten Ordner: Ihn hat der Betreiber gewählt,
  und HD10 prüft ihn ohnehin als Ordner auf dem Weg zu `host_service.claude_bin`; der Ordner des
  Ziels eines Verweises hinge davon ab, wie ein Paketverwalter installiert [abgeleitet].
  HD10 prüft die Datei, jeden Ordner auf dem Weg dorthin, jeden Ordner der Suchliste und bei
  einem Skript (erste Zeile `#!`) den Interpreter, bei `#!/usr/bin/env node` also `env` und das
  `node`, das `env` in dieser Suchliste findet, wie die Binärdatei des Helfers: Besitz durch
  einen vertrauenswürdigen Besitzer, für keinen Sitzungsbenutzer schreibbar, alles nach
  Auflösung (2.2 Punkt 6). Jeden Ordner der Suchliste prüft HD10, damit niemand in einem
  früheren Ordner der Liste einen anderen Interpreter vor den geprüften legt. Findet die
  Suchliste den Interpreter nicht, gilt `host_service.claude_bin` für HD6 und das Profil als leer
  (unten): HD6 ruft nichts auf, und bei `organization` gibt es kein Profil `claude`; HD10 prüft
  die Datei und ihre Ordner trotzdem. Grund: Der `PATH` des Profils
  geht nach der Suchliste weiter; `env` fände den Interpreter dann in einem Ordner, den HD10 nie
  geprüft hat, und dieses Programm liefe in jedem Sitzungsbenutzer [abgeleitet]. Module,
  die der Interpreter danach lädt, prüft HD10 nicht [abgeleitet: ihr Suchweg hängt vom
  Interpreter ab]; eine Installation in einem Ordner eines vertrauenswürdigen Besitzers hält auch
  sie außer Reichweite. Unter Windows ist `host_service.claude_bin` eine `.exe`; eine Stapeldatei
  (`.cmd`, `.bat`) besteht HD10 nicht, weil sie kein `#!` trägt und ihr Interpreter und die
  Programme, die sie startet, sonst ungeprüft blieben [abgeleitet]. Claude Code liegt unter
  Windows als `claude.exe` vor (https://code.claude.com/docs/en/setup, abgerufen 2026-10-11
  [belegt]). Ist der Wert leer oder gilt er als leer (der Interpreter fehlt in der Suchliste,
  oder der Pfad lässt sich nicht auflösen, 2.2 Punkt 6), fragt HD6 nur die Anmeldeorte ab, und
  `doctor` warnt mit `auth_status_unchecked` (kein Startverbot); unter macOS bleibt der
  Schlüsselbund dann ungeprüft, und bei `organization` gibt es kein Profil `claude` (HD7,
  Warnung; 11.1). Grund: Ein Sitzungsbenutzer kann ein Programm in seinem `PATH` oder in seinen
  Startdateien durch eines ersetzen, das die Abo-Anmeldung verschweigt. Restgrenze: Dieselbe
  Benutzerkennung kann die Daten ändern, die das echte Programm liest, etwa eine Anmeldung nur
  zwischen zwei Prüfungen anlegen (Bedrohungsmodell T-TM-08, RK-84). Beides trifft nur das Konto
  der Person selbst, weil sie ihren Sitzungsbenutzer mit niemandem teilt (T12).
- Bei `none` oder `organization` und erkannter Abo-Anmeldung in einem Sitzungsbenutzer
  (`authMethod` gleich `claude.ai` oder `oauth_token`, oder eine der Anmeldedateien vorhanden)
  startet `bothy-host` nicht (Ruling T2). Vorher meldet er das Ergebnis mit jedem betroffenen
  Sitzungsbenutzer an den Server, und `bothy-host doctor` nennt dieselben Namen auf dem Rechner
  (2.2, Meldung bei verweigertem Start).
- Prüfstellen: Jeder PTY-Helfer führt HD6 im Konto seines Sitzungsbenutzers aus, und zwar beim
  Start des Dienstes, vor jedem Start einer PTY und alle `host_service.credential_check_interval`;
  jedes Mal im Home des Sitzungsbenutzers und im `CLAUDE_CONFIG_DIR` des Profils `claude` (bei
  `organization` der feste Ordner im Home desselben Sitzungsbenutzers, siehe unten). Grund:
  In einer laufenden Shell kann jemand `claude` starten und sich mit `/login` per Abo anmelden. Die
  Anmeldung landet im Home seines eigenen Sitzungsbenutzers und erreicht keine Sitzung eines
  anderen Menschen (T12, T13); sie widerspricht aber der Erklärung des Owners, und HD6 macht das
  sichtbar, damit der Owner die Erklärung berichtigt oder die Anmeldung entfernen lässt.
- Treffer während des Betriebs: Der Host-Dienst lehnt neue Sitzungen ab (`open_failed` mit
  `close_detail = subscription_login_detected`), beendet alle laufenden Sitzungen des Rechners
  (`close_reason = host_policy_violation`, gleiches `close_detail`) und meldet das Ergebnis über
  `report` an den Server (HD6 nicht bestanden, je betroffenem Sitzungsbenutzer ein Eintrag mit
  seinem Namen, 2.2). Der Server schreibt einen Audit-Eintrag (`host.connect`) und sendet
  `host.updated` an alle Owner; die Konsole zeigt jeden dieser Sitzungsbenutzer und die Person
  seiner Zuordnung. Sitzungen gibt es erst wieder, wenn HD6 in jedem Sitzungsbenutzer aus
  `host_service.session_users` besteht.
  Folge, ehrlich: Eine einzelne Person mit Terminal kann so den Rechner für alle stilllegen, etwa
  mit `/login` in ihrer eigenen Sitzung. Das bleibt so, weil Ruling T2 bei Widerspruch zur
  Erklärung den Start des Host-Dienstes verbietet. Der Stillstand endet auf einem von zwei Wegen
  ([16-betrieb.md](16-betrieb.md), Abschnitt 4.4; Bedrohungsmodell T-TM-14): Der Betreiber
  entfernt die Anmeldung im genannten Sitzungsbenutzer, oder der Owner beendet dessen Zuordnung
  und der Betreiber nimmt den Sitzungsbenutzer mit `bothy-host session-user remove` aus dem
  Host-Dienst (2.2 Punkt 3); danach läuft HD6 in diesem Konto nicht mehr. Danach löscht der
  Betreiber den Benutzer samt seinen Dauerläufern, sonst kann einer davon das Home wieder öffnen
  und über HD9 den Rechner erneut stilllegen (2.2, "Grund für HD9"). Den Benutzer nur auf dem
  Rechner zu sperren oder nur seine Zuordnung zu beenden, reicht nicht: Sein PTY-Helfer läuft
  weiter, und HD6 prüft jeden Namen aus `host_service.session_users`.
- Das Profil `claude` baut seine Umgebung wie der Runner ([09-agenten.md](09-agenten.md),
  Abschnitt 18.3): bei `organization` `ANTHROPIC_API_KEY` aus dem Zustandsordner (der Host-Dienst
  reicht den Wert beim Start über den lokalen Kanal an den PTY-Helfer), ein eigener
  `CLAUDE_CONFIG_DIR` ohne Abo-Anmeldung, die OAuth-Variablen entfernt; bei `personal` keine
  API-Schlüssel; bei `none` gibt es das Profil nicht. Bei `organization` startet das Profil genau
  den aufgelösten Pfad von `host_service.claude_bin`, den HD10 geprüft hat, und der `PATH` des
  Profils beginnt mit der festen Suchliste von HD6, damit ein `#!/usr/bin/env node` denselben
  Interpreter findet, den HD10 geprüft hat; weil der Wert ohne Interpreter in der Suchliste als
  leer gilt (oben), sucht `env` nie im übrigen `PATH`. Ist `host_service.claude_bin` leer oder
  gilt er als leer, bietet der Host-Dienst das Profil bei `organization` nicht an (HD7,
  Warnung). Grund: Dieses Programm läuft
  in jedem Sitzungsbenutzer und ist darum ein Ort für alle (2.2 Punkt 6); ein `claude` aus einem
  `PATH` könnte in einem Ordner liegen, der einem anderen Konto gehört. Bei `organization` setzt
  der PTY-Helfer den
  `CLAUDE_CONFIG_DIR` auf den festen Ordner `.config/bothy-host/claude` im Home des
  Sitzungsbenutzers der Person (unter Windows `%USERPROFILE%\.config\bothy-host\claude`); er
  gehört diesem Sitzungsbenutzer und hat Modus `0700` bzw. eine ACL nur für ihn, nie ein Ordner
  für den ganzen Rechner (Ruling T13, HD11). Bei `personal` gilt der eigene des Sitzungsbenutzers
  aus seiner Login-Shell (siehe unten); den wählt der Inhaber, und nur er öffnet dort ein Terminal
  (I6). Er muss nach Auflösung (2.2 Punkt 6) im Home des Sitzungsbenutzers liegen; ist er nicht
  gesetzt, gilt der Standardordner des Coding-Agenten in diesem Home. Vor jedem Start des Profils
  fragt der PTY-Helfer nur diesen Wert in einer eigenen Login-Shell des Sitzungsbenutzers ab (nie
  eine andere Variable, also nie ein Token), prüft ihn
  und setzt ihn im Startbefehl nach den Startdateien wieder auf genau diesen Wert; so läuft der
  Coding-Agent mit dem geprüften Ordner. Liegt der Wert außerhalb des Home oder lässt er sich
  nicht auflösen (er fehlt, oder ein Verweis zeigt ins Leere; 2.2 Punkt 6), startet das Profil
  nicht (`open_failed` mit `close_detail = shared_location`); das Profil `shell` bleibt, damit der
  Inhaber seine Startdateien berichtigen oder den Ordner anlegen kann. Im Home schützt ihn das Home selbst, das HD11 als
  Ort dieses Sitzungsbenutzers prüft. Grund: Ein gemeinsamer Ordner wäre ein Ort, an dem ein
  `/login` einer Person alle anderen erreicht. Bei `personal` gibt es zwar nur Sitzungen des
  Inhabers (I6), aber Dauerläufer anderer Sitzungsbenutzer laufen weiter (6.5), und nach einem
  Wechsel auf `organization` bekommen andere Personen wieder Sitzungen; ein lesbarer Ordner
  außerhalb des Home gäbe ihnen die Abo-Anmeldung des Inhabers [abgeleitet]. Grenze: Den Wert
  setzen die Startdateien des Inhabers. Die Prüfung fängt eine Fehleinrichtung ab, nicht den
  Inhaber selbst; in seiner Shell kann er den Coding-Agenten mit jedem Ordner starten (2.2,
  Grenze).
- Bei `personal` fasst `bothy-host` keine Anmeldung an: Der Dienstbenutzer öffnet, liest, kopiert
  und übergibt sie nie, und der Zustandsordner enthält nur die Schlüssel des Rechners, die Bindungen der
  Sitzungsbenutzer an Personen (nur IDs und Namen, 2.2) und bei `organization` den API-Schlüssel. Der Inhaber meldet sich selbst im Konto seines
  Sitzungsbenutzers mit dem unveränderten Coding-Agenten an, entweder in einer Sitzung mit
  `/login` oder mit einem langlebigen Token aus `claude setup-token`, das er in einer Datei des
  Sitzungsbenutzers ablegt und in dessen Login-Shell als `CLAUDE_CODE_OAUTH_TOKEN` setzt. Das
  Profil `claude` startet bei `personal` über die Login-Shell des Sitzungsbenutzers das `claude`
  aus deren `PATH`, mit dessen eigenem `CLAUDE_CONFIG_DIR` im Home (oben), damit diese Anmeldung
  gilt. Programm und Ordner wählt der Inhaber; weil nur er auf diesem Rechner Sitzungen bekommt
  (I6), ist dieses Programm kein Ort für alle. Die Anmeldung gehört dem
  Sitzungsbenutzer des Inhabers; nur der Inhaber erreicht sie, weil I6 nur ihm Sitzungen gibt und
  T12 diesen Sitzungsbenutzer nur ihm zuordnet. Belege
  (abgerufen 2026-10-10): `claude setup-token` "Prints the token to the terminal without saving
  it" (https://code.claude.com/docs/en/cli-reference [belegt]); das Token ist ein einjähriges
  OAuth-Token für Umgebungen ohne Browser, das nur Modellanfragen kann
  (https://code.claude.com/docs/en/authentication [belegt]). Ob `/login` für einen Benutzer ohne
  Anmeldesitzung unter macOS den Schlüsselbund nutzen kann, sagt die Quelle nicht [vermutet:
  nein; dann bleibt der Weg über das Token]. Ob das Token für eine interaktive Sitzung desselben
  Menschen gedacht ist, sagt die Quelle ebenfalls nicht; das ist die Owner-Frage OQ-03 (Ruling T10,
  enger gefasst in D-W-17).
- Grund: Das Produkt sammelt, speichert und vermittelt keine Claude.ai-Zugangsdaten
  ([10-backend-richtlinie.md](10-backend-richtlinie.md), Abschnitt 2, Folgerung 3; I5). Ein Token im
  Zustandsordner des Host-Dienstes, das der Host-Dienst in die Sitzung reicht, wäre genau das.
- Grenze: Wer eine Sitzung des Profils `claude` hat, kann den API-Schlüssel bzw. das Token aus der
  Umgebung seines eigenen Prozesses lesen. Das lässt sich bei einem interaktiven Agenten nicht
  verhindern. Darum: bei `organization` ein eigener Schlüssel je Rechner mit Ausgabengrenze beim
  Anbieter, und nach dem Entzug einer Person, die nicht Owner ist, ein neuer Schlüssel
  (Bedrohungsmodell T-TM-08; Hinweis der Konsole wie in 6.5).

## 8. Rechte

### 8.1 Ziel `host` und Zusatzrecht `terminal`

- Neues Ziel `host` in `grants.target_kind` (03). Auf diesem Ziel gibt es keine Stufe
  (`grants.level` fehlt) und genau ein Zusatzrecht: `terminal`. Empfänger nur `user` und `group`,
  nie `agent` oder `app` (Check). Das Ziel `host` liegt außerhalb des Baums `instance` ->
  `space` -> `channel`; eine Freigabe auf die Instanz gibt kein Terminal.
- Freigaben auf Rechner legt nur der Owner an; `manage` gibt es auf Rechnern nicht.
- Owner haben `terminal` auf jedem Rechner implizit, außer I6 sperrt (7.3).

### 8.2 Schutzregeln

Die Regeln stehen in [07-rechte.md](07-rechte.md):

- **SR10 Terminal nur auf eigenem Rechner** (Ruling T1): Wer nicht Owner ist, bekommt ein Terminal
  nur auf einem Rechner, dessen Inhaber er ist (`hosts.assigned_user_id`), oder auf seiner eigenen
  bereiten Dev-Umgebung (`hosts.dev_environment_id`, Etappe "Mehrbenutzer-Dev-Umgebung"). Auf dem
  Server-Host nur Owner. Freigaben an andere sind anlegbar; die Vorschau sagt, dass sie nicht
  wirken (Hinweis `terminal_not_effective`). Grund: [21-dev-isolation.md](21-dev-isolation.md)
  verwirft den OS-Benutzer als alleinige Grenze zwischen Personen, und ein gemeinsames Home teilt
  LLM-Anmeldungen.
- **SR11 I6** (7.3).
- **SR12 Eigener Sitzungsbenutzer** (Ruling T12): `terminal.open` und `terminal.attach` nur mit
  einer gültigen Zuordnung der Person zu einem Sitzungsbenutzer dieses Rechners
  (`host_session_users`), dessen Namen der Rechner meldet (`hosts.reported_session_users`), auch
  für Owner; sonst `terminal_denied` mit `reason: no_session_user`.
  Grund: Ohne eigenen Sitzungsbenutzer liefe die Sitzung in einem Konto, dessen Anmeldungen auch
  ein anderer Mensch nutzt (2.2).
- Agenten und Apps öffnen nie ein Terminal: Die Aktionen verlangen einen Menschen in einer Sitzung;
  ein Agent, den Prompt-Injection steuert, erreicht keinen Rechner.

### 8.3 Entzug

Entzug der Freigabe, Ablauf, Austritt aus der Gruppe, Entzug der Owner-Rolle, Sperre der Person
oder des Geräts, Widerruf oder Ende der Anmeldesitzung, Abmeldung, Sperre oder Neukopplung des
Rechners, Ende der Zuordnung des Sitzungsbenutzers oder ein `report` ohne seinen Namen (SR12) und
ein Wechsel, der SR10 oder I6 verletzt: Laufende Sitzungen enden sofort (fail closed
wie Ruling L23). Der Server schließt die Verbindungen (`4100`) und weist den Host-Dienst an, die
Sitzung zu beenden; der PTY-Helfer beendet den ganzen Prozessbaum (6.5). `close_reason` nennt den
Grund (Abschnitt 10.3).

### 8.4 Owner auf dem Rechner eines anderen Menschen

Öffnet oder hängt ein Owner auf einem Rechner an, dessen `hosts.assigned_user_id` ein anderer
Mensch ist, zeigt der Client vor dem Step-up eine Warnung: Die Sitzung läuft zwar im eigenen
Sitzungsbenutzer des Owners (T12), aber auf einem Rechner, den dieser Mensch nutzt und einrichtet; mit einer
lokalen Rechteausweitung können seine Prozesse sie mitlesen und steuern (2.2, Grenze).
Erst nach ausdrücklicher Bestätigung fordert der Client den Step-up an. Der Server setzt im
Audit-Eintrag von `terminal.open` bzw. `terminal.attach` selbst `foreign_assignee = true`, weil er
Inhaber und Person kennt. Grund: SR10 schützt Nicht-Owner voreinander,
aber Owner haben `terminal` überall; ohne Warnung würde ein Owner Passwörter auf einem Rechner
tippen, den ein anderer Mensch kontrolliert, und die Trennung der Sitzungsbenutzer ist keine
vollständige Grenze (Bedrohungsmodell T-TM-12).

## 9. Mehrere Knoten

Mit `cluster.mode = multi` ([02-architektur.md](02-architektur.md), Abschnitt 16) gilt (Ruling T6):

1. Die Steuerverbindung des Host-Dienstes landet auf einem beliebigen Knoten. Der Knoten, der
   `terminal.open` bearbeitet, schickt die Anweisung `session.open` über `NOTIFY ephemeral` nur
   mit IDs an den Knoten der Steuerverbindung (03, Abschnitt 6.6).
2. Der Host-Dienst öffnet je Sitzung eine eigene Datenverbindung auf
   `/api/v1/hosts/sessions/{session_id}/stream`; das Gerät verbindet sich mit
   `/api/v1/terminal/sessions/{session_id}/stream`.
3. Der Proxy leitet beide Pfade derselben Sitzung per Hash über die Sitzungs-ID auf denselben
   Knoten (Betriebsanforderung in [16-betrieb.md](16-betrieb.md), Abschnitt 4.4). Dort verbindet
   das Relais die beiden Seiten im Speicher.
4. Landet eine Seite auf einem anderen Knoten, findet sie innerhalb von `terminal.open_timeout`
   keine Gegenseite und schließt mit `4102`; der Knoten zählt das in der Kennzahl
   `bothy.terminal.relay.unpaired` ([signals.md](../observability/signals.md)) und meldet `/readyz`
   weiter bereit. Grund: Eine Fehlkonfiguration des Proxys soll
   sichtbar scheitern, nicht still Daten über einen Broker leiten.

Grund: [02-architektur.md](02-architektur.md) verbietet einen Broker, und das Ereignis-Log taugt
nicht für Byte-Ströme. Mit einem Knoten ist alles trivial.

## 10. Audit, Ereignisse, Datenschutz

### 10.1 Audit

| Aktion | Wann | Details (`audit_details.v1`) |
|---|---|---|
| `host.manage` | jede Änderung | `host_id` (Ressource), `operation` (`create`, `update`, `pairing_code`, `confirm`, `llm_credentials`, `state`, `session_user`), alte und neue Werte als Codes (`from_assigned_user_id`, `to_assigned_user_id`, `name_changed`, `from_state`, `to_state`, `disabled_reason`), bei `session_user` je Zuordnung oder Ende ein Eintrag mit `change` (`assigned`, `revoked`), `user_id` und dem Namen des Sitzungsbenutzers im reservierten Feld `os_user`, Fingerabdrücke bei Neukopplung, `reason`; ohne Wirkung `no_change`; nie der Name des Rechners |
| `host.pair` | Erlaubnis und Ablehnung | `host_id`, Fingerabdruck |
| `host.connect` | Ablehnungen; Erlaubnis nur beim Wechsel von `reported_is_server_host`, `reported_doctor` oder `reported_session_users`; zweite Steuerverbindung (3.4) | `host_id`, Prüfcodes, mit dem Namen jedes Kontos, das die Meldung nennt (Regel für Namen, 2.2: bei HD6, HD10 und HD11 Sitzungsbenutzer, bei HD9 frühere), hinzugekommene und weggefallene Namen der Sitzungsbenutzer, Code `duplicate_connection` |
| `terminal.open`, `terminal.attach` | Erlaubnis und Ablehnung | `host_id`, `session_id`, `profile`, `device_id`, `pin_changed`, `foreign_assignee` |
| `terminal.close` | Erlaubnis und Ablehnung | `session_id`, wer schloss |
| `terminal.stream` | Ablehnungen (z. B. `identify` von einem Gerät, das nicht zur aktuellen Marke gehört) | `session_id`, `device_id`, Grund-Code |
| `terminal.terminate` | jedes Ende durch den Server oder den Rechner | Akteur `system`; `session_id`, `close_reason`, `close_detail` (wenn gesetzt), `bytes_in`, `bytes_out`, `duration_ms` |

Nie im Audit: Terminal-Inhalte, Befehle, Umgebungsvariablen, Pfade, Profil-Befehlszeilen.

### 10.2 Ereignisse

Dauerhaft im Ereignis-Log ([05-aktionen.md](05-aktionen.md), Abschnitt 6): `host.updated` (an alle
mit `host.read`) und `terminal_session.changed` (persönlich an die Person der Sitzung).
Kurzlebig: `host.presence_changed` (verbunden, getrennt). Der Strom selbst ist kein Ereignis und
steht nie im Ereignis-Log. Grund: Das Log ist für Zustände, nicht für Byte-Ströme, und alles im Log
lesen Server und Sicherungen im Klartext.

### 10.3 Gründe für das Ende (`terminal_sessions.close_reason`)

| Code | Auslöser |
|---|---|
| `client_closed` | Rahmen `close` oder `terminal.close` durch die Person |
| `closed_by_owner` | `terminal.close` durch einen Owner |
| `process_exited` | Prozess der PTY beendet |
| `idle_timeout`, `max_duration`, `detach_expired` | Grenzen aus 6.2 und 6.3 |
| `access_revoked` | Entzug nach 8.3 (Freigabe, Gruppe, Owner-Rolle, SR10, Ende der Zuordnung oder Name nicht mehr gemeldet nach SR12) |
| `i6_violation` | Wechsel der Erklärung oder des Inhabers (7.3) |
| `host_disabled`, `host_retired`, `host_rekeyed` | Zustandswechsel des Rechners (3.1, 3.3) |
| `host_restarted` | Neustart des Host-Dienstes: er nennt die Sitzung bei der Anmeldung nicht mehr (6.2) |
| `user_disabled`, `device_revoked`, `session_revoked`, `logout` | Identität (08) |
| `open_failed` | Handshake, Marke, Zeitüberschreitung beim Öffnen oder Ablehnung durch den Host-Dienst; Code in `close_detail` |
| `host_policy_violation` | Prüfung des Host-Dienstes während des Betriebs nicht bestanden (HD6, HD10, HD11; 2.2, 7.4); Code in `close_detail` |

`close_detail` nimmt nur Codes aus einer geschlossenen Liste: `token_invalid`, `token_expired`,
`token_replayed`, `handshake_failed`, `profile_unknown`, `session_user_unknown`, `session_user_mismatch`,
`server_host_owner_only`, `local_confirm_denied`, `subscription_login_detected`, `helper_untrusted`,
`shared_location`, `pty_failed`, `open_timeout`.

### 10.4 Datenschutz

- Inhaltsaufzeichnung gibt es nicht. `terminal_sessions` hält nur Metadaten mit Aufbewahrungsklasse
  `security` ([20-datenschutz.md](20-datenschutz.md)).
- Telemetrie: nur Zähler und Dauer ohne IDs von Menschen und ohne Rechnernamen; keine Spans je
  Rahmen.
- Kein Agent liest Terminal-Inhalte; Suche, Push und Lauf-Kontext kennen das Terminal nicht.
- Ob Nachweispflichten eine Aufzeichnung verlangen, ist eine Owner-Frage
  (Voreinstellung: nein, nur Metadaten).

## 11. Profile

### 11.1 Definition

Profile stehen in der Konfiguration des Host-Dienstes (`host_service.profiles`), nicht im Server.
Der Server kennt nur Name, Anzeigename und Art aus der Meldung des Host-Dienstes
(`hosts.reported_profiles`, Schema `host_profiles.v1`). Ein Client kann keine Befehlszeile
schicken.

| Profil | Art | Start |
|---|---|---|
| `shell` | `shell` | Login-Shell des Sitzungsbenutzers, Arbeitsordner sein Home |
| `claude` | `claude` | unveränderter Coding-Agent: bei `organization` genau `host_service.claude_bin` (leer oder als leer geltend nach 7.4: kein Profil `claude`, HD7 mit Warnung), bei `personal` `claude` über die Login-Shell des Inhabers (7.4); in einem Arbeitsordner, den die Person aus den Arbeitsordnern ihres eigenen Sitzungsbenutzers in `host_service.claude_workdirs` wählt (2.2 Punkt 6), gestartet im aufgelösten Pfad; Umgebung nach 7.4 |
| weitere | `command` | fester Befehl aus der Konfiguration mit absolutem Programmpfad (HD7, HD11) |

Die Umgebung jedes Profils ist eine Erlaubnisliste wie beim Runner (09, Abschnitt 18.2) plus
`TERM`. Login-Shells (Profil `shell`, Profil `claude` bei `personal`, 7.4) lesen danach die
eigenen Startdateien des Sitzungsbenutzers; was dort steht, gehört ihm, nicht dem Host-Dienst.
Der Arbeitsordner des Profils `claude` steht als Index in der Anfrage
(`workdir_index`), nie als Pfad. Der Index zählt nur in der Liste des eigenen Sitzungsbenutzers in
`hosts.reported_session_users`, die nur Anzeigenamen trägt; die Marke trägt ihn als `wdi`, und der
Host-Dienst löst ihn in genau der Liste von `osu` auf, die er gemeldet hat, also nach HD7 (2.2
Punkt 6, 5.3, Ruling T13).

### 11.2 Darstellung

Escape-Sequenzen, die das Gerät verlassen oder Daten zurücksenden, sind im Renderer standardmäßig
gesperrt: OSC 52 (Zwischenablage), OSC 8 (Hyperlinks öffnen ohne Bestätigung), Antworten auf
Abfragen von Fenstertiteln. Hyperlinks zeigt die Ansicht mit Ziel und öffnet sie erst nach Klick und
Bestätigung (SEC-TERM-6 in 19).

### 11.3 Claude-Code-Sitzungen

"Claude-Code-Sitzung" heißt in dieser Fassung: interaktives `claude` im Terminal, getrennt vom
Runner (Ruling T5). Einen Thread des Runners im Terminal übernehmen geht erst, wenn Runner und
Host-Dienst in einer Dev-Umgebung als derselbe Benutzer laufen (Etappe nach V2). Grund: Der Runner
sperrt Sitzungen und läuft mit Geheimnissen des Betreibers ([21-dev-isolation.md](21-dev-isolation.md),
Abschnitt 1).

## 12. Clients

| Client | Etappe | Besonderheit |
|---|---|---|
| Desktop (macOS, Windows) | M4 | Hardware-Tastatur, Tastenkürzel ohne Konflikt mit der Shell (Kürzel der App nur mit Modifikator der Plattform) |
| Mobil (iOS, Android) | M5 | Sondertasten-Leiste: Esc, Tab, Ctrl, Alt, Pfeile, `\|`, `~`, `/`, `-`; Hardware-Tastatur |
| Web-Konsole | M6 | Step-up nur per Passkey mit `userVerification = required` ([ADR 0070](../adr/0070-terminal-in-der-web-konsole.md)); gleiche CSP (19, Abschnitt 5) |

Ein Flutter-Widget für alle Clients. Renderer: Paket `xterm` von TerminalStudio, MIT, Version
4.0.0, für Android, iOS, Linux, macOS, Web und Windows (https://pub.dev/packages/xterm, abgerufen
2026-10-10 [belegt]). Die letzte Veröffentlichung liegt laut pub.dev rund zwei Jahre zurück
[belegt]; das Bau-Ticket prüft vor der Aufnahme Pflegezustand und offene Sicherheitsmeldungen und
hält die Sperren aus 11.2 mit eigenen Tests (TM-T14).

Jeder Client zeigt: Sitzungsliste je Rechner mit Zustand, Wiederanhängen, Fingerabdruck des
Rechners, Abzeichen der LLM-Anmeldung ("Persönlich", "Organisation", "Keine"), Restzeit bis zur
Höchstdauer.

PTY auf dem Rechner: Unix-PTY auf macOS und Linux; Windows über ConPTY (`CreatePseudoConsole`),
verfügbar ab Windows 10 Version 1809 und Windows Server 2019
(https://learn.microsoft.com/en-us/windows/console/createpseudoconsole, abgerufen 2026-10-10
[belegt]). Die Mindestversion Windows 10 2004 der Apps (Ruling L12) liegt darüber.

## 13. Fehler

| Problem-Code | Status | Wann |
|---|---|---|
| `terminal_denied` | `403` | Recht vorhanden, aber Schutzregel oder Grenze sperrt; `reason`: `not_host_assignee`, `server_host_owner_only`, `i6_holder_only`, `no_session_user`, `session_limit`, `profile_unknown` |
| `step_up_required` | `401` | Step-up fehlt, ist älter als `terminal.step_up_max_age`, schon verbraucht oder in der Konsole nicht per Passkey (`reason: passkey_required`) |
| `host_unavailable` | `409` | Rechner nicht `active` oder Host-Dienst nicht verbunden |
| `fingerprint_mismatch` | `409` | Bestätigung mit falschem Fingerabdruck |
| `i6_violation` | `409` | Änderung würde I6 verletzen (`hint`) |
| `conflict` | `409` | Zuordnung eines Sitzungsbenutzers, der auf dem Rechner schon einer Person gehört oder gehörte, oder zweite gültige Zuordnung derselben Person (`reason: session_user_taken`, 2.2) |
| `validation_failed` | `400` | Zuordnung eines Namens, den der Rechner nicht meldet (`errors[].code = session_user_not_on_host`, 2.2 Punkt 5); Inhaber, Person einer Zuordnung oder Inhaber der LLM-Anmeldung unbekannt, kein Mensch oder gelöscht (`errors[].code = unknown_user`); `assigned_user_id` zusammen mit `clear_assigned_user = true` (`errors[].code = mutually_exclusive`); `credential_holder_id` bei `none` oder `organization` (`errors[].code = not_allowed`, 7.2) |
| `not_found` | `404` | kein Leserecht auf den Rechner oder die Sitzung |

Schließ-Codes der Terminal-Kanäle (Quelle der Namen und Client-Reaktionen auf Englisch:
`x-close-codes` der Kanäle `terminalStream`, `hostControl` und `hostData` in
[asyncapi.yaml](../api/asyncapi.yaml)); dazu gelten die dort genannten Codes aus 04 Abschnitt 6.4:

| Code | Name | Kanal | Bedeutung und Reaktion |
|---|---|---|---|
| `4100` | `terminal_closed` | Strom, Datenverbindung | Sitzung beendet, Grund in `close_reason` (10.3); nicht neu verbinden, der Host-Dienst beendet die PTY |
| `4101` | `terminal_token_stale` | Strom | Sitzungs-Marke abgelaufen (`token_expired`) oder schon benutzt (`token_replayed`); mit neuem Step-up neu öffnen bzw. anhängen (5.3) |
| `4102` | `terminal_host_unavailable` | Strom | Gegenseite fehlt (Host-Dienst getrennt oder auf diesem Knoten keine Gegenseite, 9); Sitzung `detached`, später anhängen |
| `4103` | `terminal_replaced` | Strom | ein anderes Gerät hat angehängt (6.2); nicht von selbst neu verbinden |
| `4104` | `terminal_handshake_failed` | Strom | jeder andere Fehler der Marke oder des Handshakes (5.3); harte Warnung, kein neuer Versuch |
| `4110` | `host_unpaired` | Steuer- und Datenverbindung | Schlüssel unbekannt oder durch eine Neukopplung ersetzt (3.3); anhalten, `bothy-host pair` erneut ausführen |
| `4111` | `host_disabled` | Steuer- und Datenverbindung | Rechner `disabled` oder `retired` (3.1); alle PTYs beenden, erst nach langem Backoff neu verbinden |
| `4112` | `host_duplicate` | Steuer- und Datenverbindung | zweite lebende Steuerverbindung mit denselben Schlüsseln (3.4); Rechner gesperrt, nicht neu verbinden |
| `4113` | `host_auth_failed` | Steuer- und Datenverbindung | Signatur über die Herausforderung aus `hello` ungültig oder zu spät (`terminal.host_auth_timeout`); Uhr und Schlüssel prüfen, mit Backoff neu verbinden |
| `4114` | `host_version_unsupported` | Steuer- und Datenverbindung | Version unter der Mindestversion ([18-release.md](18-release.md)); Host-Dienst aktualisieren, nicht neu verbinden |

## 14. Tests

| ID | Test | Erwartet |
|---|---|---|
| TM-T01 | Konformität des Handshakes in Go und Dart mit den Einträgen für `Noise_NK_25519_ChaChaPoly_SHA256` aus den Vektor-Dateien in 5.2 | gleiche Bytes |
| TM-T02 | Gegenseitiger Handshake Go gegen Dart, mit falschem Prolog, falschem statischem Schlüssel, vertauschter Rechner-ID | nur der richtige gelingt |
| TM-T03 | Relais spielt Nachricht 1 erneut ein (gleiche Marke) | Host-Dienst lehnt ab (`token_replayed`, `4101`), keine zweite PTY |
| TM-T04 | Marke mit falschem `aud`, `sid`, `cek`, fremder `kid`, `typ` der Aufruf-Marke; abgelaufene Marke | keine PTY; `4104` mit `token_invalid`; abgelaufen `4101` mit `token_expired` |
| TM-T05 | Nachricht 1 und 2 vollständig, aber kein `start` | keine PTY |
| TM-T06 | Relais-Mitschnitt, Server-Logs, Datenbank-Abzug und Telemetrie nach einer Sitzung mit Kanarienwert | Kanarie nirgends (19, SEC-T10) |
| TM-T07 | `terminal.open` ohne Step-up, mit zu altem, mit schon verbrauchtem, in der Konsole mit Passwort-Step-up | `401 step_up_required` |
| TM-T08 | Nicht-Owner mit Freigabe auf fremdem Rechner, auf eigenem Rechner, auf dem Server-Host | `403 terminal_denied` `not_host_assignee`, erlaubt, `403 terminal_denied` `server_host_owner_only` |
| TM-T09 | I6: Freigabe an zweiten Menschen, Freigabe an Gruppe, Umschalten auf `personal` mit bestehender Freigabe an andere, zweiter Owner öffnet | `409 i6_violation` bzw. `403 terminal_denied` `i6_holder_only` |
| TM-T10 | Entzug je Art aus 8.3 während einer aktiven Sitzung, in der ein Hintergrundprozess mit `setsid` bzw. `nohup` (Windows: abgelöster Kindprozess) läuft | Verbindungen geschlossen, PTY und Hintergrundprozess beendet, `close_reason` richtig, Audit |
| TM-T11 | Leerlauf, Höchstdauer, Trennung über `terminal.detach_grace`, manipulierter Host-Dienst ohne Leerlauf-Meldung | Ende mit richtigem Grund, auch ohne Meldung des Host-Dienstes |
| TM-T12 | Wiederanhängen: anderes Gerät ohne Step-up, gleiches Gerät innerhalb und nach `terminal.quick_reattach_window`, Web-Konsole innerhalb des Fensters ohne Passkey | `401`, erlaubt, `401`, `401` mit `reason: passkey_required` |
| TM-T13 | `bothy-host doctor` als root, in Gruppe `sudo`, in Gruppe `docker`, mit schreibbarem Docker-Socket, mit passwortlosem sudo, mit Abo-Anmeldung bei `organization`, mit gleichem Dienst- und Sitzungsbenutzer, mit leerem `host_service.session_users`, mit zwei gleichen Sitzungsbenutzern | Start verweigert mit Code |
| TM-T14 | Renderer: OSC 52, OSC 8, Titel-Abfrage | nichts in die Zwischenablage, kein Link ohne Bestätigung, keine Antwort |
| TM-T15 | Zwei api-Knoten mit Hash-Routing; dann ohne Hash-Routing | Sitzung läuft; ohne Routing `4102` und Kennzahl |
| TM-T16 | Chaos: Neustart des Servers, Netzabbruch des Geräts, Netzabbruch des Host-Dienstes während einer Sitzung; danach Neustart des Host-Dienstes (`bothy-host restart`) während einer Sitzung, in der ein Hintergrundprozess läuft | PTY überlebt bis `terminal.detach_grace`, Wiederanhängen spielt den Rückblick ein; nach dem Neustart des Host-Dienstes endet die Sitzung mit `host_restarted`, PTY und Hintergrundprozess sind beendet, und das Anhängen scheitert (6.2) |
| TM-T17 | Agent- oder App-Token ruft eine Terminal-Aktion | `403 forbidden` mit `reason` `subject_kind_not_allowed`, Audit |
| TM-T18 | Neukopplung: Gerät mit gepinntem alten Schlüssel öffnet | harte Warnung, Verbindung erst nach Bestätigung, Audit `pin_changed` |
| TM-T19 | Kopplung mit falschem Fingerabdruck bis `terminal.fingerprint_max_attempts` | Rechner zurück auf `pairing`, Schlüssel verworfen |
| TM-T20 | Kopplungs-Code zweimal, abgelaufen, mit falscher Prüfsumme | abgelehnt; falsche Prüfsumme ohne Datenbankzugriff |
| TM-T21 | Stillstand durch HD6 (T-TM-14): (a) in einer Shell-Sitzung auf einem Rechner mit `organization` meldet sich eine erste Person per Abo an; danach öffnet eine zweite Person; (b) auch die zweite Person meldet sich in ihrem Konto per Abo an; (c) der Betreiber entfernt die Anmeldung der zweiten Person, sperrt den Sitzungsbenutzer der ersten nur auf dem Rechner, danach beendet der Owner ihre Zuordnung; (d) der Betreiber nimmt diesen Sitzungsbenutzer mit `bothy-host session-user remove` heraus, dessen Home für alle lesbar ist; (e) `bothy-host session-user add` mit dem entfernten Namen; (f) `bothy-host session-user add` mit einem neuen Namen, während die zweite Person eine Sitzung hat, danach ordnet der Owner den neuen Namen einer dritten Person zu; (g) bei angehaltenem Host-Dienst liegt im Konto der zweiten Person eine Abo-Anmeldedatei, danach startet der Rechner neu; danach entfernt der Betreiber die Datei und ruft `bothy-host restart` auf | (a) laufende Sitzungen enden mit `host_policy_violation`, die Öffnung scheitert mit `subscription_login_detected`, Audit und `host.updated`; `reported_doctor` nennt bei HD6 den Sitzungsbenutzer der ersten Person; (b) `reported_doctor` hat zwei Einträge HD6 mit je einem der beiden Namen; (c) der Rechner bleibt gesperrt, `reported_doctor` nennt weiter den ersten Namen; (d) das Home des entfernten Sitzungsbenutzers ist danach nur für ihn zugänglich, der Neustart besteht HD9 und HD6, der Rechner meldet den Namen nicht mehr, die zweite Person öffnet wieder, die erste bekommt `403 terminal_denied` mit `reason: no_session_user`; (e) abgelehnt, weil der Name im Zustandsordner an die erste Person gebunden ist; (f) die Sitzung der zweiten Person endet mit `host_restarted`, nach dem nächsten `report` steht der neue Name ohne weiteren Schritt in `reported_session_users`, und die Zuordnung gelingt; (g) `bothy-host` startet nicht, meldet vorher aber `report`: `reported_doctor` nennt bei HD6 den Namen der zweiten Person, und die Konsole zeigt ihn mit der Person, während der Rechner getrennt bleibt; `bothy-host doctor` auf dem Rechner nennt denselben Namen und endet mit einem Exit-Code ungleich 0; der Dienstverwalter startet den Host-Dienst nicht von selbst neu (genau eine Anmeldung im Server); nach `bothy-host restart` besteht HD6, und die zweite Person öffnet wieder |
| TM-T22 | Aus einer Sitzung `host_service.state_dir` und die Schlüsseldatei lesen; PTY-Helfer mit Leserecht auf den Ordner starten | Zugriff verweigert; Start verweigert (HD8) |
| TM-T23 | Zweite Steuerverbindung mit denselben Schlüsseln, ältere antwortet; dann ältere antwortet nicht | Rechner `disabled` mit `duplicate_connection`, beide mit `4112` geschlossen, Audit; ohne Antwort ältere mit `4009` geschlossen, neue angenommen |
| TM-T24 | Fremde Home-Ordner (2.2 Punkt 4): `bothy-host doctor` mit einem Sitzungsbenutzer, der den Home-Ordner eines Benutzers auflisten kann, der kein Sitzungsbenutzer ist (unter macOS: Mitglied von `staff` bei Standardrechten der Home-Ordner); dann ohne dieses Recht; dann richtet Sitzungsbenutzer A in einer Sitzung einen Dauerläufer ein (Eintrag in seiner crontab, unter Windows eine geplante Aufgabe), der sein Home zu einer festen Zeit nach dem folgenden `remove` wieder für alle öffnet, und öffnet sein Home für alle anderen, während B eine Sitzung hat; danach startet der Host-Dienst neu; danach nimmt der Betreiber A mit `bothy-host session-user remove` heraus; danach öffnet der Dauerläufer das Home von A, und der Betreiber ruft `bothy-host doctor` auf; danach entfernt der Betreiber die Dauerläufer von A, beendet seine Prozesse, löscht A samt Home und ruft `bothy-host restart` auf | Start verweigert mit Code (HD9), die Ausgabe auf dem Rechner nennt den Benutzer mit dem lesbaren Home, der Eintrag in `reported_doctor` nennt keinen Namen; ohne das Recht bestanden; mit dem offenen Home von A HD11 nicht bestanden mit dem Namen von A, nur die Sitzungen von A enden, die von B läuft weiter; nach dem Neustart startet der Rechner, HD9 besteht, und HD11 nennt weiter A; nach `session-user remove` ist das Home von A nur für A zugänglich, und der Start besteht HD9; mit dem wieder geöffneten Home besteht HD9 nicht, `doctor` endet mit einem Exit-Code ungleich 0, der Host-Dienst meldet `report` und beendet sich, `reported_doctor` hat einen Eintrag HD9 mit dem Namen von A, und die Konsole nennt ihn mit der Person seiner Zuordnung; nach dem Löschen und `bothy-host restart` besteht HD9, und B öffnet wieder |
| TM-T25 | `llm_credentials = personal`, Kanarien-Token in der Datei des Sitzungsbenutzers; Profil `claude` starten und beenden | Kanarie weder im Zustandsordner noch im lokalen Kanal noch in Meldungen an den Server; der Dienstbenutzer öffnet keine Datei im Home des Sitzungsbenutzers (Mitschnitt der Systemaufrufe); `claude auth status` in der Sitzung meldet `oauth_token` |
| TM-T26 | Ein Prozess des Sitzungsbenutzers beendet den Helfer und lauscht auf dem Endpunkt des Kanals bzw. legt die Named Pipe zuerst an; ein zweiter versucht, sich an den laufenden Helfer zu hängen (`ptrace`, `task_for_pid`, `OpenProcess` mit Schreibrecht); Owner öffnet auf dem Rechner eines anderen Menschen | der falsche Helfer bekommt keinen Rahmen und keinen Klartext, HD10 nicht bestanden, und `reported_doctor` nennt bei HD10 diesen Sitzungsbenutzer, keine Sitzung; das Anhängen scheitert; der Client zeigt die Warnung vor dem Step-up (Widget-Test), Audit `foreign_assignee = true` |
| TM-T27 | `llm_credentials = organization`, Abo-Anmeldung im Konto eines Sitzungsbenutzers; ein Programm `claude` im `PATH` und in den Startdateien des Sitzungsbenutzers meldet `authMethod = api_key`; danach `host_service.claude_bin` auf eine Datei, die der Sitzungsbenutzer schreiben kann; danach `host_service.claude_bin` als Skript mit `#!/usr/bin/env node`, ein falsches `node` im `PATH` des Sitzungsbenutzers, dann ein `node` in der Suchliste, das der Sitzungsbenutzer schreiben kann; dann unter Linux das geprüfte `node` im Ordner von `host_service.claude_bin`, aber `/bin` für den Sitzungsbenutzer schreibbar | HD6 ruft nur `host_service.claude_bin` auf und erkennt die Abo-Anmeldung, keine Sitzung; mit der schreibbaren Datei HD10 nicht bestanden; das falsche `node` läuft nie (Mitschnitt der Prozessstarts); mit dem schreibbaren Interpreter und mit dem schreibbaren Ordner der Suchliste HD10 nicht bestanden (7.4) |
| TM-T28 | Ein Sitzungsbenutzer je Person (T12): (a) Owner und Rechner-Inhaber mit Freigabe `terminal`, beide ohne Zuordnung, öffnen; (b) Owner ordnet einen Sitzungsbenutzer, der schon einer Person gehört oder gehörte, einer zweiten Person zu, und gibt einer Person eine zweite gültige Zuordnung; (c) Marke mit `osu` außerhalb von `host_service.session_users`; Marke mit `osu`, den der Host-Dienst schon einer anderen `sub` gegeben hat (Zuordnung in der Datenbank am Server vorbei geändert); (d) macOS, `llm_credentials = organization`, `host_service.claude_bin` leer, A ist Owner, B ist Rechner-Inhaber mit Freigabe `terminal`, beide mit eigener Zuordnung; A startet in ihrer Shell-Sitzung ein `claude` aus ihrem `PATH` und meldet sich mit `/login` per Abo an (Schlüsselbund), danach öffnet B auf demselben Rechner eine Shell und ruft dort `claude auth status` auf; (e) `confirmHost` und `updateHost` mit einem Namen, den der Rechner nicht meldet; danach entfernt der Betreiber einen zugeordneten Namen aus `host_service.session_users`, und seine Person öffnet | (a) `403 terminal_denied` mit `reason: no_session_user`, Audit; (b) `409 conflict` mit `reason: session_user_taken`, nichts geändert; (c) keine PTY, `4104` mit `session_user_unknown` bzw. `session_user_mismatch`; (d) `doctor` warnt mit `auth_status_unchecked`, und der Rechner meldet kein Profil `claude` (HD7, Warnung); die Sitzung von B läuft in Bs eigenem Sitzungsbenutzer, `claude auth status` in Bs Sitzung meldet `authMethod = none`, und Bs Helfer kann Home und Schlüsselbund von A nicht lesen (HD11 für das Home von A); (e) `400 validation_failed` mit `errors[].code = session_user_not_on_host`, nichts gespeichert; nach dem nächsten `report` `403 terminal_denied` mit `reason: no_session_user`, kein `4104` |
| TM-T29 | Keine geteilten Orte (T13): (a) ein Arbeitsordner von B, den A schreiben kann, und A legt dort `.claude/settings.json` mit einem Hook und einem `apiKeyHelper` an, danach öffnet B das Profil `claude` dort; (b) ein Ordner über einem Arbeitsordner von B, den A schreiben kann; (c) ein Arbeitsordner unter zwei Sitzungsbenutzern in `host_service.claude_workdirs`, in der Liste von B davor, danach ein weiterer Ordner von B; (d) ein `CLAUDE_CONFIG_DIR` des Profils `claude` von B, den A lesen kann; (e) ein Eintrag im Arbeitsordner von B, der A gehört; danach entfernt der Betreiber ihn, ein periodischer Lauf vergeht, dann ruft er `bothy-host doctor` auf; danach ein Home von B, dessen Prüfung der Einträge beim Start länger als `host_service.entry_scan_timeout` dauert, und A öffnet während dieses Laufs; (f) das Programm eines Profils `command`, das ein Sitzungsbenutzer schreiben kann; eines, das einem anderen Benutzer des Rechners gehört, der weder Sitzungsbenutzer noch vertrauenswürdiger Besitzer ist; eines in einem Ordner, der einem solchen Benutzer gehört; bei `organization` dieselben drei Fälle für `host_service.claude_bin`, etwa in einem Ordner eines Paketverwalters, der dem Konto einer Person mit Administratorrechten gehört; unter Windows `host_service.claude_bin` als `.cmd`; dazu das Programm eines Profils `command`, das erst im Betrieb für einen Sitzungsbenutzer schreibbar wird, während A und B Sitzungen haben; (g) B schickt einen `workdir_index` außerhalb der eigenen Liste; eine Marke mit `wdi` außerhalb der Liste von `osu` (Server am Host-Dienst vorbei); (h) in einem Ordner über einem Arbeitsordner von B eine `CLAUDE.md`, die A gehört; dort eine `CLAUDE.md` von `root`, die A schreiben kann; dort ein Ordner `.claude`, der A gehört; (i) Windows 10 und Windows 11 mit Standardrechten an `C:\`, Helfer unter `C:\Program Files\bothy-host`, `host_service.claude_bin` unter `C:\Program Files`, Home und Arbeitsordner von B unter `C:\Users`, dazu ein Testordner in `C:\`, dessen DACL einen geerbten Eintrag vor einem eigenen trägt: `doctor` vor `bothy-host install`; `install` mit abgelehnter, dann mit gegebener Bestätigung; danach versucht A `mkdir C:\.git`; danach ein `C:\.git`, das A vor der Installation angelegt hat; (j) ein Arbeitsordner von B, dessen konfigurierter Pfad über einen Verweis von `root` in einen Ordner von A führt, den B durchqueren kann; über einem Arbeitsordner von B eine `CLAUDE.md` als Verweis von `root` auf eine Datei von A; derselbe Arbeitsordner bei A und bei B außerhalb der Home-Ordner, bei B über einen Verweis, unter macOS und Windows zusätzlich in anderer Groß- und Kleinschreibung; B hat weitere Arbeitsordner in seinem Home, und ein Arbeitsordner von B führt über einen Verweis in einen Arbeitsordner von A im Home von A (Modus `0700`), ein anderer fehlt; (k) `llm_credentials = personal`, die Login-Shell des Inhabers setzt `CLAUDE_CONFIG_DIR` auf einen Ordner außerhalb seines Home, danach auf einen Verweis im Home, der nach außerhalb zeigt, danach auf einen Verweis im Home, der ins Leere zeigt, danach auf einen Ordner im Home; (l) `llm_credentials = organization`, ein anderes `claude` im `PATH` des Sitzungsbenutzers und des Dienstes, B öffnet das Profil `claude`; danach `host_service.claude_bin` als Skript mit `#!/usr/bin/env node`, kein `node` in der Suchliste, ein `node` nur im übrigen `PATH` des Profils, B öffnet das Profil `claude` | (a), (b), (d), (e) und (h) HD11 nicht bestanden mit dem Namen von B, B bekommt keine Sitzung (`open_failed` mit `shared_location`, laufende enden mit `host_policy_violation`), Hook und `apiKeyHelper` laufen nie (Mitschnitt der Prozessstarts), andere Sitzungsbenutzer bekommen weiter Sitzungen; bei (e) bleibt B nach dem Entfernen des Eintrags über den periodischen Lauf hinaus gesperrt und bekommt erst nach `bothy-host doctor` wieder Sitzungen, und mit dem langen Lauf besteht HD11 für B nicht (`entry_scan_timeout` mit dem Namen von B), während A schon vor dem Ende dieses Laufs eine Sitzung bekommt; (c) HD7 nicht bestanden, der Ordner fehlt in beiden Listen, `pair` und `report` melden die Listen ohne ihn, und der Index des weiteren Ordners in der gemeldeten Liste von B startet `claude` in genau diesem Ordner; (f) in allen Fällen Start verweigert (HD10, HD11); im Betrieb lehnt der Rechner neue Sitzungen ab, die von A und B enden mit `host_policy_violation` und `shared_location`, die Meldung nennt keinen Sitzungsbenutzer, und nach der Berichtigung öffnen beide nach dem nächsten periodischen Lauf wieder; (g) `403 terminal_denied` mit `reason: profile_unknown`; mit der Marke keine PTY, `4104` mit `profile_unknown`; (i) vor `install` und nach abgelehnter Bestätigung HD11 nicht bestanden mit `drive_root_writable`, Start verweigert, die ACL von `C:\` unverändert; nach der Bestätigung fehlt an `C:\` nur der Eintrag `(AD)`, die übrigen Einträge stehen in derselben Reihenfolge mit denselben Steuerbits, und die Sicherheitsbeschreibung jedes direkten Unterordners von `C:\` ist vorher und nachher gleich, auch die des Testordners, den eine Weitergabe der Vererbung umgeordnet oder geschützt hätte (2.2 Punkt 6), HD10 und HD11 bestehen, `mkdir C:\.git` als A scheitert; mit dem alten `C:\.git` HD11 nicht bestanden für jeden anderen Sitzungsbenutzer mit seinem Namen, nach Entfernen des Eintrags bestanden (T-TM-14); (j) die ersten beiden wie (a), der dritte wie (c); der Ordner im Home von A und der fehlende Ordner fehlen nur in der gemeldeten Liste von B (HD7, Warnung), die Liste von A behält ihren Ordner, HD11 besteht für A und B, und B öffnet das Profil `claude` in seinen Arbeitsordnern im Home (sie hat sein PTY-Helfer aufgelöst); (k) in den ersten drei Fällen startet das Profil `claude` nicht (`open_failed` mit `shared_location`), das Profil `shell` öffnet; im vierten läuft `claude` mit genau diesem Ordner als `CLAUDE_CONFIG_DIR`; (l) es startet genau der aufgelöste Pfad von `host_service.claude_bin`, das andere `claude` nie (Mitschnitt der Prozessstarts); mit dem Skript ohne `node` in der Suchliste meldet der Rechner kein Profil `claude` (HD7, Warnung), `doctor` warnt mit `auth_status_unchecked`, das Öffnen scheitert mit `403 terminal_denied` und `reason: profile_unknown`, und das `node` aus dem übrigen `PATH` läuft nie (Mitschnitt der Prozessstarts) |

Die Fälle aus TM-T07 bis TM-T10, TM-T17 und TM-T28 (a) stehen zusätzlich in der Testmatrix von 07
(AZ-M54 bis AZ-M61).

## 15. Etappen

| Etappe | Inhalt |
|---|---|
| M2 | Modul `terminal` (Rechner, Kopplung, Sitzungen, Relais), `bothy-host` (PTY für macOS, Linux, Windows; `doctor`; Profile), Noise und Sitzungs-Marke, I6 und Rechte, Konformitäts- und Chaos-Tests, Red Team Terminal |
| M4 | Terminal-Ansicht der Desktop-App |
| M5 | Terminal-Ansicht der mobilen App mit Sondertasten |
| M6 | Terminal in der Web-Konsole ([ADR 0070](../adr/0070-terminal-in-der-web-konsole.md)) |
| Mehrbenutzer-Dev-Umgebung | `hosts.dev_environment_id`, SR10 für eigene Umgebungen |

Red-Team-Punkte am Ende von M2, M4, M5 und M6 (Bedrohungsmodell, Abschnitt 6.7).

## 16. Entscheidungen dieses Kapitels

| Nr | Entscheidung | Grund |
|---|---|---|
| R-TM-01 | Eigener Host-Dienst mit nur ausgehender Verbindung statt SSH oder eingehendem Port. | Geht hinter NAT und ohne Tailscale-Pflicht; der Server bleibt Autorisierungspunkt. |
| R-TM-02 | Noise `NK` mit Prolog und Einmal-Marke im ersten Paket. | Kleiner, geprüfter Standard mit Testvektoren; Marke und `cek` schließen Wiedereinspielen und Relais-Tausch. |
| R-TM-03 | Schutzziel ehrlich ohne bösartigen Server; Pinning je Gerät und lokale Bestätigung als Rückfall. | NK beweist nur den verteilten Schlüssel (Ruling T3). |
| R-TM-04 | Ziel `host` ohne Stufe, nur Zusatzrecht `terminal`, nur für Menschen und Gruppen. | Stufen haben auf einem Rechner keine Bedeutung; Agenten und Apps nie. |
| R-TM-05 | SR10: Nicht-Owner nur auf dem eigenen Rechner. | OS-Benutzer allein ist keine Grenze zwischen Personen (Ruling T1, ADR 0046). |
| R-TM-06 | I6 mit Owner-Ausnahme, transaktional und bei jedem Öffnen. | Persönliche Abos nur für die eigene Nutzung; persönliche Rechner bei mehreren Ownern möglich (Ruling T2). |
| R-TM-07 | Step-up je Öffnung frisch und verbraucht; schnelles Wiederanhängen vom selben Gerät ohne neuen Step-up, nie in der Web-Konsole. | Owner verlangt Bestätigung beim Öffnen; Funklöcher sollen nicht zum blinden Bestätigen erziehen. |
| R-TM-08 | Profile nur in der Konfiguration des Host-Dienstes. | Ein Client kann keine Befehlszeile einschleusen. |
| R-TM-09 | Getrennte Datenverbindung je Sitzung und Hash-Routing im Proxy für mehrere Knoten. | Kein Broker (02); ein Knoten hält beide Enden (Ruling T6). |
| R-TM-10 | Kein Inhalt in Log, Audit, Telemetrie; keine Aufzeichnung in dieser Fassung. | Datensparsamkeit; Aufzeichnung ist eine Owner-Frage. |
| R-TM-11 | Leerlauf misst der Host-Dienst, der Server prüft zusätzlich grob. | Nur der Host-Dienst sieht Rahmentypen; ein manipulierter Host-Dienst darf Sitzungen nicht offen halten. |
| R-TM-12 | Dienstbenutzer mit den Schlüsseln und Sitzungsbenutzer für die PTYs getrennt, verbunden über einen lokalen Kanal. | Eine Shell hätte sonst die Schlüssel des Rechners und könnte ihn nach einem Entzug von anderswo vertreten. |
| R-TM-13 | Eine zweite lebende Steuerverbindung sperrt den Rechner, statt die ältere zu verdrängen. | Kopierte Schlüssel werden sichtbar; stilles Verdrängen gäbe einem Angreifer alle späteren Sitzungen. |
| R-TM-14 | HD6 vor jeder PTY und periodisch; Treffer beendet alle Sitzungen des Rechners; die Meldung nennt jeden betroffenen Sitzungsbenutzer mit eigenem Eintrag. | Eine Abo-Anmeldung kann während einer Sitzung entstehen (I6, Ruling T2). Weil so eine Person den Rechner für alle stilllegen kann, muss der Betreiber wissen, in welchem Konto er sucht (D-W-64, Bedrohungsmodell T-TM-14). |
| R-TM-15 | Jede Sitzung in eigener Prozessgruppe, cgroup bzw. Job Object; das Ende beendet den ganzen Baum. | Entzug soll auch Hintergrundprozesse beenden; Dauerläufer bleiben als ehrlich genannte Grenze. |
| R-TM-16 | Abgelaufene oder verbrauchte Marke `4101`, jeder andere Fehler der Marke oder des Handshakes `4104`. | Ein Fall ohne Angriff darf nicht als Manipulation gemeldet werden. |
| R-TM-17 | HD9 prüft mit einem Lesetest, dass der Sitzungsbenutzer keinen Home-Ordner eines Benutzers auflisten kann, der kein Sitzungsbenutzer ist, statt Gruppen zu prüfen; das Home eines Sitzungsbenutzers prüft HD11 als dessen Ort. | Unter macOS öffnet die Standardgruppe `staff` fremde Home-Ordner; welche Gruppe oder welche Rechte das auf anderen Plattformen tun, ist je Rechner verschieden, das Ergebnis des Lesetests nicht. Ein offenes Home eines Sitzungsbenutzers ist ein Fehler in dessen Konto: HD11 sperrt nur ihn und nennt ihn, statt dass HD9 den Start des ganzen Rechners verweigert (D-W-88). |
| R-TM-18 | Der PTY-Helfer ist ein Systemdienst unter dem Sitzungsbenutzer mit Binärdatei, Definition und Kanal-Endpunkt im Besitz eines vertrauenswürdigen Besitzers (2.2 Punkt 6); `bothy-host` prüft die Gegenstelle über Benutzer und Prozess-ID des Dienstverwalters (HD10); Owner bekommen auf dem Rechner eines anderen Menschen eine Warnung mit Bestätigung. | Ein Helfer, den der Sitzungsbenutzer ersetzen kann, fälscht Prüfungen und liest spätere Sitzungen mit; dieselbe Benutzerkennung bleibt ein gemeinsamer Vertrauensbereich (D-W-18). |
| R-TM-19 | Bei `personal` fasst `bothy-host` keine Abo-Anmeldung an; der Inhaber meldet sich selbst im Konto seines Sitzungsbenutzers an. | Das Produkt speichert und vermittelt keine Claude.ai-Zugangsdaten (10, Folgerung 3; D-W-17). |
| R-TM-20 | HD6 fragt den Coding-Agenten nur über den absoluten Pfad `host_service.claude_bin` mit fester Suchliste ab; HD10 prüft Datei, Ordnerkette und Interpreter wie den Helfer. | Ein Programm im `PATH` des Sitzungsbenutzers kann er selbst ersetzen und die Abo-Anmeldung verschweigen lassen (D-W-45, D-W-57). |
| R-TM-21 | Ein Sitzungsbenutzer je Person und Rechner, nie geteilt, auch nicht nach dem Ende einer Zuordnung; ohne Zuordnung kein Terminal (SR12); der Host-Dienst bindet jeden Sitzungsbenutzer an die erste Person. HD6 ist nur Zusatzprüfung. | Dritte Runde derselben Fehlerklasse: ein Orakel im Konto eines geteilten Benutzers ist fälschbar oder abgeschaltet; nur getrennte Konten machen "keine fremde Abo-Anmeldung" unabhängig von einer Prüfung wahr (Ruling T12, D-W-57). |
| R-TM-22 | Keine geteilten Orte zwischen Sitzungsbenutzern: Arbeitsordner und `CLAUDE_CONFIG_DIR` je Sitzungsbenutzer, der Index des Arbeitsordners (`wdi`) zählt nur in der gemeldeten Liste von `osu` nach HD7, Programme samt Ordnerkette im Besitz eines vertrauenswürdigen Besitzers, eine Prüfung HD11 für jeden konfigurierten Pfad samt Ordnerkette und Kettendateien. | Ein gemeinsamer Arbeitsordner oder Konfigurationsordner verband die getrennten Konten wieder; eine allgemeine Regel statt eines Flickens je Ort (Ruling T13, D-W-62). |
| R-TM-23 | Der Host-Dienst meldet die Namen seiner Sitzungsbenutzer; der Server ordnet nur gemeldete Namen zu, und eine Zuordnung wirkt nur, solange der Name gemeldet wird. | Ein Tippfehler soll nicht als Manipulationswarnung `4104` auffallen (R-TM-16, D-W-63). |
| R-TM-24 | `bothy-host session-user remove` nimmt einen Sitzungsbenutzer aus `host_service.session_users` und `host_service.claude_workdirs`, entfernt seinen PTY-Helfer, schließt sein Home und startet den Host-Dienst neu; die Bindung an seine Person bleibt, und `session-user add` lehnt einen gebundenen Namen ab. | HD6 läuft in jedem Namen aus `host_service.session_users`; Sperren des Benutzers oder Ende der Zuordnung beenden den Stillstand durch eine Abo-Anmeldung nicht (D-W-67). Der Name bleibt belegt (T12). |
| R-TM-25 | HD7, HD10 und HD11 benutzen je Plattform feste Begriffe: vertrauenswürdige Besitzer (Linux und macOS `root`; Windows `SYSTEM`, `Administrators`, `NT SERVICE\TrustedInstaller`); "schreibbar" schließt bei Ordnern Anlegen, Löschen und Umbenennen von Einträgen und bei allem das Anhängen und das Ändern von Attributen, Rechten und Besitz ein; jeder Pfad wird vor der Prüfung vollständig aufgelöst, Orte eines Sitzungsbenutzers von seinem PTY-Helfer, Orte für alle vom Dienstbenutzer; ein Pfad, der sich nicht auflösen lässt, gilt als nicht vorhanden; Verweise zählen wie ihr Ziel, HD7 vergleicht Ordner nach ihrer Identität, die jeder Helfer meldet (unter Windows mit der 128-Bit-Datei-ID). Unter Windows entfernt `bothy-host install` nach Bestätigung den Eintrag `Authenticated Users:(AD)` an der Wurzel jedes Laufwerks mit einem Ort und schreibt dabei nur die DACL der Wurzel, ohne Weitergabe an die Ordner darunter; sonst meldet HD11 `drive_root_writable`. | Ohne diese Festlegung bestünde HD11 unter Windows mit Standardrechten nie, oder eine Person könnte mit einem Ordner `C:\.git` alle anderen sperren; die Systemordner gehören `TrustedInstaller`; eine Prüfung ohne Auflösung ließe einen Verweis in den Ordner einer anderen Person durch; der Dienstbenutzer kommt in die Orte eines Sitzungsbenutzers nicht hinein; eine Weitergabe der Vererbung änderte das ganze Laufwerk (D-W-80, D-W-83, D-W-84, D-W-86, D-W-87). |
| R-TM-26 | Das Profil `claude` startet bei `organization` genau `host_service.claude_bin` (leer: kein Profil) mit der Suchliste von HD6 vorn im `PATH`; die Suchliste beginnt mit dem konfigurierten Ordner von `host_service.claude_bin`, HD10 prüft jeden ihrer Ordner, und findet sie den Interpreter eines Skripts nicht, gilt der Wert für HD6 und das Profil als leer. Bei `personal` startet es `claude` über die Login-Shell des Inhabers, und dessen `CLAUDE_CONFIG_DIR` muss sich auflösen lassen und im Home liegen. | Das Programm läuft in jedem Sitzungsbenutzer und ist ein Ort für alle; ohne Interpreter in der Suchliste fände `env` einen ungeprüften im übrigen `PATH`; ein `CLAUDE_CONFIG_DIR` außerhalb des Home wäre ein Ort mit Anmeldung, den HD11 nicht kennt (D-W-81, D-W-82, D-W-85). |
| R-TM-27 | Ein Neustart des Host-Dienstes beendet alle Sitzungen des Rechners (`host_restarted`); der Host-Dienst nennt bei der Anmeldung die Sitzungen, die er noch kennt (`live_sessions`). `session-user add` und `remove` starten ihn selbst neu, jede andere Änderung der Konfiguration wirkt nach `bothy-host restart`. | Der neue Prozess liest die Konfiguration neu und prüft alles neu; eine Sitzung aus der Zeit davor liefe ungeprüft unter der alten weiter. Ohne Neustart nach `add` lehnte der Server den neuen Namen ab, bis jemand von Hand neu startet (D-W-90). |
| R-TM-28 | HD11 im Betrieb: ein Verstoß an einem Ort für alle beendet alle Sitzungen und lässt keine neuen zu; die Prüfung der Einträge läuft je Sitzungsbenutzer, nur er wartet auf sie, `host_service.entry_scan_timeout` begrenzt sie, und ihren Fund hebt nur ein späterer voller Lauf auf. | Für einen Ort für alle war im Betrieb keine Folge festgelegt; ein großes Home soll nur seinen Sitzungsbenutzer aufhalten, und nur diese Prüfung sieht, ob ein Eintrag noch da ist (D-W-89). |
| R-TM-29 | `bothy-host doctor` nennt auf dem Rechner die betroffenen Namen und endet mit einem Exit-Code ungleich 0; verweigert es beim Start den Start, meldet der Host-Dienst das Ergebnis vorher mit `report` und beendet sich ohne Sitzung, mit eigenem Exit-Code, nach dem ihn der Dienstverwalter nicht neu startet. | Sonst zeigte die Konsole nach einem Treffer beim Start nur einen getrennten Rechner, und die Namen aus HD6 und HD11 erreichten den Betreiber nicht; ein Neustart behebt keinen dieser Verstöße (D-W-91). |
| R-TM-30 | Regel für Namen: Kann eine Person mit Terminal, auch eine frühere, einen Verstoß herbeiführen, nennt die Meldung an den Server das Konto, in dem der Betreiber suchen muss (HD6, HD10 mit falscher Gegenstelle, HD11 an einem Ort eines Sitzungsbenutzers, HD9 bei einem früheren Sitzungsbenutzer); Namen anderer Benutzer gehen nie an den Server. Nach `bothy-host session-user remove` löscht der Betreiber den Benutzer samt seinen Dauerläufern; Sperren allein reicht nicht. | Ein Dauerläufer eines früheren Sitzungsbenutzers kann dessen Home nach `remove` wieder öffnen und so über HD9 den Start des ganzen Rechners verhindern, und ein Prozess einer Person, der sich an die Stelle ihres Helfers setzt, legt über HD10 den Rechner still; ohne Namen fände der Betreiber das Konto nur auf dem Rechner. Eine Regel statt einer Ergänzung je Prüfung. Sperren beendet keinen laufenden Prozess (D-W-92). |

## 17. Konfigurationswerte dieses Kapitels

Spalte "Ort": Server = Konfigurationsreferenz des Servers; Host-Dienst = Konfiguration von
`bothy-host` auf dem Rechner.

| Name | Ort | Standard | Bedeutung |
|---|---|---|---|
| `terminal.pairing_code_ttl` | Server | `15m` | Gültigkeit eines Kopplungs-Codes für einen Rechner |
| `terminal.fingerprint_confirm_chars` | Server | `8` | so viele Zeichen des Fingerabdrucks tippt der Owner bei der Bestätigung |
| `terminal.fingerprint_max_attempts` | Server | `3` | Fehlversuche bei der Bestätigung, danach Rückfall auf `pairing` |
| `terminal.step_up_max_age` | Server | `60s` | so jung muss der Step-up für `terminal.open` und `terminal.attach` sein |
| `terminal.session_token_ttl` | Server | `30s` | Gültigkeit einer Sitzungs-Marke |
| `terminal.open_timeout` | Server | `20s` | Frist, bis beide Seiten im Relais verbunden sind und der Handshake steht |
| `terminal.detach_grace` | Server | `15m` | so lange bleibt eine getrennte Sitzung anhängbar |
| `terminal.quick_reattach_window` | Server | `60s` | Wiederanhängen vom selben Gerät ohne neuen Step-up innerhalb dieser Zeit nach der Trennung |
| `terminal.idle_timeout` | Server | `30m` | ohne Eingabe des Geräts endet die Sitzung |
| `terminal.idle_grace` | Server | `2m` | Zuschlag, nach dem der Server selbst wegen Leerlaufs beendet |
| `terminal.max_session_duration` | Server | `12h` | Höchstdauer einer Sitzung |
| `terminal.max_sessions_per_person` | Server | `5` | offene Sitzungen je Mensch |
| `terminal.max_sessions_per_host` | Server | `10` | offene Sitzungen je Rechner |
| `terminal.relay_buffer_max_bytes` | Server | `262144` | Puffer des Relais je Richtung und Sitzung |
| `terminal.host_heartbeat_interval` | Server | `20s` | Abstand der Heartbeats der Steuerverbindung (in `hello` gemeldet) |
| `terminal.host_heartbeat_miss_limit` | Server | `3` | fehlende Heartbeats bis zum Schließen der Steuerverbindung |
| `terminal.host_auth_timeout` | Server | `10s` | Frist zwischen `hello` und `identify` der Steuer- und Datenverbindung |
| `terminal.sweep_interval` | Server | `30s` | Abstand des Jobs `terminal.sweep` ([02-architektur.md](02-architektur.md), Abschnitt 12.3) |
| `terminal.host_duplicate_probe_timeout` | Server | `5s` | so lange wartet der Server auf die Antwort der älteren Steuerverbindung (3.4) |
| `host_service.state_dir` | Host-Dienst | `/var/lib/bothy-host` (Windows: `%ProgramData%\bothy-host`) | Zustandsordner mit den Schlüsseln des Rechners, den Bindungen der Sitzungsbenutzer an Personen und bei `organization` dem API-Schlüssel des Profils `claude`; nie eine Abo-Anmeldung (7.4) |
| `host_service.session_users` | Host-Dienst | leer | Sitzungsbenutzer dieses Rechners, je Person einer; je Eintrag ein PTY-Helfer; nie der Dienstbenutzer; leer heißt: HD1 nicht bestanden; gelesen nur beim Start, geändert mit `bothy-host session-user add` und `remove`, die den Host-Dienst dafür neu starten (2.2) |
| `host_service.extra_forbidden_groups` | Host-Dienst | leer | weitere Gruppen, die HD2 zusätzlich zur festen Liste sperrt |
| `host_service.credential_check_interval` | Host-Dienst | `60s` | Abstand der Prüfungen HD6 und HD11 während des Betriebs (2.2, 7.4) |
| `host_service.entry_scan_timeout` | Host-Dienst | `10m` | Höchstdauer der Prüfung der Einträge von HD11 je Sitzungsbenutzer; danach bricht sein PTY-Helfer sie ab, und HD11 besteht für ihn nicht (`entry_scan_timeout`, 2.2) |
| `host_service.profiles` | Host-Dienst | `[shell]` | angebotene Profile (11.1) |
| `host_service.claude_bin` | Host-Dienst | leer | absoluter Pfad des Coding-Agenten, den HD6 für `claude auth status` aufruft und den das Profil `claude` bei `organization` startet; gehört mit Ordnerkette, Interpreter und den Ordnern der Suchliste einem vertrauenswürdigen Besitzer (2.2 Punkt 6, HD10); unter Windows eine `.exe`; leer heißt: HD6 prüft nur die Anmeldeorte und warnt, bei `organization` gibt es kein Profil `claude`, die Durchsetzung trägt der eigene Sitzungsbenutzer (7.4); für HD6 und das Profil gilt der Wert auch als leer, wenn sich der Pfad nicht auflösen lässt oder die Suchliste den Interpreter eines Skripts nicht findet |
| `host_service.claude_workdirs` | Host-Dienst | leer | Arbeitsordner des Profils `claude` je Sitzungsbenutzer (Name aus `host_service.session_users` -> Liste von Ordnern mit Anzeigenamen; gemeldet werden nur die Anzeigenamen); jeder Ordner gehört genau diesem Sitzungsbenutzer (Ruling T13, HD7, HD11); ein Ordner bei zwei Sitzungsbenutzern fällt aus beiden Listen, gemeldet und für `wdi` benutzt wird die Liste danach; gelesen nur beim Start, eine Änderung wirkt nach `bothy-host restart`; ohne Eintrag kein Profil `claude` für diesen Sitzungsbenutzer, leer heißt kein Profil `claude` |
| `host_service.scrollback_bytes` | Host-Dienst | `1048576` | Rückblick-Puffer je Sitzung, nur im Speicher |
| `host_service.require_local_confirm` | Host-Dienst | `false` | jede PTY erst nach Zustimmung am Rechner (6.4) |
| `host_service.local_confirm_timeout` | Host-Dienst | `60s` | Frist für die Zustimmung am Rechner |

Werte anderer Kapitel, auf die dieses Kapitel verweist: `auth.clock_leeway`
([08-identitaet.md](08-identitaet.md)), `cluster.mode` ([02-architektur.md](02-architektur.md)),
`ratelimit.rl.terminal.*` und `ratelimit.rl.host.*` ([05-aktionen.md](05-aktionen.md)),
`retention.security_after_end` ([20-datenschutz.md](20-datenschutz.md)).
