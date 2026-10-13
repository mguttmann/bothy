# M4-10: Device key for macOS and iOS

Status: ready | Depends on: M4-01 | Milestone: M4

## Goal

After this ticket `packages/device_key` is a working Flutter plugin for macOS and iOS: a typed Pigeon interface from Dart to native code, and a Swift core that creates, uses and deletes the two device keys of 08 6.1 (D-L10): the `dpop` key without user confirmation and the `step_up` key that requires biometrics or the device password, both ES256 (P-256) in the Secure Enclave where the device has one, and, without an enclave, a key in the Data Protection keychain that the API reports as not hardware-bound. It returns the public JWK, the RFC 7638 thumbprint and signatures in the JOSE form, an attestation where the platform has one, and a `hardwareBound` flag for the label. The Swift core is a Swift package that the Notification Service Extension and the Share Extension of M5 reuse (15 9: "the Swift and Kotlin parts use the same implementation as the extensions"). XCTest runs on a Mac with an Apple chip, the fallback without an enclave is tested in the iOS simulator, and the platform tests run in the `apps-macos` CI job of M4-01. Why: the DPoP binding and the step-up of every native client rest on keys that cannot be exported (08 6.1, R-CL-04, R-CL-05); a stolen refresh token is worthless without them (15 9).

## Read first

- `docs/PLAN.md`: section 3; section 10 (Risks: accessibility, TPM; rows M4-09 to M4-11, M4-18); section 11 rows M5-01 and M5-04 (Android key and the extensions reuse this core).
- `docs/spec/15-client-architecture.md`: 3.1 (rule 4: platform code only in `device_key` and `apps/client`), 9 "Secure storage and device key" (the table rows iOS and macOS, the plugin offer: create, public JWK and thumbprint, sign, attestation, delete), 10.1 (the extension signs with the device key without confirmation), 21 (R-CL-04, R-CL-05).
- `docs/spec/08-identity.md`: 5.1 (`dpop_jwk`, `step_up_jwk`, `attestation`), 5.4 (no key change; a lost key means a new device, R-ID-11), 6.1 (two keys, storage table: step-up key bound to the currently enrolled biometrics), 6.2 (what the step-up key signs), 9 (keys only on this device, never in cloud keychains or backups).
- `docs/DECISIONS.md`: D-L10, D-ID-02, D-W-100, D-W-114; section 10.
- RFC 7638 (thumbprint of an EC key: members `crv`, `kty`, `x`, `y` in this order), RFC 7518 3.4 (ES256 `R || S`), RFC 7517 (JWK).
- Apple documentation as cited in 15 9 (Secure Enclave supports only P-256; `SecAccessControl` flags `.privateKeyUsage`, `.biometryCurrentSet`, `.userPresence`; keychain accessibility `kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly`; Data Protection keychain on macOS).
- Briefs: `docs/briefs/M4-01-workspace-arch.md` (member rules, `platform-tests.json`, CI jobs), `docs/briefs/M4-09-auth-dpop.md` (the signer port this plugin backs through an adapter in the app).

This ticket touches no API operation, no table, no action, no event and no configuration key.

## Fixed requirements that apply

Owner's fixed build rules (apply verbatim):

1. No MVP. New from scratch and directly in the final state; stages are only the order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface with a docstring/doc
   comment (purpose, parameters, return value, errors, example where useful); comments explain the
   why, not the what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in MkDocs).
4. Documentation in the repo: README per module/service, ADR for every architecture decision,
   CHANGELOG per release, keep the documentation site current. A change without matching
   documentation is not done.
5. Quality gates (SonarQube, linters, type checking, security scanners, test coverage) block the
   merge.

Rules for every brief:

6. Test first: write the failing test, run it and keep the red output for the report, then make it green, then clean up.
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them, and the ARB test of M4-06 rejects them in interface texts. Use a hyphen or rephrase.
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. Interface texts come from `packages/l10n`, never as literals in widgets.
9. No configuration key is added; the plugin has no limits of its own.
10. Specification first: 15 9 and 08 6.1 differ on the step-up access control (`.userPresence` in 15 9, "bound to the currently enrolled biometrics" in 08 6.1); this ticket follows Design note 3 and updates 15 9 in the same change.

Product requirements implemented here:

