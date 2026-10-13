# M5-01: Device key for Android

Status: ready | Depends on: M4-10 | Milestone: M5

## Goal

After this ticket `packages/device_key` works on Android with the same Dart API and Pigeon interface as on macOS and iOS (M4-10): a Kotlin core creates, uses and deletes the two device keys of 08 6.1 (D-L10) in the Android Keystore, both ES256 (P-256), in StrongBox where the device has it (`setIsStrongBoxBacked(true)`), with a fallback to the trusted execution environment on `StrongBoxUnavailableException` and, where neither exists (emulators without a secure element), a key that the API reports as not hardware-bound. The `dpop` key signs without user confirmation (the push service of M5-05 uses it in the background); the `step_up` key requires user authentication for every use through `BiometricPrompt` (biometrics or the device credential) and is invalidated when new biometrics are enrolled. Signatures come back in the JOSE form `R || S`, the public JWK and RFC 7638 thumbprint come from the shared Dart code of M4-10, and Android key attestation (the certificate chain of `setAttestationChallenge`) is returned where the device offers it. The Kotlin core is a separate Gradle module that the app's push service (M5-05) and share activity reuse (15 9). Instrumented tests run on an Android emulator in a new CI job `apps-android`, and the fallback is labelled. Why: DPoP binding and step-up of the Android app rest on keys that cannot be exported (08 6.1, R-CL-04, R-CL-05); a stolen refresh token is worthless without them (15 9).

## Read first

