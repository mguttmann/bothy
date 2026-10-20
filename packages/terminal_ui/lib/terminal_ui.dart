// SPDX-License-Identifier: Apache-2.0

/// The terminal view of every client (docs/spec/15-client-architektur.md 23,
/// docs/spec/24-terminal.md): [TerminalController] runs the flow of 15 23.2 through the ports
/// of `ports.dart`; [TerminalPane] shows a tab with the renderer `xterm` behind the
/// [EscapeFilter], the header, the tabs, the states and the extra keys bar; the dialogs
/// "Open terminal" and "Key changed" and the warning of 24 8.4 answer [TerminalPrompts]
/// ([DialogTerminalPrompts]).
///
/// The package imports neither `sync_engine` nor `local_store` (15 3.1 rule 7): pins, host
/// events and the stream come in through the ports.
library;

export 'src/controller/close_reactions.dart';
export 'src/controller/frame_pump.dart'
    show FrameTickScheduler, maxDataFrameBytes;
export 'src/controller/session_state.dart';
export 'src/controller/terminal_controller.dart';
export 'src/controller/terminal_prompts.dart';
export 'src/pinning/pin_check.dart';
export 'src/ports.dart';
export 'src/render/escape_filter.dart';
export 'src/render/paste_guard.dart';
export 'src/render/semantics.dart';
export 'src/render/terminal_screen.dart';
export 'src/render/terminal_theme.dart';
export 'src/render/terminal_view.dart';
export 'src/widgets/dialog_prompts.dart';
export 'src/widgets/extra_keys_bar.dart';
export 'src/widgets/foreign_host_warning.dart';
export 'src/widgets/key_changed_dialog.dart';
export 'src/widgets/link_confirm_dialog.dart';
export 'src/widgets/open_terminal_dialog.dart';
export 'src/widgets/session_tabs.dart';
export 'src/widgets/terminal_header.dart';
export 'src/widgets/terminal_pane.dart';
export 'src/widgets/terminal_states.dart';
