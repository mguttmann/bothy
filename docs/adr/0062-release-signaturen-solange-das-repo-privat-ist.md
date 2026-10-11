---
status: accepted
date: 2026-10-10
decision-makers: Leitung der Planung
consulted: Team H (Qualität und Repo), Team F (Release)
informed: alle Planungsteams
---

# Release-Signaturen solange das Repo privat ist

## Context and Problem Statement

ADR 0036 legt fest: Jedes Artefakt ist mit cosign keyless signiert, hat eine SBOM und, sobald das
Repository öffentlich ist, eine Attestation. Bis zum Go des Owners bleibt das Produkt-Repository
privat. Zwei Dinge gehen dann nicht wie geplant: Artifact Attestations gibt es in GitHub Free, Pro
und Team nur für öffentliche Repositorys ([18-release.md](../spec/18-release.md), Abschnitt 6), und
eine keyless Signatur schreibt einen Eintrag in das öffentliche Transparenz-Log Rekor, dessen
Zertifikat Repository und Workflow nennt [Q1, Q2, belegt]. Damit würde ein privates Repository
öffentlich sichtbar. Wie werden Releases signiert, solange das Repository privat ist?

## Decision Drivers

- Nichts über das private Repository (Name, Konto, Tags, Zeitpunkte) gelangt in öffentliche Logs
  (White Label, Freigabe der Sichtbarkeit nur durch den Owner).
- Releases sind signiert, auch im privaten Zustand (Anforderung des Owners an signierte Releases);
  die Instanz des Owners kann vor der Bereitstellung prüfen.
- Signier-Geheimnisse liegen nur in geschützten Umgebungen der Tag-Workflows.
- Der Prüfweg der Release-Kette läuft vor dem Öffentlich-Schalten einmal echt (Bau-Ticket M7-10).
- Keine Kosten ohne Entscheidung des Owners.

## Considered Options

- Schlüsselpaar für cosign im privaten Zustand, Signatur ohne Transparenz-Log; ab dem
  Öffentlich-Schalten keyless und Attestations nach ADR 0036
- Keyless mit dem öffentlichen Sigstore auch im privaten Zustand
- Keine Signatur bis zum Öffentlich-Schalten, nur Prüfsummen, SBOM und Provenance im
  Registry-Eintrag
- GitHub Enterprise Cloud, dessen eigene Sigstore-Instanz private Attestations ohne öffentliches
  Log ausstellt

## Decision Outcome

Gewählte Option: "Schlüsselpaar ohne Transparenz-Log im privaten Zustand", weil sie signierte und
prüfbare Releases liefert, ohne etwas zu veröffentlichen, und ohne Kosten auskommt.

1. Ein cosign-Schlüsselpaar (ECDSA P-256) entsteht einmal für die private Phase. Der private
   Schlüssel und sein Passwort liegen nur als Geheimnisse der Umgebung `release`, die nur für Tags
   der Form `v*` und `*/v*` freigegeben ist. Der öffentliche Schlüssel liegt im Repository.
2. Die Release-Kette signiert Image-Digests und `SHA256SUMS` mit diesem Schlüssel und einer
   Signing-Config ohne Rekor, Fulcio und Zeitstempeldienst (`cosign signing-config create` mit
   `--no-default-rekor`, `--no-default-fulcio`, `--no-default-tsa`, `--no-default-oidc`)
   [Q3, belegt]. Das veraltete Flag `--tlog-upload=false` wird nicht genutzt; cosign 3.0.3 hat es
   abgekündigt [Q4, belegt]. Die Signatur-Bundles liegen neben den Artefakten.
3. Geprüft wird mit dem öffentlichen Schlüssel (`cosign verify --key`); weil es keinen
   Log-Eintrag gibt, mit `--insecure-ignore-tlog` [Q5, belegt]. Den genauen Befehl für die gepinnte
   cosign-Version legt das Bau-Ticket M7-10 fest.
4. SBOM und `SHA256SUMS` entstehen wie in ADR 0036; Attestations nicht (`release.yml` führt sie nur
   bei `visibility == 'public'` aus).
5. Beim Öffentlich-Schalten ([public-switch.md](../quality/public-switch.md)) stellt die Kette auf
   keyless und Attestations um (ADR 0036). Der private Schlüssel wird gelöscht; der öffentliche
   bleibt im Repository, damit alte Releases prüfbar bleiben.

### Consequences

- Gut, weil kein öffentlicher Log-Eintrag das private Repository verrät.
- Gut, weil die Instanz des Owners Images vor dem Start gegen den öffentlichen Schlüssel prüfen
  kann und der Signierweg vor dem Öffentlich-Schalten in Betrieb ist.