- `docs/PLAN.md`: section 3; section 11 "M5 Mobile clients" (Goal, Exit criteria, Test strategy, Red team: T-DV-01), rows M5-01, M5-03, M5-05; section 10 row M4-10.
- `docs/spec/15-client-architecture.md`: 2 (Android minimum API 26), 3.1 (rule 4: platform code only in `device_key` and `apps/client`), 9 "Secure storage and device key" (row Android: Android Keystore, StrongBox if available, fallback on `StrongBoxUnavailableException`, P-256; step-up with user authentication per operation through `BiometricPrompt`; the plugin offer), 10.2 (the push service signs with the device key), 21 (R-CL-04, R-CL-05).
- `docs/spec/08-identity.md`: 5.1 (`dpop_jwk`, `step_up_jwk`, `attestation`), 5.4 (no key change, R-ID-11), 6.1 (row Android: step-up key with mandatory user authentication), 6.2, 9 (keys only on this device, never in backups).
- `docs/DECISIONS.md`: D-L10, D-ID-02, D-W-100 and its extension to M5 and M6 (D-W-120); section 10 (format of `D-M5-01-<n>` rows) and the rows `D-M4-10-<n>` (decisions the Android side follows).
- Android documentation: Keystore (`KeyGenParameterSpec.Builder`, `setIsStrongBoxBacked`, `setUserAuthenticationRequired`, `setUserAuthenticationParameters` from API 30 and `setUserAuthenticationValidityDurationSeconds(-1)` before it, `setInvalidatedByBiometricEnrollment`, `setAttestationChallenge`, `KeyInfo.getSecurityLevel` from API 31 and `isInsideSecureHardware` before it), `BiometricPrompt` with a `CryptoObject` around a `Signature` (https://developer.android.com/privacy-and-security/keystore, https://developer.android.com/identity/sign-in/biometric-auth).
- RFC 7518 3.4 (ES256 `R || S`), RFC 7638, RFC 7517.
- Briefs (merged names win, D-W-100): `docs/briefs/M4-10-device-key-apple.md` (all of it: the Pigeon interface `DeviceKeyHostApi`, `DeviceKeyClient`, `KeyPurpose`, `DeviceKeyInfo`, error codes `user_cancelled`, `key_invalidated`, `not_available`, `internal`, the `platform-tests.json` shape), `docs/briefs/M4-01-workspace-arch.md` (`tools/ci/apps_platform.py`, the platform CI jobs and their rules in `ci_ok.py`, `tools/arch-check/rules.yaml` rule 4), `docs/briefs/M4-09-auth-dpop.md` (the signer port).
- Code on the base branch: `packages/device_key` (Dart API, Pigeon source and generated code, `darwin/`), `tools/ci/apps_platform.py`, `.github/workflows/ci.yml`, `tools/ci/ci_ok.py` (or where M4-01 put the platform job rules), `tools/ci/check_runners.py`.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. Interface texts come from `packages/l10n`, never as literals in widgets; the `BiometricPrompt` title and subtitle are passed in from Dart.
9. No configuration key is added; the plugin has no limits of its own.
10. Specification first: if the Android behaviour forces a change of 15 9 or 08 6.1 (for example the device credential as an allowed step-up authenticator), update them in the same change. The core contract is not edited by this ticket.

Product requirements implemented here:

11. Two keys per device and server (`dpop`, `step_up`), separated per server scope like M4-10 (08 6.1, D-L10, R-CL-05).
12. ES256 only (P-256) (15 9, R-CL-04).
13. StrongBox where available, the trusted execution environment otherwise, a software-backed Keystore key on devices without either; `hardwareBound` reports the real security level (15 9 row Android).
14. Keys never leave the device: Keystore keys are not exportable; no key material in backups (the Keystore is excluded by the system; the app's `allowBackup` settings are the app's concern, M5-03).
15. The `step_up` key requires user authentication for every signature and is invalidated by a new biometric enrollment; the `dpop` key requires none (08 6.1).
16. No key change: `delete` removes both keys of a scope (08 5.4, R-ID-11).
17. Platform code only in `packages/device_key` and `apps/client` (15 3.1 rule 4).

## Scope

Create or fill in `packages/device_key/`:

- `android/` plugin module: `build.gradle.kts` (or the Groovy file the pinned Flutter plugin template generates), `src/main/kotlin/.../DeviceKeyPlugin.kt` (Pigeon host implementation), generated `DeviceKeyApi.g.kt` (Pigeon output for Kotlin added to the existing Pigeon source; Design note 1).
- `android/devicekeycore/`: a Gradle library module with the Kotlin core (`DeviceKeyCore`, `KeyStoreAccess` interface with a real and an in-memory implementation, `StepUpSigner` that runs `BiometricPrompt` through an injected prompt host) and its unit tests (`src/test`, JVM with the in-memory store) and instrumented tests (`src/androidTest`) (Design notes 2 to 4).
- `pubspec.yaml` plugin section: `android` with `package` and `pluginClass`.
- `platform-tests.json`: an `android` entry (Design note 5).

Change:

- `pigeons/device_key_api.dart`: Kotlin output options; regenerate all outputs; `make codegen-check` compares them.
- `tools/ci/apps_platform.py` and its tests: a system `android` that runs when `adb devices` lists exactly one device or emulator and otherwise prints `SKIP apps-platform <member> android: no device`; selection by an explicit `--system android` argument as well as by the host check, because an Android emulator can run on Linux and macOS (Design note 5).
- `.github/workflows/ci.yml`: job `apps-android` on `ubuntu-24.04` with hardware acceleration, the Android emulator runner action pinned by SHA (API level 26 and the current stable API, x86_64, Google APIs image without Play Store), running `tools/ci/apps_platform.py --system android`; `timeout-minutes`, `permissions: contents: read`; the job feeds `ci-ok` through the same rule mechanism as `apps-macos` and `apps-windows`; `make runners-check` and `make lint` (actionlint, zizmor) stay green.
- `tools/arch-check/rules.yaml` only if the module layout needs a folder name the rules do not list (Kotlin sources under `packages/device_key/android` are allowed by rule 4).
- `packages/device_key/README.md` (English: Android storage, StrongBox fallback, biometric enrollment invalidation, how the push service reuses the core, how to run the instrumented tests), `docs/spec/15-client-architecture.md` 9 (row Android: the module for reuse, the device credential rule of Design note 3), `docs/quality/gates.md` (the job), root `CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M5-01-1` to `D-M5-01-6`.

## Out of scope

- The adapter from this plugin to the signer port of `auth` and its use in the Android app (M5-03 wires the Android app; the adapter of M4-18 is reused).
- The push service and the share activity (M5-05, M5-06).
- Secret storage of tokens and the database key (`flutter_secure_storage`, M4-12 and M5-03).
- Windows (M4-11), macOS and iOS (M4-10).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M5-01-device-key-android`; one writer per worktree.
- Conventional Commits with the package or component as scope (for example `feat(device-key): ...`, `ci(apps): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`. Never publish a Dart package (`publish_to: none`).
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` with port 0 and make no network call. Temporary directories only via `mktemp -d`, removed at the end. Stop only processes you started, by PID; an emulator you started is stopped by its PID or with `adb emu kill` for its own serial.
- Never read or print secrets. No network calls except package downloads for pinned tools, pub packages, Gradle dependencies and system images, and what the gates of `make check` do themselves.
- The Flutter SDK at the pinned version, a JDK and the Android SDK with an emulator image must be available for the instrumented tests; without them `make apps-platform` prints its SKIP line, the `apps-android` CI job is the evidence, and the report names where the tests ran. The JVM unit tests run in `make test` on every host with a JDK; without a JDK the gate fails by design and the ticket stops with `Verdict: BLOCKED <tool> missing`.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never exclude hand-written code from a lint, format, coverage or doc gate.
- If the core contract or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M5 and M6 by D-W-120). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- Every test key uses a test-only alias prefix and is deleted in teardown, also after a failure.

