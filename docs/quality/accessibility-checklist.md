# Accessibility checklist of the desktop app

Status: M4-32 and M4-37 filled. The build tickets of milestone M4 fill this page (D-W-116): M4-32 writes the
sections for the chat, settings and administration screens A1 to A6 and the notification area,
M4-36 the section for the terminal view, and M4-37 the section for screen A8. The phone apps have
their own page, [accessibility-mobile.md](accessibility-mobile.md) (M5-10). The rules come from [01 Product, section 8](../spec/01-product.md#8-accessibility);
each row names the automated evidence (test file and name) and the manual pass with VoiceOver on
macOS and Narrator on Windows, which runs before every release
([15 Client architecture, section 20](../spec/15-client-architecture.md#20-test-strategy)).

## Chat, settings and administration (M4-32)

The automated evidence runs in `flutter test` of `apps/client`, `packages/design_system` and
`packages/admin_ui` on every pull request, and the platform tree test runs in the CI jobs
`apps (macOS)` and `apps (Windows)` (`make apps-platform`). Every screen test uses the shared
harness `dsAccessibilityTests` of `package:design_system/testing.dart`, which registers three
tests per screen: roles, labels and states for screen readers (the audit `dsSemanticsProblems`:
no control without a name, no tappable node without a role, no enabled button a screen reader
cannot press, no empty or decorative node, no name read twice); contrast, labelled targets and
the desktop target size of `density.target-min` in light and dark (text contrast on macOS only,
D-M4-07-10); and text scale 2 in German and English without overflow with every control still
reachable. The column "Manual pass" is ticked in the release checklist of the section below.

### Rules for every screen

| Rule of 01 section 8 | Where | Platform | Automated evidence | Manual pass |
|---|---|---|---|---|
| Focus order per column, F6 between columns | sidebar, post list, history, input, session column | macOS, Windows | `apps/client/test/keyboard/column_focus_test.dart`: "F6 and Shift+F6 cycle through the four columns"; "Tab goes on in reading order inside a column"; `apps/client/integration_test/keyboard_walkthrough_test.dart`: "a keyboard pass through the main view" | VoiceOver, Narrator |
| Focus ring always visible | every control: `DsFocusRing` draws the ring for controls without one of their own | macOS, Windows | `packages/design_system/test/theme/focus_ring_test.dart`: "IconButton draws the focus ring in light"; "Switch draws the focus ring in dark"; `apps/client/test/a11y/app_semantics_test.dart`: "the app draws the focus ring for every control" | VoiceOver, Narrator |
| Reduced motion | the system setting and the own setting switch to `motion.reduced` | macOS, Windows | `packages/design_system/test/settings_resolver_test.dart`: "on gives reduced (platform $platform)"; "system follows the platform ($platform)" | by sight |
| Language of the app in the semantics tree | the application locale follows the system language until the person chooses one | macOS, Windows | `apps/client/test/a11y/app_semantics_test.dart`: "the language of the app is set in the semantics tree: de from the system"; "the language of the app is set in the semantics tree: en from the system" | VoiceOver, Narrator |
| What VoiceOver and Narrator receive | buttons, text fields and the live box of the thread view and the settings exist in the platform tree with their names and roles | macOS, Windows | `apps/client/integration_test/a11y_platform_tree_test.dart`: "VoiceOver and Narrator receive names and roles of the views" | VoiceOver, Narrator |
| Text from assistive tools | the composer and every text field: what dictation and text tools write through the macOS Accessibility API (selected text or value of the focused field) lands at the cursor as typing does | macOS | `apps/client/integration_test/composer_ax_insert_test.dart`: "text written through the Accessibility API reaches the composer"; "a write of another process through the Accessibility API reaches the composer" (skipped without `BOTHY_AX_CROSS_PROCESS=1` and the Accessibility permission, so in CI) | a dictation tool |
| Menus | every entry of a menu is a button; the chosen level says "selected" | macOS, Windows | `apps/client/test/a11y/channel_menu_semantics_test.dart`: "the channel menu: roles, labels and states for screen readers"; "the channel menu: contrast, labelled targets and target size in light"; `packages/design_system/test/components/menu_item_test.dart`: "entries are buttons, the chosen one is selected" | VoiceOver, Narrator |
| No control below a text field | a text field is a named node of its own; lists to choose from are `DsDropdownField`, menus `DsMenuItem` | macOS, Windows | `packages/design_system/test/components/dropdown_field_test.dart`: "a named text field without a node below it"; `apps/client/test/a11y/history_column_semantics_test.dart`: "the history column: the composer field is a node of its own"; `apps/client/test/a11y/design_system_controls_test.dart`: "menus and drop-down fields come from the design system" | VoiceOver, Narrator |

### Onboarding E1 to E7

| Rule of 01 section 8 | Where | Platform | Automated evidence | Manual pass |
|---|---|---|---|---|
| Every control has a role and a name | fields, buttons and links of E1 to E7 (every state of E1, E2 with and without a password, E3 to E7) | macOS, Windows | `apps/client/test/a11y/onboarding_semantics_test.dart`: "onboarding e1_idle: roles, labels and states for screen readers" | VoiceOver, Narrator |
| Contrast (text 4.5:1, controls 3:1) and targets of `density.target-min` | the screen in light and dark | macOS, Windows | `apps/client/test/a11y/onboarding_semantics_test.dart`: "onboarding e1_idle: contrast, labelled targets and target size in light and dark" | by sight |
| Text scaling to 200 percent | the screen at text scale 2 in German and English | macOS, Windows | `apps/client/test/a11y/onboarding_semantics_test.dart`: "onboarding e1_idle at text scale 2 in de and en: no overflow, every control reachable" | by sight |
| Codes are read character by character | the enrollment code of E4 and the pairing code of E5 | macOS, Windows | `apps/client/test/a11y/onboarding_semantics_test.dart`: "onboarding e4 and e5 codes: roles, labels and states for screen readers" | VoiceOver, Narrator |
| Changes are announced | the remaining time of E4 and E5, at most once per `a11y.live_announce_min_interval` | macOS, Windows | `apps/client/test/phone/onboarding_semantics_test.dart`: "the remaining time is announced at most once per interval" | VoiceOver, Narrator |

### Sidebar

| Rule of 01 section 8 | Where | Platform | Automated evidence | Manual pass |
|---|---|---|---|---|
| Every control has a role and a name | spaces as tree nodes with their expanded state, channels with unread, mentions, running agent and muted in words, the selected channel, Jump to, Running now, settings | macOS, Windows | `apps/client/test/a11y/sidebar_semantics_test.dart`: "the sidebar: roles, labels and states for screen readers" | VoiceOver, Narrator |
| Contrast (text 4.5:1, controls 3:1) and targets of `density.target-min` | the screen in light and dark | macOS, Windows | `apps/client/test/a11y/sidebar_semantics_test.dart`: "the sidebar: contrast, labelled targets and target size in light and dark" | by sight |
| Text scaling to 200 percent | the screen at text scale 2 in German and English | macOS, Windows | `apps/client/test/a11y/sidebar_semantics_test.dart`: "the sidebar at text scale 2 in de and en: no overflow, every control reachable" | by sight |
| Nothing is said by color alone | unread, mentions, run active and muted are words in the label of the row | macOS, Windows | `apps/client/test/a11y/sidebar_semantics_test.dart`: "the sidebar: roles, labels and states for screen readers" | VoiceOver, Narrator |

### Post list

| Rule of 01 section 8 | Where | Platform | Automated evidence | Manual pass |
|---|---|---|---|---|
| Every control has a role and a name | New post, the filter chips with their state, every post with title, state, time and unread count | macOS, Windows | `apps/client/test/a11y/post_list_semantics_test.dart`: "the post list: roles, labels and states for screen readers" | VoiceOver, Narrator |
| Contrast (text 4.5:1, controls 3:1) and targets of `density.target-min` | the screen in light and dark | macOS, Windows | `apps/client/test/a11y/post_list_semantics_test.dart`: "the post list: contrast, labelled targets and target size in light and dark" | by sight |
| Text scaling to 200 percent | the screen at text scale 2 in German and English | macOS, Windows | `apps/client/test/a11y/post_list_semantics_test.dart`: "the post list at text scale 2 in de and en: no overflow, every control reachable" | by sight |
| Nothing is said by color alone | "Waiting for you", the run state and the unread count are words; the badge is not read twice | macOS, Windows | `apps/client/test/a11y/post_list_semantics_test.dart`: "the post list: roles, labels and states for screen readers"; `packages/design_system/test/components/screen_reader_test.dart`: "a post waiting for the person says so once" | VoiceOver, Narrator |

### Thread view

| Rule of 01 section 8 | Where | Platform | Automated evidence | Manual pass |
|---|---|---|---|---|
| Every control has a role and a name | the header actions, every message with author, time and text, edited, deleted and failed messages in words, the pending rows | macOS, Windows | `apps/client/test/a11y/thread_semantics_test.dart`: "the thread view: roles, labels and states for screen readers" | VoiceOver, Narrator |
| Contrast (text 4.5:1, controls 3:1) and targets of `density.target-min` | the screen in light and dark | macOS, Windows | `apps/client/test/a11y/thread_semantics_test.dart`: "the thread view: contrast, labelled targets and target size in light and dark" | by sight |
| Text scaling to 200 percent | the screen at text scale 2 in German and English | macOS, Windows | `apps/client/test/a11y/thread_semantics_test.dart`: "the thread view at text scale 2 in de and en: no overflow, every control reachable" | by sight |
| Language | agent text without a language tag keeps the language of the app (no own locale in the tree) | macOS, Windows | `apps/client/test/a11y/thread_semantics_test.dart`: "the thread view: roles, labels and states for screen readers" | VoiceOver, Narrator |

### Composer

| Rule of 01 section 8 | Where | Platform | Automated evidence | Manual pass |
|---|---|---|---|---|
| Every control has a role and a name | the field with its name read once, Send, attach and voice, the offline, read-only and too-long states in words | macOS, Windows | `apps/client/test/a11y/composer_semantics_test.dart`: "the composer: roles, labels and states for screen readers" | VoiceOver, Narrator |
| Contrast (text 4.5:1, controls 3:1) and targets of `density.target-min` | the screen in light and dark | macOS, Windows | `apps/client/test/a11y/composer_semantics_test.dart`: "the composer: contrast, labelled targets and target size in light and dark" | by sight |
| Text scaling to 200 percent | the screen at text scale 2 in German and English | macOS, Windows | `apps/client/test/a11y/composer_semantics_test.dart`: "the composer at text scale 2 in de and en: no overflow, every control reachable" | by sight |
| Every name is read once | the field's name and its placeholder are one text | macOS, Windows | `packages/design_system/test/components/screen_reader_test.dart`: "the composer field reads its name once (offline false)"; "the composer field reads its name once (offline true)" | VoiceOver, Narrator |

### Live box

| Rule of 01 section 8 | Where | Platform | Automated evidence | Manual pass |
|---|---|---|---|---|
| Every control has a role and a name | the box is a container whose label summarizes agent, state, time and context; Stop names the agent and can be pressed | macOS, Windows | `apps/client/test/a11y/live_box_semantics_test.dart`: "the live box: roles, labels and states for screen readers" | VoiceOver, Narrator |
| Contrast (text 4.5:1, controls 3:1) and targets of `density.target-min` | the screen in light and dark | macOS, Windows | `apps/client/test/a11y/live_box_semantics_test.dart`: "the live box: contrast, labelled targets and target size in light and dark" | by sight |
| Text scaling to 200 percent | the screen at text scale 2 in German and English | macOS, Windows | `apps/client/test/a11y/live_box_semantics_test.dart`: "the live box at text scale 2 in de and en: no overflow, every control reachable" | by sight |
| Live content | only state changes and questions are announced, at most once per `a11y.live_announce_min_interval`, patches never | macOS, Windows | `packages/design_system/test/components/live_box_test.dart`: "the live box announces a state change, never a patch"; "the collapsed and the mobile line announce state changes too"; `apps/client/test/agents/announcer_test.dart`: "only state changes and questions are announced, merged within the interval" | VoiceOver, Narrator |
| Trigger state as its own label | received, queued, working, done, failed, follow-up delivered and canceled each have a label | macOS, Windows | `packages/design_system/test/components/trigger_state_test.dart`: "every trigger state has its icon, color token and label" | VoiceOver, Narrator |
| A button can be pressed by a screen reader | Stop and every button with its own label keep their tap action | macOS, Windows | `packages/design_system/test/components/tappable_semantics_test.dart`: "a labelled button can be pressed by a screen reader" | VoiceOver, Narrator |

### Cards

| Rule of 01 section 8 | Where | Platform | Automated evidence | Manual pass |
|---|---|---|---|---|
| Every control has a role and a name | every button of a card; not permitted, sent, used, expired, disabled and error in words | macOS, Windows | `apps/client/test/a11y/cards_semantics_test.dart`: "the cards: roles, labels and states for screen readers" | VoiceOver, Narrator |
| Contrast (text 4.5:1, controls 3:1) and targets of `density.target-min` | the screen in light and dark | macOS, Windows | `apps/client/test/a11y/cards_semantics_test.dart`: "the cards: contrast, labelled targets and target size in light and dark" | by sight |
| Text scaling to 200 percent | the screen at text scale 2 in German and English | macOS, Windows | `apps/client/test/a11y/cards_semantics_test.dart`: "the cards at text scale 2 in de and en: no overflow, every control reachable" | by sight |

### Commands

| Rule of 01 section 8 | Where | Platform | Automated evidence | Manual pass |
|---|---|---|---|---|
| Every control has a role and a name | the slash suggestions as options with description and owner, the follow-up form, the agent part of the session column | macOS, Windows | `apps/client/test/a11y/commands_semantics_test.dart`: "the commands and the session agent part: roles, labels and states for screen readers" | VoiceOver, Narrator |
| Contrast (text 4.5:1, controls 3:1) and targets of `density.target-min` | the screen in light and dark | macOS, Windows | `apps/client/test/a11y/commands_semantics_test.dart`: "the commands and the session agent part: contrast, labelled targets and target size in light and dark" | by sight |
| Text scaling to 200 percent | the screen at text scale 2 in German and English | macOS, Windows | `apps/client/test/a11y/commands_semantics_test.dart`: "the commands and the session agent part at text scale 2 in de and en: no overflow, every control reachable" | by sight |

### Files and voice

| Rule of 01 section 8 | Where | Platform | Automated evidence | Manual pass |
|---|---|---|---|---|
| Every control has a role and a name | file tiles with name, size and state read once, image previews with a label, voice messages with play and transcript | macOS, Windows | `apps/client/test/a11y/files_voice_semantics_test.dart`: "the file tiles: roles, labels and states for screen readers" | VoiceOver, Narrator |
| Contrast (text 4.5:1, controls 3:1) and targets of `density.target-min` | the screen in light and dark | macOS, Windows | `apps/client/test/a11y/files_voice_semantics_test.dart`: "the file tiles: contrast, labelled targets and target size in light and dark" | by sight |
| Text scaling to 200 percent | the screen at text scale 2 in German and English | macOS, Windows | `apps/client/test/a11y/files_voice_semantics_test.dart`: "the file tiles at text scale 2 in de and en: no overflow, every control reachable" | by sight |
| Images | previews of images and voice messages | macOS, Windows | `apps/client/test/a11y/files_voice_semantics_test.dart`: "the image previews: roles, labels and states for screen readers"; "the voice messages: roles, labels and states for screen readers" | VoiceOver, Narrator |

### Search

| Rule of 01 section 8 | Where | Platform | Automated evidence | Manual pass |
|---|---|---|---|---|
| Every control has a role and a name | the field, the filters and their state, every hit; the local-only note in words | macOS, Windows | `apps/client/test/a11y/search_semantics_test.dart`: "the search search_results: roles, labels and states for screen readers" | VoiceOver, Narrator |
| Contrast (text 4.5:1, controls 3:1) and targets of `density.target-min` | the screen in light and dark | macOS, Windows | `apps/client/test/a11y/search_semantics_test.dart`: "the search search_results: contrast, labelled targets and target size in light and dark" | by sight |
| Text scaling to 200 percent | the screen at text scale 2 in German and English | macOS, Windows | `apps/client/test/a11y/search_semantics_test.dart`: "the search search_results at text scale 2 in de and en: no overflow, every control reachable" | by sight |
| Local search | the results of the local index | macOS, Windows | `apps/client/test/a11y/search_semantics_test.dart`: "the search search_local: roles, labels and states for screen readers" | VoiceOver, Narrator |

### Quick switcher

| Rule of 01 section 8 | Where | Platform | Automated evidence | Manual pass |
|---|---|---|---|---|
| Every control has a role and a name | the field, every entry with kind and name, the filters, Close | macOS, Windows | `apps/client/test/a11y/switcher_semantics_test.dart`: "the quick switcher: roles, labels and states for screen readers" | VoiceOver, Narrator |
| Contrast (text 4.5:1, controls 3:1) and targets of `density.target-min` | the screen in light and dark | macOS, Windows | `apps/client/test/a11y/switcher_semantics_test.dart`: "the quick switcher: contrast, labelled targets and target size in light and dark" | by sight |
| Text scaling to 200 percent | the screen at text scale 2 in German and English | macOS, Windows | `apps/client/test/a11y/switcher_semantics_test.dart`: "the quick switcher at text scale 2 in de and en: no overflow, every control reachable" | by sight |

### Mentions

| Rule of 01 section 8 | Where | Platform | Automated evidence | Manual pass |
|---|---|---|---|---|
| Every control has a role and a name | every entry with who, where and the text | macOS, Windows | `apps/client/test/a11y/mentions_semantics_test.dart`: "the mentions view: roles, labels and states for screen readers" | VoiceOver, Narrator |
| Contrast (text 4.5:1, controls 3:1) and targets of `density.target-min` | the screen in light and dark | macOS, Windows | `apps/client/test/a11y/mentions_semantics_test.dart`: "the mentions view: contrast, labelled targets and target size in light and dark" | by sight |
| Text scaling to 200 percent | the screen at text scale 2 in German and English | macOS, Windows | `apps/client/test/a11y/mentions_semantics_test.dart`: "the mentions view at text scale 2 in de and en: no overflow, every control reachable" | by sight |

### Running now

| Rule of 01 section 8 | Where | Platform | Automated evidence | Manual pass |
|---|---|---|---|---|
| Every control has a role and a name | every run with agent, place, state, duration, context and cost in words; the filters with their choice | macOS, Windows | `apps/client/test/a11y/running_now_semantics_test.dart`: "Running now: roles, labels and states for screen readers" | VoiceOver, Narrator |
| Contrast (text 4.5:1, controls 3:1) and targets of `density.target-min` | the screen in light and dark | macOS, Windows | `apps/client/test/a11y/running_now_semantics_test.dart`: "Running now: contrast, labelled targets and target size in light and dark" | by sight |
| Text scaling to 200 percent | the screen at text scale 2 in German and English | macOS, Windows | `apps/client/test/a11y/running_now_semantics_test.dart`: "Running now at text scale 2 in de and en: no overflow, every control reachable" | by sight |

### Settings, devices and sessions

| Rule of 01 section 8 | Where | Platform | Automated evidence | Manual pass |
|---|---|---|---|---|
| Every control has a role and a name | every section; a switch or a drop-down is named by its row; devices and sessions with their state | macOS, Windows | `apps/client/test/a11y/settings_semantics_test.dart`: "the settings section devices: roles, labels and states for screen readers" | VoiceOver, Narrator |
| Contrast (text 4.5:1, controls 3:1) and targets of `density.target-min` | the screen in light and dark | macOS, Windows | `apps/client/test/a11y/settings_semantics_test.dart`: "the settings section devices: contrast, labelled targets and target size in light and dark" | by sight |
| Text scaling to 200 percent | the screen at text scale 2 in German and English | macOS, Windows | `apps/client/test/a11y/settings_semantics_test.dart`: "the settings section devices at text scale 2 in de and en: no overflow, every control reachable" | by sight |
| Every section | appearance, notifications, language, keyboard, privacy and about | macOS, Windows | `apps/client/test/a11y/settings_semantics_test.dart`: "the settings section appearance: roles, labels and states for screen readers"; "the settings section notifications: roles, labels and states for screen readers"; "the settings section language: roles, labels and states for screen readers"; "the settings section keyboard: roles, labels and states for screen readers"; "the settings section privacy: roles, labels and states for screen readers"; "the settings section about: roles, labels and states for screen readers" | VoiceOver, Narrator |

### Notification settings

| Rule of 01 section 8 | Where | Platform | Automated evidence | Manual pass |
|---|---|---|---|---|
| Every control has a role and a name | the default level, the preview choice, quiet hours, the tree of levels with each target naming its level control | macOS, Windows | `apps/client/test/a11y/notification_settings_semantics_test.dart`: "the notification settings: roles, labels and states for screen readers" | VoiceOver, Narrator |
| Contrast (text 4.5:1, controls 3:1) and targets of `density.target-min` | the screen in light and dark | macOS, Windows | `apps/client/test/a11y/notification_settings_semantics_test.dart`: "the notification settings: contrast, labelled targets and target size in light and dark" | by sight |
| Text scaling to 200 percent | the screen at text scale 2 in German and English | macOS, Windows | `apps/client/test/a11y/notification_settings_semantics_test.dart`: "the notification settings at text scale 2 in de and en: no overflow, every control reachable" | by sight |
| Every control has a role and a name | the quiet hours editor: days, times, "All day", the named time zone field, the switch | macOS, Windows | `apps/client/test/a11y/notification_settings_semantics_test.dart`: "the quiet hours editor: roles, labels and states for screen readers" | VoiceOver, Narrator |

### Notification area and toasts

| Rule of 01 section 8 | Where | Platform | Automated evidence | Manual pass |
|---|---|---|---|---|
| Language | the menu of the notification area (Open, Quit) speaks the language the person chose | Windows | `apps/client/test/a11y/tray_language_test.dart`: "the tray menu follows the language the person chose" | Narrator |
| Every control has a name | the tray icon has the product name as its tooltip; Open and Quit are system menu items | Windows | `apps/client/test/desktop/window_behavior_test.dart`: "the tray menu has Open and Quit and drives the window" | Narrator |
| Nothing is said by color alone | a toast carries its channel, thread, author and text as words | macOS, Windows | `apps/client/test/notifications/toast_content_test.dart`: "title and text from the preview fields (mode full)" | VoiceOver, Narrator |

### Administration A1 People

| Rule of 01 section 8 | Where | Platform | Automated evidence | Manual pass |
|---|---|---|---|---|
| Every control has a role and a name | the administration shell, people, a person, a group, the queue of waiting devices; people, group and audit rows are buttons | macOS, Windows | `packages/admin_ui/test/a11y/a1_people_semantics_test.dart`: "A1 shell: roles, labels and states for screen readers"; "A1 people: roles, labels and states for screen readers"; "A1 person: roles, labels and states for screen readers"; "A1 group: roles, labels and states for screen readers"; "A1 waiting_devices: roles, labels and states for screen readers" | VoiceOver, Narrator |
| Contrast and targets | the views in light and dark | macOS, Windows | `packages/admin_ui/test/a11y/a1_people_semantics_test.dart`: "A1 shell: contrast, labelled targets and target size in light and dark"; "A1 people: contrast, labelled targets and target size in light and dark"; "A1 person: contrast, labelled targets and target size in light and dark"; "A1 group: contrast, labelled targets and target size in light and dark"; "A1 waiting_devices: contrast, labelled targets and target size in light and dark" | by sight |
| Text scaling to 200 percent | the views at text scale 2 in German and English | macOS, Windows | `packages/admin_ui/test/a11y/a1_people_semantics_test.dart`: "A1 shell at text scale 2 in de and en: no overflow, every control reachable"; "A1 people at text scale 2 in de and en: no overflow, every control reachable"; "A1 person at text scale 2 in de and en: no overflow, every control reachable"; "A1 group at text scale 2 in de and en: no overflow, every control reachable"; "A1 waiting_devices at text scale 2 in de and en: no overflow, every control reachable" | by sight |

### Administration A2 Create grant

| Rule of 01 section 8 | Where | Platform | Automated evidence | Manual pass |
|---|---|---|---|---|
| Every control has a role and a name | the three steps of the wizard and the plain-language preview | macOS, Windows | `packages/admin_ui/test/a11y/a2_grant_semantics_test.dart`: "A2 wizard_who: roles, labels and states for screen readers"; "A2 wizard_where: roles, labels and states for screen readers"; "A2 wizard_preview: roles, labels and states for screen readers" | VoiceOver, Narrator |
| Contrast and targets | the views in light and dark | macOS, Windows | `packages/admin_ui/test/a11y/a2_grant_semantics_test.dart`: "A2 wizard_who: contrast, labelled targets and target size in light and dark"; "A2 wizard_where: contrast, labelled targets and target size in light and dark"; "A2 wizard_preview: contrast, labelled targets and target size in light and dark" | by sight |
| Text scaling to 200 percent | the views at text scale 2 in German and English | macOS, Windows | `packages/admin_ui/test/a11y/a2_grant_semantics_test.dart`: "A2 wizard_who at text scale 2 in de and en: no overflow, every control reachable"; "A2 wizard_where at text scale 2 in de and en: no overflow, every control reachable"; "A2 wizard_preview at text scale 2 in de and en: no overflow, every control reachable" | by sight |

### Administration A3 Who has access?

| Rule of 01 section 8 | Where | Platform | Automated evidence | Manual pass |
|---|---|---|---|---|
| Every control has a role and a name | people, groups and agents with their effective level in words | macOS, Windows | `packages/admin_ui/test/a11y/a3_who_has_access_semantics_test.dart`: "A3 who_has_access: roles, labels and states for screen readers" | VoiceOver, Narrator |
| Contrast and targets | the views in light and dark | macOS, Windows | `packages/admin_ui/test/a11y/a3_who_has_access_semantics_test.dart`: "A3 who_has_access: contrast, labelled targets and target size in light and dark" | by sight |
| Text scaling to 200 percent | the views at text scale 2 in German and English | macOS, Windows | `packages/admin_ui/test/a11y/a3_who_has_access_semantics_test.dart`: "A3 who_has_access at text scale 2 in de and en: no overflow, every control reachable" | by sight |

### Administration A4 View as person

| Rule of 01 section 8 | Where | Platform | Automated evidence | Manual pass |
|---|---|---|---|---|
| Every control has a role and a name | the start, the frame banner and the read-only structure | macOS, Windows | `packages/admin_ui/test/a11y/a4_view_as_semantics_test.dart`: "A4 banner: roles, labels and states for screen readers"; "A4 structure: roles, labels and states for screen readers"; "A4 structure_later: roles, labels and states for screen readers" | VoiceOver, Narrator |
| Contrast and targets | the views in light and dark | macOS, Windows | `packages/admin_ui/test/a11y/a4_view_as_semantics_test.dart`: "A4 banner: contrast, labelled targets and target size in light and dark"; "A4 structure: contrast, labelled targets and target size in light and dark"; "A4 structure_later: contrast, labelled targets and target size in light and dark" | by sight |
| Text scaling to 200 percent | the views at text scale 2 in German and English | macOS, Windows | `packages/admin_ui/test/a11y/a4_view_as_semantics_test.dart`: "A4 banner at text scale 2 in de and en: no overflow, every control reachable"; "A4 structure at text scale 2 in de and en: no overflow, every control reachable"; "A4 structure_later at text scale 2 in de and en: no overflow, every control reachable" | by sight |

### Administration A5 Agents

| Rule of 01 section 8 | Where | Platform | Automated evidence | Manual pass |
|---|---|---|---|---|
| Every control has a role and a name | the list, an agent, the backend switch, the switch of all agents and the guided flow | macOS, Windows | `packages/admin_ui/test/a11y/a5_agents_semantics_test.dart`: "A5 list: roles, labels and states for screen readers"; "A5 page: roles, labels and states for screen readers"; "A5 switch: roles, labels and states for screen readers"; "A5 switch_all: roles, labels and states for screen readers"; "A5 guided_preview: roles, labels and states for screen readers"; "A5 i1: roles, labels and states for screen readers"; "A5 guided_result: roles, labels and states for screen readers" | VoiceOver, Narrator |
| Contrast and targets | the views in light and dark | macOS, Windows | `packages/admin_ui/test/a11y/a5_agents_semantics_test.dart`: "A5 list: contrast, labelled targets and target size in light and dark"; "A5 page: contrast, labelled targets and target size in light and dark"; "A5 switch: contrast, labelled targets and target size in light and dark"; "A5 switch_all: contrast, labelled targets and target size in light and dark"; "A5 guided_preview: contrast, labelled targets and target size in light and dark"; "A5 i1: contrast, labelled targets and target size in light and dark"; "A5 guided_result: contrast, labelled targets and target size in light and dark" | by sight |
| Text scaling to 200 percent | the views at text scale 2 in German and English | macOS, Windows | `packages/admin_ui/test/a11y/a5_agents_semantics_test.dart`: "A5 list at text scale 2 in de and en: no overflow, every control reachable"; "A5 page at text scale 2 in de and en: no overflow, every control reachable"; "A5 switch at text scale 2 in de and en: no overflow, every control reachable"; "A5 switch_all at text scale 2 in de and en: no overflow, every control reachable"; "A5 guided_preview at text scale 2 in de and en: no overflow, every control reachable"; "A5 i1 at text scale 2 in de and en: no overflow, every control reachable"; "A5 guided_result at text scale 2 in de and en: no overflow, every control reachable" | by sight |

### Administration A6 Audit

| Rule of 01 section 8 | Where | Platform | Automated evidence | Manual pass |
|---|---|---|---|---|
| Every control has a role and a name | the filters and entries and the result of the chain check in words | macOS, Windows | `packages/admin_ui/test/a11y/a6_audit_semantics_test.dart`: "A6 list: roles, labels and states for screen readers"; "A6 chain_ok: roles, labels and states for screen readers"; "A6 chain_broken: roles, labels and states for screen readers" | VoiceOver, Narrator |
| Contrast and targets | the views in light and dark | macOS, Windows | `packages/admin_ui/test/a11y/a6_audit_semantics_test.dart`: "A6 list: contrast, labelled targets and target size in light and dark"; "A6 chain_ok: contrast, labelled targets and target size in light and dark"; "A6 chain_broken: contrast, labelled targets and target size in light and dark" | by sight |
| Text scaling to 200 percent | the views at text scale 2 in German and English | macOS, Windows | `packages/admin_ui/test/a11y/a6_audit_semantics_test.dart`: "A6 list at text scale 2 in de and en: no overflow, every control reachable"; "A6 chain_ok at text scale 2 in de and en: no overflow, every control reachable"; "A6 chain_broken at text scale 2 in de and en: no overflow, every control reachable" | by sight |

Only the manual pass proves: that VoiceOver names a text field (the macOS engine of Flutter
gives its own text field no name in the platform tree, so the probe compares only the role there,
D-M4-32-6), that VoiceOver and Narrator speak the names in the order of the screen, that the live box is announced politely without interrupting (the platform event of a
live region or announcement is not read by the probes), the pronunciation of mixed German and
English text, the reading of the notification toasts by the system, and the readability of the
focus ring for a person with low vision.

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

## Status page (M7-21)

The same page on the desktop, the phones and in the console (`StatusScreen` of `admin_ui`).

| Rule of 01 section 8 | Where on the status page | Automated evidence |
|---|---|---|
| Every control has a name for screen readers | "Add component", "Open incident or maintenance", "Edit", "Delete", "Add update", the fields of the forms; the sidebar entry "Status page" | `packages/admin_ui/test/status/accessibility_test.dart` ("labelled controls, contrast, headings and the state in words", `labeledTapTargetGuideline`) |
| Contrast of text | the page in the default theme; the state roles in light and dark | same test (`textContrastGuideline`); `packages/admin_ui/test/goldens/status_golden_test.dart` |
| Headings structure the page | "Components", "Open incidents", "Maintenance windows", "History" are headings | same test |
| Nothing is said by color alone | every component state and the overall state say their word next to the icon; incidents carry kind, impact and state as text; maintenance uses the role `info`, never an outage role | `packages/admin_ui/test/status/status_screen_test.dart` ("every component state has its word", "incidents, maintenance windows and history apart") |
| Changes are announced | "Server unreachable" is a live-region banner | `accessibility_test.dart` ("Server unreachable is announced") |
| Text scaling up to 200 percent | the page and "Server unreachable" at text scale 1 and 2 | `status_golden_test.dart` |
| VoiceOver, TalkBack and Narrator by hand | not proven automatically; part of the manual pass below | open for the release check |

## Manual pass before a release (M4-32)

Run before every release (15 20, row "Accessibility by hand") on a macOS and a Windows machine
with the release build, signed in to a test instance with the people Alex, Sam and Robin, a
forum with a post and an agent that can run. Note each finding with the screen and the step in
the release issue; a step that fails blocks the release until it is fixed or a ruling says why
it waits.

### VoiceOver on macOS

1. Turn VoiceOver on (Command+F5) and open the app; VoiceOver says the product name and the
   window.
2. E1 to E2: move with VO+Right through the server address, Connect, the login fields and the
   buttons; every control says its name and role. On E4 and E5 the code is read character by
   character, and the remaining time is announced at most once per interval.
3. Sidebar: VO+Right through the spaces; a space says "expanded" or "collapsed"; a channel says
   unread, mentions, a running agent and muted in words. Press F6 to move to the next column.
4. Post list: New post and the filters say their state; a post says its title, its state
   ("Waiting for you" once), its time and unread count. Open a post with VO+Space.
5. Thread view: the header actions, the messages with author and time, an edited and a deleted
   message. Mention the agent in the composer: the composer says its name once (on macOS check
   that VoiceOver says "Message to" or "Follow-up to" and not only "text field", D-M4-32-6).
6. Live box: the box says agent, state, time and context; only state changes and the question
   are announced, the progress text is not read while it changes; Stop can be pressed with
   VO+Space. Cards: each button says its name; a card that may not be used says why.
7. Files and voice: a file says name, size and state once; play a voice message.
8. Search (Command+K or the menu), the quick switcher, Mentions and Running now: the field, the
   filters and the results say their names; a run says its state in words.
9. Settings: every switch and drop-down is named by its row; the devices and sessions say their
   state. Notification settings and the channel menu (Shift+F10 on a channel): every entry is a
   button, the chosen level says "selected".
10. Administration A1 to A6: lists, wizards and the audit filters, each control with its name;
    the chain check result is read in words.
11. Turn VoiceOver off; with "Reduce motion" on in the system settings no transition plays; with
    the text size of the system at its largest step no text is cut off.

### Narrator on Windows

1. Turn Narrator on (Windows+Ctrl+Enter) and open the app; Narrator says the product name.
2. Repeat steps 2 to 10 of VoiceOver with Caps Lock+Right for the next element, Tab for the next
   control and Caps Lock+Enter or Space to press.
3. Notification area: open the menu of the tray icon; Open and Quit are read in the language
   chosen in the app. A toast is read with channel, thread, author and text.
4. Turn Narrator off; with "Show animations" off no transition plays; with the text size of
   Windows at 200 percent no text is cut off.
