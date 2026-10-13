# M4-11: Device key for Windows

Status: ready | Depends on: M4-10 | Milestone: M4

## Goal

After this ticket `packages/device_key` also works on Windows: the Pigeon interface of M4-10 gets a C++ host implementation in `windows/` that creates, uses and deletes the two device keys of 08 6.1 (D-L10) with CNG. The `dpop` key is an ECDSA P-256 key in the Microsoft Platform Crypto Provider (TPM 2.0) after `NCryptIsAlgSupported` has confirmed P-256 on this TPM; without a TPM, or when the TPM lacks P-256, it is a key of the Microsoft Software Key Storage Provider that the API reports as not hardware-bound. The `step_up` key requires a Windows Hello confirmation (face, fingerprint or PIN) before each signature, with the strongest binding the platform offers for a P-256 key (Design note 3), and the API reports which binding it has. The Dart API of M4-10 stays unchanged (`hardwareBound` and `requiresUserPresence` already carry the labels). Tests run on a Windows machine with and without a TPM (the second through a forced fallback), and the fallback is labeled. Why: Windows is a desktop target of M4 (15 2, R-CL-12); its DPoP binding and step-up must rest on keys that cannot be exported (08 6.1, R-CL-04, R-CL-05), and the plan names Windows without TPM 2.0 or without P-256 as a risk (PLAN 10 "Risks").

## Read first

- `docs/PLAN.md`: section 3; section 10 (Risks: "Windows without TPM 2.0 or without P-256"; rows M4-10, M4-11, M4-18, M4-34).
- `docs/spec/15-client-architecture.md`: 2 (Windows 10 2004 minimum), 3.1 (rule 4: platform code only in `device_key` and `apps/client`), 9 (row Windows: CNG with the Platform Crypto Provider, `NCryptIsAlgSupported`, software fallback reported as not hardware-bound, step-up "TPM key with Windows Hello confirmation [assumed, details not verified]"), 21 (R-CL-04, R-CL-05, R-CL-12).
- `docs/spec/08-identity.md`: 5.1 (`dpop_jwk`, `step_up_jwk`, `attestation`), 5.4 (no key change, R-ID-11), 6.1 (two keys, storage table), 6.2 (what the step-up key signs), 9 (keys never leave the device).
- `docs/DECISIONS.md`: D-L10, D-W-100, D-W-114; the rows `D-M4-10-<n>` of M4-10 (one plugin package with Pigeon, JWK and thumbprint in Dart, error codes); section 10.
- Microsoft documentation as cited in 15 9: `NCryptOpenStorageProvider` with `MS_PLATFORM_CRYPTO_PROVIDER` and `MS_KEY_STORAGE_PROVIDER`, `NCryptIsAlgSupported`, `NCryptCreatePersistedKey` (`BCRYPT_ECDSA_P256_ALGORITHM`), `NCryptFinalizeKey`, `NCryptSignHash`, `NCryptExportKey` with `BCRYPT_ECCPUBLIC_BLOB`, `NCryptDeleteKey`, the property `NCRYPT_EXPORT_POLICY_PROPERTY`, and `Windows.Security.Credentials.UI.UserConsentVerifier` (C++/WinRT).
- RFC 7518 3.4 (ES256 `R || S`), RFC 7638 (thumbprint; computed in Dart by M4-10).
- Briefs: `docs/briefs/M4-10-device-key-apple.md` (the Pigeon interface, the Dart API, the error codes, `platform-tests.json`), `docs/briefs/M4-01-workspace-arch.md` (Design note 6: `apps-windows` CI job and `tools/ci/apps_platform.py`).
- Code on the base branch: `packages/device_key/` as M4-10 left it (`pigeons/device_key_api.dart`, the generated Dart and Swift files, `lib/src/device_key.dart`, `platform-tests.json`).

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
10. Specification first: 15 9 marks the Windows step-up as assumed; this ticket verifies the mechanism, follows Design note 3 and updates the row Windows of 15 9 in the same change.

Product requirements implemented here:

