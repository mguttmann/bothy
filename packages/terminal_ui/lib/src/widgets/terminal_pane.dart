// SPDX-License-Identifier: Apache-2.0

/// The terminal view of one host (screen D11 of docs/spec/01-produkt.md, M8 on the phone;
/// docs/design/components.md 12): header, tabs with "Detach" and "End", the state banner, the
/// screen, the footer and, on the phone, the extra keys bar.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:terminal_ui/src/controller/session_state.dart';
import 'package:terminal_ui/src/controller/terminal_controller.dart';
import 'package:terminal_ui/src/render/terminal_view.dart';
import 'package:terminal_ui/src/widgets/extra_keys_bar.dart';
import 'package:terminal_ui/src/widgets/session_tabs.dart';
import 'package:terminal_ui/src/widgets/terminal_header.dart';
import 'package:terminal_ui/src/widgets/terminal_states.dart';

/// The largest share of the pane's height that the parts above the screen (header, tabs) and
/// below it (footer, extra keys bar) may each take; they scroll beyond it.
const double chromeHeightShare = 0.3;

/// The largest share of the pane's height that the state banner takes; it scrolls beyond it.
const double bannerHeightShare = 0.25;

/// The whole view of the selected tab.
///
/// The app owns the tabs (one controller per session) and passes them as [tabs] with the
/// [selected] index; without [tabs] the pane shows one tab named after the profile.
/// [maxSessionDuration] is `terminal.max_session_duration` as the app knows it (brief M4-35
/// rule 9); [now] defaults to the system time and drives the remaining time, refreshed every
/// [remainingRefresh]. [showExtraKeys] shows the bar (phones without a hardware keyboard).
///
/// ```dart
/// TerminalPane(controller: c, fontSize: config.terminalFontSizeDefault.toDouble(),
///     maxSessionDuration: const Duration(hours: 12), showExtraKeys: isPhone)
/// ```
class TerminalPane extends StatefulWidget {
  /// Creates the pane.
  const TerminalPane({
    super.key,
    required this.controller,
    required this.fontSize,
    this.tabs,
    this.selected = 0,
    this.onSelect,
    this.maxSessionDuration,
    this.now,
    this.remainingRefresh = const Duration(minutes: 1),
    this.showExtraKeys = false,
    this.readClipboard,
  });

  /// The controller of the selected tab.
  final TerminalController controller;

  /// The font size of the screen in logical pixels.
  final double fontSize;

  /// The tabs of the host; null shows the one tab of [controller].
  final List<SessionTabData>? tabs;

  /// The index of the selected tab in [tabs].
  final int selected;

  /// Selects another tab.
  final ValueChanged<int>? onSelect;

  /// `terminal.max_session_duration`; null hides the remaining time.
  final Duration? maxSessionDuration;

  /// The clock of the remaining time; null uses the system time.
  final DateTime Function()? now;

  /// How often the remaining time is redrawn.
  final Duration remainingRefresh;

  /// Whether the extra keys bar is shown.
  final bool showExtraKeys;

  /// Reads the clipboard for paste; null uses the system clipboard.
  final Future<String?> Function()? readClipboard;

  @override
  State<TerminalPane> createState() => _TerminalPaneState();
}

class _TerminalPaneState extends State<TerminalPane> {
  final ExtraKeysController _keys = ExtraKeysController();
  Timer? _tick;

  @override
  void initState() {
    super.initState();
    widget.controller.inputTransform = _keys.transform;
    _startTick();
  }

  @override
  void didUpdateWidget(TerminalPane old) {
    super.didUpdateWidget(old);
    if (old.controller != widget.controller) {
      old.controller.inputTransform = null;
      widget.controller.inputTransform = _keys.transform;
    }
    if (old.maxSessionDuration != widget.maxSessionDuration) _startTick();
  }

  @override
  void dispose() {
    _tick?.cancel();
    widget.controller.inputTransform = null;
    _keys.dispose();
    super.dispose();
  }

