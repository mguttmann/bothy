# M4-01: Workspace and architecture test of the apps

Status: ready | Depends on: M1-01 | Milestone: M4

## Goal

After this ticket the repository has one Dart pub workspace (15 3, R-CL-02) with every package and both apps that chapter 15 names: `packages/design_system`, `l10n`, `domain`, `api_client`, `auth`, `device_key`, `local_store`, `sync_engine`, `admin_ui`, `bothy_viz`, `bothy_noise`, `terminal_ui`, and `apps/client` and `apps/console`, each a buildable member with its manifest, analysis options, README and a first test, so that every later M4 to M6 ticket adds code to an existing member instead of inventing layout. An architecture test, `tools/arch-check`, holds the dependency rules of 15 3.1 for every member on every run of the gate: a forbidden import (for example `package:flutter` in `domain`, or `sync_engine` in `apps/console`) fails `make arch`. Documentation in code becomes a gate for Dart (23 "Documentation in code"): the lint `public_member_api_docs` is active in every member (generated code excluded), a public symbol without a doc comment fails `make lint`, `dart doc` runs per package without a warning, and `make docs` puts the result into the documentation site. The root `Makefile` and CI handle the workspace (one `pubspec.lock`, generated Dart code excluded from coverage and doc lint), and two CI jobs on `macos-15` and `windows-2025` run the platform tests that the later tickets add (`make apps-platform`). Why: rule 2 of the owner ("strictly modular, dependencies only through defined interfaces") needs a mechanical check before the first line of client code, and the later tickets of M4, M5 and M6 run in parallel on these members.

## Read first

- `docs/PLAN.md`: section 3 "Way of working per ticket"; section 5 "Test strategy" (rows "Unit", "Golden", "End to end"); section 10 "M4 Desktop clients" (Goal, Scope, Exit criteria item 1, Test strategy, Risks, the ticket table: this row, M4-02, M4-04, M4-08, M4-10, M4-18, M4-35, which build on the members created here); section 11 rows M5-01 and M5-04 (native code in `device_key` and `apps/client`); section 12 rows M6-01 and M6-10 (console and `bothy_viz`); section 19 P-01, P-09, P-10, P-11, P-16.
- `docs/spec/15-client-architecture.md`: 1 "Principles", 2 "Platforms", 3 "Workspace", 3.1 "Dependency rules" (the diagram and rules 1 to 7), 4 "State" (Riverpod 3), 6 (drift, the `hooks` excerpt), 20 "Test strategy", 21 (R-CL-01, R-CL-02, R-CL-13, R-CL-14), 23.1 "Packages and ports".
- `docs/spec/23-quality.md`: 1 "Principles" (no silent skip, R-QA-02), 2 "Gates", 3 "Component contract" (row "Dart package") and "Documentation in code" (row "Dart"), 4 "CI" (runners `macos-15` and `windows-2025` for the app builds), 10 (R-QA-02, R-QA-14), 11 (`quality.coverage.dart_min_percent`).
- `docs/spec/02-architecture.md`: 6 "Code layout" (blocks `apps/` and `packages/`, "Places that carry the product name").
- ADRs: `0027-flutter-for-all-apps.md` ("Confirmation": the architecture test), `0028` (web only for the console), `0029-client-state-local-database-and-offline-queue.md` ("Confirmation": widgets read only through providers), `0038-local-gates-first-one-command-identical-to-ci.md`, `0039-only-github-hosted-runners-public-from-the-first-push.md`, `0045` (documentation site).
- `docs/DECISIONS.md`: D-W-54 (MkDocs and Zensical), D-W-94 (English), D-W-100, D-W-114; section 10 for the format of new rows.
- `docs/quality/gates.md`, `docs/quality/toolchain.md` (row `subosito/flutter-action`), `docs/quality/licenses.md`, `docs/quality/red-team-gate.md`, `docs/quality/push-discipline.md`, `CONTRIBUTING.md` ("Development setup").
- Briefs: `docs/briefs/M1-01-server-gates.md` (the gate pattern and `tools/ci/tests/test_make_server_gates.py`); `docs/briefs/M2-38-noise-core.md` (Design note 3: the Dart lines of `test` and `coverage`, the CI toolchain steps; Design note 8: the Flutter pin and the workspace takeover of `packages/bothy_noise`); `docs/briefs/M2-23-generated-clients.md` (Design note 8: generated code and gate scope).
- Code on the base branch: root `Makefile` (`DART_PKGS`, `DART_ON`, `fmt`, `fmt-check`, `lint`, `typecheck`, `test`, `coverage`, `arch`, `docs`, `licenses`, `osv`, `sonar`); `tools/ci/components.py` and its tests; `.github/workflows/ci.yml` (jobs `plan`, `gates`, `sonar`, `ci-ok`), `.github/workflows/docs.yml`; `tools/ci/check_runners.py`; `sonar-project.properties`; `mkdocs.yml`; `apps/README.md`, `apps/client/README.md`, `apps/console/README.md`, `packages/README.md`; `packages/bothy_noise/` if M2-38 has created it.