11. Two keys per device and server: `dpop` (no user confirmation per signature) and `step_up` (a Windows Hello confirmation per signature) (08 6.1, D-L10, R-CL-05).
12. ES256 only (P-256) on every path, also in the fallback (15 9, R-CL-04).
13. Hardware-bound in the TPM when the Platform Crypto Provider is available and `NCryptIsAlgSupported` confirms `BCRYPT_ECDSA_P256_ALGORITHM`; otherwise a software key reported as not hardware-bound (15 9 row Windows).
14. Keys are not exportable: export policy `0` (no `NCRYPT_ALLOW_EXPORT_FLAG`, no plaintext export) on both providers; keys are per user (no machine key) and never roam.
15. No key change: no operation replaces a key of an existing registration; `deleteKeys` removes both keys (08 5.4, R-ID-11).
16. Platform code only in `packages/device_key` (15 3.1 rule 4); the Dart side of M4-10 stays the single implementation of JWK and thumbprint.

## Scope

Create in `packages/device_key/`:

- `windows/CMakeLists.txt` and the plugin glue `windows/device_key_plugin.cpp`, `windows/device_key_plugin.h`, `windows/include/device_key/device_key_plugin_c_api.h` as the pinned Flutter's plugin template expects.
- The generated Pigeon C++ files `windows/device_key_api.g.h` and `windows/device_key_api.g.cpp` (Pigeon option `cppHeaderOut`, `cppSourceOut`, namespace `device_key`) next to the existing Dart and Swift outputs; `pigeons/device_key_api.dart` only gains the C++ output options.
- `windows/core/` a static library `device_key_core` with no Flutter dependency (Design note 2): `key_provider.h` (interface over the CNG calls), `cng_key_provider.cpp`, `device_key_store.cpp` and `.h` (labels, provider choice, sign, delete), `user_consent.cpp` and `.h` (Windows Hello through C++/WinRT, Design note 3), `der.cpp` and `.h` only if a signature arrives in DER (CNG returns `R || S` for ECDSA; keep the converter out if not needed).
- `windows/core/tests/` with `CMakeLists.txt`, a minimal assertion header `check.h` (no third-party test framework) and the test sources of Tests; registered with CTest.
- `platform-tests.json`: the `windows` entry (Design note 5).

Change:

- `packages/device_key/pubspec.yaml` (plugin platform `windows` with `pluginClass: DeviceKeyPluginCApi`), `packages/device_key/README.md` (storage table row Windows, how to run the native tests, the binding of the step-up key).
- Root `Makefile` `codegen-check`: the Pigeon comparison covers the C++ outputs.
- `docs/spec/15-client-architecture.md` 9 (row Windows: the verified step-up binding, Design note 3).
- `docs/DECISIONS.md` section 10: rows `D-M4-11-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- macOS and iOS (M4-10), Android (M5-01), web (no device key).
- The adapter from this plugin to the signer port of `auth` and the registration in the login flow: M4-18.
- Storage of tokens and the database key on Windows (`flutter_secure_storage`): M4-12 and M4-18.
- Key attestation of the TPM (`NCRYPT_PCP_TPM12_IDBINDING` and similar): `attestation` returns no value on Windows in this ticket; the field stays optional (08 5.1).
- Windows notifications, MSIX and package identity (M4-30, M7).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-11-device-key-windows`; one writer per worktree.
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
- Native tests need Windows with the Visual Studio C++ build tools that the pinned Flutter requires; on another host `make apps-platform` prints its SKIP line and the `apps-windows` CI job of M4-01 is the evidence. The report names where the native tests ran.
- Never leave a key in the key storage of the build host: every test key uses a test-only name prefix (`device-key-test-`) and is deleted in teardown, also after a failure, and a test fixture deletes leftovers with that prefix before it starts.
- A test never waits for a person: the Windows Hello prompt is replaced by an injected consent function in every automated test (Design note 3); the real prompt is checked by hand once and named in the report.

## Design notes