  void _startTick() {
    _tick?.cancel();
    _tick = widget.maxSessionDuration == null
        ? null
        : Timer.periodic(widget.remainingRefresh, (_) {
            if (mounted) setState(() {});
          });
  }

  @override
  Widget build(BuildContext context) {
    final l = dsL10n(context);
    final c = context.dsColors;
    final controller = widget.controller;
    return ListenableBuilder(
      listenable: controller,
      builder: (context, _) {
        final state = controller.state;
        final host = controller.host;
        final live =
            state.phase == TerminalPhase.connected ||
            state.phase == TerminalPhase.exited;
        final tabs =
            widget.tabs ??
            [SessionTabData(label: l.terminalTabLabel(state.profile ?? '', 1))];
        final caption = dsTextStyle(DsType.caption)
            .copyWith(color: c.textSecondary);
        return ColoredBox(
          color: c.surfaceCanvas,
          child: LayoutBuilder(
            builder: (context, box) => Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                // Header and tabs scroll within a share of the height, so large text or a
                // narrow phone never squeezes the screen to nothing.
                ConstrainedBox(
                  constraints: BoxConstraints(
                    maxHeight: box.maxHeight * chromeHeightShare,
                  ),
                  child: SingleChildScrollView(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        if (host != null)
                          TerminalHeader(
                            host: host,
                            state: state,
                            now: (widget.now ?? DateTime.now)(),
                            maxSessionDuration: widget.maxSessionDuration,
                          ),
                        if (state.sessionId != null)
                          SessionTabs(
                            tabs: tabs,
                            selected: widget.selected,
                            onSelect: widget.onSelect,
                            onDetach: live ? controller.detach : null,
                            onEnd:
                                state.phase == TerminalPhase.ended ||
                                    state.phase == TerminalPhase.blocked
                                ? null
                                : controller.end,
                          ),
                      ],
                    ),
                  ),
                ),
                // The banner has its own share, so "Reattach" and a hard warning stay in view
                // even when the header and tabs fill theirs.
                ConstrainedBox(
                  constraints: BoxConstraints(
                    maxHeight: box.maxHeight * bannerHeightShare,
                  ),
                  child: SingleChildScrollView(
                    child: TerminalStateBanner(
                      state: state,
                      onReattach: state.phase == TerminalPhase.detached
                          ? controller.attach
                          : null,
                    ),
                  ),
                ),
                Expanded(
                  child: ColoredBox(
                    color: c.terminalBg,
                    child: Padding(
                      padding: const EdgeInsets.all(DsSpace.s4),
                      child: TerminalScreenView(
                        controller: controller,
                        fontSize: widget.fontSize,
                        readClipboard: widget.readClipboard,
                      ),
                    ),
                  ),
                ),
                ConstrainedBox(
                  constraints: BoxConstraints(
                    maxHeight: box.maxHeight * chromeHeightShare,
                  ),
                  child: SingleChildScrollView(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        Padding(
                          padding: const EdgeInsets.symmetric(
                            horizontal: DsSpace.s12,
                            vertical: DsSpace.s4,
                          ),
                          child: Wrap(
                            spacing: DsSpace.s12,
                            crossAxisAlignment: WrapCrossAlignment.center,
                            children: [
                              Row(
                                mainAxisSize: MainAxisSize.min,
                                children: [
                                  DsIcon(
                                    DsIcons.securityLock,
                                    size: DsIconSizeRole.sm,
                                    color: c.textSecondary,
                                    decorative: true,
                                  ),
                                  const SizedBox(width: DsSpace.s4),
                                  Text(
                                    l.terminalFooterEncrypted,
                                    style: caption,
                                  ),
                                ],
                              ),
                              if (state.profile case final profile?)
                                Text(
                                  l.terminalFooterProfile(profile),
                                  style: caption,
                                ),
                              Text(l.terminalFooterIdle, style: caption),
                            ],
                          ),
                        ),
                        if (widget.showExtraKeys)
                          ExtraKeysBar(
                            keys: _keys,
                            terminal: () => controller.screen.terminal,
                          ),
                      ],
                    ),
                  ),
                ),
              ],
            ),
          ),
        );
      },
    );
  }
}
