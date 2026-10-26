// SPDX-License-Identifier: Apache-2.0

/// The renderer `xterm` of one tab (docs/spec/24-terminal.md 12, docs/spec/15-client-architecture.md
/// 23.4): the theme from the tokens, the font size of `client.terminal.font_size_default`, the
/// escape filter in front (in `TerminalScreen`), copy only by the person, the paste guard, and
/// hyperlinks that open only after a click and a confirmation.
library;

import 'dart:async';
import 'dart:developer' show Timeline;

import 'package:design_system/design_system.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/gestures.dart' show kTouchSlop;
import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';
import 'package:flutter/services.dart';
import 'package:terminal_ui/src/controller/terminal_controller.dart';
import 'package:terminal_ui/src/perf/perf_hooks.dart';
import 'package:terminal_ui/src/render/paste_guard.dart';
import 'package:terminal_ui/src/render/semantics.dart';
import 'package:terminal_ui/src/render/terminal_marks.dart';
import 'package:terminal_ui/src/render/terminal_screen.dart';
import 'package:terminal_ui/src/render/terminal_theme.dart';
import 'package:terminal_ui/src/widgets/link_confirm_dialog.dart';
import 'package:xterm/xterm.dart' as xterm show TerminalController;
import 'package:xterm/xterm.dart'
    show CellOffset, Terminal, TerminalStyle, TerminalView, TerminalViewState;

/// The intent of a paste through the guard; the shortcuts of the view map the paste keys to
/// it instead of xterm's own paste, which would paste without asking.
class GuardedPasteIntent extends Intent {
  /// Creates the intent.
  const GuardedPasteIntent();
}

/// The shortcuts of the screen: copy and paste only with the platform modifier (Cmd on Apple,
/// Ctrl+Shift elsewhere, so Ctrl+C and Ctrl+V stay with the shell, 15 23.4). The remaining
/// shortcuts of the app come in through [TerminalScreenView.shortcuts] (M4-36).
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

/// Whether the screen reads only the hardware keyboard on [platform]: Windows, whose embedder
/// refuses the text input client of xterm 4.0.0 (it names no view). Elsewhere the client stays,
/// so dead keys, input methods and the soft keyboard keep working (D-M4-36-10).
bool hardwareKeyboardOnlyOn(TargetPlatform platform) =>
    platform == TargetPlatform.windows;

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
    this.shortcuts = const {},
  });

  /// The controller of the tab.
  final TerminalController controller;

  /// The font size in logical pixels.
  final double fontSize;

  /// Reads the text of the clipboard; null uses [Clipboard.getData].
  final Future<String?> Function()? readClipboard;

  /// Whether the screen takes the focus when it appears.
  final bool autofocus;

  /// Further shortcuts of the app, handled by the renderer before it sends a key to the shell;
  /// copy and paste of [terminalShortcuts] win over a stroke given here.
  final Map<ShortcutActivator, Intent> shortcuts;

  @override
  State<TerminalScreenView> createState() => _TerminalScreenViewState();
}

class _TerminalScreenViewState extends State<TerminalScreenView> {
  final GlobalKey<TerminalViewState> _view = GlobalKey();
  // The selection, the focus and the scroll position of the view; the marks painter follows
  // them (D-M4-35-18).
  final xterm.TerminalController _selection = xterm.TerminalController();
  final FocusNode _focus = FocusNode(debugLabel: 'terminal');
  final ScrollController _scroll = ScrollController();
  final _Pulse _terminalChanged = _Pulse();
  Terminal? _watched;
  String? _hoverTarget;
  Offset? _down;

  TerminalController get _c => widget.controller;

  @override
  void initState() {
    super.initState();
    _c.screen.onTextInsert = _insert;
    if (terminalPerfHooksEnabled) _c.screen.addListener(_afterOutput);
  }

  // Test-only (brief M4-38, 15 19): after new output, the next frame reports itself as drawn
  // once its post-frame callbacks run, so the performance level can pair it with the
  // rasterization of that frame. Never registered without the define TERMINAL_PERF_HOOKS.
  bool _drawnPending = false;

  void _afterOutput() {
    if (_drawnPending) return;
    _drawnPending = true;
    final tab = _c;
    SchedulerBinding.instance.addPostFrameCallback((_) {
      _drawnPending = false;
      if (terminalPerfHooksEnabled) {
        TerminalPerfHooks.probe?.frameDrawn(tab, Timeline.now);
      }
    });
  }

