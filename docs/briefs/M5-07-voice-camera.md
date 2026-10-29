# M5-07: Voice recording, camera and photo

Status: ready | Depends on: M3-09, M4-22, M5-03 | Milestone: M5

## Goal

After this ticket the phone apps record voice messages and send photos (01 5.3 M5, 3.4; 15 13, 14; 12 8): pressing and holding the microphone in the input records AAC-LC in an MP4 container (`.m4a`, mono, `client.voice.bitrate`) into a temporary file with the package `record`; swiping left discards, swiping up locks the recording, releasing sends; the remaining time appears as text, a notice comes before `voice.max_duration` (delivered by `getInstance` as `voice_max_duration_ms`) runs out, and a recording that reaches the limit stops by itself, keeps what was recorded and tells the person why. Sending goes through the outbox like every file (15 7.5, 14): the voice file is an upload entry, then a message with `kind = voice` and exactly one attachment (12 8.1); the transcript appears when `file.updated` reports it, and "Transcribe" calls `file.transcribe` where the instance offers it (12 8.2). The camera takes a photo and the photo library picker selects photos; both are uploaded as JPEG (15 14, ADR 0047) and attached through the same outbox path. The microphone, camera and photo permissions are asked at first use with texts from `l10n`, and a denied permission explains how to change it. A device test proves that a recording over the limit stops with a notice. Why: voice and photos from the phone are core features of 01 3.4, and the server accepts only the formats and sizes of 12.

## Read first

