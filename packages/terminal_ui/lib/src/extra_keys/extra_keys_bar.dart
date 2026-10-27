// SPDX-License-Identifier: Apache-2.0

/// The extra keys bar of the phone (screen M8 of docs/spec/01-product.md, R-PR-11,
/// docs/spec/24-terminal.md 12; docs/design/components.md 12 "Extra keys bar"): Esc, Tab, Ctrl,
/// Alt, the arrows, `|`, `~`, `/` and `-`; Ctrl and Alt one-shot or latched.
library;

import 'dart:math' as math;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart' show AppLocalizations;
import 'package:terminal_ui/src/extra_keys/extra_keys_controller.dart';
import 'package:terminal_ui/src/extra_keys/key_mapping.dart';
import 'package:xterm/xterm.dart' show Terminal;

/// The bar: every key goes to [terminal]'s output (which hands it to the controller), with the
/// modifiers of [keys].
///
/// The keys of the bar send their sequence directly ([ExtraKeysController.sendKey]), so the
/// arrows follow the cursor key mode of the renderer and the paste guard never sees them; Ctrl
/// and Alt change the next character the person types ([ExtraKeysController.transform]).
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
              _key(l.terminalKeyEsc, l.terminalKeyEscName, ExtraKey.escape),
              _key(l.terminalKeyTab, l.terminalKeyTabName, ExtraKey.tab),
              _modifier(
                l,
                l.terminalKeyCtrl,
                l.terminalKeyCtrlName,
                keys.ctrl,
                keys.tapCtrl,
                keys.latchCtrl,
              ),
              _modifier(
                l,
                l.terminalKeyAlt,
                l.terminalKeyAltName,
                keys.alt,
                keys.tapAlt,
                keys.latchAlt,
              ),
              _key(l.terminalKeyUp, l.terminalKeyUpName, ExtraKey.up),
              _key(l.terminalKeyDown, l.terminalKeyDownName, ExtraKey.down),
              _key(l.terminalKeyLeft, l.terminalKeyLeftName, ExtraKey.left),
              _key(l.terminalKeyRight, l.terminalKeyRightName, ExtraKey.right),
              _key(l.terminalKeyPipe, l.terminalKeyPipeName, ExtraKey.pipe),
              _key(l.terminalKeyTilde, l.terminalKeyTildeName, ExtraKey.tilde),
              _key(l.terminalKeySlash, l.terminalKeySlashName, ExtraKey.slash),
              _key(l.terminalKeyMinus, l.terminalKeyMinusName, ExtraKey.minus),
            ],
          ),
        ),
      ),
    );
  }

  void _press(ExtraKey key) {
    final t = terminal();
    keys.sendKey(
      key,
      applicationCursor: t.cursorKeysMode,
      // The renderer's output path: the same way as a key of the renderer, without the paste
      // guard of typed text.
      send: (data) => t.onOutput?.call(data),
    );
  }

  Widget _key(String visible, String name, ExtraKey key) =>
      _KeyButton(visible: visible, name: name, onTap: () => _press(key));

  Widget _modifier(
    AppLocalizations l,
    String visible,
    String name,
    ModifierLatch state,
    VoidCallback onTap,
    VoidCallback onLatch,
  ) => _KeyButton(
    visible: visible,
    name: switch (state) {
      ModifierLatch.off => name,
      ModifierLatch.oneShot => l.terminalKeyOneShot(name),
      ModifierLatch.latched => l.terminalKeyLatched(name),
    },
    onTap: onTap,
    onLongPress: onLatch,
    state: state,
  );
}

class _KeyButton extends StatelessWidget {
  const _KeyButton({
    required this.visible,
    required this.name,
    required this.onTap,
    this.onLongPress,
    this.state,
  });

  final String visible;
  final String name;
  final VoidCallback onTap;
  final VoidCallback? onLongPress;

  /// The state of a modifier key; null for the other keys.
  final ModifierLatch? state;

  @override
  Widget build(BuildContext context) {
    final c = context.dsColors;
    // At least the touch target of the density (density.target-min): on a phone a key is
    // touched, by a finger or by TalkBack and VoiceOver.
    final density = context.dsDensity;
    final height = math.max(density.controlHeight, density.targetMin);
    // components.md 12: a latched modifier is filled in color.accent.bg; a one-shot one keeps
    // the surface and gets the accent border, so both states differ from off without color
    // alone (the label names the state, too).
    final latched = state == ModifierLatch.latched;
    final oneShot = state == ModifierLatch.oneShot;
    return Padding(
      padding: const EdgeInsets.all(DsSpace.s2),
      child: Semantics(
        button: true,
        toggled: state == null ? null : state != ModifierLatch.off,
        label: name,
        // The node replaces the InkWell's semantics, so it carries the tap and the long press
        // itself: a screen reader presses and latches the key through these actions.
        onTap: onTap,
        onLongPress: onLongPress,
        excludeSemantics: true,
        child: Material(
          color: latched ? c.accentBg : c.surfaceRaised,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(DsRadius.md),
            side: BorderSide(
              color: oneShot ? c.accentBg : c.borderControl,
              width: oneShot ? 2 : 1,
            ),
          ),
          child: InkWell(
            onTap: onTap,
            onLongPress: onLongPress,
            borderRadius: BorderRadius.circular(DsRadius.md),
            // A key of the bar never takes the focus from the screen, so the soft keyboard
            // stays open while the person taps it.
            canRequestFocus: false,
            child: ConstrainedBox(
              constraints: BoxConstraints(minHeight: height, minWidth: height),
              child: Center(
                child: Padding(
                  padding: const EdgeInsets.symmetric(horizontal: DsSpace.s8),
                  child: Text(
                    visible,
                    style: dsTextStyle(DsType.label).copyWith(
                      color: latched ? c.textOnAccent : c.textPrimary,
                      decoration: oneShot ? TextDecoration.underline : null,
                    ),
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