11. Two keys per device and server: `dpop` (no user confirmation per signature, usable by the extensions through the shared keychain access group) and `step_up` (user confirmation per signature) (08 6.1, D-L10, R-CL-05).
12. ES256 only (P-256), because the Secure Enclave supports only P-256 (15 9, R-CL-04).
13. Hardware-bound in the Secure Enclave where available; otherwise a keychain key reported as not hardware-bound (15 9 row macOS).
14. Keys never leave the device: not exportable, never in iCloud keychain or backups (`ThisDeviceOnly` accessibility), DPoP key `AfterFirstUnlockThisDeviceOnly` in the access group shared with the extensions (15 9 row iOS).
15. No key change: there is no operation that replaces a key of an existing registration; `delete` removes both keys (08 5.4, R-ID-11).
16. Platform code only in `packages/device_key` and `apps/client` (15 3.1 rule 4).

## Scope

Create or fill in `packages/device_key/`:

- `pigeons/device_key_api.dart` (Pigeon source), generated `lib/src/generated/device_key_api.g.dart` and `darwin/Classes/DeviceKeyApi.g.swift` (Design note 1).
- `lib/device_key.dart` and `lib/src/device_key.dart`: the Dart API (`DeviceKeyClient`, `KeyPurpose`, `DeviceKeyInfo`, `DeviceKeyException` with codes) (Design note 2).
- `darwin/DeviceKeyCore/` Swift package (`Package.swift`, `Sources/DeviceKeyCore/*.swift`, `Tests/DeviceKeyCoreTests/*.swift`) and the plugin glue `darwin/Classes/DeviceKeyPlugin.swift`; `darwin/device_key.podspec` only if the pinned Flutter still needs CocoaPods for plugins, otherwise Swift Package Manager integration as the pinned Flutter supports it (Design note 4).
- `pubspec.yaml` plugin section (`platforms: ios, macos` with `sharedDarwinSource: true`, `pluginClass`), `platform-tests.json` (Design note 5).
- Dart tests `test/device_key_test.dart` (against a fake host API).

Change:

