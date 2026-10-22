// SPDX-License-Identifier: Apache-2.0

/// The extra keys bar of the phone (screen M8 of docs/spec/01-product.md, docs/spec/24-terminal.md
/// 12; docs/design/components.md 12 "Extra keys bar"): Esc, Tab, Ctrl, Alt, the arrows, `|`,
/// `~`, `/` and `-`; Ctrl and Alt as one-shot or held modifiers.
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart' show AppLocalizations;
import 'package:xterm/xterm.dart' show Terminal, TerminalKey;

/// The state of one modifier of the bar.
enum ModifierLatch {
  /// Off.
  off,

  /// On for the next key only (a tap).
  oneShot,

  /// On until tapped again (a long press).
  held,
}

/// The modifiers of the bar and how they change input.
///
/// A tap on Ctrl or Alt latches it for the next key ([ModifierLatch.oneShot]); a long press
/// holds it ([ModifierLatch.held]) until the next tap. [transform] applies them to typed text,
/// so a letter of the soft keyboard after Ctrl becomes its control character.
final class ExtraKeysController extends ChangeNotifier {
  ModifierLatch _ctrl = ModifierLatch.off;
  ModifierLatch _alt = ModifierLatch.off;
  bool _bypass = false;

  /// The state of Ctrl.
  ModifierLatch get ctrl => _ctrl;

  /// The state of Alt.
  ModifierLatch get alt => _alt;

  /// A tap: off becomes one-shot, any other state becomes off.
  void tapCtrl() => _setCtrl(
    _ctrl == ModifierLatch.off ? ModifierLatch.oneShot : ModifierLatch.off,
  );

  /// A long press: Ctrl is held until the next tap.
  void holdCtrl() => _setCtrl(ModifierLatch.held);

  /// A tap on Alt, as [tapCtrl].
  void tapAlt() => _setAlt(
    _alt == ModifierLatch.off ? ModifierLatch.oneShot : ModifierLatch.off,
  );

  /// A long press on Alt, as [holdCtrl].
  void holdAlt() => _setAlt(ModifierLatch.held);

  /// Applies the latched modifiers to [text] typed by the person and releases one-shot ones.
  ///
  /// Ctrl turns one letter or one of `@[\]^_ ?` into its control character; Alt puts ESC in
  /// front. Text of more than one character (a paste, a renderer answer) passes unchanged and
  /// releases nothing.
  String transform(String text) {
    if (_bypass ||
        text.runes.length != 1 ||
        (_ctrl == ModifierLatch.off && _alt == ModifierLatch.off)) {
      return text;
    }
    var out = text;
    if (_ctrl != ModifierLatch.off) out = _control(out);
    if (_alt != ModifierLatch.off) out = '\x1b$out';
    _release();
    return out;
  }

  /// Runs [send], which already applied the modifiers itself (a special key of the bar), so
  /// that [transform] leaves its output alone; then releases one-shot modifiers.
  void sendApplied(void Function() send) {
    _bypass = true;
    try {
      send();
    } finally {
      _bypass = false;
    }
    _release();
  }

  void _release() {
    var changed = false;
    if (_ctrl == ModifierLatch.oneShot) {
      _ctrl = ModifierLatch.off;
      changed = true;
    }
    if (_alt == ModifierLatch.oneShot) {
      _alt = ModifierLatch.off;
      changed = true;
    }
    if (changed) notifyListeners();
  }

  void _setCtrl(ModifierLatch v) {
    _ctrl = v;
    notifyListeners();
  }

  void _setAlt(ModifierLatch v) {
    _alt = v;
    notifyListeners();
  }

  static String _control(String c) {
    final code = c.toUpperCase().codeUnitAt(0);
    if (code >= 0x40 && code <= 0x5f) return String.fromCharCode(code & 0x1f);
    if (c == '?') return '\x7f';
    if (c == ' ') return '\x00';
    return c;
  }
}

/// The bar: every key goes to [terminal] (which hands it to the controller), with the
/// modifiers of [keys].
///
/// ```dart
/// ExtraKeysBar(keys: keys, terminal: () => controller.screen.terminal)
/// ```
class ExtraKeysBar extends StatelessWidget {
  /// Creates the bar.
  const ExtraKeysBar({super.key, required this.keys, required this.terminal});

  /// The modifiers, shared with the input path of the view.
  final ExtraKeysController keys;

  /// The current terminal model (a replay replaces it).
  final Terminal Function() terminal;

