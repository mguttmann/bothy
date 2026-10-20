# Accessibility checklist of the desktop app

Status: skeleton. The build tickets of milestone M4 fill this page (D-W-116): M4-32 writes the
sections for the chat, settings and administration screens A1 to A6 and the notification area,
M4-36 the section for the terminal view, and M4-37 the section for screen A8. The mobile apps add
their sections in M5. The rules come from [01 Product, section 8](../spec/01-product.md#8-accessibility);
each row names the automated evidence (test file and name) and the manual pass with VoiceOver on
macOS and Narrator on Windows, which runs before every release
([15 Client architecture, section 20](../spec/15-client-architecture.md#20-test-strategy)).

## Chat, settings and administration (M4-32)

To be written by M4-32.

## Terminal view (M4-36)

To be written by M4-36.

## Screen A8: hosts (M4-37)

| Rule of 01 section 8 | Where on A8 and in the pairing wizard | Automated evidence |
|---|---|---|
| Every control has a name for screen readers | buttons, choice chips, pickers and fields of the host page and the three pairing steps carry their text as label | `packages/admin_ui/test/hosts/accessibility_test.dart`: "the host page: labelled controls, contrast, a heading" and "the pairing wizard: labelled fields, an announced mismatch" (`labeledTapTargetGuideline`) |
| Contrast of text | host page and pairing step 3 in the default theme | same tests (`textContrastGuideline`); light and dark in the goldens `packages/admin_ui/test/goldens/hosts_golden_test.dart` |
| Headings structure the page | the host name is a heading; the wizard title is a heading | "the host page: labelled controls, contrast, a heading" |
| Changes are announced | a wrong fingerprint (`fingerprint_mismatch` with the attempts left), the fallback to pairing, an I6 violation and refused session users are live-region banners | "the pairing wizard: labelled fields, an announced mismatch"; `pairing_test.dart`, `i6_test.dart`, `session_users_test.dart` |
| Nothing is said by color alone | check results say "ok" or "failed" with the code next to the icon; "Not effective" is a badge with text | `doctor_results_test.dart`, `host_grants_test.dart` |
| Text scaling up to 200 percent | host page, pairing steps and check results at text scale 1 and 2 | `packages/admin_ui/test/goldens/hosts_golden_test.dart` |
| Codes are read character by character, never guessed | the pairing command is selectable text; the fingerprint field takes the typed characters with separators and any case | `pairing_test.dart` ("step 1 creates the host; step 2 shows the command and the time", "the right prefix confirms with the answers and the session users") |

Manual pass: VoiceOver on macOS and Narrator on Windows read the pairing command, the remaining
time and the fingerprint field; this runs with the manual pass before a release (section below).

## Manual pass before a release (M4-32)

To be written by M4-32.