This ticket touches no API operation, no table of chapter 03, no action, no event and no configuration key.

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
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; this ticket adds none. The coverage threshold stays `quality.coverage.dart_min_percent` (`COVERAGE_DART_MIN`).
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract is not edited by this ticket.

Product requirements implemented here:

11. One pub workspace with one resolution and one `pubspec.lock` at the root; no Melos; tasks run through the make targets (15 3, R-CL-02).
12. The members and their kind (15 3): Flutter packages `design_system`, `l10n`, `device_key` (federated plugin, Pigeon), `admin_ui`, `bothy_viz`, `terminal_ui`; pure Dart packages `domain`, `api_client`, `auth`, `local_store`, `sync_engine`, `bothy_noise`; Flutter apps `apps/client` (macOS, Windows, later iOS and Android) and `apps/console` (web).
13. Dependency rules 1 to 7 of 15 3.1 and its diagram, checked on every run; in addition: a member depends on another member only if the diagram reaches that member from it (directly or through other members), and only `device_key` and `apps/client/{ios,macos,android,windows}` contain platform code (rule 4).
14. Documentation in code (23): `public_member_api_docs` in every member, generated code excluded; `dart doc` per package without a warning; the result in the documentation site.
15. No member is published (`publish_to: none`); every member has tests under `test/` (23 3), and the gates fail for a member without tests or without coverage report (as M2-38 Design note 3 fixed it).

## Scope

Create:

- Root `pubspec.yaml` (workspace root, Design note 1) and the root `pubspec.lock`; root `analysis_options.yaml` with the shared rules that every member includes (Design note 3).
- For each of the twelve packages under `packages/<name>/`: `pubspec.yaml`, `analysis_options.yaml`, `README.md` (English: purpose, status "skeleton (M4-01)", allowed dependencies from 15 3.1, the ticket that fills it), `lib/<name>.dart` (library doc comment, no other public symbol), `test/<name>_test.dart` (one test that imports the library). For `bothy_noise`: only if M2-38 has not created it yet; otherwise take the existing package into the workspace (Design note 1).
- `apps/client/` and `apps/console/`: `pubspec.yaml`, `analysis_options.yaml`, `lib/main.dart` (an app that shows an empty `MaterialApp` without text, doc comment on `main`), `test/app_test.dart` (pumps the app). No platform folder: M4-18 runs `flutter create` for macOS and Windows, M5 for iOS and Android, M6-01 for web.
- `tools/arch-check/`: `arch_check.py`, `rules.yaml`, `README.md` (English) and `tests/test_arch_check.py` (Design note 2).
- `tools/docs/dart_api_docs.py` and `tools/docs/tests/test_dart_api_docs.py`; the site page `docs/reference/dart-api.md` (Design note 5).
- `tools/ci/apps_platform.py` and `tools/ci/tests/test_apps_platform.py` (Design note 6).
- `tools/ci/tests/test_make_dart_workspace.py` (Tests).

Change:

- Root `Makefile`: `arch` gains the Dart architecture test; `fmt-check`, `lint`, `test`, `coverage` follow the workspace and exclude generated Dart code from coverage; new target `apps-platform` (not part of `gates`); `docs` calls `dart_api_docs.py` (Design notes 2 to 6).
- `.github/workflows/ci.yml`: Flutter from the root `pubspec.yaml` in the jobs `gates` and `sonar`; new jobs `apps-macos` and `apps-windows`; `ci-ok` needs them (Design note 6). `.github/workflows/docs.yml`: Flutter for `make docs`.
- `tools/ci/components.py` and its tests: the root `pubspec.yaml`, `pubspec.lock`, `analysis_options.yaml` and `tools/arch-check/` select `apps` (Design note 7).
- `sonar-project.properties`: Dart sources and lcov paths of the members, generated Dart files excluded (Design note 4).
- `mkdocs.yml` (navigation entry for `docs/reference/dart-api.md`), `.gitignore` (the generated API pages and `.dart_tool/`, `build/`).
- `docs/spec/15-client-architecture.md` 3 and 3.1 (the extra rules of requirement 13, the workspace root), `docs/spec/23-quality.md` 3 (row "Dart package": workspace member, generated code), `docs/quality/gates.md` (rows `arch`, `lint`, `docs`, new row `apps-platform`), `docs/quality/toolchain.md` (Flutter pin source), `CONTRIBUTING.md` ("Development setup": Flutter from the root `pubspec.yaml`).
- `apps/README.md`, `apps/client/README.md`, `apps/console/README.md`, `packages/README.md` (English: status and layout).
- `docs/DECISIONS.md` section 10: rows `D-M4-01-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- Code inside the members beyond the skeleton: `api_client` generation (M4-02), tokens and theme (M4-04), localization (M4-06), domain entities (M4-08), login (M4-09), device keys (M4-10, M4-11), local database (M4-12), sync engine (M4-13 to M4-16), app shell and platform folders (M4-18), `admin_ui` screens (M4-25 onward), `terminal_ui` (M4-35), `bothy_viz` (M6-10 onward), console (M6-01).
- The lint rule "no direct API call from widgets" of ADR 0029 "Confirmation": M4-18 (the first widgets with data).
- Building and signing release artifacts (`release-build`, `release-apple`, `release-windows`): M7.
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-01-workspace-arch`; one writer per worktree.
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

## Design notes

1. **Workspace root and members.** Root `pubspec.yaml`: `name: bothy_workspace`, `publish_to: none`, `environment` with `sdk` as a caret range of the Dart version of the pinned Flutter release and `flutter` with the exact stable Flutter version `flutter --version` prints on the build host (at least the release 15 2 names), `workspace:` listing all fourteen members in the order of 15 3. Every member has `resolution: workspace`, `publish_to: none`, `version: 0.0.0`, the same `environment`, and only the dependencies the skeleton needs (`flutter` and `flutter_test` for Flutter members, `test` and `lints` or `flutter_lints` as dev dependencies). One `pubspec.lock` at the root, tracked; no member tracks its own lock file. Ruling for docs/DECISIONS.md: the Flutter pin lives in the root `pubspec.yaml`, CI reads it there (`flutter-version-file: pubspec.yaml`), and every member repeats the same exact `flutter` line, which `test_flutter_pins_agree` (M2-38) or the test of this ticket holds equal. If `packages/bothy_noise` exists (M2-38 merged): add it to the workspace, set `resolution: workspace`, delete its `pubspec.lock`, keep its code and tests unchanged, and move the CI Flutter pin from that package to the root file (adapt `tools/ci/tests/test_ci_toolchains.py` to the new source). If it does not exist: create the skeleton; M2-38 then joins the workspace by its own Design note 8.