  @override
  Widget build(BuildContext context) {
    final l = dsL10n(context);
    return Semantics(
      container: true,
      label: l.terminalExtraKeys,
      child: ListenableBuilder(
        listenable: keys,
        builder: (context, _) => SingleChildScrollView(
          scrollDirection: Axis.horizontal,
          child: Row(
            children: [
              _key(
                context,
                l.terminalKeyEsc,
                l.terminalKeyEscName,
                () => _press(TerminalKey.escape),
              ),
              _key(
                context,
                l.terminalKeyTab,
                l.terminalKeyTabName,
                () => _press(TerminalKey.tab),
              ),
              _modifier(
                context,
                l,
                l.terminalKeyCtrl,
                l.terminalKeyCtrlName,
                keys.ctrl,
                keys.tapCtrl,
                keys.holdCtrl,
              ),
              _modifier(
                context,
                l,
                l.terminalKeyAlt,
                l.terminalKeyAltName,
                keys.alt,
                keys.tapAlt,
                keys.holdAlt,
              ),
              _key(
                context,
                l.terminalKeyUp,
                l.terminalKeyUpName,
                () => _press(TerminalKey.arrowUp),
              ),
              _key(
                context,
                l.terminalKeyDown,
                l.terminalKeyDownName,
                () => _press(TerminalKey.arrowDown),
              ),
              _key(
                context,
                l.terminalKeyLeft,
                l.terminalKeyLeftName,
                () => _press(TerminalKey.arrowLeft),
              ),
              _key(
                context,
                l.terminalKeyRight,
                l.terminalKeyRightName,
                () => _press(TerminalKey.arrowRight),
              ),
              _key(
                context,
                l.terminalKeyPipe,
                l.terminalKeyPipeName,
                () => _text('|'),
              ),
              _key(
                context,
                l.terminalKeyTilde,
                l.terminalKeyTildeName,
                () => _text('~'),
              ),
              _key(
                context,
                l.terminalKeySlash,
                l.terminalKeySlashName,
                () => _text('/'),
              ),
              _key(
                context,
                l.terminalKeyMinus,
                l.terminalKeyMinusName,
                () => _text('-'),
              ),
            ],
          ),
        ),
      ),
    );
  }

  void _press(TerminalKey key) {
    final ctrl = keys.ctrl != ModifierLatch.off;
    final alt = keys.alt != ModifierLatch.off;
    keys.sendApplied(() => terminal().keyInput(key, ctrl: ctrl, alt: alt));
  }

  // The text goes the way of typed text: the input path applies the modifiers (transform).
  void _text(String text) => terminal().textInput(text);

  Widget _key(
    BuildContext context,
    String visible,
    String name,
    VoidCallback onTap,
  ) => _KeyButton(visible: visible, name: name, onTap: onTap);

  Widget _modifier(
    BuildContext context,
    AppLocalizations l,
    String visible,
    String name,
    ModifierLatch state,
    VoidCallback onTap,
    VoidCallback onHold,
  ) => _KeyButton(
    visible: visible,
    name: switch (state) {
      ModifierLatch.off => name,
      ModifierLatch.oneShot => l.terminalKeyLatched(name),
      ModifierLatch.held => l.terminalKeyHeld(name),
    },
    onTap: onTap,
    onLongPress: onHold,
    latched: state != ModifierLatch.off,
  );
}

class _KeyButton extends StatelessWidget {
  const _KeyButton({
    required this.visible,
    required this.name,
    required this.onTap,
    this.onLongPress,
    this.latched = false,
  });

  final String visible;
  final String name;
  final VoidCallback onTap;
  final VoidCallback? onLongPress;
  final bool latched;

  @override
  Widget build(BuildContext context) {
    final c = context.dsColors;
    final height = context.dsDensity.controlHeight;
    return Padding(
      padding: const EdgeInsets.all(DsSpace.s2),
      child: Semantics(
        button: true,
        toggled: onLongPress == null ? null : latched,
        label: name,
        excludeSemantics: true,
        child: Material(
          color: latched ? c.accentBg : c.surfaceRaised,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(DsRadius.md),
            side: BorderSide(color: c.borderControl),
          ),
          child: InkWell(
            onTap: onTap,
            onLongPress: onLongPress,
            borderRadius: BorderRadius.circular(DsRadius.md),
            child: ConstrainedBox(
              constraints: BoxConstraints(minHeight: height, minWidth: height),
              child: Center(
                child: Padding(
                  padding: const EdgeInsets.symmetric(horizontal: DsSpace.s8),
                  child: Text(
                    visible,
                    style: dsTextStyle(
                      DsType.label,
                    ).copyWith(color: latched ? c.textOnAccent : c.textPrimary),
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
