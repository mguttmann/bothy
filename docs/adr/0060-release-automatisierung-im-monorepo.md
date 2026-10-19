---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
consulted: Team F (Betrieb und Release), Team H (Qualität und Repo)
informed: alle Planungsteams
---

# Release-Automatisierung im Monorepo

## Context and Problem Statement

Jede Komponente des Monorepos hat eine eigene Version in `VERSION`, ein eigenes `CHANGELOG.md`
nach "Keep a Changelog" und Tags der Form `<komponente>/v<semver>` (ADR 0036,
[18-release.md](../spec/18-release.md), Abschnitte 2 bis 4). Commits auf `main` sind Squash-Commits
mit Conventional-Commits-Kopf, der Scope nennt die Komponente
([push-discipline.md](../quality/push-discipline.md)). Jedes Ticket trägt seine Änderung unter
"Unreleased" im Changelog der Komponente ein ([PLAN.md](../PLAN.md), Abschnitt 3, Regel 7). Kapitel
18 lässt offen, welches Werkzeug die nächste Version bestimmt und den Release vorbereitet. Welches
Werkzeug, und wie weit automatisiert es?

## Decision Drivers

- Lokal zuerst: ein Befehl, gleich lokal und in CI (ADR 0038); keine Pflicht zu GitHub-Diensten.
- Push, Tag und Pull Request nur nach Freigabe des Owners; Tags sind signiert (18, Abschnitt 2).
- Workflows dürfen nur auf Pull Requests und Release-Tags laufen, nie auf Pushes in Zweige
  (`tools/ci/check_runners.py`, Regel `push-on-branch`; ADR 0039).
- Ein Mensch oder Agent prüft jeden Changelog-Abschnitt vor dem Tag (18, Abschnitt 4).
- Monorepo mit Go, Dart, Python und TypeScript; Version steht in `VERSION`, nicht in einem
  Paketmanifest.
- Vor 1.0.0 darf eine Minor-Version brechen (18, Abschnitt 3).

## Considered Options

- git-cliff als gepinnte lokale Kommandozeile hinter einem make-Ziel; Version aus Conventional
  Commits je Pfad, Changelog aus den Einträgen der Tickets, Tag signiert von Hand
- release-please im Manifest-Modus als GitHub Action mit Release-Pull-Requests
- semantic-release mit Monorepo-Erweiterungen, voll automatisch nach dem Merge
- GoReleaser mit Monorepo-Funktion
- Eigenes Skript ohne fremdes Werkzeug

## Decision Outcome

Gewählte Option: "git-cliff lokal hinter einem make-Ziel", weil es offline läuft, Commits je Pfad
und Tag-Muster auswertet und den Tag dem Menschen lässt.

1. `make release-prep COMPONENT=<name> [CHANNEL=beta]` ruft ein getestetes Werkzeug-Skript auf.
   Es liest Verzeichnis und Tag-Präfix der Komponente aus `tools/ci/components.py` (eine Quelle für
   CI-Pfadfilter, Tag-Auflösung und Release-Vorbereitung).
2. git-cliff bestimmt die nächste Version aus den Commits seit dem letzten Tag der Komponente
   (`--tag-pattern`, `--include-path` auf das Verzeichnis der Komponente, für `server` zusätzlich
   `docs/api/`) mit `--bumped-version` [Q1, belegt]. Regeln: `fix` ergibt Patch, `feat` Minor, ein
   Bruch Major; vor 1.0.0 ergibt ein Bruch Minor (`breaking_always_bump_major = false`) [Q2,
   belegt]. `CHANNEL=beta` hängt `-beta.N` an, N aus den vorhandenen Tags.
3. Der Text des Changelogs kommt aus den Einträgen unter "Unreleased", die jedes Ticket schreibt.
   Das Skript benennt den Abschnitt in `[X.Y.Z] - <Datum>` um, schreibt `VERSION` und zeigt die
   Commits der Komponente, die ihr `CHANGELOG.md` nicht geändert haben, als Prüfliste. Die
   Pflichtabschnitte der Server-Release-Notes (Upgrade-Hinweise, Migrationen, Kompatibilität,
   Sicherheit) ergänzt der Mensch oder Agent im Review.
4. Commit `chore(release): <komponente> vX.Y.Z`, signierter Tag `git tag -s <komponente>/vX.Y.Z`
   durch den Maintainer, Push nach Freigabe. Danach läuft `release.yml` (ADR 0036, ADR 0062).
5. git-cliff kommt als gepinnte Binärdatei mit Prüfsumme über `tools/bin/tools.lock.json`, wie die
   übrigen Release-Binärdateien der Werkzeugkette ([toolchain.md](../quality/toolchain.md)).

6. For the component `bothy`, `SERVER_VERSION=<version>` additionally writes the server version
   that the product bundle installs into the line `BOTHY_VERSION=` of
   `infra/compose/.env.example`, only if the tag `server/v<version>` exists
   ([18-release.md](../spec/18-release.md), section 2.1; D-W-76). Without `CHANNEL=beta`
   (stable product) it rejects a `<version>` with a pre-release identifier and names the value;
   without `SERVER_VERSION` it rejects while the line carries the placeholder. So the error shows
   up before commit and tag; `make release-build` keeps the check as a second safeguard, and the
   gate `release-pins` checks the committed state in the pre-push of the branch and in the
   required check `ci-ok` of the pull request, that is before the tag. The push of the tag alone
   starts no check, because its commit is already on `origin`
   ([18-release.md](../spec/18-release.md), section 2.1, rule 1; D-W-98).

### Consequences