2. **Architecture test `tools/arch-check`.** Python standard library plus PyYAML from the pinned `tools/` environment, run as `uv run --project tools --locked --quiet python tools/arch-check/arch_check.py`. `rules.yaml` is the single source of the rules: per member its kind (`dart`, `flutter`, `plugin`, `app`), its allowed member dependencies (every member that the diagram of 15 3.1 reaches from it, that is the edges and their transitive closure: `apps/client` may import `domain` because it reaches it through `sync_engine`, while `auth` may not import `local_store` because no path leads there), forbidden imports (`package:flutter/` and `dart:ui` for rule 1 and 7 members; for `bothy_noise` also `dart:io` and any package except `cryptography` with crypto code), and platform folders allowed (rule 4). Checks, each with a message that names the file, the line and the rule number of 15 3.1:
    - every `import` and `export` directive in `lib/`, `test/`, `integration_test/` and `bin/` of a member (parsed line by line with a regular expression that understands `import '...'`, `export "..."`, conditional imports `if (dart.library.io)` and multi-line directives) that names a `package:<member>/` that the member does not reach, `package:flutter` where forbidden, or any `../` path that leaves the member (rule 2);
    - every `dependencies` and `dev_dependencies` entry of a member `pubspec.yaml` that names another member that it does not reach (a `dev_dependency` on a member for tests follows the same reach);
    - rule 3 (`apps/console` without `sync_engine` and `local_store`, directly or through a member that depends on them), rule 6 (`bothy_viz` without `sync_engine` and `local_store`), rule 7 (`terminal_ui` without them; `bothy_noise` without Flutter and with `cryptography` as its only runtime dependency);
    - rule 4: a folder `ios`, `macos`, `android`, `windows`, `linux` or `web` with native sources (`*.swift`, `*.m`, `*.mm`, `*.kt`, `*.java`, `*.cpp`, `*.cc`, `*.h`) only in `packages/device_key` and `apps/client`; `apps/console/web` holds no native source;
    - every tracked `pubspec.yaml` under `apps/` and `packages/` is a member listed in `rules.yaml` and in the root `workspace:` list, and vice versa (a new package without rules fails).
    - Exit 0 with `arch-check: <n> members, <m> files, ok`, exit 1 with one line per finding.
    `make arch` runs it after the Go part when Dart members are tracked (`DART_ON`), otherwise prints `SKIP arch-dart ...` with the `skip` macro; the Go part stays unchanged. Ruling for docs/DECISIONS.md: the reach of the diagram is an allow list, because a deny list would let every new edge in silently; a new edge needs a change of 15 3.1 and `rules.yaml` in the same change.

3. **Analysis options.** Root `analysis_options.yaml`: `include: package:flutter_lints/flutter.yaml` is not usable for pure Dart members, so the root file holds only the `analyzer` and `linter` sections that all share: `strict-casts`, `strict-inference`, `strict-raw-types`, `errors: {public_member_api_docs: error}`, `exclude: ['**/*.g.dart', '**/*.freezed.dart', '**/*.drift.dart', '**/lib/src/generated/**', 'build/**']`, and the rules `public_member_api_docs`, `always_declare_return_types`, `avoid_print`, `prefer_final_locals`, `unawaited_futures`, `directives_ordering`. Each member includes `package:lints/recommended.yaml` (pure Dart) or `package:flutter_lints/flutter.yaml` (Flutter) and then the root rules by a relative `include` (Dart 3.6 allows a list under `include`; if the pinned SDK does not, copy the shared rules into each member and add a test that holds the copies equal). Ruling for docs/DECISIONS.md: generated code is excluded from the doc lint, the style lint and coverage, and stays in the compile check and `flutter analyze` errors (as M2-23 Design note 8 does for the other languages).

