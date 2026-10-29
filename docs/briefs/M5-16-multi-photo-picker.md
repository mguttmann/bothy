# M5-16: Several photos from the library at once on the phone

Status: ready | Depends on: M5-07 | Milestone: M5

## Goal

After this ticket a person on iOS or Android can select several photos in the system photo picker at once ("Photo library" in the attachment menu of the phone composer): every selected photo becomes one attachment of the same message, in the order the picker returns them, up to `message_max_attachments`. The camera still takes one photo. Desktop (macOS, Windows) is unchanged: the open dialog, drag and drop and paste already take several files. Why: before this ticket `MediaPickerPort` returned one photo per opening (`ImagePicker.pickImage`), so a person who wanted to send four screenshots opened the menu four times, while server and runner already deliver all images of one message to an agent.

## Read first

- `docs/PLAN.md`: section 11 rows M5-07 and M5-16.
- Dependencies: the header names M5-07 (camera and photo library on the phone); everything below it is implied.
- `docs/spec/15-client-architecture.md` 14 (file upload, JPEG for photos from the library, preview before sending); `docs/spec/12-files.md` 7.1; ADR 0047.
- `docs/briefs/M5-07-voice-camera.md` Design note 4; `docs/decisions.d/M5-07.md` (D-M5-07-4, D-M5-07-5).
- Code on the base branch: `apps/client/lib/phone/compose/media_picker.dart` (`MediaPickerPort`, `ImagePickerPort`, `MediaPicker`, `attachPhoto`), `apps/client/lib/files/attach_controller.dart` (`AttachController.add`, `TooManyFiles`), `apps/client/lib/composer/thread_composer.dart` (attachment menu of the phone, chips).

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
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them. Use a hyphen or rephrase.
8. White label: test data uses `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. Interface texts come from `packages/l10n`.
9. Limits only through the instance limits (`message_max_attachments`, `file_max_upload_bytes` of `getInstance`), never a literal in code; no new configuration key.
10. No GUI test on a maintainer's own machine; widget tests run headless, device tests run where the CI runs them.

Product requirements implemented here:

11. The library entry opens the system photo picker for several photos (`PHPickerViewController` with a selection limit on iOS, the Android photo picker with `EXTRA_PICK_IMAGES_MAX`), limited to the free attachment slots (`message_max_attachments` minus the attachments already in the composer). The camera stays one photo.
12. With no free slot the entry shows the existing `TooManyFiles` refusal and does not open the picker. If a platform ignores the limit, `AttachController.add` refuses the whole set as before.
13. Each picked photo is converted as in M5-07 (JPEG kept, other decodable images encoded as JPEG, HEIC refused), in picker order; all or nothing: one photo that cannot be converted or read refuses the whole set with the reason naming that file. Every picker temporary file and every JPEG already written for a refused set is deleted.
14. All photos of one pick go to `AttachController.add` in one call.
15. A denied permission, a cancel and a failing picker keep their outcomes and texts.

## Scope

- `apps/client/lib/phone/compose/media_picker.dart`: the port with a camera call (one photo) and a library call (a list with a limit), the conversion of a list, the outcome with a list, `attachPhoto` with the free slots.
- `apps/client/lib/files/attach_controller.dart`: the free slots and the refusal without opening a picker.
- `apps/client/lib/composer/thread_composer.dart`: the outcome without room; the chips of up to `message_max_attachments` attachments stay within the phone screen.
- Tests under `apps/client/test/phone/compose/`, the fake port in `apps/client/test/support/file_fakes.dart`, the device test `apps/client/integration_test/phone/voice_test.dart`.

Fragments: `docs/decisions.d/M5-16.md` (`D-M5-16-<n>`), changelog fragment `changelog.d/M5-16.md`; README of `apps/client`; `docs/spec/15-client-architecture.md` 14; a note in `docs/briefs/M5-07-voice-camera.md` Design note 4.

## Out of scope

- The upload rate limit (`ratelimit.rl.upload.burst` and three requests per file): the outbox already retries `429` with `Retry-After`, a batch of ten only waits.
- The runner prompt (a sentence that tells the agent to open image attachments).
- Several photos from the camera in one opening; video from the library.
- The l10n label "Photo library" ("Fotomediathek") names no count and stays.

## Boundaries

- Work in a dedicated git worktree on its own branch; one writer per worktree.
- Conventional Commits with the package or component as scope and a DCO sign-off on every commit (`git commit -s`).
- Never push to `main`, never tag; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the fragments and READMEs named in this brief.
- Tests bind no port and make no network call; the system photo picker is a fake port in widget and device tests.
- Never lower a threshold, disable a gate or delete a test.
- This brief was written together with the change, in the manner of D-W-100 and D-W-114. Where the merged code names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Port.** `MediaPickerPort.camera()` returns one photo or null; `MediaPickerPort.library(limit:)` returns a list, empty for a cancel. `ImagePickerPort` calls `pickMultiImage` with `photoJpegQuality`, no full metadata and the limit; `image_picker` itself opens the single picker for a limit of 1.
2. **All or nothing.** `MediaPicker` converts in picker order into the scratch folder with an index in the file name (two photos with the same name stay apart). On any refusal or failure it deletes the JPEGs it wrote; the picker files are deleted in every case.
3. **Free slots.** `AttachController.freeSlots` is `message_max_attachments` minus the attachments, null while the limits are unknown (no limit is passed then and the server checks). `AttachController.hasRoom` sets `TooManyFiles` without writing anything; `attachPhoto` then returns `PhotoNoRoom` and opens nothing, for the camera as for the library.
4. **Chips.** Up to ten chips with previews no longer fit a phone screen; the chip area scrolls within a quarter of the screen height.
5. **Rulings for `docs/decisions.d/M5-16.md`**: the port shape; all or nothing with the cleanup; the free slots and the camera without room; the chip area; the device test with the fake port.

## Tests (write them first)

- `apps/client/test/phone/compose/media_picker_test.dart` (group `MediaPicker`): three library files give three JPEG attachments in order; the limit passed to the port is the one given, null when unknown; one unsupported file refuses the set and deletes all temporary files; an unreadable file fails the set and deletes all; an empty pick is `PhotoCancelled`; the camera returns one photo and ignores the limit.
- Same file (group "in the phone composer"): a library pick of 4 photos shows 4 pending attachments and sends one message with 4 attachment keys; with 9 attachments present the picker gets limit 1; with 10 it does not open and the refusal shows; a set of 11 from a picker that ignores the limit is refused before the outbox and deleted.
- `apps/client/integration_test/phone/voice_test.dart`: three photos picked at once arrive at the fake server as one message with three JPEG attachments in order (run by `integration_test/phone_suite_test.dart` on the iOS simulator and the Android emulator in the CI).

Commands that must pass, from the repository root:

- `cd apps/client && flutter analyze --fatal-infos && flutter test`
- `make lint`, `make fmt-check`, `make docs`
- `make check` (in the pull request CI)

## Acceptance criteria

- [ ] The library entry picks several photos up to the free attachment slots; the camera takes one.
- [ ] One message carries all photos of one pick in picker order, or none of them with the reason.
- [ ] No picker or scratch file stays after a refused or failed pick.
- [ ] Spec 15 14, brief M5-07 note, README, rulings and changelog fragment updated; `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol; comments cite 15 14 or M5-16.
- Rulings in `docs/decisions.d/M5-16.md`; changelog fragment as in Scope.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`).
- Land the change as `AGENTS.md` ("How work lands on `main`") describes.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits, changed files, test results (the red run before the change and the green run after).
3. Review and red team verdicts and the findings fixed.
4. What is not proven (for example the real system picker, which no test drives).
5. Follow-ups with reasons and differences to the names of this brief.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
