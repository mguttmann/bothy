# Modul `identity`

Status: geplant (M0). Dieses Verzeichnis enthält noch keinen Code. Normativ ist
[docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Prinzipale, Menschen, Owner, Anmelde-Treiber, Identitäten, lokale Anmeldedaten, Einladungs-Codes, Anmeldevorgänge, Passkeys, Geräte und Geräteschlüssel, Sitzungen, Refresh-Familien, API-Tokens, Step-up, Zugriffstokens.

## Port (Wurzelpaket `internal/identity`)

- Eingehend: `Auth` (Anmeldung, Refresh, Logout, Kind-Sitzungen, Step-up), `Directory`, `Devices`, `Tokens`, `Owners`; deklariert `AvatarFiles`.
- Gibt In-Tx-Ereignisse aus: `DeviceRevoked`, `SessionEnded`.
- Abonniert In-Tx-Ereignisse: keine.
- Implementiert Quellen-Ports und Querschnitts-Ports: kernel `actor.Authenticator`, `authz.PrincipalSource`, `authz.CredentialSource` (Sitzung, Gerät, Token für die Prüfung `check_active`, 07 Abschnitt 4.2), `instance.LoginOptionsSource`, `instance.FlagUserSource`, kernel `personaldata.Participant`.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: `instance`, `authz`.

Grund: Signierschlüssel für Zugriffstokens; implementiert `PrincipalSource` und `CredentialSource`.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`principals`, `users`, `owners`, `identity_providers`, `user_identities`, `local_credentials`, `enrollment_codes`, `auth_login_flows`, `auth_challenges`, `device_pairings`, `passkey_credentials`, `devices`, `device_keys`, `auth_sessions`, `refresh_token_families`, `refresh_tokens`, `api_tokens`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`instance.bootstrap`, `auth.login.begin`, `auth.login.complete`, `auth.token.refresh`, `auth.logout`, `auth.step_up`, `identity_provider.read`, `identity_provider.manage`, `owner.add`, `owner.remove`, `owner.recover`, `self.read`, `user.read`, `user.create`, `user.update`, `user.disable`, `user.enable`, `self.profile.update`, `identity.read`, `identity.link`, `identity.unlink`, `local_credential.set`, `passkey.register`, `passkey.revoke`, `device.list`, `device.approve`, `device.revoke`, `device.expire`, `session.list`, `session.revoke`, `session.extension.create`, `token.list`, `token.create`, `token.revoke`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: `user.updated`, `session.revoked`.
- Kurzlebig: keine.
- WebSocket-Ops (Client an Server): keine.

## Hintergrundjobs

| Job | Auslöser | Warteschlange |
|---|---|---|
| `identity.session_cleanup` | periodisch | `maintenance` |

## Hinweise für den Bau

- Tokens nur als HMAC mit Server-Pepper; Refresh-Rotation mit Sperre auf der Familie, Wiederverwendung widerruft Familie und Sitzung (03, 4.2).
- Implementiert `kernel/actor.Authenticator` für `platform/httpserver` und `gateway`.
- Widerruf von Gerät oder Sitzung erhöht die Rechte-Version (05, Spalte Rechte) und schließt Verbindungen mit `4012` über das Ereignis `session.revoked`.

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [08-identitaet.md](../../../../docs/spec/08-identitaet.md)