4. **Root Makefile.** `DART_PKGS` keeps discovering members by their `pubspec.yaml` (the root `pubspec.yaml` is not a member and is excluded explicitly). `fmt-check` formats only hand-written files: run `dart format --output=none --set-exit-if-changed` over the tracked `*.dart` files of a member minus the generated patterns of note 3 (generators format their output as part of their own generation, M4-02, M4-12). `lint` runs `flutter analyze --fatal-infos` once at the root (the workspace analyzes all members in one run) and `dart run custom_lint` per member that declares it. `test` keeps M2-38's rules per member (`flutter test --coverage`, a member with code and without `test/` fails). `coverage` removes the records of generated files (`SF:` paths matching the patterns of note 3) from each `coverage/lcov.info` before summing, with a small script `tools/ci/lcov_filter.py` (standard library, tested), then applies `COVERAGE_DART_MIN`; a member without `coverage/lcov.info` fails (M2-38). `osv` and `licenses` read the root `pubspec.lock` once instead of per member. `sonar-project.properties`: `sonar.sources` and `sonar.tests` gain `apps,packages` with the test globs, `sonar.dart.lcov.reportPaths` lists the lcov file of every member, and `sonar.exclusions` gains the generated patterns. Run every changed target once with `COMPONENTS=apps` and once with `COMPONENTS=server` (Dart lines print their SKIP lines) and fix what fails.

5. **API documentation in the site.** `tools/docs/dart_api_docs.py --out docs/api/dart`: for each member that is a package (not the apps), run `dart doc --output <out>/<member>` in the member; fail on a non-zero exit or on any line of the output that contains `warning:` or `error:`; write nothing outside `--out`. `make docs` runs it before `mkdocs build --strict` (with `DART_ON`; otherwise `SKIP dart-api-docs ...`), and the generated folder `docs/api/dart/` is in `.gitignore`, so MkDocs copies the HTML as static files into the site. `docs/reference/dart-api.md` (tracked, English) lists every package with a link to `../api/dart/<member>/index.html` and one sentence on its purpose from 15 3; `test_dart_api_docs.py` holds the list equal to the members in `tools/arch-check/rules.yaml`. `.github/workflows/docs.yml` installs Flutter from the root `pubspec.yaml`. Ruling for docs/DECISIONS.md: the HTML is generated per build and never tracked, so it cannot drift from the code. Verify that `make linkcheck` (tracked Markdown) and `mkdocs build --strict` stay green without loosening any validation setting; if MkDocs reports the links into the generated folder as unresolved, generate the folder before the build in both `make docs` and the Pages job and keep the strict setting.

6. **Platform tests and CI jobs.** The pure gates run on `ubuntu-24.04`; native code and desktop integration tests need macOS and Windows (ADR 0039 names `macos-15` and `windows-2025` for app builds). `tools/ci/apps_platform.py` (standard library): reads `platform-tests.json` of every member that has one (`{"macos": [["flutter", "test", "integration_test", "-d", "macos"]], "windows": [...]}`, argument lists without a shell), runs the lists for the host system (`sys.platform` `darwin` is `macos`, `win32` is `windows`) in the member folder in order and stops at the first failure; prints `SKIP apps-platform <member> <system>: other host` for the other systems and `SKIP apps-platform: no member with platform tests` when none exists. No member has the file after this ticket; M4-10 is the first. Root target `apps-platform` calls the script; it is not part of `gates` because it needs a desktop session. `.github/workflows/ci.yml`: jobs `apps-macos` (`runs-on: macos-15`) and `apps-windows` (`runs-on: windows-2025`), each with `needs: plan`, `if:` the `apps` component is selected, checkout, `subosito/flutter-action` with `flutter-version-file: pubspec.yaml` and cache, `astral-sh/setup-uv`, then `uv run --project tools --locked --quiet python tools/ci/apps_platform.py` with `shell: bash`; `timeout-minutes`, `permissions: contents: read`, actions pinned by SHA as every other job; `ci-ok` adds both to `needs` and treats `skipped` like the other component jobs. `make runners-check` and `make lint` (actionlint, zizmor) stay green. Ruling for docs/DECISIONS.md.

7. **Component selection.** `GATE_COMPONENTS["apps"]` already owns `apps/`, `packages/` and `design/`; add the root files `pubspec.yaml`, `pubspec.lock`, `analysis_options.yaml` and the prefix `tools/arch-check/` (a change there must run the Dart gates); keep every existing case of `test_components.py` green and add one per new entry.