- `docs/spec/01-product.md`: 3.4 (files and voice), 5.3 M3 (input with the microphone), M5 (record voice message: press and hold, swipe left discards, swipe up locks, release sends, notice before the limit), 5.4 (Android: the same), 6.6 (offline queue), 8 (accessibility: the recording controls need labels and a non-gesture alternative).
- `docs/spec/15-client-architecture.md`: 7.5 (outbox: uploads before the message), 13 "Voice recording" (all of it: `record` package, AAC-LC in MP4, mono, `client.voice.bitrate`, `voice.max_duration`, permission at first use, temporary file, upload with `kind = voice`, `file.transcribe`, playback speeds 1, 1.5 and 2, transcript from `file.updated`), 14 "File upload" (size check against `file.max_upload_bytes`, JPEG for photos from the library, preview locally before sending), 22 (`client.voice.bitrate`).
- `docs/spec/12-files.md`: 3 (upload flow), 7.1 (images: the server does not strip HEIC losslessly; JPEG from the library), 8 (voice messages: format, duration from `mvhd`, `voice_too_long`, transcript modes, `file.transcribe` and `transcription_unavailable`).
- `docs/api/openapi.yaml`: `createUpload`, `completeUpload`, `createMessage` (`kind`), `requestTranscription`, `getInstance` (`InstanceLimits.voice_max_duration_ms`, `file_max_upload_bytes`); event `file.updated` in `asyncapi.yaml`.
- ADRs: 0047 (remove image metadata losslessly), 0029 (outbox).
- `docs/DECISIONS.md`: D-W-100 and its extension to M5 and M6 (D-W-120), D-W-115; section 10 (format of `D-M5-07-<n>` rows).
- Briefs (merged names win, D-W-100): the brief of M4-22 (desktop files, images and voice messages: upload entries in the outbox, `file_not_ready`, previews only from server files, the voice player and transcript display), the brief of M4-15 (outbox entries), `M3-09-voice-transcript.md` (transcript states, `file.transcribe`), `M5-02-iphone-layout.md` and `M5-03-android-layout.md` (phone input, runners, platform adaptivity).
- Package `record` (https://pub.dev/packages/record) at the version 15 13 names or the newest compatible one, with its license in the gate.

This ticket touches no API operation, no table, no action, no event and no server configuration key.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. Interface texts and permission usage descriptions come from `packages/l10n` (the Info.plist strings are generated per language like the strings of M5-04 Design note 6).
9. Limits only through named configuration values: `voice_max_duration_ms` and `file_max_upload_bytes` from `InstanceLimits` (never from `AppConfig`, D-W-115), `client.voice.bitrate` and `client.voice.limit_notice_lead` from `AppConfig`; code comments name the key, never the number.
10. Specification first: a change to behaviour updates `docs/spec/` in the same change. The core contract is not edited by this ticket.

Product requirements implemented here:

11. Voice: AAC-LC in MP4 (`audio/mp4`, `.m4a`), mono, bitrate `client.voice.bitrate`; at most `voice.max_duration` (from `voice_max_duration_ms`); the recording stops at the limit with a notice and is never sent longer than the limit (12 8.1, 15 13).
12. Gestures of M5 (press and hold, swipe left discard, swipe up lock, release send) and an accessible alternative without gestures (a button mode with record, stop, send, discard), because a press-and-hold gesture is not usable with every assistive technology (01 8).
13. Sending through the outbox only: the upload entry before the message, the message waits until the file is `complete`; offline the entry waits (15 7.5, 6.6).
14. Photos from the library and from the camera are uploaded as JPEG; other files unchanged (15 14, ADR 0047).
15. Permissions are requested at first use, never at start; a denied permission shows the way to the system settings.
16. Temporary recordings and photos are deleted after the outbox took them or after a discard; nothing stays in a shared or public location.

## Scope

Create:

- `apps/client/lib/phone/compose/voice_recorder.dart`: the recorder controller (states idle, recording, locked, stopping, discarded, ready; elapsed time from an injected clock; the limit; the notice threshold) over the voice recorder port that M4-22 built over `record` (extend it where the phone needs more, keep one port and one implementation; Design notes 1 and 2).
- `apps/client/lib/phone/compose/voice_button.dart`: the gesture widget and the accessible button mode (Design note 3).
- `apps/client/lib/phone/compose/media_picker.dart`: camera and library access behind a `MediaPickerPort` (implementation over the system pickers through `image_picker` or the platform APIs through Pigeon; choose by license and maintenance and record it) with JPEG conversion (Design note 4).
- Tests (Tests).

Change:

- `apps/client/lib` input of the phone shell (M5-02): the microphone and the attach menu (camera, photo library, file).
- The outbox entries of M4-15 and M4-22: a voice upload entry with `kind = voice` on its message; nothing new if M4-22 already supports it.
- iOS `Info.plist` (`NSMicrophoneUsageDescription`, `NSCameraUsageDescription`, and `NSPhotoLibraryUsageDescription` only if the chosen picker needs it; `PHPickerViewController` does not), localized per language; Android manifest `RECORD_AUDIO`, `CAMERA` (and no storage permission when the photo picker of the platform is used).
- `packages/l10n` ARB files: recording, limit notice, permission texts.
- Docs: `apps/client/README.md` (media permissions, how the recorder is tested), `docs/spec/15-client-architecture.md` 13 (the accessible button mode, the temporary file rule) and 22 (the key `client.voice.limit_notice_lead`), `docs/operations/configuration.md` (its row), root `CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M5-07-1` to `D-M5-07-5`.

## Out of scope

- Desktop recording (M4-22 covers files and voice playback on the desktop; recording on the desktop is not in the plan).
- Transcription itself (server, M3-09); video messages.
- Sharing from other apps (M5-06).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M5-07-voice-camera`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(client): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` with port 0 and use the fake server of M4-17; device tests use the simulator's or emulator's audio input and a camera image from the emulator, never a recording of a real person. Temporary directories only via `mktemp -d` (Dart `Directory.systemTemp.createTemp`), removed at the end. Stop only processes you started, by PID; simulators by UDID, emulators by serial.
- Never read or print secrets. No network calls except package downloads for pinned tools and pub packages.
- The device tests need macOS with Xcode and an iOS simulator, and an Android emulator; where missing, `make apps-platform` prints its SKIP line; the report names where they ran.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never exclude hand-written code from a lint, format, coverage or doc gate.
- No permission logic in the client (15 1 principle 1): whether the person may upload into a thread comes from the server (`403` shows its reason).
- If the core contract or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M5 and M6 by D-W-120). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Recorder** (15 13; 12 8.1; Ruling for docs/DECISIONS.md). `record` with `AudioEncoder.aacLc`, one channel, the bitrate of `client.voice.bitrate`, the sample rate the package default for AAC-LC, output to a file in the app's temporary directory. The controller arms a timer at `voice_max_duration_ms` minus `client.voice.limit_notice_lead` (a new client key in `AppConfig`, added to 15 22 and the configuration reference first, so the lead is never a number in code) and stops a margin before the limit (one encoder frame plus the container overhead, computed from the bitrate), so that the `mvhd` duration the server reads never exceeds `voice.max_duration` (12 8.1, `voice_too_long`); the stopped file is the recording. The device test asserts the `mvhd` duration of a recording that hit the limit is at most the limit. Elapsed time comes from an injected clock so the tests run without waiting.

