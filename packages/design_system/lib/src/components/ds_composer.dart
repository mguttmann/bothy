// SPDX-License-Identifier: Apache-2.0

/// The input with slash suggestions (components.md section 9).
library;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../generated/design_tokens.g.dart';
import '../icons/ds_icon.dart';
import '../theme/ds_theme.dart';
import 'ds_common.dart';
import 'ds_suggestion_list.dart';
import 'view_models.dart';

/// The message input of a channel or thread.
///
/// Anatomy of components.md 9: the field on `color.surface.input` with the border
/// `color.border.control`, `radius.lg` and at least `density.composer-min-height`, growing up to
/// [maxLines] lines (`ui.composer_max_lines`); attach on the left, voice and send on the right;
/// above it the typing line and, while [suggestions] is not empty, the [DsSuggestionList]. The
/// placeholder and field label read "Message to {target}", during an active run "Follow-up to
/// {agent}" ([followupAgentName]).
///
/// States: empty (send disabled), text, command selected ([command] with its options as
/// placeholders), incomplete options (send disabled with the reason "Missing option: {name}"),
/// [offline] (a hint that sending goes to the queue; sending stays possible) and [readOnly]
/// (the field is replaced by the note "Read only in this channel").
///
/// Keyboard: Enter sends and Shift+Enter makes a new line; with [enterSends] false the two
/// swap (`composer.enter_sends`). Arrow up in the empty field calls [onEditLast]. While
/// suggestions show, arrow up and down move the active row, Tab or Enter call
/// [onAcceptSuggestion] and Escape calls [onDismissSuggestions]. The component performs no
/// I/O and does not throttle; the caller passes the suggestions.
///
/// ```dart
/// DsComposer(targetName: channel.name, maxLines: config.composerMaxLines,
///     enterSends: settings.enterSends, onSend: outbox.send)
/// ```
class DsComposer extends StatefulWidget {
  /// Creates the input.
  const DsComposer({
    super.key,
    required this.targetName,
    required this.maxLines,
    required this.enterSends,
    required this.onSend,
    this.followupAgentName,
    this.controller,
    this.onChanged,
    this.onEditLast,
    this.onAttach,
    this.onVoice,
    this.command,
    this.offline = false,
    this.readOnly = false,
    this.typingNames = const [],
    this.suggestions = const [],
    this.onAcceptSuggestion,
    this.onDismissSuggestions,
  });

  /// The channel or thread name (a foreign name).
  final String targetName;

  /// `ui.composer_max_lines`: lines the field grows to before it scrolls.
  final int maxLines;

  /// `composer.enter_sends`: whether Enter sends (else Shift+Enter sends).
  final bool enterSends;

  /// Called with the text when the person sends.
  final void Function(String text) onSend;

  /// The agent of the active run, or null; switches the placeholder to the follow-up form.
  final String? followupAgentName;

  /// The text controller, or null for an own one, which the composer clears after a send.
  final TextEditingController? controller;

  /// Called on every change of the text.
  final ValueChanged<String>? onChanged;

  /// Arrow up in the empty field: edit the own last message.
  final VoidCallback? onEditLast;

  /// Opens the attachment picker, or null to hide the button.
  final VoidCallback? onAttach;

  /// Starts a voice recording, or null to hide the button.
  final VoidCallback? onVoice;

  /// The chosen command, or null.
  final DsComposerCommand? command;

  /// Whether the device is offline.
  final bool offline;

  /// Whether the person may not write here.
  final bool readOnly;

  /// The people typing now (foreign names).
  final List<String> typingNames;

  /// The suggestions to show; empty hides the list.
  final List<DsSuggestion> suggestions;

  /// Called when a suggestion is accepted.
  final ValueChanged<DsSuggestion>? onAcceptSuggestion;

  /// Called on Escape while suggestions show.
  final VoidCallback? onDismissSuggestions;

  @override
  State<DsComposer> createState() => DsComposerState();
}

/// State of [DsComposer]: the controller, the focus and the active suggestion.
class DsComposerState extends State<DsComposer> {
  TextEditingController? _own;
  late final FocusNode _focus = FocusNode(
    debugLabel: 'composer',
    onKeyEvent: _onKey,
  );
  int _active = 0;

  TextEditingController get _text =>
      widget.controller ?? (_own ??= TextEditingController());

  @override
  void initState() {
    super.initState();
    _text.addListener(_changed);
  }

