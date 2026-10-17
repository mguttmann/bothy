# Login with Microsoft Entra ID (OIDC)

Status: built (M1-41). Normative are [08-identity.md](../spec/08-identity.md) sections 2.2,
2.3 and 2.6; where they differ, the chapter wins. Microsoft Entra ID is the reference example;
every other OpenID Connect provider is set up the same way.

## 1. The flow in short

The server is a confidential OIDC client (relying party) at Entra and at the same time the
authorization service of its apps. The app opens the login in the system browser, Entra redirects
back to the server, the server checks the ID token and gives the app a one-time `login_code` that
only the app instance with its `client_verifier` can redeem. Provider tokens never reach a device
and are never stored. Both legs use PKCE `S256`.

## 2. App registration in Entra

1. New app registration as a **web application** (confidential client).
2. Exactly one redirect URI: `https://<host>/api/v1/auth/oidc/callback`, for example
   `https://chat.example.com/api/v1/auth/oidc/callback`. No further redirects are needed: the
   redirect back to the app (loopback or a private scheme) is handled by the server.
3. Under "Certificates & secrets" upload a **certificate** (client authentication
   `private_key_jwt`, preferred). A client secret (`client_secret_basic`) works as well, but it
   expires and is one more secret.
4. Optionally release the `groups` claim in the token; group sync is described by M1-42
   (08 section 3.4). Without `groups_claim` in the configuration it is not read.
5. For an organization with several tenants choose "Accounts in any organizational directory" and
   enter the allowed tenant IDs in `allowed_tenant_ids`.

Sources (retrieved 2026-10-10): redirect URIs
https://learn.microsoft.com/en-us/entra/identity-platform/reply-url, certificate assertion
https://learn.microsoft.com/en-us/entra/identity-platform/certificate-credentials [verified].

## 3. Storing the secrets

Client key and client secret never stand in the configuration of the provider, only as a reference
to the secret provider. Only files below `<secrets.dir>/idp/` and environment variables with the
prefix `BOTHY_IDP_` are allowed; a reference to a secret of the server itself (for example
`auth.token_pepper_ref`) is refused (docs/DECISIONS.md M1-36/14 and M1-41).

- Certificate: store the private key as PKCS#8 PEM at `<secrets.dir>/idp/entra_client_key.pem`
  (mode `0600`). `client_key_alg` is `PS256` (default), `client_cert_x5t_s256` the SHA-256
  fingerprint of the certificate as base64url without padding (header `x5t#S256`).
- Secret: as the file `<secrets.dir>/idp/entra_client_secret` or as the variable
  `BOTHY_IDP_ENTRA_SECRET`.

## 4. Outgoing connections

The server reaches discovery, keys (JWKS) and the token endpoint only through the egress client:
allowed are the host of the issuer and the hosts of `token_endpoint` and `jwks_uri` from the
checked discovery document, only `https` (19 SEC-NET-2). Entra needs no entry in
`egress.allowed_hosts`. Discovery and keys are cached for `auth.oidc_jwks_cache_ttl`; on an
unknown `kid` the server reloads the keys once.

For an issuer with `{tenantid}` the server reads the discovery document of the alias
`organizations` (`https://login.microsoftonline.com/organizations/v2.0/.well-known/openid-configuration`),
whose `issuer` is the template itself. Every token must then carry a `tid` from
`allowed_tenant_ids` and an `iss` with exactly that tenant (ID-T05).

## 5. Example configuration

Create it with `createIdentityProvider` (owner with step-up), kind `oidc`:

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

With a single tenant `issuer` is `https://login.microsoftonline.com/<tid>/v2.0` and
`allowed_tenant_ids` is omitted. Desktop apps use the loopback redirect
`http://127.0.0.1:<port>/callback` or `http://[::1]:<port>/callback`, which is allowed without an
entry; `localhost` is never allowed.

## 6. Accounts

- Matching only through `(provider_id, issuer, subject)` or an enrollment code (08 section 3.1);
  email, `preferred_username` and `upn` are display only.
- A person without a linked identity logs in for the first time with an enrollment code
  (`createEnrollmentCode`, purpose `identity_link` or `first_login`); the app sends it in the field
  `enrollment_code` of `beginOidcLogin`. The code is redeemed only at completion.
- With `allow_provisioning: true` an unknown subject gets a person in the status `pending` without
  grants and without email; the owner activates it with `enableUser`.
- A person disabled at the provider keeps their session at most until
  `auth.idp_recheck_interval`; the owner blocks at once with `user.disable` (08 section 3.5).
- The person links a further identity themselves with `beginIdentityLink` and
  `completeIdentityLink` (step-up).

## 7. Configuration values

`auth.login_flow_ttl`, `auth.server_code_ttl`, `auth.oidc_jwks_cache_ttl`,
`auth.oidc_client_assertion_ttl`, `auth.idp_recheck_interval`, `auth.clock_leeway`,
`egress.timeout`, `egress.max_response_bytes`; defaults in the
[configuration reference](configuration.md).