- Gut, weil die Vorbereitung lokal und ohne Netz läuft und denselben Stand wie CI nutzt.
- Gut, weil kein Werkzeug Schreibrechte auf das Repository braucht; Tags bleiben signiert.
- Gut, weil die Versionsregel aus den Commits folgt und nicht aus dem Gedächtnis.
- Schlecht, weil die Version nur so gut ist wie Scope und Typ der Squash-Commits; der Pfadfilter
  fängt falsche Scopes ab, falsche Typen nicht. Gegenmittel: Prüfung des PR-Titels in
  `make commitlint`.
- Schlecht, weil der Changelog-Text Handarbeit der Tickets bleibt; dafür steht dort Sprache für
  Nutzer statt Commit-Betreffzeilen.
- Neutral, weil eine weitere Binärdatei gepinnt und von Renovate aktualisiert wird (ADR 0041).

### Confirmation

- Tests des Werkzeug-Skripts gegen ein Fixture-Repository mit Commits in mehreren Komponenten:
  richtige Version je Komponente (Patch, Minor, Bruch vor und nach 1.0.0, Beta-Zähler), Commits
  anderer Komponenten zählen nicht, ein Commit ohne Änderung am `CHANGELOG.md` erscheint in der
  Prüfliste.
- Bestehende Checks: `tools/ci/components.py release` lehnt einen Tag ab, dessen Version nicht in
  `VERSION` steht; `release.yml` bricht ab, wenn `CHANGELOG.md` keinen Abschnitt für die Version
  hat.
- Tests for `SERVER_VERSION` against the fixture repository: without a tag `server/v<version>`
  abort, without `CHANNEL=beta` with a `<version>` with a pre-release identifier abort naming the
  value, without `SERVER_VERSION` with the placeholder abort, each without a written file (PLAN
  M7-29).
- `make runners-check`: Kein Workflow läuft auf Pushes in Zweige; ein Release-Bot-Workflow würde
  daran scheitern.
- Pin-Prüfung `test_toolchain_doc.py`: Die Version von git-cliff in `toolchain.md` gleicht dem Pin.

## Pros and Cons of the Options

### git-cliff lokal

- Gut, weil Pfadfilter, Tag-Muster und Versionsregel eingebaut sind [Q1, Q2, belegt] und es unter
  MIT oder Apache-2.0 steht [Q3, belegt].
- Schlecht, weil ein Wrapper-Skript für Komponenten, Beta-Zähler und Changelog-Umbenennung nötig
  ist.

### release-please im Manifest-Modus

- Gut, weil Release-Pull-Requests, Monorepo-Konfiguration und Tags mit Komponente fertig sind;
  Squash-Merges werden empfohlen [Q4, belegt].
- Schlecht, weil es über die GitHub-API arbeitet: Pull Requests anlegen und Tags setzen braucht
  Schreibrechte, lokal läuft nichts, und die vom Werkzeug gesetzten Tags sind nicht vom Maintainer
  signiert [abgeleitet].
- Schlecht, weil der übliche Auslöser ein Push auf `main` ist; den lehnt `make runners-check` ab.

### semantic-release

- Gut, weil voll automatisch.
- Schlecht, weil kein Mensch den Release vor dem Veröffentlichen prüft ("no human is directly
  involved") [Q5, belegt]; das widerspricht 18, Abschnitt 4, und der Freigabe durch den Owner.

### GoReleaser mit Monorepo-Funktion

- Gut, weil Build und Veröffentlichung von Go-Binärdateien eingebaut sind.
- Schlecht, weil die Monorepo-Funktion nur in der kostenpflichtigen Pro-Ausgabe steckt [Q6, belegt]
  und Apps, SDKs und Adapter nicht abdeckt.

### Eigenes Skript ohne fremdes Werkzeug

- Gut, weil keine Abhängigkeit.
- Schlecht, weil Commit-Parsing, Tag-Auswahl und Versionsregel nachgebaut und getestet werden
  müssten, die git-cliff schon hat.

## More Information

- Quellen (abgerufen 2026-10-10): Q1 git-cliff, Kommandozeilen-Argumente,
  https://github.com/orhun/git-cliff/blob/main/website/docs/usage/args.md, und Monorepos,
  https://github.com/orhun/git-cliff/blob/main/website/docs/usage/monorepos.md [belegt]; Q2
  git-cliff, Abschnitt `bump`,
  https://github.com/orhun/git-cliff/blob/main/website/docs/configuration/bump.md [belegt]; Q3
  https://github.com/orhun/git-cliff (Lizenz) [belegt]; Q4 release-please,
  https://github.com/googleapis/release-please und
  https://github.com/googleapis/release-please/blob/main/docs/manifest-releaser.md [belegt]; Q5
  semantic-release, https://github.com/semantic-release/semantic-release [belegt]; Q6 GoReleaser
  Monorepo, https://goreleaser.com/customization/monorepo/ [belegt].
- Tag-Form: Für ein Go-Modul in einem Unterverzeichnis ist das Unterverzeichnis das Präfix der Tags
  (https://go.dev/ref/mod, Abschnitt "Module paths") [belegt]. Das Server-Modul liegt in
  `services/server` und wird nie als Abhängigkeit geladen ([PLAN.md](../PLAN.md), P-05); für den Bau
  ist die Tag-Form darum gleichgültig, `server/vX.Y.Z` bleibt gültig.
- Verwandt: ADR 0036 (Releases), ADR 0038 (lokale Gates), ADR 0039 (Runner-Strategie), ADR 0062
  (Signaturen im privaten Zustand).