- `packages/device_key/README.md` (English: API, storage table per platform, how the extensions reuse the core, how to run the native tests).
- Root `Makefile` `codegen-check` (Pigeon regeneration compared, a loop over members with `pigeons/`), `tools/arch-check/rules.yaml` only if the plugin layout needs a folder name the rules do not list.
- `docs/spec/15-client-architecture.md` 9 (step-up access control, the Swift package for the extensions), `docs/spec/08-identity.md` 6.1 only if a sentence must point to 15 9.
- `docs/DECISIONS.md` section 10: rows `D-M4-10-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- Windows (M4-11), Android (M5-01), web (no device key).
- The adapter from this plugin to the `DeviceKeySigner` port of `auth` and the registration in the login flow: M4-18.
- Secret storage of tokens and the database key (`flutter_secure_storage`): M4-12 and M4-18.
- The Notification Service Extension and Share Extension themselves: M5-04, M5-06.
- App Attest or DeviceCheck attestation on iOS: return no attestation in this ticket; the attestation field stays optional (08 5.1, `auth.require_key_attestation` planned later).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-10-device-key-apple`; one writer per worktree.
- Conventional Commits with the package or component as scope (for example `feat(client): ...`, `feat(sync-engine): ...`, `build(gates): ...`, `docs(spec): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`. Never publish a Dart package (`publish_to: none` in every `pubspec.yaml`).
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` with port 0 and make no network call; fakes replace the server unless a section below names a container. Temporary directories only via `mktemp -d` (Dart `Directory.systemTemp.createTemp`, pytest `tmp_path`), removed at the end. Stop only processes you started, by PID.
- Never read or print secrets. No network calls except package downloads for pinned tools and pub packages and what the gates of `make check` do themselves.
- The Flutter SDK at the version pinned in the root `pubspec.yaml` must be on `PATH` (and Go and Docker where a section names them); without them the gates fail by design (R-QA-02), and the ticket stops with `Verdict: BLOCKED <tool> missing` instead of skipping.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never exclude hand-written code from a lint, format, coverage or doc gate.
- The product name appears only at the places of 02 "Code layout" ("Places that carry the product name"); the app shows `branding.product_name` from the instance, never a literal.
- No permission logic in the client (15 1 principle 1): controls follow server responses, snapshots and events, never an own derivation from grants.
- If the core contract (chapters 00, 03, 04, 05) or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M4 by D-W-114). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- Native tests need macOS with Xcode; on another host `make apps-platform` prints its SKIP line and the `apps-macos` CI job is the evidence. The report names where the native tests ran.
- Never write a test that leaves a key in the login keychain of the build host: every test key uses a test-only label prefix and is deleted in teardown, also after a failure.

## Design notes

1. **Pigeon interface.** `@HostApi() abstract class DeviceKeyHostApi` with: `DeviceKeyInfoMessage ensureKey(String scope, KeyPurposeMessage purpose)` (create if missing; `scope` separates keys per server and is hashed into the key label), `DeviceKeyInfoMessage? keyInfo(String scope, KeyPurposeMessage purpose)`, `Uint8List sign(String scope, KeyPurposeMessage purpose, Uint8List data)` (async, because the step-up key shows the system prompt), `Uint8List? attestation(String scope, KeyPurposeMessage purpose, Uint8List challenge)`, `void deleteKeys(String scope)`. `DeviceKeyInfoMessage` carries `x` and `y` (32 bytes each, big-endian), `hardwareBound`, `requiresUserPresence`. Errors are `PlatformException`s with stable codes: `user_cancelled`, `key_invalidated` (biometrics changed), `not_available`, `internal`. Generated code is checked in and regenerated in `make codegen-check`. Ruling for docs/DECISIONS.md: one Flutter plugin package with platform folders instead of separate federated packages, because 15 3 names one member `device_key`; the Pigeon interface is the boundary that "federated" asks for.

2. **Dart API.** `DeviceKeyClient({DeviceKeyHostApi? api})`; `Future<DeviceKeyInfo> ensure(String scope, KeyPurpose purpose)`; `Future<Uint8List> sign(...)` returns the JOSE `R || S` form (64 bytes; the native side converts from DER, Design note 3); `DeviceKeyInfo` computes `publicJwk` (`{"crv":"P-256","kty":"EC","x":...,"y":...}` with base64url without padding) and `thumbprint` (RFC 7638: SHA-256 over the JSON with exactly those members in lexicographic order, base64url) in Dart, so every platform shares one implementation; `hardwareBound` and `requiresUserPresence` from native. `delete(String scope)`. Errors map to `DeviceKeyException(DeviceKeyError code)`; messages contain no key material.

3. **Swift core `DeviceKeyCore`.** CryptoKit and Security only.
    - Secure Enclave available (`SecureEnclave.isAvailable`): `SecureEnclave.P256.Signing.PrivateKey(compactRepresentable: false, accessControl:)`; access control for `dpop`: `[.privateKeyUsage]` with accessibility `kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly`; for `step_up`: `[.privateKeyUsage, .biometryCurrentSet]` when biometrics are enrolled, otherwise `[.privateKeyUsage, .userPresence]` (device password), accessibility `kSecAttrAccessibleWhenPasscodeSetThisDeviceOnly` on iOS and the macOS equivalent. Store the key's `dataRepresentation` (an enclave-wrapped blob, not the private key) as a generic password item in the Data Protection keychain (`kSecUseDataProtectionKeychain` on macOS), with `kSecAttrSynchronizable = false`, the shared access group from the app's entitlements for `dpop` (the group name comes from the build configuration through the plugin's registration call; tests pass none), and the label `<prefix>.<sha256(scope)>.<purpose>`.
    - No enclave (older Macs, simulators): a software `P256.Signing.PrivateKey` stored the same way with the same accessibility; for `step_up` a `SecAccessControl` with `.userPresence` on the keychain item, so reading the key for a signature asks the user; `hardwareBound = false`. Ruling for docs/DECISIONS.md: biometry-bound step-up where biometrics exist (08 6.1), `.userPresence` otherwise; reason: 08 6.1 requires invalidation on new biometrics, while a Mac without Touch ID must still be able to confirm.
    - Signatures with `signature(for:)` over the given bytes (SHA-256 inside), returned as `rawRepresentation` (`R || S`).
    - A `KeyStore` protocol wraps the keychain calls so the unit tests can run against an in-memory store; the real keychain is used in one round-trip test (Tests).
    - `key_invalidated`: an enclave key whose access control was invalidated by a biometry change reports this code; the app then registers as a new device (08 5.4).

4. **Packaging.** `sharedDarwinSource: true` with the Swift sources under `darwin/`; the core as a local Swift package that the plugin target depends on; follow the plugin packaging that the pinned Flutter supports (Swift Package Manager for plugins where enabled, otherwise a podspec that includes the core's sources). Minimum versions iOS 15 and macOS 12 (15 2).

5. **Platform tests.** `platform-tests.json` with `macos`: `["swift", "test", "--package-path", "darwin/DeviceKeyCore"]` and an iOS simulator run `["xcodebuild", "test", "-scheme", "DeviceKeyCore", "-destination", "platform=iOS Simulator,OS=latest,name=<a simulator the runner has>"]` run from the core's folder (choose the destination with `xcrun simctl list` in a small wrapper script `darwin/run_ios_sim_tests.sh` that picks the first available iPhone, so no device name is hard-coded). The `apps-macos` CI job of M4-01 runs them. On the CI runner (a virtual machine) the Secure Enclave is usually not available; the tests for the hardware path then report themselves as skipped with the reason `secure enclave not available on this host`, and the report must name a run on a Mac with an Apple chip where they passed (the build host of the ticket, if it is such a Mac).

6. **Rulings for docs/DECISIONS.md** (`D-M4-10-<n>`): one plugin package with Pigeon (note 1); JWK and thumbprint computed in Dart (note 2); step-up access control and accessibility classes (note 3); the Swift package for reuse by the extensions (note 3, 15 9); skip rule of the hardware tests on virtual machines (note 5).

## Tests (write them first)

Swift, `darwin/DeviceKeyCore/Tests/DeviceKeyCoreTests/` (XCTest):

- `testCreatesTwoDistinctKeysPerScope` (`dpop` and `step_up` have different public keys; a second `ensureKey` returns the same key).
- `testScopesAreSeparated` (two scopes give two key pairs).
- `testSignatureIsRawAndVerifies` (64 bytes; verifies with `P256.Signing.PublicKey` over the same data; fails for other data).
- `testHardwarePathOnAppleChip` (with `SecureEnclave.isAvailable`: the key is an enclave key, `hardwareBound` true, the private key has no exportable raw form; skipped with the reason otherwise).
- `testFallbackWithoutEnclave` (forced by an injected availability flag and run in the iOS simulator: software key, `hardwareBound` false, signature verifies).
- `testStepUpRequiresUserPresenceFlag` (the access control of the `step_up` item contains `.userPresence` or `.biometryCurrentSet`; the `dpop` item contains neither).
- `testItemsAreThisDeviceOnlyAndNotSynchronizable` (attributes read back from the in-memory store and, in the round-trip test, from the keychain).
- `testKeychainRoundTrip` (real keychain with a test label prefix; skipped with the reason when the process lacks the keychain entitlement, error `errSecMissingEntitlement`).
- `testDeleteRemovesBothKeys`.
- `testErrorsCarryNoKeyMaterial`.

Dart, `packages/device_key/test/device_key_test.dart` (fake host API): JWK members and base64url of known `x` and `y`; the RFC 7638 thumbprint of the example key of RFC 7638 section 3.1 is not EC, so use the EC example key of RFC 7517 appendix A.1 and the thumbprint computed independently in the test from its canonical JSON; `sign` returns the native bytes unchanged; each native error code maps to its `DeviceKeyError`.

Commands that must pass, from the repository root:

- `cd packages/device_key && flutter analyze --fatal-infos && flutter test --coverage`
- on macOS: `make apps-platform` (runs `swift test` and the simulator tests); elsewhere it prints its SKIP line
- `make codegen-check`, `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] XCTest is green on a Mac with an Apple chip, including the hardware path; the simulator fallback test is green; the `apps-macos` CI job runs the native tests.
- [ ] Two keys per device and server (D-L10), ES256, enclave-bound where available, labeled as not hardware-bound otherwise.
- [ ] Items are `ThisDeviceOnly`, not synchronizable; the step-up key requires biometrics or the device password; the DPoP key does not.
- [ ] The Dart API returns the public JWK, the RFC 7638 thumbprint and `R || S` signatures; errors carry no key material.
- [ ] The Pigeon code is generated and checked; native code only in `packages/device_key`.
- [ ] 15 9 updated; rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every public Dart symbol and Swift documentation comments on every public Swift declaration; comments explain why.
- `packages/device_key/README.md` (English) as in Scope.
- 15 9 updated; rulings of Design note 6 in `docs/DECISIONS.md` section 10.
- `apps/client/CHANGELOG.md` does not exist yet; root `CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. Where the native tests ran (Mac model class with or without Apple chip, macOS and Xcode versions, simulator runtime) and which tests were skipped with which reason.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