## Design notes

1. **Pigeon** (M4-10 Design note 1). The interface stays exactly as M4-10 defined it; add the Kotlin output and the Android implementation. `sign` for `step_up` is asynchronous and shows the prompt; the prompt texts (`title`, `subtitle`, `negativeButton` where the API needs it) come from Dart in an optional message member added to the Pigeon source only if M4-10 has none (then the Apple side ignores it, because the system shows its own text; record it).

2. **Key generation** (15 9; 08 6.1; Ruling for docs/DECISIONS.md). Alias `<prefix>.<sha256(scope)>.<purpose>`. `KeyPairGenerator.getInstance(KEY_ALGORITHM_EC, "AndroidKeyStore")` with `ECGenParameterSpec("secp256r1")`, purposes `SIGN`, digest `SHA256`. `dpop`: no user authentication. `step_up`: `setUserAuthenticationRequired(true)`; from API 30 `setUserAuthenticationParameters(0, AUTH_BIOMETRIC_STRONG or AUTH_DEVICE_CREDENTIAL)`, before API 30 `setUserAuthenticationValidityDurationSeconds(-1)` (per operation, biometric only on those levels); `setInvalidatedByBiometricEnrollment(true)`. Try `setIsStrongBoxBacked(true)` first; on `StrongBoxUnavailableException` generate again without it. With a challenge from the caller set `setAttestationChallenge` and return the certificate chain as the attestation (DER certificates concatenated in the encoding the M4-10 attestation member already uses, or a JSON array of base64 certificates if it has none; record it).

3. **Step-up with the device credential** (08 6.1, 15 9; Ruling for docs/DECISIONS.md). On API 30 and later the step-up accepts strong biometrics or the device credential, like `.userPresence` on Apple without biometrics (M4-10 Design note 3); on API 26 to 29 a per-operation key cannot be used with the device credential, so the step-up key there accepts strong biometrics only, and a device without enrolled biometrics on those levels gets `not_available` for `step_up` (the app then explains that step-up needs biometrics; the person can still use the app without step-up actions). Write this rule into 15 9.

4. **Signatures and security level.** `Signature.getInstance("SHA256withECDSA")`, for `step_up` wrapped in `BiometricPrompt.CryptoObject` and authenticated before `sign`; DER output converted to `R || S` with 32 bytes each (leading zero handling tested). `hardwareBound`: from API 31 `KeyInfo.securityLevel` in `SECURITY_LEVEL_STRONGBOX` or `SECURITY_LEVEL_TRUSTED_ENVIRONMENT`; before it `isInsideSecureHardware`. Error mapping: user cancel and negative button to `user_cancelled`; `KeyPermanentlyInvalidatedException` to `key_invalidated`; no biometrics or no hardware for the purpose to `not_available`; everything else `internal`; messages carry no key material.