8. **Skeleton content.** Each `lib/<name>.dart` starts with the SPDX header and a `library;` directive carrying the doc comment (purpose from 15 3, "Skeleton created by M4-01; filled by <ticket>"). Each first test asserts something real about the member that later tickets keep true (for example `domain`: the library imports without Flutter; a Flutter member: a widget test pumps an empty `SizedBox` from the package's test helper). The two apps show no text, so no l10n is needed yet. `device_key` is created as a federated plugin skeleton with `flutter create --template=plugin --platforms=macos,windows` reduced to the Dart interface package without native code (M4-10 adds the platform implementations); if the template produces native files, delete them so that rule 4 and the no-code state agree.

9. **Rulings for docs/DECISIONS.md**, one row each with date, decision, reason and place (`D-M4-01-<n>`): Flutter pin in the root `pubspec.yaml` (note 1); the `bothy_noise` takeover (note 1); the reach of the diagram as an allow list (note 2); generated code out of doc lint, style lint and coverage (note 3); one root `flutter analyze` (note 4); API documentation generated per build (note 5); platform tests outside `gates` in two CI jobs (note 6); component selection of the root files (note 7).

## Tests (write them first)

Repository tools (pytest, `tmp_path`, run from `tools/`):

- `arch-check/tests/test_arch_check.py`, each on a throwaway tree with fake members written by the test:
    - `test_clean_workspace_passes` (members along the edges of the diagram, including a transitive import such as `apps/client` importing `domain`);
    - `test_flutter_import_in_domain_fails` (`import 'package:flutter/widgets.dart';` in `packages/domain/lib/src/x.dart` gives a finding naming rule 1, file and line);
    - `test_dart_ui_import_in_sync_engine_fails`;
    - `test_console_depends_on_sync_engine_fails` (in `pubspec.yaml`, rule 3) and `test_console_imports_local_store_through_admin_ui_fails` (`admin_ui` gets a dependency on `local_store`: rule 3 by transitivity and the missing edge);
    - `test_dependency_not_reached_fails` (`auth` imports `local_store`);
    - `test_relative_import_leaving_member_fails` (`import '../../domain/lib/x.dart'`);
    - `test_conditional_and_multiline_imports_are_read`;
    - `test_native_source_outside_allowed_places_fails` (`packages/design_system/macos/Classes/x.swift`) and `test_native_source_in_device_key_passes`;
    - `test_bothy_noise_extra_dependency_fails` (a second runtime dependency) and `test_bothy_noise_dart_io_fails`;
    - `test_unlisted_member_fails` and `test_listed_member_missing_fails`;
    - `test_rules_cover_every_member_of_the_repository` (the real `rules.yaml` against the real tree);
    - `test_real_repository_passes` (the real tree, exit 0).
- `docs/tests/test_dart_api_docs.py`: `test_warning_line_fails` and `test_nonzero_exit_fails` (a fake `dart` on `PATH` that prints `warning: ...` or exits 1); `test_writes_only_below_out`; `test_page_lists_every_package` (the real `docs/reference/dart-api.md` against `rules.yaml`).
- `ci/tests/test_apps_platform.py`: `test_runs_commands_for_host_system` (fake `flutter` on `PATH` records its arguments), `test_prints_skip_for_other_system`, `test_stops_at_first_failure`, `test_no_member_prints_skip`, `test_rejects_string_command` (a command that is a string instead of a list fails the run).
- `ci/tests/test_lcov_filter.py`: generated records dropped by every pattern of note 3, hand-written records kept, the sum of `LF` and `LH` recomputed.
- `ci/tests/test_make_dart_workspace.py` (a temporary git repository with a copy of the root `Makefile`, fake `flutter`, `dart` and `uv` first on `PATH`, `MAKEFLAGS` and `MAKELEVEL` dropped, the pattern of `test_make_server_gates.py`): `test_arch_runs_dart_check_when_members_exist`; `test_arch_skips_dart_without_members`; `test_lint_runs_one_root_analyze`; `test_coverage_ignores_generated_files` (an lcov with a fully uncovered `*.g.dart` and a covered hand file passes); `test_member_without_tests_fails`; `test_docs_runs_dart_api_docs`; `test_apps_platform_not_in_gates`.
- `ci/tests/test_ci_toolchains.py` (extend if M2-38 created it, otherwise create): `test_flutter_version_file_is_root_pubspec` (jobs `gates`, `sonar`, `apps-macos`, `apps-windows`, and the docs workflow); `test_flutter_pins_agree` (root and every member: the same exact `environment.flutter`); `test_platform_jobs_feed_ci_ok`.
- `ci/tests/test_components.py`: new cases for the root `pubspec.yaml`, `pubspec.lock`, `analysis_options.yaml` and `tools/arch-check/rules.yaml` (each selects `apps`).

Dart (in the members, run by `make test`):

- one test per member as in Design note 8.
- Doc lint proof, as a pytest in `ci/tests/test_make_dart_workspace.py`: `test_public_symbol_without_doc_fails_lint` copies one real member into a temporary workspace, adds `int undocumented() => 1;` to its library and asserts that `flutter analyze --fatal-infos` fails naming `public_member_api_docs`; marked to run only when `flutter` is on `PATH` (the gate host has it; the test prints why it is skipped otherwise and the report states where it ran).

Commands that must pass, from the repository root:

- `flutter pub get` at the root (one resolution, one `pubspec.lock`).
- `cd tools && uv run --locked --quiet pytest arch-check/tests ci/tests/test_apps_platform.py ci/tests/test_lcov_filter.py ci/tests/test_make_dart_workspace.py ci/tests/test_ci_toolchains.py ci/tests/test_components.py docs/tests/test_dart_api_docs.py ci/tests/test_make_server_gates.py`
- `make arch`, `make fmt-check`, `make lint`, `make test`, `make coverage`, `make docs`, `make linkcheck`, `make licenses`, `make osv`, `make runners-check`
- `make apps-platform` (prints its SKIP line)
- `make check`

## Acceptance criteria

- [ ] `flutter pub get` in the root folder resolves all fourteen members with one `pubspec.lock`; every member has `publish_to: none`, `resolution: workspace`, tests and a README.
- [ ] `make arch` fails on a forbidden import (`package:flutter` in `domain`), on `sync_engine` or `local_store` in `apps/console` (directly or transitively), on a dependency the diagram does not reach, on native code outside the allowed places and on an unlisted member; it passes on the repository.
- [ ] A public symbol without a doc comment fails `make lint`; generated code is excluded from doc lint, style lint and coverage and stays in the compile check.
- [ ] `dart doc` runs per package without a warning, and `make docs` puts the result into the site; `docs/reference/dart-api.md` lists every package.
- [ ] CI installs Flutter from the root `pubspec.yaml`; the jobs `apps-macos` and `apps-windows` run `tools/ci/apps_platform.py` and feed `ci-ok`; `make runners-check` is green.
- [ ] The root files and `tools/arch-check/` select the `apps` gates.
- [ ] 15 3 and 3.1, 23 3, gates.md, toolchain.md and CONTRIBUTING.md describe the workspace, the extra rules and the gates; the rulings of Design note 9 are recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; docstring or doc comment on every public symbol (Python with ruff `D`, Dart with `public_member_api_docs`); comments explain why.
- READMEs of `tools/arch-check`, every member, `apps`, `apps/client`, `apps/console` and `packages` written or updated (layout, allowed dependencies, which ticket fills the member, how to run the platform tests).
- `docs/quality/gates.md`, `docs/quality/toolchain.md` and `CONTRIBUTING.md` updated; 15 and 23 changed in the same change; rulings of Design note 9 in `docs/DECISIONS.md` section 10.
- Root `CHANGELOG.md` entry under 'Unreleased' (workspace, architecture test, Dart API docs, platform jobs).
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. The pinned Flutter and Dart versions, the members created and whether `bothy_noise` was taken over or created; the CI run of `apps-macos` and `apps-windows` (SKIP lines expected).
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
