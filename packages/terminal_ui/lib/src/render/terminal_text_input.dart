// SPDX-License-Identifier: Apache-2.0

/// The text input client of the terminal on the phones (docs/spec/15-client-architecture.md
/// 23.4; brief M5-12 fixed requirement 15 and Design note 3; docs/decisions.d/M5-12.md
/// D-M5-12-2): the soft keyboard feeds the terminal through a client that asks the system for
/// no suggestions, no autocorrection, no smart punctuation and no personalized learning, so no
/// terminal input enters the keyboard's dictionary.
///
/// xterm 4.0.0 attaches its own client with suggestions and learning off, but with smart dashes
/// and quotes on (the defaults of `TextInputConfiguration`) and without a way to change them, so
/// on iOS and Android the renderer reads the hardware keyboard only and this client takes the
/// soft keyboard. A third-party keyboard may still record what is typed; the app cannot
/// prevent that (residual risk, D-M5-12-2).
library;

import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:flutter/widgets.dart';
import 'package:xterm/xterm.dart' show Terminal, TerminalKey;

/// Whether the terminal on [platform] takes the soft keyboard through [TerminalTextInput]:
/// the phones, iOS and Android.
bool privateTextInputOn(TargetPlatform platform) =>
    platform == TargetPlatform.iOS || platform == TargetPlatform.android;

/// The configuration of the client on [platform].
///
/// Android gets `visiblePassword`, which keeps the IME from predicting and learning even when
/// it ignores the flags; iOS keeps the plain text keyboard, because its `visiblePassword` is
/// the ASCII keyboard without the person's languages and dictation (R-PR-11), and the flags
/// switch the QuickType bar, the autocorrection and the smart punctuation off there.
TextInputConfiguration terminalTextInputConfiguration(
  TargetPlatform platform,
) => TextInputConfiguration(
  inputType: platform == TargetPlatform.android
      ? TextInputType.visiblePassword
      : TextInputType.text,
  inputAction: TextInputAction.newline,
  autocorrect: false,
  enableSuggestions: false,
  enableIMEPersonalizedLearning: false,
  smartDashesType: SmartDashesType.disabled,
  smartQuotesType: SmartQuotesType.disabled,
  keyboardAppearance: Brightness.dark,
  autofillConfiguration: AutofillConfiguration.disabled,
);

// The text the client keeps in the field: two characters before the caret, so a backspace of
// the soft keyboard shortens it (it would change nothing on an empty field) and every change is
// a difference against this value.
const TextEditingValue _sentinel = TextEditingValue(
  text: '  ',
  selection: TextSelection.collapsed(offset: 2),
);

/// Feeds the soft keyboard into [terminal] while [focusNode] has the focus.
///
/// Typed text goes through `Terminal.textInput` (so the extra keys bar's modifiers and the
/// paste guard of 15 23.4 apply), a deletion sends backspace, return sends return, and an
/// input method's composition is sent only when it ends. The field is put back to its sentinel
/// after every change, so it never holds terminal input.
///
/// ```dart
/// TerminalTextInput(focusNode: focus, terminal: () => screen.terminal, child: view)
/// ```
class TerminalTextInput extends StatefulWidget {
  /// Creates the client around [child].
  const TerminalTextInput({
    super.key,
    required this.focusNode,
    required this.terminal,
    required this.child,
  });

  /// The focus node of the renderer; the client attaches while it has the focus.
  final FocusNode focusNode;

  /// The current terminal model (a replay replaces it).
  final Terminal Function() terminal;

  /// The renderer.
  final Widget child;

  @override
  TerminalTextInputState createState() => TerminalTextInputState();
}

/// The state of [TerminalTextInput]; [show] opens the soft keyboard again.
class TerminalTextInputState extends State<TerminalTextInput>
    with TextInputClient {
  TextInputConnection? _connection;
  TextEditingValue _value = _sentinel;

  @override
  void initState() {
    super.initState();
    widget.focusNode.addListener(_onFocus);
  }

  @override
  void didUpdateWidget(TerminalTextInput oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.focusNode != widget.focusNode) {
      oldWidget.focusNode.removeListener(_onFocus);
      widget.focusNode.addListener(_onFocus);
    }
  }

  @override
  void dispose() {
    widget.focusNode.removeListener(_onFocus);
    _close();
    super.dispose();
  }

  /// Shows the soft keyboard: attaches when the renderer has the focus, otherwise asks for it.
  void show() {
    if (!widget.focusNode.hasFocus) {
      widget.focusNode.requestFocus();
      return;
    }
    _open();
  }

  void _onFocus() {
    if (widget.focusNode.hasFocus) {
      _open();
    } else {
      _close();
    }
  }

  void _open() {
    final connection = _connection;
    if (connection != null && connection.attached) {
      connection.show();
      return;
    }
    _value = _sentinel;
    _connection =
        TextInput.attach(
            this,
            terminalTextInputConfiguration(defaultTargetPlatform),
          )
          ..show()
          ..setEditingState(_value);
  }

  void _close() {
    final connection = _connection;
    _connection = null;
    if (connection != null && connection.attached) connection.close();
  }

  @override
  TextEditingValue? get currentTextEditingValue => _value;

  @override
  AutofillScope? get currentAutofillScope => null;

  @override
  void updateEditingValue(TextEditingValue value) {
    _value = value;
    // An input method composes: nothing goes out until the composition ends.
    if (!value.composing.isCollapsed) return;
    final text = value.text;
    final terminal = widget.terminal();
    if (text.length < _sentinel.text.length) {
      for (var i = text.length; i < _sentinel.text.length; i++) {
        terminal.keyInput(TerminalKey.backspace);
      }
    } else if (text != _sentinel.text) {
      // The keyboard may have replaced the sentinel; what follows it is the new input.
      final inserted = text.startsWith(_sentinel.text)
          ? text.substring(_sentinel.text.length)
          : text.trimLeft();
      _insert(terminal, inserted);
    }
    if (text != _sentinel.text || value.selection != _sentinel.selection) {
      _value = _sentinel;
      _connection?.setEditingState(_sentinel);
    }
  }

  void _insert(Terminal terminal, String text) {
    if (text.isEmpty) return;
    if (text == '\n' || text == '\r') {
      terminal.keyInput(TerminalKey.enter);
      return;
    }
    terminal.textInput(text);
  }

  @override
  void performAction(TextInputAction action) {
    switch (action) {
      case TextInputAction.newline ||
          TextInputAction.done ||
          TextInputAction.go ||
          TextInputAction.send:
        widget.terminal().keyInput(TerminalKey.enter);
      default:
        break;
    }
  }

  @override
  void connectionClosed() {
    _connection = null;
    _value = _sentinel;
  }

  @override
  void updateFloatingCursor(RawFloatingCursorPoint point) {}

  @override
  void showAutocorrectionPromptRect(int start, int end) {}

  @override
  void performPrivateCommand(String action, Map<String, dynamic> data) {}

  @override
  void insertTextPlaceholder(Size size) {}

  @override
  void removeTextPlaceholder() {}

  @override
  void showToolbar() {}

  @override
  Widget build(BuildContext context) => widget.child;
}