5. **Platform tests and CI** (M4-01 Design note 6). `platform-tests.json` `android`: `[["sh", "android/run_instrumented_tests.sh"]]`, a wrapper that runs the instrumented tests of `devicekeycore` with Gradle against the one attached device and fails on any failure. `tools/ci/apps_platform.py` gains the system `android` as described in Scope; on macOS and Windows hosts without a device it prints its SKIP line, so `make apps-platform` stays usable everywhere. The CI job `apps-android` runs it on the emulator; StrongBox is not available on emulators, so the StrongBox path test reports itself as skipped with the reason `strongbox not available on this device`, and the report must name a run on a physical device with StrongBox where it passed, or state under "not proven" that it did not run.

6. **Rulings for `docs/DECISIONS.md`** (`D-M5-01-<n>`): key parameters and alias (note 2); attestation encoding (note 2); the device credential rule per API level (note 3); signature conversion and security level mapping (note 4); the `android` platform system and the CI job (note 5); the separate core module for reuse by the push service (Scope).

## Tests (write them first)

Kotlin, `android/devicekeycore/src/test` (JVM, in-memory `KeyStoreAccess`):

- `derToRawConversion` (lengths 31, 32, 33 of `r` and `s`, leading zeros), `aliasPerScopeAndPurpose`, `errorMappingCarriesNoKeyMaterial`.

Kotlin, `android/devicekeycore/src/androidTest` (instrumented, emulator or device):

- `createsTwoDistinctKeysPerScope`, `scopesAreSeparated`, `signatureIsRawAndVerifies` (with `java.security` verification against the public key), `dpopSignsWithoutPrompt`, `stepUpRequiresAuthentication` (the key's `KeyInfo` reports user authentication required; a `sign` without a prompt fails with `UserNotAuthenticatedException` mapped to the expected error), `stepUpInvalidatedByEnrollmentFlag` (the spec flag is set; the real invalidation is a device test listed in the report), `strongBoxWhenAvailable` (skipped with the reason on emulators), `fallbackLabelledNotHardwareBound` (forced through an injected availability flag on the emulator), `attestationChainReturned` (present on images that support key attestation, otherwise skipped with the reason), `deleteRemovesBothKeys`.

Dart, `packages/device_key/test/` (fake host API): the Android error codes map to `DeviceKeyError`; the prompt texts pass through.

Python, `tools/ci/tests/test_apps_platform.py`: `test_android_runs_with_one_device`, `test_android_skips_without_device`, `test_android_fails_with_two_devices` (an ambiguous target is an error, not a guess), `test_explicit_system_argument`. `tools/ci/tests/test_ci_toolchains.py` (or the M4-01 file holding the platform jobs): `apps-android` exists, is pinned, feeds `ci-ok`.

Commands that must pass, from the repository root:

- `cd packages/device_key && flutter analyze --fatal-infos && flutter test --coverage`
- `cd packages/device_key/android && ./gradlew :devicekeycore:test` (or the Gradle invocation the plugin layout gives)
- with an attached emulator or device: `make apps-platform`; otherwise it prints its SKIP line
- `make codegen-check`, `make arch`, `make lint`, `make fmt-check`, `make coverage`, `make runners-check`
- `make check`

## Acceptance criteria

- [ ] Instrumented tests on Android are green on the emulator in the `apps-android` CI job; the StrongBox path ran on a physical device or is listed as not proven.
- [ ] Two keys per device and server, ES256, StrongBox where available, labelled as not hardware-bound in the fallback.
- [ ] The step-up key requires authentication per signature and carries the biometric enrollment invalidation; the DPoP key signs without a prompt.
- [ ] The Dart API is unchanged and returns `R || S` signatures; errors carry no key material.
- [ ] The Kotlin core is a separate module that the app can reuse; native code only in `packages/device_key`.
- [ ] 15 9, gates.md, README, CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every public Dart symbol and KDoc on every public Kotlin declaration; comments explain why.
- `packages/device_key/README.md` updated in English.
- Rulings of Design note 6 recorded in `docs/DECISIONS.md` section 10.
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
4. Where the instrumented tests ran (emulator image and API level, physical devices with StrongBox), which tests were skipped with which reason.
5. What is not proven (for example the real invalidation after a new fingerprint, which needs a device and a person).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
