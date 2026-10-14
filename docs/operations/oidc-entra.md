# Anmeldung mit Microsoft Entra ID (OIDC)

Status: gebaut (M1-41; groups from the group claim M1-42, section 7). Normativ sind [08-identitaet.md](../spec/08-identitaet.md) Abschnitte 2.2,
2.3 und 2.6; bei Abweichung gilt das Kapitel. Microsoft Entra ID ist das Referenzbeispiel; jeder
andere OpenID-Connect-Anbieter wird genauso eingerichtet.

## 1. Ablauf in Kürze

Der Server ist vertraulicher OIDC-Client (Relying Party) bei Entra und zugleich der
Autorisierungsdienst seiner Apps. Die App öffnet die Anmeldung im Systembrowser, Entra leitet zum
Server zurück, der Server prüft das ID-Token und gibt der App einen einmaligen `login_code`, den
nur die App-Instanz mit ihrem `client_verifier` einlösen kann. Anbieter-Tokens erreichen nie ein
Gerät und werden nie gespeichert. Beide Strecken nutzen PKCE `S256`.

## 2. App-Registrierung in Entra

1. Neue App-Registrierung als **Web-Anwendung** (vertraulicher Client).
2. Genau eine Umleitungs-URI: `https://<host>/api/v1/auth/oidc/callback`, zum Beispiel
   `https://chat.example.com/api/v1/auth/oidc/callback`. Weitere Umleitungen braucht es nicht:
   die Rückleitung zur App (Loopback oder eigenes Schema) liegt beim Server.
3. Unter "Zertifikate und Geheimnisse" ein **Zertifikat** hochladen (Client-Authentisierung
   `private_key_jwt`, bevorzugt). Ein Client-Secret (`client_secret_basic`) geht ebenfalls, läuft
   aber ab und ist ein weiteres Geheimnis.
4. Optional den `groups`-Claim im Token freigeben; den Abgleich von Gruppen beschreibt Abschnitt 7
   (08 Abschnitt 3.4). Ohne `groups_claim` in der Konfiguration wird er nicht gelesen.
5. Für eine Organisation mit mehreren Mandanten "Konten in einem beliebigen
   Organisationsverzeichnis" wählen und die erlaubten Mandanten-IDs in `allowed_tenant_ids`
   eintragen.

Belege (abgerufen 2026-10-10): Redirect-URIs
https://learn.microsoft.com/en-us/entra/identity-platform/reply-url, Zertifikats-Assertion
https://learn.microsoft.com/en-us/entra/identity-platform/certificate-credentials [belegt].

## 3. Geheimnisse ablegen

Client-Schlüssel und Client-Secret stehen nie in der Konfiguration des Anbieters, nur als Verweis
auf den Secret-Provider. Erlaubt sind ausschließlich Dateien unter `<secrets.dir>/idp/` und
Umgebungsvariablen mit dem Präfix `BOTHY_IDP_`; ein Verweis auf ein Geheimnis des Servers selbst
(zum Beispiel `auth.token_pepper_ref`) wird abgelehnt (docs/DECISIONS.md M1-36/14 und M1-41).

- Zertifikat: den privaten Schlüssel als PKCS#8-PEM unter
  `<secrets.dir>/idp/entra_client_key.pem` ablegen (Modus `0600`). `client_key_alg` ist `PS256`
  (Standard), `client_cert_x5t_s256` der SHA-256-Fingerabdruck des Zertifikats als base64url ohne
  Auffüllung (Kopfzeile `x5t#S256`).
- Secret: als Datei `<secrets.dir>/idp/entra_client_secret` oder als Variable
  `BOTHY_IDP_ENTRA_SECRET`.

## 4. Ausgehende Verbindungen

Discovery, Schlüssel (JWKS) und Token-Endpunkt erreicht der Server nur über den Egress-Client:
erlaubt sind der Host des Ausstellers und die Hosts von `token_endpoint` und `jwks_uri` aus dem
geprüften Discovery-Dokument, nur `https` (19 SEC-NET-2). Für Entra ist kein Eintrag in
`egress.allowed_hosts` nötig. Discovery und Schlüssel werden `auth.oidc_jwks_cache_ttl` lang
zwischengespeichert; bei unbekannter `kid` lädt der Server die Schlüssel einmal neu.

Bei einem Aussteller mit `{tenantid}` liest der Server das Discovery-Dokument des Alias
`organizations` (`https://login.microsoftonline.com/organizations/v2.0/.well-known/openid-configuration`),
dessen `issuer` die Vorlage selbst ist. Jedes Token muss dann `tid` aus `allowed_tenant_ids`
tragen und `iss` mit genau diesem Mandanten (ID-T05).

## 5. Beispielkonfiguration

Anlegen mit `createIdentityProvider` (Owner mit Step-up), Art `oidc`:

