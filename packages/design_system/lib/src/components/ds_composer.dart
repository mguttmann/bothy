// SPDX-License-Identifier: Apache-2.0

/// The input with slash suggestions (components.md section 9).
library;

import 'dart:math' as math;

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
/// [maxLines] lines (`ui.composer_max_lines`) and then scrolling; attach on the left, the
/// caller's [trailing] control, voice and send on the right. The text field fills the field
/// between the buttons and is the tap target there, with `density.composer-pad-x` inside it;
/// the text starts at the left and a single line sits on the middle of the buttons (M5-17);
/// above it the typing line and, while [suggestions] is not empty, the [DsSuggestionList]. The
/// placeholder and field label read "Message to {target}", during an active run "Follow-up to
/// {agent}" ([followupAgentName]).
///
/// States: empty (send disabled), text, command selected ([command] with its options as
/// placeholders), incomplete options (send disabled with the reason "Missing option: {name}"),
/// [offline] (a hint that sending goes to the queue; sending stays possible) and [readOnly]
/// (the field is replaced by the note "Read only in this channel"). With [sendEnabled] false
/// the send button is disabled and Enter does not send; the text stays.
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
    this.trailing,
    this.hasAttachments = false,
    this.sendEnabled = true,
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

  /// Called on every change of the text, also the new line and the clearing after a send that
  /// the composer makes itself.
  final ValueChanged<String>? onChanged;

  /// Arrow up in the empty field: edit the own last message.
  final VoidCallback? onEditLast;

  /// Opens the attachment picker, or null to hide the button.
  final VoidCallback? onAttach;

  /// Starts a voice recording, or null to hide the button.
  final VoidCallback? onVoice;

  /// A control of the caller inside the field, after the text and before Send (the phone's
  /// press-and-hold microphone, brief M5-17), or null.
  final Widget? trailing;

  /// Whether files are attached: then the message may be sent without text.
  final bool hasAttachments;

  /// Whether the caller allows sending now. False disables the send button and makes Enter
  /// do nothing (the text stays in the field; Shift+Enter still makes a new line): a caller
  /// that needs more than text, for example a required title, keeps sending off until it has
  /// it. True leaves the other rules (text or attachments, no missing option) in charge.
  final bool sendEnabled;

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

  bool get _canSend =>
      widget.sendEnabled &&
      (_text.text.trim().isNotEmpty || widget.hasAttachments) &&
      _missingOption == null;

  void _send() {
    if (!_canSend) return;
    widget.onSend(_text.text);
    // Only the own controller is cleared: a caller's controller stays with the caller, which
    // decides when its text goes (D-M4-07-11).
    if (widget.controller == null && _own != null) {
      _own!.clear();
      widget.onChanged?.call('');
    }
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
    widget.onChanged?.call(text);
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
    final text = dsTextStyle(DsType.body).copyWith(color: c.textPrimary);
    // The decorator keeps a single line at the top of a field taller than the line (the
    // touch target of 48), so the padding centers it on the buttons (brief M5-17); a scaled
    // or a second line grows the field with the plain padding.
    final line =
        MediaQuery.textScalerOf(context).scale(text.fontSize!) * text.height!;
    final padY = math.max(DsSpace.s8, (density.targetMin - line) / 2);
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
          key: const ValueKey('ds-composer-field'),
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
                // A node of its own: without the container the field's semantics went into
                // the node of the column around the composer, which became one text field
                // with the column's content below it, a leaf for VoiceOver (brief M4-32).
                child: Semantics(
                  container: true,
                  label: fieldLabel,
                  child: TextField(
                    controller: _text,
                    focusNode: _focus,
                    onChanged: widget.onChanged,
                    minLines: 1,
                    maxLines: widget.maxLines,
                    keyboardType: TextInputType.multiline,
                    // Brief M5-17: the text starts at the left of the field.
                    textAlign: TextAlign.start,
                    style: text,
                    // The field itself is the tap target from button to button and at least
                    // density.target-min high: its padding (density.composer-pad-x) lies
                    // inside it, so a tap on the padding focuses it too (brief M5-17).
                    decoration: InputDecoration(
                      border: InputBorder.none,
                      isDense: true,
                      contentPadding: EdgeInsets.symmetric(
                        horizontal: density.composerPadX,
                        vertical: padY,
                      ),
                      constraints: BoxConstraints(minHeight: density.targetMin),
                      // The placeholder repeats the field's name, which the label above
                      // already gives; a screen reader reads it once (brief M4-32).
                      hint: ExcludeSemantics(
                        child: Text(
                          placeholder,
                          style: dsTextStyle(DsType.body)
                              .copyWith(color: c.textSecondary),
                        ),
                      ),
                    ),
                  ),
                ),
              ),
              ?widget.trailing,
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