  @override
  void didUpdateWidget(DsComposer oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.controller != widget.controller) {
      (oldWidget.controller ?? _own)?.removeListener(_changed);
      _text.addListener(_changed);
    }
    if (_active >= widget.suggestions.length) _active = 0;
  }

  @override
  void dispose() {
    _text.removeListener(_changed);
    _own?.dispose();
    _focus.dispose();
    super.dispose();
  }

  void _changed() => setState(() {});

  String? get _missingOption {
    for (final o in widget.command?.options ?? const <DsCommandOption>[]) {
      if (o.missing) return o.name;
    }
    return null;
  }

  bool get _canSend => _text.text.trim().isNotEmpty && _missingOption == null;

  void _send() {
    if (!_canSend) return;
    widget.onSend(_text.text);
    // Only the own controller is cleared: a caller's controller stays with the caller, which
    // decides when its text goes (D-M4-07-11).
    if (widget.controller == null) _own?.clear();
  }

  void _newline() {
    final value = _text.value;
    final sel = value.selection.isValid
        ? value.selection
        : TextSelection.collapsed(offset: value.text.length);
    final text = value.text.replaceRange(sel.start, sel.end, '\n');
    _text.value = TextEditingValue(
      text: text,
      selection: TextSelection.collapsed(offset: sel.start + 1),
    );
  }

  KeyEventResult _onKey(FocusNode node, KeyEvent event) {
    if (event is! KeyDownEvent && event is! KeyRepeatEvent) {
      return KeyEventResult.ignored;
    }
    // While an input method composes (Japanese, Chinese, Korean), Enter, Tab, the arrows and
    // Escape belong to the input method: they confirm or move the candidate.
    final composing = _text.value.composing;
    if (composing.isValid && !composing.isCollapsed) {
      return KeyEventResult.ignored;
    }
    final key = event.logicalKey;
    final shift = HardwareKeyboard.instance.isShiftPressed;
    final list = widget.suggestions;
    if (list.isNotEmpty) {
      if (key == LogicalKeyboardKey.arrowDown) {
        setState(() => _active = (_active + 1) % list.length);
        return KeyEventResult.handled;
      }
      if (key == LogicalKeyboardKey.arrowUp) {
        setState(() => _active = (_active - 1 + list.length) % list.length);
        return KeyEventResult.handled;
      }
      if (key == LogicalKeyboardKey.tab ||
          (key == LogicalKeyboardKey.enter && !shift)) {
        widget.onAcceptSuggestion?.call(list[_active]);
        return KeyEventResult.handled;
      }
      if (key == LogicalKeyboardKey.escape) {
        widget.onDismissSuggestions?.call();
        return KeyEventResult.handled;
      }
    }
    if (key == LogicalKeyboardKey.enter ||
        key == LogicalKeyboardKey.numpadEnter) {
      if (shift == widget.enterSends) {
        _newline();
      } else {
        _send();
      }
      return KeyEventResult.handled;
    }
    if (key == LogicalKeyboardKey.arrowUp && _text.text.isEmpty) {
      widget.onEditLast?.call();
      return widget.onEditLast == null
          ? KeyEventResult.ignored
          : KeyEventResult.handled;
    }
    return KeyEventResult.ignored;
  }

  Widget _iconButton(
    DsGlyph glyph,
    String label,
    VoidCallback? onTap,
    Color color,
  ) {
    return DsTappable(
      onTap: onTap,
      label: label,
      excludeChildSemantics: true,
      child: Center(
        widthFactor: 1,
        heightFactor: 1,
        child: DsIcon(
          glyph,
          size: DsIconSizeRole.md,
          color: color,
          decorative: true,
        ),
      ),
    );
  }

  Widget _note(DsGlyph glyph, String text, Color color) {
    return Padding(
      padding: const EdgeInsets.only(bottom: DsSpace.s4),
      child: Row(
        children: [
          DsIcon(
            glyph,
            size: DsIconSizeRole.sm,
            color: color,
            decorative: true,
          ),
          const SizedBox(width: DsSpace.s6),
          Expanded(
            child: Text(
              text,
              style: dsTextStyle(DsType.caption).copyWith(color: color),
            ),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l = dsL10n(context);
    final c = context.dsColors;
    final density = context.dsDensity;
    final shape = BorderRadius.circular(DsRadius.lg);
    if (widget.readOnly) {
      return Container(
        constraints: BoxConstraints(minHeight: density.composerMinHeight),
        padding: const EdgeInsets.symmetric(horizontal: DsSpace.s12),
        decoration: BoxDecoration(
          color: c.surfaceSunken,
          borderRadius: shape,
          border: Border.all(
            color: c.borderSubtle,
            width: DsBorderWidth.hairline,
          ),
        ),
        child: Row(
          children: [
            DsIcon(
              DsIcons.securityLock,
              size: DsIconSizeRole.sm,
              color: c.textSecondary,
              decorative: true,
            ),
            const SizedBox(width: DsSpace.s8),
            Expanded(
              child: Text(
                l.composerReadOnly,
                style: dsTextStyle(DsType.bodySm)
                    .copyWith(color: c.textSecondary),
              ),
            ),
          ],
        ),
      );
    }
    final placeholder = widget.followupAgentName == null
        ? l.composerMessageTo(dsIsolate(widget.targetName))
        : l.composerFollowupTo(dsIsolate(widget.followupAgentName!));
    final fieldLabel = widget.followupAgentName == null
        ? l.composerMessageTo(widget.targetName)
        : l.composerFollowupTo(widget.followupAgentName!);
    final typing = widget.typingNames;
    final missing = _missingOption;
    final command = widget.command;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        if (typing.isNotEmpty)
          Padding(
            padding: const EdgeInsets.only(bottom: DsSpace.s4),
            child: Semantics(
              liveRegion: true,
              child: Text(
                typing.length == 1
                    ? l.composerTyping(dsIsolate(typing.single))
                    : l.composerTypingSeveral,
                style: dsTextStyle(DsType.caption)
                    .copyWith(color: c.textSecondary),
              ),
            ),
          ),
        if (widget.suggestions.isNotEmpty)
          Padding(
            padding: const EdgeInsets.only(bottom: DsSpace.s4),
            child: DsSuggestionList(
              widget.suggestions,
              activeIndex: _active,
              onAccept: (s) => widget.onAcceptSuggestion?.call(s),
            ),
          ),
        if (widget.offline)
          _note(DsIcons.stateOffline, l.composerOffline, c.warningFg),
        if (missing != null)
          _note(
            DsIcons.stateWarning,
            l.composerOptionMissing(missing),
            c.warningFg,
          ),
        if (command != null && command.options.isNotEmpty)
          Padding(
            padding: const EdgeInsets.only(bottom: DsSpace.s4),
            child: Wrap(
              spacing: DsSpace.s6,
              runSpacing: DsSpace.s4,
              children: [
                Text(
                  command.name,
                  style: dsTextStyle(DsType.codeSm)
                      .copyWith(color: c.textPrimary),
                ),
                for (final o in command.options)
                  Text(
                    o.value == null || o.value!.isEmpty
                        ? o.name
                        : '${o.name}: ${o.value}',
                    style: dsTextStyle(DsType.codeSm).copyWith(
                      color: o.value == null || o.value!.isEmpty
                          ? c.textSecondary
                          : c.textPrimary,
                    ),
                  ),
              ],
            ),
          ),
        Container(
          constraints: BoxConstraints(minHeight: density.composerMinHeight),
          decoration: BoxDecoration(
            color: c.surfaceInput,
            borderRadius: shape,
            border: Border.all(
              color: c.borderControl,
              width: DsBorderWidth.hairline,
            ),
          ),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              if (widget.onAttach != null)
                _iconButton(
                  DsIcons.actionAttach,
                  l.composerAttach,
                  widget.onAttach,
                  c.textSecondary,
                ),
              Expanded(
                child: Padding(
                  padding: const EdgeInsets.symmetric(horizontal: DsSpace.s12),
                  child: Semantics(
                    label: fieldLabel,
                    child: TextField(
                      controller: _text,
                      focusNode: _focus,
                      onChanged: widget.onChanged,
                      minLines: 1,
                      maxLines: widget.maxLines,
                      keyboardType: TextInputType.multiline,
                      style: dsTextStyle(DsType.body)
                          .copyWith(color: c.textPrimary),
                      // The field itself is the tap target, so it is at least
                      // density.target-min high; the padding sits inside it.
                      decoration: InputDecoration(
                        border: InputBorder.none,
                        isDense: true,
                        contentPadding: const EdgeInsets.symmetric(
                          vertical: DsSpace.s8,
                        ),
                        constraints: BoxConstraints(
                          minHeight: density.targetMin,
                        ),
                        hintText: placeholder,
                        hintStyle: dsTextStyle(DsType.body)
                            .copyWith(color: c.textSecondary),
                      ),
                    ),
                  ),
                ),
              ),
              if (widget.onVoice != null)
                _iconButton(
                  DsIcons.actionVoice,
                  l.composerVoice,
                  widget.onVoice,
                  c.textSecondary,
                ),
              Semantics(
                hint: missing == null ? null : l.composerOptionMissing(missing),
                child: _iconButton(
                  DsIcons.actionSend,
                  l.composerSend,
                  _canSend ? _send : null,
                  _canSend ? context.dsAccent.accentBg : c.textMuted,
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}