1. **Pigeon for C++.** Add `cppOptions: CppOptions(namespace: 'device_key')` and the C++ output paths to `pigeons/device_key_api.dart`; regenerate all outputs and check them in. The host API is the same `DeviceKeyHostApi` as on macOS (`ensureKey`, `keyInfo`, `sign`, `attestation`, `deleteKeys`), with the same message types and the same error codes `user_cancelled`, `key_invalidated`, `not_available`, `internal` as `FlutterError` codes. `sign` runs off the platform thread (the Hello prompt and TPM operations block for up to seconds) and answers through the Pigeon result callback.

2. **Core library and provider choice.** `device_key_core` wraps CNG behind `KeyProvider` (open provider, check algorithm, create persisted key, open key, export public blob, sign hash, delete key) so the tests can inject a fake that simulates "no TPM", "TPM without P-256" and failures. `DeviceKeyStore::Ensure(scope, purpose)`:
    - Key name `device-key.<sha256(scope) hex>.<purpose>` (no product name: 02 "Places that carry the product name") (the `scope` separates servers as on macOS); per user (no `NCRYPT_MACHINE_KEY_FLAG`).
    - Try `MS_PLATFORM_CRYPTO_PROVIDER`; if `NCryptOpenStorageProvider` fails or `NCryptIsAlgSupported(BCRYPT_ECDSA_P256_ALGORITHM)` returns `NTE_NOT_SUPPORTED`, use `MS_KEY_STORAGE_PROVIDER` and set `hardwareBound = false`. An existing key is opened in the provider where it was created (try the platform provider first, then the software provider) and is never recreated in the other one.
    - `NCRYPT_EXPORT_POLICY_PROPERTY` = `0` before `NCryptFinalizeKey`.
    - Public key from `NCryptExportKey(BCRYPT_ECCPUBLIC_BLOB)`: check the magic `BCRYPT_ECDSA_PUBLIC_P256_MAGIC` and length 32 for X and Y, then return `x` and `y` big-endian as on macOS.
    - `Sign(scope, purpose, data)`: SHA-256 over `data` with BCrypt, then `NCryptSignHash`; CNG returns the 64-byte `R || S` form for ECDSA P-256 (check the length; any other length is `internal`).
    Ruling for docs/DECISIONS.md: provider choice per key at creation, never migrated; reason: a key moved between providers would change the JWK and need a new device (08 5.4).

3. **Step-up binding (verify, then decide).** 15 9 assumes "a TPM key with Windows Hello confirmation". Windows Hello's own key API (`KeyCredentialManager`) creates RSA keys only, so it cannot hold an ES256 key. Verify in the Microsoft documentation and on the build host which of these the platform offers for a P-256 CNG key, in this order, and use the first that works:
    - (a) a TPM key with a CNG UI policy (`NCRYPT_UI_POLICY_PROPERTY` with `NCRYPT_UI_FORCE_HIGH_PROTECTION_FLAG`) whose confirmation the system shows on every use of the key (cryptographic binding by the platform);
    - (b) a TPM or software key that the plugin uses only after `UserConsentVerifier::RequestVerificationAsync` returned `Verified` (binding by the plugin, not by the key).
    Report the binding through `requiresUserPresence = true` in both cases and record in the README and in 15 9 which one applies. If `UserConsentVerifier::CheckAvailabilityAsync` reports no Hello on the device, `ensureKey` for `step_up` fails with `not_available` (the app then offers the other step-up methods of the instance, 08 6). Ruling for docs/DECISIONS.md with the verified sources and the reason. Option (b) is weaker against malware running as the same user; name that in the README and in the report item 5.

4. **Errors.** `user_cancelled` (Hello cancelled or `NTE_USER_CANCELLED`), `not_available` (no Hello for the step-up key; provider not usable at all), `key_invalidated` (the key exists but cannot be opened, for example after a TPM clear: `NTE_BAD_KEYSET` on an existing name; the app then registers as a new device, 08 5.4), `internal` (everything else, with the `HRESULT` in hex and no key material). Messages carry no key bytes and no data that was signed.