  @override
  void didUpdateWidget(TerminalScreenView oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (!identical(oldWidget.controller, widget.controller)) {
      if (oldWidget.controller.screen.onTextInsert == _insert) {
        oldWidget.controller.screen.onTextInsert = null;
      }
      _c.screen.onTextInsert = _insert;
      if (terminalPerfHooksEnabled) {
        oldWidget.controller.screen.removeListener(_afterOutput);
        _c.screen.addListener(_afterOutput);
      }
    }
  }

  @override
  void dispose() {
    if (_c.screen.onTextInsert == _insert) _c.screen.onTextInsert = null;
    if (terminalPerfHooksEnabled) _c.screen.removeListener(_afterOutput);
    _watched?.removeListener(_terminalChanged.fire);
    _terminalChanged.dispose();
    _selection.dispose();
    _focus.dispose();
    _scroll.dispose();
    super.dispose();
  }

  void _watch(Terminal terminal) {
    if (identical(terminal, _watched)) return;
    _watched?.removeListener(_terminalChanged.fire);
    _watched = terminal..addListener(_terminalChanged.fire);
  }

  TerminalGeometry? _geometry() {
    final view = _view.currentState;
    if (view == null || view.context.findRenderObject() == null) return null;
    final render = view.renderTerminal;
    if (!render.hasSize) return null;
    // xterm paints a cell at column times cell width, without the left padding that its
    // getOffset adds, and at getOffset's row position.
    return _Geometry(
      render.cellSize,
      (cell) =>
          Offset(cell.x * render.cellSize.width, render.getOffset(cell).dy),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l = dsL10n(context);
    final c = context.dsColors;
    final mono = DsType.code;
    return ListenableBuilder(
      listenable: _c.screen,
      builder: (context, _) {
        final terminal = _c.screen.terminal;
        _watch(terminal);
        final textStyle = TerminalStyle(
          fontSize: widget.fontSize,
          fontFamily: mono.fontFamily!,
          fontFamilyFallback: mono.fontFamilyFallback!,
        );
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
                        controller: _selection,
                        focusNode: _focus,
                        scrollController: _scroll,
                        theme: buildTerminalTheme(c),
                        textStyle: textStyle,
                        autofocus: widget.autofocus,
                        // Windows reads the hardware keyboard only (D-M4-36-10).
                        hardwareKeyboardOnly: hardwareKeyboardOnlyOn(
                          defaultTargetPlatform,
                        ),
                        shortcuts: {
                          ...widget.shortcuts,
                          ...terminalShortcuts(defaultTargetPlatform),
                        },
                      ),
                    ),
                  ),
                ),
              ),
              // The cursor and the selection, opaque in the token pairings; xterm paints
              // them transparent (D-M4-35-18).
              Positioned.fill(
                child: IgnorePointer(
                  child: CustomPaint(
                    painter: TerminalMarksPainter(
                      terminal: terminal,
                      geometry: _geometry,
                      selection: () => _selection.selection,
                      focused: () => _focus.hasFocus,
                      colors: terminalMarkColors(c),
                      textStyle: textStyle,
                      textScaler: MediaQuery.textScalerOf(context),
                      devicePixelRatio: MediaQuery.devicePixelRatioOf(context),
                      repaint: Listenable.merge([
                        _terminalChanged,
                        _selection,
                        _focus,
                        _scroll,
                      ]),
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
    await _guardedPaste(text);
  }

  // The paste guard of 15 23.4 for the clipboard and for text the soft keyboard inserts.
  Future<void> _guardedPaste(String text) async {
    final clean = sanitizePaste(text);
    if (clean.isEmpty || !await confirmPaste(context, clean)) return;
    if (mounted) _c.screen.paste(clean);
  }

  void _insert(String text) {
    if (mounted) unawaited(_guardedPaste(text));
  }
}

/// Turns xterm's own listener callbacks into a [Listenable] for the marks painter.
final class _Pulse extends ChangeNotifier {
  void fire() => notifyListeners();
}

final class _Geometry implements TerminalGeometry {
  _Geometry(this.cellSize, this._offsetOf);

  @override
  final Size cellSize;

  final Offset Function(CellOffset cell) _offsetOf;

  @override
  Offset offsetOf(CellOffset cell) => _offsetOf(cell);
}
