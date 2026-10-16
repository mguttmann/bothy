# Release signing keys

`cosign-private-phase.pub` is the public key of the cosign key pair that signs releases while
the repository is private ([ADR 0062](../../docs/adr/0062-release-signatures-while-the-repo-is-private.md)).
The owner creates the key pair once and commits only this public key here; the private key and
its password exist only as the secrets `COSIGN_PRIVATE_KEY` and `COSIGN_PASSWORD` of the GitHub
environment `release` ([release.md](../../docs/operations/release.md) section 2.2). Until the
file exists, a server release stops at its verification step, before any image tag.

After the switch to public ([public-switch.md](../../docs/quality/public-switch.md) step 20) the
file stays, so releases of the private phase remain verifiable.