```json
{
  "kind": "oidc",
  "name": "Entra ID",
  "enabled": true,
  "allow_provisioning": false,
  "config": {
    "issuer": "https://login.microsoftonline.com/{tenantid}/v2.0",
    "client_id": "00000000-0000-0000-0000-EXAMPLE00001",
    "client_auth": "private_key_jwt",
    "client_key_ref": "file:<secrets.dir>/idp/entra_client_key.pem",
    "client_key_alg": "PS256",
    "client_key_id": "EXAMPLE-cert-1",
    "client_cert_x5t_s256": "EXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLE0",
    "allowed_tenant_ids": ["11111111-2222-3333-4444-EXAMPLE00002"],
    "app_redirect_uris": ["com.example.chat:/oidc", "https://app.example.com/oidc"]
  }
}
```

Mit einem einzigen Mandanten ist `issuer` `https://login.microsoftonline.com/<tid>/v2.0` und
`allowed_tenant_ids` entfällt. Desktop-Apps nutzen die Loopback-Umleitung
`http://127.0.0.1:<port>/callback` oder `http://[::1]:<port>/callback`, die ohne Eintrag erlaubt
ist; `localhost` ist nie erlaubt.

## 6. Konten

- Zuordnung nur über `(provider_id, issuer, subject)` oder einen Einladungs-Code (08 Abschnitt
  3.1); E-Mail, `preferred_username` und `upn` sind nur Anzeige.
- Eine Person ohne verknüpfte Identität meldet sich zum ersten Mal mit einem Einladungs-Code an
  (`createEnrollmentCode`, Zweck `identity_link` oder `first_login`); die App sendet ihn im Feld
  `enrollment_code` von `beginOidcLogin`. Eingelöst wird der Code erst beim Abschluss.
- Mit `allow_provisioning: true` entsteht für ein unbekanntes Subjekt eine Person im Status
  `pending` ohne Freigaben und ohne E-Mail; der Owner schaltet sie mit `enableUser` frei.
- Eine im Anbieter gesperrte Person behält ihre Sitzung höchstens bis `auth.idp_recheck_interval`;
  sofort sperrt der Owner mit `user.disable` (08 Abschnitt 3.5).
- Eine weitere Identität verknüpft die Person selbst mit `beginIdentityLink` und
  `completeIdentityLink` (Step-up).

## 7. Groups from the group claim

Built in M1-42 (08 section 3.4). The provider names its group claim in `groups_claim`; without it
the server never reads groups and never changes memberships at a login of that provider.

- **Claim in Entra.** Under "Token configuration" add the groups claim to the ID token ("Security
  groups" or "Groups assigned to the application"; the latter keeps the token small). Entra sends
  the group object IDs as an array of strings in `groups`, so the provider configuration gets
  `"groups_claim": "groups"`.
- **Mapping.** The owner creates one group per claim value with `createGroup`, `source:
  oidc_claim`, the `provider_id` of the provider and the claim value (the Entra object ID) as
  `external_ref`, and gives that group its grants. Claim values without such a group are ignored;
  the server never creates a group by itself. A group belongs to its provider: another provider
  with the same claim value never touches it.
- **At every login** of the provider the memberships of the person in its claim groups follow the
  claim: new ones get `expires_at = login time + auth.group_claim_max_age`, present ones (also
  expired ones) are extended to that time, and ones whose value is gone are removed. Every change
  is a rights change (`access.changed`, audit action `group.member.sync` with group ids only); a
  login without change writes nothing. Members of claim groups cannot be added or removed by hand
  (`409 claim_managed_group`).
- **Overage.** With more groups than fit into the token Entra sends a reference in `_claim_names`
  instead of `groups`. The server never calls Microsoft Graph: like a missing claim this changes
  nothing, and the memberships run out after `auth.group_claim_max_age`. Limit the claim to the
  groups that matter (application groups) so the overage does not happen.
- **Wrong form.** A `groups` claim that is not an array of strings fails the login
  (`error=login_failed`).
- **I1.** A sync that would let someone other than the credential holder trigger a personal agent
  (for example a claim group with `trigger` on a channel such an agent serves) fails the whole
  login with `409 i1_violation` and the hint "Sign-in blocked by a group mapping. An owner must
  switch the affected agent to organization first."; nothing of the login is kept, and the audit
  log has a deny entry of `group.member.sync` naming the person, the provider, the groups and the
  agents.

## 8. Konfigurationswerte

`auth.login_flow_ttl`, `auth.server_code_ttl`, `auth.oidc_jwks_cache_ttl`,
`auth.oidc_client_assertion_ttl`, `auth.idp_recheck_interval`, `auth.group_claim_max_age`, `auth.clock_leeway`,
`egress.timeout`, `egress.max_response_bytes`; Standards in der
[Konfigurationsreferenz](configuration.md).
