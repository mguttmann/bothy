# WCAG 2.2: evidence per success criterion

Status: built (M4 to M6). The goal of
[01 Product, section 8](../spec/01-product.md#8-accessibility) is WCAG 2.2 level AA in the apps
and the console, checked by tests. That section maps the goal to rules by topic, and the two
checklists ([desktop](accessibility-checklist.md), [phones](accessibility-mobile.md)) map the
rules to tests per screen. This page turns the view around: it lists every success criterion of
the levels A and AA and says what holds it today and what is missing. It is no conformance
report: a claim of conformance needs the manual passes of the checklists and a check of the
released apps, and no release exists yet (section "Conformance report" below).

The list is the one of the recommendation (https://www.w3.org/TR/WCAG22/, retrieved 2026-11-12
[verified]): the criteria of level A and of level AA, without 4.1.1 "Parsing", which the
recommendation marks as obsolete and removed. The criteria of level AAA are no goal; where the
design system names one (2.3.3, 2.4.13), it is a target of its own and has no row here.

## How to read this page

| State | Meaning |
|---|---|
| `tested` | A rule of 01 section 8 or of a checklist covers the criterion, and the named automated tests hold it on the named surfaces. The column "Open or reason" names what the tests do not reach. |
| `partly` | Tests hold a part of the criterion; the column "Open or reason" names the rest. |
| `open` | No rule or no automated evidence. The column "Open or reason" says what exists. |
| `not applicable` | The apps and the console have no content of the kind the criterion covers; the column "Open or reason" gives the reason. |

Surfaces: "desktop" is the app on macOS and Windows, "phones" the app on iOS and Android,
"console" the web console. The console shows the administration screens of `packages/admin_ui`
and the views of `packages/bothy_viz`, which are tested in their packages; its own pages (login,
step-up, the shell with the navigation) have no accessibility test:
`apps/console/test/semantics_test.dart` only holds that the semantics are on. Where a row says
"every screen test", it means the harnesses `dsAccessibilityTests` and
`dsPhoneAccessibilityTests` of `package:design_system/testing.dart`, which the checklists list
per screen; the row names one screen as an example.

The evidence names a test file and its test names as the checklists do. A test
(`tools/docs/tests/test_wcag_criteria.py`) fails when a criterion of the recommendation has no
row here, when a row of the state `tested` or `partly` names no test, when a named test does not
exist, or when a row of another state gives no reason. A row changes its state in the change
that adds or removes its evidence.

## 1. Perceivable

| Criterion | Level | State | Evidence | Open or reason |
|---|---|---|---|---|
| 1.1.1 Non-text Content | A | tested | An icon has a name or is marked as decoration, and the audit of every screen test reports an image without a name: `packages/design_system/test/ds_icon_test.dart`: "an icon without label and not decorative throws"; "a decorative icon is excluded from semantics"; `packages/design_system/test/testing/semantics_audit_test.dart`: "an unlabelled image and an empty leaf are decorative findings"; previews: `apps/client/test/a11y/files_voice_semantics_test.dart`: "the image previews: roles, labels and states for screen readers" | The preview of an uploaded image is named by its file name; a person cannot give an image a description. The console's own pages. |
| 1.2.1 Audio-only and Video-only (Prerecorded) | A | not applicable | | The apps and the console ship no audio and no video of their own. Voice messages and uploaded media are content of the people of an instance; a voice message shows its transcript where the instance transcribes (`apps/client/test/voice/voice_test.dart`: "the transcript appears when file.updated reports it"). |
| 1.2.2 Captions (Prerecorded) | A | not applicable | | As 1.2.1: no video of the product; an uploaded video is content of a person and has no caption track. |
| 1.2.3 Audio Description or Media Alternative (Prerecorded) | A | not applicable | | As 1.2.1. |
| 1.2.4 Captions (Live) | AA | not applicable | | No live audio or video: voice and video channels are excluded for good (01 section 3.1, milestone "never"). |
| 1.2.5 Audio Description (Prerecorded) | AA | not applicable | | As 1.2.1. |
| 1.3.1 Info and Relationships | A | tested | Structure is in the semantics tree: the sidebar as a tree, the run graph as an outline, headings on the administration pages, a table for every chart: `apps/client/test/sidebar/space_tree_test.dart`: "tree semantics: expanded and collapsed spaces, channel labels"; `packages/bothy_viz/test/run_graph/semantics_test.dart`: "the outline has tree semantics with the values of the rows"; `packages/admin_ui/test/status/accessibility_test.dart`: "labelled controls, contrast, headings and the state in words"; `packages/admin_ui/test/stats/stats_screen_test.dart`: "every chart has a table with the same values" | Headings in the chat views and in the settings have no rule. The console's own pages. |
| 1.3.2 Meaningful Sequence | A | partly | Phones: the semantics snapshot of every screen holds the reading order: `apps/client/test/a11y/phone/m1_spaces_a11y_test.dart`: "M1 Spaces on the phone: roles, labels and states for screen readers". Desktop: `apps/client/test/keyboard/column_focus_test.dart`: "Tab goes on in reading order inside a column" | Desktop and console: no test reads the order of the whole semantics tree. |
| 1.3.3 Sensory Characteristics | A | open | | No rule and no test. No text of the interface is checked for instructions that rely on shape, color, size or position alone. |
| 1.3.4 Orientation | AA | open | | No test turns a screen. The phone apps lock no orientation (portrait and landscape in `apps/client/ios/Runner/Info.plist`, no `screenOrientation` in the Android manifest). |
| 1.3.5 Identify Input Purpose | AA | partly | Sign-in and step-up fields say what they collect: `packages/admin_ui/test/self/step_up_dialog_test.dart`: "asks for password and a 6-digit code and returns both"; `apps/client/test/phone/onboarding_semantics_test.dart`: "the TOTP field offers the one-time codes of the system" | The other fields that collect data about the person (display name, handle) have no rule. |
| 1.4.1 Use of Color | A | tested | States are words and icon shapes: `apps/client/test/a11y/sidebar_semantics_test.dart`: "the sidebar: roles, labels and states for screen readers"; `packages/bothy_viz/test/marks_semantics_test.dart`: "grouped series of one metric read differently, not by color alone"; "every run bar shows the icon of its state, not color alone" | A link inside a message is underlined; no test holds the underline. |
| 1.4.2 Audio Control | A | open | | No rule and no test. No code starts audio by itself today: a voice message plays after a press. |
| 1.4.3 Contrast (Minimum) | AA | tested | Every text pair of the tokens, in light and dark and with every accent, and the text of every screen test: `packages/design_system/test/theme/token_contrast_test.dart`: "every pair meets WCAG AA with the accent $accent"; `apps/client/test/a11y/sidebar_semantics_test.dart`: "the sidebar: contrast, labelled targets and target size in light and dark"; the gate `make tokens` ([contrast.md](../design/contrast.md)) | The measured text contrast of a screen runs on macOS hosts only (D-M4-07-10). The system settings for more contrast are not followed (AUD-CA1/7). |
| 1.4.4 Resize Text | AA | tested | Every screen test at text scale 2 in German and English: `apps/client/test/a11y/sidebar_semantics_test.dart`: "the sidebar at text scale 2 in de and en: no overflow, every control reachable"; `apps/client/test/phone/text_scaling_test.dart`: "the pages of the tabs, search and settings at text scale 2" | The console in a browser zoomed to 200 percent. |
| 1.4.5 Images of Text | AA | open | | No rule and no test. The interface draws its text as text and its icons from an icon font; nothing checks that no image of text is added. |
| 1.4.10 Reflow | AA | partly | A banner with an action stacks instead of squeezing its text; the new chat view scrolls: `packages/design_system/test/components/banner_text_scale_test.dart`: "the text of a banner with an action keeps at least half its width at text scale $scale"; `apps/client/test/forum/new_chat_layout_test.dart`: "at text scale 2 in a short window the view scrolls instead of overflowing" | No test lays out the console at a width of 320 CSS pixels or the desktop app in its smallest window. |
| 1.4.11 Non-text Contrast | AA | tested | The edge that identifies a control and the `ui` pairs of the tokens: `packages/design_system/test/theme/control_outline_contrast_test.dart`: "the outline role is the control border, the variant stays decoration"; "a text field shows its underline"; `packages/design_system/test/theme/token_contrast_test.dart`: "every pair meets WCAG AA with the accent $accent" | The edges are measured for the controls the test names, not on every screen. |
| 1.4.12 Text Spacing | AA | open | | No rule and no test. The apps follow the text settings of the system; whether a person can override line, paragraph, letter and word spacing in the console was not checked. |
| 1.4.13 Content on Hover or Focus | AA | open | | No rule and no test for tooltips (dismiss without moving, hover over the content, stay until dismissed). |

## 2. Operable

| Criterion | Level | State | Evidence | Open or reason |
|---|---|---|---|---|
| 2.1.1 Keyboard | A | partly | Desktop: a pass through the main view and the sidebar with the keyboard alone, the column borders with the arrow keys: `apps/client/integration_test/keyboard_walkthrough_test.dart`: "a keyboard pass through the main view"; `apps/client/integration_test/sidebar_keyboard_test.dart`: "a keyboard pass through spaces, channels and thread rows"; `apps/client/test/app/column_widths_test.dart`: "the arrow keys move the border that has the keyboard focus; one key press is one report" | The two passes cover the main view and the sidebar; the settings, the administration and the console have none. Phones with a hardware keyboard or a switch are a manual pass (row "Switch and keyboard" of the phone checklist). |
| 2.1.2 No Keyboard Trap | A | partly | The terminal takes every plain key, and every app shortcut keeps the platform modifier, so the keyboard leaves it: `apps/client/test/terminal/terminal_shortcuts_spec_test.dart`: "every stroke carries the platform modifier, so plain keys stay with the shell" | No test moves the focus into and out of every dialog, menu and embedded view. |
| 2.1.4 Character Key Shortcuts | A | tested | No shortcut is a single character key: every row of the shortcut table has a modifier or a non-character key, and no plain key is taken from a text field: `apps/client/test/keyboard/bindings_spec_test.dart`: "every row equals the table of 01 7 on macOS and Windows"; "the app layer binds no plain key a text field needs" | Held for the shortcut table of the desktop app; no test covers the key bindings of the console. |
| 2.2.1 Timing Adjustable | A | partly | The banner of a hint stays for a person who uses a screen reader: `apps/client/test/notifications/in_app_banner_test.dart`: "with a screen reader the banner stays until it is dismissed" | The time of an enrollment or pairing code, the undo window after a deletion and the idle end of a console session have no rule against this criterion. |
| 2.2.2 Pause, Stop, Hide | A | partly | Reduced motion stops every animation: `packages/bothy_viz/test/views_reduced_motion_test.dart`: "${kind.name} $state runs no animation in the reduced context"; `packages/design_system/test/ds_icon_test.dart`: "switching to reduced motion stops the spin" | The live box and the live views update by themselves, and the TV mode rotates without a pause (a key press ends it); only the timeline has play and pause. |
| 2.3.1 Three Flashes or Below Threshold | A | open | | No rule and no test. No component flashes by design. |
| 2.4.1 Bypass Blocks | A | partly | Desktop: F6 jumps from column to column: `apps/client/test/keyboard/column_focus_test.dart`: "F6 and Shift+F6 cycle through the four columns" | The console has no tested way past its navigation. |
| 2.4.2 Page Titled | A | open | | No test. The browser tab of the console carries one title for every page (`consoleTitle` with the product name), so the title does not name the current page. |
| 2.4.3 Focus Order | A | partly | Desktop: `apps/client/test/keyboard/column_focus_test.dart`: "Tab goes on in reading order inside a column"; "collapsed columns are skipped". Phones: the order of the semantics snapshot of every screen: `apps/client/test/a11y/phone/m1_spaces_a11y_test.dart`: "M1 Spaces on the phone: roles, labels and states for screen readers" | The focus order inside dialogs and of the console. |
| 2.4.4 Link Purpose (In Context) | A | partly | Every control, a link included, has a name: `packages/design_system/test/testing/semantics_audit_test.dart`: "a control without a name is a label finding" | No rule that the name of a link says where it leads; links inside message text read their text. |
| 2.4.5 Multiple Ways | AA | open | | No rule. The apps reach a thread through the sidebar, the search and the quick switcher; nothing holds a second way for the pages of the console. |
| 2.4.6 Headings and Labels | AA | partly | Administration pages: `packages/admin_ui/test/hosts/accessibility_test.dart`: "the host page: labelled controls, contrast, a heading"; `packages/admin_ui/test/status/accessibility_test.dart`: "labelled controls, contrast, headings and the state in words" | Headings of the other screens; whether a label describes its field is a manual pass. |
| 2.4.7 Focus Visible | AA | tested | Desktop: `packages/design_system/test/theme/focus_ring_test.dart`: "IconButton draws the focus ring in light"; "Switch draws the focus ring in dark"; `apps/client/test/a11y/app_semantics_test.dart`: "the app draws the focus ring for every control"; `apps/client/test/keyboard/column_focus_test.dart`: "the focus ring is painted around the focused item" | No test of the console holds it. |
| 2.4.11 Focus Not Obscured (Minimum) | AA | open | | No rule and no test that a pinned live box, a banner or the composer never covers the focused control completely. |
| 2.5.1 Pointer Gestures | A | open | | No rule. Recording a voice message has a tap mode while a screen reader is on (`apps/client/test/phone/compose/voice_button_a11y_test.dart`: "a tap records, a second tap stops, then Send or Discard"); no test holds a single-tap way for the other gestures. |
| 2.5.2 Pointer Cancellation | A | open | | No rule and no test. The controls of the framework act when the pointer is released. |
| 2.5.3 Label in Name | A | open | | No rule and no test compares the visible text of a control with its name. The audit holds that a name is read once (`repeated`). |
| 2.5.4 Motion Actuation | A | not applicable | | No function is operated by moving the device: no app and no package reads a motion sensor. |
| 2.5.7 Dragging Movements | AA | partly | The order of the sidebar also changes without dragging, through "Move up" and "Move down" of the menu: `apps/client/test/sidebar/sidebar_drag_test.dart`: "keyboard: the channel menu moves the channel one place down and up; the ends are disabled"; "keyboard: the post menu moves a post; the space menu a space" | A column border moves by dragging or with the arrow keys (2.1.1); it has no way for a single pointer without dragging. The other views were not checked for dragging. |
| 2.5.8 Target Size (Minimum) | AA | tested | Every screen test: `density.target-min` on the desktop, the platform sizes on the phones: `apps/client/test/a11y/sidebar_semantics_test.dart`: "the sidebar: contrast, labelled targets and target size in light and dark"; `apps/client/test/phone/tap_target_test.dart`: "the Me page and the tab bar meet the tap target guidelines" | The console's own pages. |

## 3. Understandable

| Criterion | Level | State | Evidence | Open or reason |
|---|---|---|---|---|
| 3.1.1 Language of Page | A | partly | Apps: `apps/client/test/a11y/app_semantics_test.dart`: "the language of the app is set in the semantics tree: de from the system"; "the language of the app is set in the semantics tree: en from the system"; `apps/client/test/a11y/phone/m1_spaces_a11y_test.dart`: "M1 Spaces on the phone: the semantics tree speaks the language of the app in de and en" | `apps/console/web/index.html` declares German; no test holds that the language of the page follows the language of the person. |
| 3.1.2 Language of Parts | AA | open | | By rule of 01 section 8 a text of an agent or a person without a language tag is not switched, so a message in another language is read in the language of the app. |
| 3.2.1 On Focus | A | open | | No rule and no test that moving the focus alone never changes the context. |
| 3.2.2 On Input | A | open | | No rule and no test that changing a setting alone never changes the context. |
| 3.2.3 Consistent Navigation | AA | open | | No rule and no test. The navigation of every surface is one component (sidebar, tab bar, console navigation). |
| 3.2.4 Consistent Identification | AA | open | | No rule and no test. Controls with the same function come from one component of the design system. |
| 3.2.6 Consistent Help | A | not applicable | | The apps and the console have no help mechanism of the kinds the criterion names (contact details, a contact form, a chat for help, a help page); the section "About" of the settings shows versions and licenses. |
| 3.3.1 Error Identification | A | partly | An error is text beside its field and is announced: `packages/admin_ui/test/hosts/accessibility_test.dart`: "the pairing wizard: labelled fields, an announced mismatch"; `packages/admin_ui/test/self/step_up_dialog_test.dart`: "a rejected attempt shows the error line" | No rule for every form; the forms of the chat views and of the settings have no such test. |
| 3.3.2 Labels or Instructions | A | tested | Every field has a name of its own: `packages/design_system/test/components/dropdown_field_test.dart`: "a named text field without a node below it"; `apps/client/test/a11y/onboarding_semantics_test.dart`: "onboarding e1_idle: roles, labels and states for screen readers" | The console's own pages. |
| 3.3.3 Error Suggestion | AA | open | | No rule and no test that an error names a correction where one is known. |
| 3.3.4 Error Prevention (Legal, Financial, Data) | AA | partly | A deletion can be undone inside its window: `apps/client/test/delete/delete_flow_test.dart`: "names the item, ends with the window and Undo restores" | The other changes of data a person controls have no rule against this criterion. |
| 3.3.7 Redundant Entry | A | open | | No rule and no test for the steps of onboarding, pairing and the wizards. |
| 3.3.8 Accessible Authentication (Minimum) | AA | open | | No rule and no test. Signing in offers a passkey or an identity provider next to a password with a one-time code, and the fields for both take the suggestions of the system (1.3.5). |

## 4. Robust

| Criterion | Level | State | Evidence | Open or reason |
|---|---|---|---|---|
| 4.1.2 Name, Role, Value | A | tested | The audit of every screen test (name, role, state, a button a screen reader can press), the tree the platforms receive, and the platform audits of the phones: `packages/design_system/test/testing/semantics_audit_test.dart`: "a control without a name is a label finding"; "a tappable node without a role is a role finding"; "an enabled button without a tap is an action finding"; `apps/client/integration_test/a11y_platform_tree_test.dart`: "VoiceOver and Narrator receive names and roles of the views"; `apps/client/ios/RunnerUITests/AccessibilityAuditTests.swift`: "testM1Spaces" | The console's own pages; what a browser exposes of the console to NVDA and VoiceOver is a manual pass. |
| 4.1.3 Status Messages | AA | tested | Only state changes and questions are announced, at most once per interval: `apps/client/test/agents/announcer_test.dart`: "only state changes and questions are announced, merged within the interval"; `packages/admin_ui/test/status/accessibility_test.dart`: "Server unreachable is announced"; `packages/bothy_viz/test/run_graph/semantics_test.dart`: "announcements only on node end and state change, rate-limited" | The count of runs on the phone changes without an announcement, as a state (phone checklist, M1). |

## Conformance report

No conformance report exists (an Accessibility Conformance Report after the VPAT form, or a
statement after EN 301 549), and none is promised for a date. It makes sense with the first
release, because it describes released apps on named platforms with named assistive tools. It
is then derived from this page: every row of the state `tested` with its manual pass from the
checklists, every row `partly` or `open` either closed or named as not supported. Whether the
project publishes such a report, and in which form, is a decision of the owner
([decisions.d/AUD-docs-spec-01.md](../decisions.d/AUD-docs-spec-01.md)).
