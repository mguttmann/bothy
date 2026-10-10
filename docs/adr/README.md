# Architekturentscheidungen (ADR)

Tragende Entscheidungen stehen hier als Architecture Decision Records im Format MADR 4
([ADR 0001](0001-adrs-im-madr-format-decisions-als-ruling-log.md)). Neue ADR: Vorlage
[0000](0000-vorlage-madr-4.md) kopieren, nächste freie Nummer, dann `make adr-index`.
Kleinere Festlegungen stehen im Ruling-Log [DECISIONS.md](../DECISIONS.md).

Die Tabelle unten erzeugt `tools/docs/adr_index.py` aus den Dateien; `make docs` schlägt
fehl, wenn sie veraltet ist. Nicht von Hand bearbeiten.

<!-- adr-index:start -->
| Nr | Titel | Status | Datum |
|---|---|---|---|
| [0000](0000-vorlage-madr-4.md) | Vorlage (MADR 4) | template | 2026-10-10 |
| [0001](0001-adrs-im-madr-format-decisions-als-ruling-log.md) | ADRs im MADR-Format, DECISIONS.md als Ruling-Log | accepted | 2026-10-10 |
| [0002](0002-zwei-repos-oeffentliches-produkt-privates-ops-repo.md) | Zwei Repos: öffentliches Produkt, privates Ops-Repo | accepted | 2026-10-10 |
| [0003](0003-lizenz-apache-2-0-mit-dco-statt-cla.md) | Lizenz Apache-2.0 mit DCO statt CLA | accepted | 2026-10-10 |
| [0004](0004-modularer-monolith-statt-microservices.md) | Modularer Monolith statt Microservices | accepted | 2026-10-10 |
| [0005](0005-go-als-server-sprache.md) | Go als Server-Sprache | accepted | 2026-10-10 |
| [0006](0006-postgresql-ab-tag-1-statt-sqlite.md) | PostgreSQL ab Tag 1 statt SQLite | accepted | 2026-10-10 |
| [0007](0007-typisierter-datenbankzugriff-sqlc-und-migrationswerkzeug.md) | Typisierter Datenbankzugriff (sqlc) und Migrationswerkzeug | accepted | 2026-10-10 |
| [0008](0008-hexagonale-module-und-gepruefte-abhaengigkeitsregeln.md) | Hexagonale Module und geprüfte Abhängigkeitsregeln | accepted | 2026-10-10 |
| [0009](0009-commit-geordnetes-ereignis-log-mit-globaler-sequenz.md) | Commit-geordnetes Ereignis-Log mit globaler Sequenz | accepted | 2026-10-10 |
| [0010](0010-objektspeicher-hinter-s3-schnittstelle-lokaler-treiber-als-standard.md) | Objektspeicher hinter S3-Schnittstelle, lokaler Treiber als Standard | accepted | 2026-10-10 |
| [0011](0011-12-factor-konfiguration-und-secret-provider.md) | 12-Factor-Konfiguration und Secret-Provider-Schnittstelle | accepted | 2026-10-10 |
| [0012](0012-oci-images-compose-opentofu.md) | OCI-Images multi-arch, Compose als Installation, OpenTofu je Ziel | accepted | 2026-10-10 |
| [0013](0013-spec-first-openapi-31-und-asyncapi-30-normativ.md) | Spec-first: OpenAPI 3.1 und AsyncAPI 3.0 normativ, Codegenerierung | accepted | 2026-10-10 |
| [0014](0014-api-versionierung-und-deprecation-policy.md) | API-Versionierung und Deprecation-Policy | accepted | 2026-10-10 |
| [0015](0015-fehlerformat-rfc-9457-idempotenz-paginierung.md) | Fehlerformat RFC 9457, Idempotenz, Paginierung | accepted | 2026-10-10 |
| [0016](0016-webhooks-nach-standard-webhooks.md) | Webhooks nach Standard Webhooks | accepted | 2026-10-10 |
| [0017](0017-eine-autorisierungsfunktion-404-statt-403-registrierungszwang.md) | Eine Autorisierungsfunktion, 404 statt 403, Registrierungszwang | accepted | 2026-10-10 |
| [0018](0018-freigabe-modell-stufen-zusatzrechte-vererbung-kein-verbot.md) | Freigabe-Modell: Stufen, Zusatzrechte, Vererbung, kein Verbot | accepted | 2026-10-10 |
| [0019](0019-identitaetsanbieter-oidc-proxy-header-lokale-passkey-konten.md) | Identitätsanbieter: OIDC, Proxy-Header, lokale Passkey-Konten | accepted | 2026-10-10 |
| [0020](0020-geraetebindung-dpop-step-up-fuer-admin-aktionen.md) | Gerätebindung, DPoP, Step-up für Admin-Aktionen | accepted | 2026-10-10 |
| [0021](0021-audit-log-hash-verkettet-und-getrennt-von-telemetrie.md) | Audit-Log hash-verkettet und getrennt von Telemetrie | accepted | 2026-10-10 |
| [0022](0022-agenten-und-plugins-ausserhalb-des-server-prozesses.md) | Agenten und Plugins außerhalb des Server-Prozesses | accepted | 2026-10-10 |
| [0023](0023-signierte-aufruf-marke-und-pruefung-im-adapter.md) | Signierte Aufruf-Marke und Prüfung im Adapter | accepted | 2026-10-10 |
| [0024](0024-backend-richtlinie-personal-organization-mit-invariante.md) | Backend-Richtlinie personal/organization mit Invariante | accepted | 2026-10-10 |
| [0025](0025-lauf-als-eigenes-objekt-fuer-die-live-box.md) | Lauf (Run) als eigenes Objekt für die Live-Box | accepted | 2026-10-10 |
| [0026](0026-sdks-generierter-kern-plus-handschicht.md) | SDKs: generierter Kern plus Handschicht (Python, TypeScript) | accepted | 2026-10-10 |
| [0027](0027-flutter-fuer-alle-apps.md) | Flutter für alle Apps | accepted | 2026-10-10 |
| [0028](0028-web-ziel-nur-fuer-admin-und-statistik-konsole.md) | Web-Ziel nur für Admin- und Statistik-Konsole | accepted | 2026-10-10 |
| [0029](0029-client-zustand-lokale-datenbank-und-offline-warteschlange.md) | Client-Zustand, lokale Datenbank und Offline-Warteschlange | accepted | 2026-10-10 |
| [0030](0030-design-tokens-als-eine-quelle-mit-generator.md) | Design-Tokens als eine Quelle mit Generator | accepted | 2026-10-10 |
| [0031](0031-push-ohne-inhalt-inhalt-holt-die-app-selbst.md) | Push ohne Inhalt, Inhalt holt die App selbst | accepted | 2026-10-10 |
| [0032](0032-volltextsuche-in-postgresql.md) | Volltextsuche in PostgreSQL | accepted | 2026-10-10 |
| [0033](0033-malware-scan-der-uploads-als-getrennter-dienst.md) | Malware-Scan der Uploads als getrennter Dienst | accepted | 2026-10-10 |
| [0034](0034-opentelemetry-ueber-collector-lokaler-standard-stack.md) | OpenTelemetry über einen Collector, lokaler Standard-Stack | accepted | 2026-10-10 |
| [0035](0035-slos-mit-fehlerbudgets-und-burn-rate-alarmen.md) | SLOs mit Fehlerbudgets und Burn-Rate-Alarmen | accepted | 2026-10-10 |
| [0036](0036-releases-semver-je-komponente-signiert-sbom-provenance.md) | Releases: SemVer je Komponente, signierte Artefakte, SBOM, Provenance | accepted | 2026-10-10 |
| [0037](0037-app-verteilung-und-auto-update-kanaele.md) | App-Verteilung und Auto-Update-Kanäle | proposed | 2026-10-10 |
| [0038](0038-lokale-gates-zuerst-ein-befehl-identisch-zu-ci.md) | Lokale Gates zuerst: ein Befehl identisch zu CI | accepted | 2026-10-10 |
| [0039](0039-runner-strategie-privat-oeffentlich-und-release-ausnahmen.md) | Runner-Strategie privat/öffentlich und Release-Ausnahmen | accepted | 2026-10-10 |
| [0040](0040-sonarqube-lokal-und-cloud-mit-gleichem-profil-und-gate.md) | SonarQube lokal und Cloud mit gleichem Profil und Gate | accepted | 2026-10-10 |
| [0041](0041-abhaengigkeits-updates-mit-renovate.md) | Abhängigkeits-Updates mit Renovate | accepted | 2026-10-10 |
| [0042](0042-deny-list-scanner-mit-liste-aus-dem-ops-repo.md) | Deny-List-Scanner mit Liste aus dem Ops-Repo | accepted | 2026-10-10 |
| [0043](0043-discord-import-nur-lesen-trockenlauf-herkunftsschluessel.md) | Discord-Import: nur lesen, Trockenlauf, Herkunftsschlüssel | accepted | 2026-10-10 |
| [0044](0044-arbeitsname-und-branding-als-konfiguration.md) | Arbeitsname und Branding als Konfiguration | accepted | 2026-10-10 |
| [0045](0045-dokumentations-site-generator.md) | Dokumentations-Site-Generator | accepted | 2026-10-10 |
| [0046](0046-mehrbenutzer-dev-umgebung-nur-mit-abschottung.md) | Mehrbenutzer-Dev-Umgebung nur mit Abschottung | accepted | 2026-10-10 |
<!-- adr-index:end -->