5. **Platform tests.** `platform-tests.json` gets `"windows": [["cmake", "-S", "windows/core", "-B", "build/core-tests"], ["cmake", "--build", "build/core-tests", "--config", "Debug"], ["ctest", "--test-dir", "build/core-tests", "-C", "Debug", "--output-on-failure"]]` (argument lists, no shell), run by `tools/ci/apps_platform.py` of M4-01 in the `apps-windows` job. `build/` is git-ignored already (M4-01). The TPM test reports itself as skipped with the reason `platform crypto provider not available on this host` when the provider cannot be opened or lacks P-256; the fallback test always runs by forcing the software provider through the injected `KeyProvider` choice and once against the real software provider.

6. **Rulings for docs/DECISIONS.md** (`D-M4-11-<n>`): provider choice per key (note 2); the verified step-up binding (note 3); the minimal test harness without a third-party framework (Scope); the skip rule of the TPM test (note 5).

## Tests (write them first)

C++, `packages/device_key/windows/core/tests/` (CTest):

- `creates_two_distinct_keys_per_scope` (`dpop` and `step_up` differ; a second `Ensure` returns the same public key).
- `scopes_are_separated`.
- `signature_is_raw_and_verifies` (64 bytes; verified with `BCryptVerifySignature` over the same hash with the exported public key; fails for other data).
- `tpm_path_when_available` (real platform provider: `hardwareBound` true, export of the private blob fails; skipped with the reason otherwise).
- `software_fallback_without_tpm` (fake provider without the platform provider, and fake provider "TPM without P-256": software key, `hardwareBound` false, signature verifies).
- `keys_are_not_exportable` (export policy property reads `0`; `NCryptExportKey` with `BCRYPT_ECCPRIVATE_BLOB` fails on both providers).
- `step_up_requires_consent` (with an injected consent function: `Sign` for `step_up` calls it once per signature and fails with `user_cancelled` when it declines; `Sign` for `dpop` never calls it).
- `step_up_not_available_without_hello` (injected availability `false`: `Ensure(step_up)` fails with `not_available`).
- `existing_key_is_not_recreated_in_other_provider`.
- `delete_removes_both_keys`.
- `errors_carry_no_key_material`.

Dart, `packages/device_key/test/` (fake host API, already present from M4-10): one test that the Windows error code mapping is the same as on macOS (the codes are shared constants).

Commands that must pass, from the repository root:

- `cd packages/device_key && flutter analyze --fatal-infos && flutter test --coverage`
- on Windows: `make apps-platform` (runs the CTest suite); elsewhere it prints its SKIP line
- `make codegen-check`, `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] The CTest suite is green in the `apps-windows` CI job; the TPM test ran on a machine with a TPM 2.0 that supports P-256, or its skip and the reason are in the report (item 5) together with the run of the fallback. Rule for the verdict: a missing TPM on every host the session can reach is not a reason for `BLOCKED`; the ticket ends with `Verdict: DONE` when every other test is green, and the TPM path stays listed as not proven.
- [ ] Two keys per device and server (D-L10), ES256 on every path, TPM-bound where available, labeled as not hardware-bound otherwise.
- [ ] Keys are not exportable on either provider; the step-up key needs a Windows Hello confirmation per signature, the DPoP key does not; the binding is documented.
- [ ] The Pigeon C++ code is generated and checked by `make codegen-check`; native code only in `packages/device_key`.
- [ ] 15 9 row Windows updated; rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public Dart symbol and a documentation comment on every public C++ declaration; comments explain why and cite the Microsoft API where a rule comes from.
- `packages/device_key/README.md` (English) as in Scope.
- 15 9 updated; rulings of Design note 6 in `docs/DECISIONS.md` section 10.
- Root `CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. Where the native tests ran (CI runner or a named class of Windows machine, Windows build, TPM present and P-256 supported or not), which tests were skipped with which reason, which step-up binding of Design note 3 was chosen with its sources, and whether the Hello prompt was checked by hand.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