- Schlecht, weil ein langlebiger Schlüssel geschützt werden muss. Auf GitHub Pro gibt es für private
  Repositorys keine Pflicht-Prüfer an Umgebungen, nur Umgebungs-Geheimnisse und Regeln für Zweige
  und Tags [Q6, belegt]; wer Tags setzen kann, kann signieren lassen. Im privaten Zustand ist das
  nur der Owner.
- Schlecht, weil ohne Transparenz-Log ein gestohlener Schlüssel unbemerkt signieren könnte.
  Gegenmittel: kurze Lebensdauer (nur die private Phase), Löschen beim Öffentlich-Schalten, Wechsel
  des Schlüssels nach jedem Verdacht.
- Schlecht, weil Betreiber ab dem Öffentlich-Schalten anders prüfen (Identität statt Schlüssel);
  im privaten Zustand gibt es nur einen Betreiber, den Owner.
- Neutral, weil die Release-Kette zwei Signierwege kennt, gesteuert über die Sichtbarkeit wie die
  Attestations.

### Confirmation

- Bau-Ticket M7-10 in [PLAN.md](../PLAN.md): `cosign verify` mit dem öffentlichen Schlüssel und
  `sha256sum --check` gegen ein Probe-Release sind grün; ein Release ohne Signatur bricht ab.
- Test der Release-Kette im privaten Zustand: Die Signing-Config enthält keinen Rekor-, Fulcio- oder
  Zeitstempel-Dienst, und jedes erzeugte Bundle hat keinen Eintrag eines Transparenz-Logs.
- `make runners-check`: `release.yml` läuft nur auf Tag-Pushes. Eine Ergänzung im Bau-Ticket M7-10
  prüft, dass Signierschritte nur in Jobs mit `environment: release` stehen.
- Checkliste [public-switch.md](../quality/public-switch.md): Umstellung auf keyless, Löschen des
  privaten Schlüssels, Probe-Release mit `gh attestation verify`.

## Pros and Cons of the Options

### Schlüsselpaar ohne Transparenz-Log

- Gut, weil signiert, prüfbar und ohne Öffentlichkeit.
- Schlecht, weil Schlüsselverwaltung und kein öffentliches Log.

### Keyless mit öffentlichem Sigstore

- Gut, weil keine Schlüsselverwaltung.
- Schlecht, weil jeder Signiervorgang öffentlich im Log steht, mit Zertifikat samt Repository und
  Workflow [Q1, Q2, belegt]; das veröffentlicht das private Repository vor dem Go des Owners.

### Keine Signatur bis zum Öffentlich-Schalten

- Gut, weil nichts zu verwalten ist; so arbeitet `release.yml` heute.
- Schlecht, weil eine Prüfsumme nur Übertragungsfehler zeigt, keine Herkunft (ADR 0036); wer
  Release-Assets ändern kann, ändert auch `SHA256SUMS`.
- Schlecht, weil der Signier- und Prüfweg erst nach dem Öffentlich-Schalten zum ersten Mal läuft.

### GitHub Enterprise Cloud

- Gut, weil private Repositorys Attestations über GitHubs eigene Sigstore-Instanz ohne
  Transparenz-Log bekommen [Q2, belegt].
- Schlecht, weil ein anderer Tarif mit laufenden Kosten nötig ist; das entscheidet nur der Owner.

## More Information

- Quellen (abgerufen 2026-10-10): Q1 Sigstore, Cosign Signing Overview,
  https://docs.sigstore.dev/cosign/signing/overview/, und Fulcio OID-Erweiterungen (Source
  Repository URI, Build Signer URI),
  https://github.com/sigstore/fulcio/blob/main/docs/oid-info.md [belegt]; Q2 GitHub, Artifact
  attestations,
  https://docs.github.com/en/actions/concepts/security/artifact-attestations [belegt]; Q3 cosign
  `signing-config create`,
  https://github.com/sigstore/cosign/blob/main/doc/cosign_signing-config_create.md [belegt]; Q4
  cosign Changelog v3.0.3 "Deprecate tlog-upload flag",
  https://github.com/sigstore/cosign/blob/main/CHANGELOG.md [belegt]; Q5 cosign `verify`,
  https://github.com/sigstore/cosign/blob/main/doc/cosign_verify.md [belegt]; Q6 GitHub Docs,
  Verfügbarkeit von Umgebungen,
  https://github.com/github/docs/blob/main/data/reusables/gated-features/environments.md [belegt].
- Kosten und Tarif (Option GitHub Enterprise Cloud) entscheidet der Owner; Voreinstellung ist diese
  ADR.
- Verwandt: ADR 0036 (Releases), ADR 0039 (Runner und Veröffentlichung), ADR 0060
  (Release-Vorbereitung).
