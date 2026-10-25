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

The desktop app on macOS and Windows (01 section 8, row "Terminal"; screen D11).

| Rule of 01 section 8 | Where in the terminal view | Automated evidence |
|---|---|---|
| Every control has a name for screen readers | "New session", the tabs, "Detach", "End", "Reattach", the search field and its close button; every row of the group "Hosts" reads name, connection or "disabled" and the count of own sessions | `apps/client/test/terminal/accessibility_test.dart` ("the view of a host: labelled controls and contrast", "the group Hosts: every row has its name, state and count"; `labeledTapTargetGuideline`); `apps/client/test/terminal/hosts_group_test.dart` |
| Contrast of text | the view of a host and its empty state in the default theme; the renderer's theme from the tokens `color.terminal.*` | `accessibility_test.dart` (`textContrastGuideline`); `packages/terminal_ui/test/theme_test.dart`, `terminal_marks_test.dart` (cursor and selection pairings in light and dark) |
| The line at the cursor and, on command, the visible screen are readable; running output is not read aloud | the semantics layer of the screen | `packages/terminal_ui/test/semantics_test.dart` |
| The font size follows the system setting and can be set separately | Cmd+Plus and Cmd+Minus on macOS, Ctrl+Shift+Plus and Ctrl+Shift+Minus on Windows change the font of the view | `apps/client/test/terminal/terminal_shortcuts_test.dart` ("on Windows Ctrl+Shift detaches, sizes the font and opens a session") |
| Every key reaches the shell; app shortcuts only with the platform modifier | Ctrl+C, Ctrl+D, Ctrl+K, Ctrl+L, Alt and Esc go to the host; the table "In the terminal" with Cmd or Ctrl+Shift | `terminal_shortcuts_test.dart`, `terminal_shortcuts_spec_test.dart` |
| States are said in words, not by color alone | connected, connecting, detached with its cause, ended with its reason, blocked as a danger banner; a refused opening names the server's reason | `packages/terminal_ui/test/denial_reason_test.dart`; `apps/client/test/terminal/terminal_page_test.dart` ("a refused opening shows the reason the server gave") |

Manual pass: VoiceOver on macOS and Narrator on Windows read the line at the cursor, the
header (host, connection, login badge, fingerprint start, time left) and the banners; this runs
with the manual pass before a release (section below).

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