2. **States** (01 5.3 M5). Idle, recording (finger down), locked (after the swipe up; buttons stop and send appear), stopping, ready (the file exists and is under the limit), discarded (swipe left or discard button: the file is deleted at once). Release in recording sends; stop in locked shows a preview with play, send and discard. A phone call or the app going to the background stops the recording and keeps it as locked (the person decides), never sends by itself.

3. **Accessibility** (01 8; Ruling for docs/DECISIONS.md). When an accessibility service is active (`MediaQuery.accessibleNavigation`) or the person turns it on in settings, the microphone shows a button mode: tap to record, tap to stop, then send or discard; the remaining time is a live region announced at most once per `a11y.live_announce_min_interval`; the limit notice is announced.

4. **Photos** (15 14; 12 7.1; ADR 0047; Ruling for docs/DECISIONS.md). Library: the system photo picker (`PHPickerViewController` on iOS, the Android photo picker), requesting a JPEG representation (on iOS `NSItemProvider` with `public.jpeg` or conversion from HEIC with `CGImageDestination`; on Android decode and encode to JPEG at the quality you document); camera: capture as JPEG directly. Check the size against `file_max_upload_bytes` before the outbox (the server checks again, `413`). The local preview before sending uses the temporary file; after sending previews come from the server's thumbnails (15 14). Note (M5-16): the library picker is multi-select up to `message_max_attachments` (the free slots of the message), and all photos of one pick are attached together; the camera stays one photo.

5. **Rulings for `docs/DECISIONS.md`** (`D-M5-07-<n>`): recorder parameters and the notice lead (note 1); background behaviour of a running recording (note 2); the accessible button mode (note 3); the picker choice and JPEG conversion (note 4); temporary file deletion (requirement 16).

## Tests (write them first)

Widget and unit (`apps/client/test/phone/compose/`, fake recorder and picker, fake clock):

- `voice_recorder_test.dart`: press, hold and release sends one outbox entry pair (upload, then a message with `kind = voice`); swipe left discards and deletes the file; swipe up locks; the notice appears at the lead before `voice_max_duration_ms`; at the limit the recording stops by itself with the notice and the state is ready (PLAN row); background stops and keeps as locked.
- `voice_button_a11y_test.dart`: button mode with labels; the live region announces at most once per interval; guidelines pass.
- `media_picker_test.dart`: library and camera results become JPEG files; a file over `file_max_upload_bytes` is rejected before the outbox with the reason.
- Goldens of the recording states (M5 screen) in light and dark, text scaling 1 and 2, de and en.

Device (`patrol`, `apps/client/integration_test/phone/voice_test.dart`, iOS simulator and Android emulator, fake server):

- the microphone permission dialog is shown at the first press and granted; a recording of a few seconds is sent and the fake server receives an `audio/mp4` upload whose `mvhd` duration matches; a recording with the instance limit set very low in the fake `getInstance` stops at the limit with the notice (PLAN row: a recording over the limit stops with a notice); a photo from the library is sent as JPEG.

Commands that must pass, from the repository root:

- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- on macOS with Xcode and with an attached Android emulator: `make apps-platform`; otherwise the SKIP lines
- `make codegen-check`, `make arch`, `make lint`, `make fmt-check`, `make coverage`, `make licenses`
- `make check`

## Acceptance criteria

- [ ] Device test: a recording over the limit stops with a notice; a normal recording arrives as AAC-LC in MP4 with `kind = voice`.
- [ ] Gestures of M5 and the accessible button mode work; the remaining time and the notice are announced within the interval rule.
- [ ] Photos from the library and the camera are uploaded as JPEG through the outbox; temporary files are removed.
- [ ] Permissions are requested at first use with localized texts.
- [ ] README, 15 13, ARB files, CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every public Dart symbol; comments explain why.
- `apps/client/README.md` updated in English.
- Rulings of Design note 5 recorded in `docs/DECISIONS.md` section 10.
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
4. Where the device tests ran; the version and license of `record` and of the picker.
5. What is not proven (for example recording quality on physical devices).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
