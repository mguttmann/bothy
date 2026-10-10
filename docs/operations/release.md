# Ein Release machen

Status: Entwurf M0. Ablauf zu [18-release.md](../spec/18-release.md). Gates und Runner legen
ADR 0038 und 0039 fest (Team H); dieser Ablauf nennt nur die Schritte, die zum Release gehören.

![Release-Kette](diagrams/release-pipeline.svg)

## 1. Voraussetzungen

1. Hauptzweig grün in allen lokalen Gates und in CI.
2. Portabilitätstest der letzten Beta grün ([16-betrieb.md](../spec/16-betrieb.md) 11.1).
3. `CHANGELOG.md` der Komponente hat einen geprüften Abschnitt für die neue Version; Release Notes
   mit den Pflichtabschnitten ([18-release.md](../spec/18-release.md) 4).
4. Für `server`: Migrationen erfüllen M1 bis M12 ([upgrade.md](upgrade.md) 4); der
   N-1-Kompatibilitätstest ist grün.
5. Mindest-App-Versionen und Deprecation-Daten in den Release Notes stimmen mit ADR 0014.

## 2. Server

1. `VERSION` setzen, Changelog-Abschnitt abschließen, Commit.
2. Signierten Tag setzen: `server/vX.Y.Z` (Beta: `server/vX.Y.Z-beta.N`).
3. Die Release-Kette baut je Architektur, erzeugt das Multi-Arch-Manifest, SBOMs, Signaturen,
   Prüfsummen und (im öffentlichen Repository) die Attestation, und veröffentlicht nach `ghcr.io`.
4. Prüfen wie ein Betreiber (Abschnitt 5).
5. Beta: nach `staging`; stable: Release als "latest release" markieren, Betreiber informieren.

## 3. SDKs und Adapter

1. `VERSION`, Changelog, Tag `<komponente>/vX.Y.Z`.
2. Build, SBOM, Signatur der Dateien (`cosign sign-blob`), Veröffentlichung auf PyPI bzw. npm über
   Trusted Publishing des Registers, wo vorhanden [vermutet: Verfügbarkeit je Register im Ticket
   prüfen].
3. Beta: Vorabversion, npm-Tag `beta`.

## 4. Apps

Apps baut der Herausgeber mit seiner `distribution.yaml` ([18-release.md](../spec/18-release.md) 8).

| Plattform | Schritte |
|---|---|
| iOS | Build mit Build-Nummer, Upload nach App Store Connect, TestFlight-Gruppe beta; nach Bewährung Gruppe stable bzw. Einreichung |
| macOS | Build, Signatur mit Developer ID, `xcrun notarytool submit --wait`, `xcrun stapler staple`, Archiv mit `sign_update` (EdDSA) signieren, Eintrag im Appcast mit `sparkle:channel` beta; nach Bewährung Element entfernen |
| Windows | MSIX bauen und signieren (Artifact Signing oder OV mit HSM), `beta.appinstaller` auf die neue Version; nach Bewährung `stable.appinstaller` |
| Android | AAB bauen und signieren, interner Testkanal; nach Bewährung nächster Kanal |
| Web-Konsole | mit dem Server-Release (Vorschlag, ADR 0028) |

Vor dem Wechsel nach stable: Absturzrate des Beta-Builds prüfen (falls ein Ziel für
Absturzberichte konfiguriert ist) und keine offenen Fehler der Stufe kritisch.

## 5. Prüfen wie ein Betreiber

```
cosign verify ghcr.io/<owner>/bothy-server@sha256:<digest> \
  --certificate-identity-regexp '^https://github.com/<owner>/bothy/.github/workflows/release-server.yaml@refs/tags/server/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
gh attestation verify oci://ghcr.io/<owner>/bothy-server@sha256:<digest> --repo <owner>/bothy
cosign verify-blob SHA256SUMS --bundle SHA256SUMS.sigstore.json \
  --certificate-identity-regexp '^https://github.com/<owner>/bothy/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
sha256sum --check SHA256SUMS
```

Der Name der Workflow-Datei `release-server.yaml` ist ein Vorschlag; Team H legt ihn fest.

## 6. Rückruf

1. Server: Release als zurückgezogen markieren, Hinweis an Betreiber mit Rückweg
   ([upgrade.md](upgrade.md) 3), korrigierte Patch-Version.
2. Apps: korrigierten Build mit höherer Nummer; bei Sicherheitsfehlern `client.min_version.<plattform>`
   auf die korrigierte Version setzen lassen.
3. SDKs: Version zurückziehen (PyPI yank, npm deprecate), Korrektur veröffentlichen.
