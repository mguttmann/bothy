// SPDX-License-Identifier: Apache-2.0

/// The renderer `xterm` of one tab (docs/spec/24-terminal.md 12, docs/spec/15-client-architektur.md
/// 23.4): the theme from the tokens, the font size of `client.terminal.font_size_default`, the
/// escape filter in front (in `TerminalScreen`), copy only by the person, the paste guard, and
/// hyperlinks that open only after a click and a confirmation.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/gestures.dart' show kTouchSlop;
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:terminal_ui/src/controller/terminal_controller.dart';
import 'package:terminal_ui/src/render/paste_guard.dart';
import 'package:terminal_ui/src/render/semantics.dart';
import 'package:terminal_ui/src/render/terminal_screen.dart';
import 'package:terminal_ui/src/render/terminal_theme.dart';
import 'package:terminal_ui/src/widgets/link_confirm_dialog.dart';
import 'package:xterm/xterm.dart'
    show
        CellOffset,
        TerminalCursorType,
        TerminalStyle,
        TerminalView,
        TerminalViewState;

/// The intent of a paste through the guard; the shortcuts of the view map the paste keys to
/// it instead of xterm's own paste, which would paste without asking.
class GuardedPasteIntent extends Intent {
  /// Creates the intent.
  const GuardedPasteIntent();
}

/// The shortcuts of the screen: copy and paste only with the platform modifier (Cmd on Apple,
/// Ctrl+Shift elsewhere, so Ctrl+C and Ctrl+V stay with the shell, 15 23.4). The remaining
/// shortcuts of the app are bound by M4-36.
Map<ShortcutActivator, Intent> terminalShortcuts(TargetPlatform platform) {
  final apple =
      platform == TargetPlatform.macOS || platform == TargetPlatform.iOS;
  return {
    SingleActivator(
      LogicalKeyboardKey.keyC,
      meta: apple,
      control: !apple,
      shift: !apple,
    ): CopySelectionTextIntent.copy,
    SingleActivator(
      LogicalKeyboardKey.keyV,
      meta: apple,
      control: !apple,
      shift: !apple,
    ): const GuardedPasteIntent(),
  };
}

/// The screen of [controller] with the renderer, the semantics and the link handling.
///
/// [fontSize] is `client.terminal.font_size_default` or the size the person chose
/// (01 8); [readClipboard] defaults to the system clipboard and exists for tests.
class TerminalScreenView extends StatefulWidget {
  /// Creates the view.
  const TerminalScreenView({
    super.key,
    required this.controller,
    required this.fontSize,
    this.readClipboard,
    this.autofocus = false,
  });

  /// The controller of the tab.
  final TerminalController controller;

  /// The font size in logical pixels.
  final double fontSize;

  /// Reads the text of the clipboard; null uses [Clipboard.getData].
  final Future<String?> Function()? readClipboard;

  /// Whether the screen takes the focus when it appears.
  final bool autofocus;

  @override
  State<TerminalScreenView> createState() => _TerminalScreenViewState();
}

class _TerminalScreenViewState extends State<TerminalScreenView> {
  final GlobalKey<TerminalViewState> _view = GlobalKey();
  String? _hoverTarget;
  Offset? _down;

  TerminalController get _c => widget.controller;

  @override
  Widget build(BuildContext context) {
    final l = dsL10n(context);
    final c = context.dsColors;
    final mono = DsType.code;
    return ListenableBuilder(
      listenable: _c.screen,
      builder: (context, _) {
        final terminal = _c.screen.terminal;
        return Actions(
          actions: {
            GuardedPasteIntent: CallbackAction<GuardedPasteIntent>(
              onInvoke: (_) => _paste(),
            ),
          },
          child: Stack(
            children: [
              Positioned.fill(
                child: MouseRegion(
                  onHover: _hover,
                  onExit: (_) => _setHover(null),
                  // xterm 4.0.0 never calls TerminalView.onTapUp (its gesture detector drops
                  // the callback), so a click on a link is detected here, outside the gesture
                  // arena, without taking anything from the renderer.
                  child: Listener(
                    onPointerDown: (e) => _down = e.localPosition,
                    onPointerUp: _up,
                    child: TerminalScreenSemantics(
                      screen: _c.screen,
                      child: TerminalView(
                        terminal,
                        key: _view,
                        theme: buildTerminalTheme(c),
                        textStyle: TerminalStyle(
                          fontSize: widget.fontSize,
                          fontFamily: mono.fontFamily!,
                          fontFamilyFallback: mono.fontFamilyFallback!,
                        ),
                        autofocus: widget.autofocus,
                        // xterm 4.0.0 cannot draw the character under a block cursor in
                        // color.terminal.cursor-text; an underline keeps the character at full
                        // contrast and the cursor opaque (D-M4-35-9).
                        cursorType: TerminalCursorType.underline,
                        shortcuts: terminalShortcuts(defaultTargetPlatform),
                      ),
                    ),
                  ),
                ),
              ),
              if (_hoverTarget case final target?)
                Positioned(
                  left: DsSpace.s8,
                  bottom: DsSpace.s8,
                  child: IgnorePointer(
                    child: DecoratedBox(
                      decoration: BoxDecoration(
                        color: c.surfaceOverlay,
                        borderRadius: BorderRadius.circular(DsRadius.sm),
                        border: Border.all(color: c.borderDefault),
                      ),
                      child: Padding(
                        padding: const EdgeInsets.all(DsSpace.s4),
                        child: Text(
                          l.terminalLinkHover(target),
                          style: dsTextStyle(DsType.codeSm)
                              .copyWith(color: c.textPrimary),
                        ),
                      ),
                    ),
                  ),
                ),
            ],
          ),
        );
      },
    );
  }

  void _up(PointerUpEvent event) {
    final down = _down;
    _down = null;
    final view = _view.currentState;
    if (down == null || view == null) return;
    if ((event.localPosition - down).distance > kTouchSlop) return;
    unawaited(_tap(view.renderTerminal.getCellOffset(event.localPosition)));
  }

  void _hover(PointerHoverEvent event) {
    final view = _view.currentState;
    if (view == null) return;
    final offset = view.renderTerminal.getCellOffset(event.localPosition);
    _setHover(_c.screen.linkAt(offset)?.target);
  }

  void _setHover(String? target) {
    if (target == _hoverTarget) return;
    setState(() => _hoverTarget = target);
  }

  Future<void> _tap(CellOffset offset) async {
    final link = _c.screen.linkAt(offset);
    if (link == null) return;
    final open = await showDialog<bool>(
      context: context,
      builder: (_) => LinkConfirmDialog(target: link.target),
    );
    if (open == true) await _c.openLink(link.target);
  }

  Future<void> _paste() async {
    final read = widget.readClipboard;
    final text = read != null
        ? await read()
        : (await Clipboard.getData(Clipboard.kTextPlain))?.text;
    if (text == null || text.isEmpty || !mounted) return;
    final clean = sanitizePaste(text);
    if (clean.isEmpty || !await confirmPaste(context, clean)) return;
    _c.screen.paste(clean);
  }
}
