// SPDX-License-Identifier: Apache-2.0

/// The state of the composer of a thread (brief M4-20 Design note 3; components.md 9; 04 12,
/// 04 14): the text, the reply being prepared, the mention suggestions, the draft sync and the
/// send path with the content limit of the instance.
library;

import 'dart:async';
import 'dart:convert';

import 'package:domain/domain.dart';
import 'package:flutter/widgets.dart';
import 'package:sync_engine/sync_engine.dart';

import '../thread/thread_actions.dart';
import 'draft_sync.dart';
import 'mention_suggestions.dart';

/// Why a send was refused before it reached the outbox.
sealed class ComposerError {
  const ComposerError();
}

/// The content is larger than `message_max_content_bytes` of the instance (D-W-115).
final class ContentTooLong extends ComposerError {
  /// Creates the error.
  const ContentTooLong(this.limit);

  /// The limit in bytes.
  final int limit;
}

/// The composer of one thread.
///
/// Every change of the text by the person ([onChanged]) marks the draft as being edited
/// (`DraftEditing`, 04 14), writes it through the [DraftWriter], sends `typing.start` through
/// the throttle (or `typing.stop` when the field is cleared, 04 12) and updates the mention
/// suggestions. [send] checks the limit, puts the message into the outbox with `clear_draft`,
/// drops the waiting draft write, sends `typing.stop` and clears the field.
///
/// Example:
/// ```dart
/// final composer = ComposerController(thread: t, channel: c, actions: a, drafts: d,
///     editing: registry, typing: engine.typing);
/// composer.onChanged('Hello @sa');
/// ```
final class ComposerController extends ChangeNotifier {
  /// Creates the controller of [thread] in [channel].
  ComposerController({
    required this.thread,
    required this.channel,
    required this._actions,
    required this._drafts,
    required this._editing,
    this._typing,
  });

  /// The thread.
  final ThreadId thread;

  /// Its channel.
  final ChannelId channel;

  final ThreadActions _actions;
  final DraftWriter _drafts;
  final DraftEditingRegistry _editing;
  final TypingThrottle? _typing;

  /// The text of the field.
  final TextEditingController text = TextEditingController();

  /// The limits of the instance, or null while unknown (the server checks then).
  InstanceLimits? limits;

  /// The stored principals (mention suggestions).
  Map<PrincipalId, Principal> principals = const {};

  /// The refusal of the last send, or null.
  ComposerError? error;

  /// The message a reply quotes, or null.
  MessageId? replyTo;

  /// The author of [replyTo].
  String? replyName;

  /// The mention suggestions for the token at the cursor.
  List<Principal> suggestions = const [];

  ResourceVersion? _draftVersion;
  bool _focused = false;
  bool _dirty = false;

  /// Whether the person changed the text since the stored draft was applied.
  bool get dirty => _dirty;

  /// The field got or lost the focus; without it the person is not typing (04 14), so a
  /// draft of another device may be applied again.
  void setFocused(bool focused) {
    _focused = focused;
    if (!focused) _dirty = false;
    _markEditing();
  }

  void _markEditing() =>
      _editing.setEditing(channel, thread, editing: _focused && _dirty);

  /// The person changed the text to [value].
  void onChanged(String value) {
    _dirty = true;
    error = null;
    _markEditing();
    final typing = _typing;
    if (typing != null) {
      if (value.isEmpty) {
        typing.onCleared(thread);
      } else {
        typing.onInput(thread);
      }
    }
    unawaited(_writeDraft(value));
    _suggest();
    notifyListeners();
  }

  Future<void> _writeDraft(String value) =>
      _drafts.write(value, replyTo: replyTo, baseVersion: _draftVersion);

  void _suggest() {
    final cursor = text.selection.isValid
        ? text.selection.baseOffset
        : text.text.length;
    final query = mentionQuery(text.text, cursor);
    suggestions = query == null
        ? const []
        : suggestMentions(principals.values, query.prefix);
  }

  /// Applies the stored [draft] (another device's change, or the draft at opening) unless the
  /// person is typing in it or a draft write of this device has not been confirmed (04 14: the
  /// next local write wins then, and a stale base version reports `overwrote_newer`). The
  /// version is taken over only with the text, or when the text is already the field's.
  Future<void> applyStored(Draft? draft) async {
    final call = ++_applyCalls;
    if (_typingNow) return;
    // An own write that is not confirmed is newer than any stored draft; its answer, which can
    // arrive after a send, must not bring the sent text back.
    if (await _drafts.hasUnsent() || call != _applyCalls || _typingNow) return;
    final next = draft?.contentMd ?? '';
    if (draft != null) _draftVersion = draft.version;
    if (next == text.text) return;
    text.value = TextEditingValue(
      text: next,
      selection: TextSelection.collapsed(offset: next.length),
    );
    replyTo = draft?.replyToMessageId;
    notifyListeners();
  }

  int _applyCalls = 0;

  bool get _typingNow =>
      _dirty || _editing.isEditing(channelId: channel, threadId: thread);

  /// Puts the text of a draft write that was not confirmed back into the field (the composer
  /// opened again, or the app started again while offline, 04 13); the stored draft is older.
  Future<void> restoreUnsent() async {
    final unsent = await _drafts.restore();
    if (unsent == null || _typingNow || unsent == text.text) return;
    text.value = TextEditingValue(
      text: unsent,
      selection: TextSelection.collapsed(offset: unsent.length),
    );
    notifyListeners();
  }

  /// Accepts the suggestion [principal]: the token becomes `@handle `.
  void acceptSuggestion(Principal principal) {
    final cursor = text.selection.isValid
        ? text.selection.baseOffset
        : text.text.length;
    final query = mentionQuery(text.text, cursor);
    if (query == null) return;
    final (next, at) = insertMention(
      text.text,
      cursor,
      query,
      principal.handle,
    );
    text.value = TextEditingValue(
      text: next,
      selection: TextSelection.collapsed(offset: at),
    );
    suggestions = const [];
    onChanged(next);
  }

  /// Hides the suggestions (Escape).
  void dismissSuggestions() {
    suggestions = const [];
    notifyListeners();
  }

  /// Prepares a reply to [message] of [authorName]: its first lines as a block quote before
  /// the text, and `reply_to_message_id` (brief M4-20 Design note 4).
  void startReply(Message message, String authorName) {
    replyTo = message.id;
    replyName = authorName;
    final lines = message.contentMd.trim().split('\n').take(_quoteLines);
    final quote = '${[for (final line in lines) '> $line'].join('\n')}\n\n';
    insert(quote, atStart: true);
  }

  /// The number of lines a reply quotes; the rest is reachable through the reply reference.
  static const int _quoteLines = 3;

  /// Drops the prepared reply.
  void cancelReply() {
    replyTo = null;
    replyName = null;
    unawaited(_writeDraft(text.text));
    notifyListeners();
  }

  /// Inserts [content] at the cursor (a template, 01 3.3), or before the text with [atStart].
  void insert(String content, {bool atStart = false}) {
    final value = text.value;
    final at = atStart
        ? 0
        : value.selection.isValid
        ? value.selection.end
        : value.text.length;
    final next = value.text.replaceRange(at, at, content);
    text.value = TextEditingValue(
      text: next,
      selection: TextSelection.collapsed(
        offset: atStart ? next.length : at + content.length,
      ),
    );
    onChanged(next);
  }

  /// Sends [value]; returns false and sets [error] when it is over the limit of the
  /// instance (checked before sending, D-W-115).
  Future<bool> send(String value) async {
    final body = value.trim();
    if (body.isEmpty) return false;
    final max = limits?.messageMaxContentBytes;
    if (max != null && utf8.encode(body).length > max) {
      error = ContentTooLong(max);
      notifyListeners();
      return false;
    }
    final reply = replyTo;
    text.clear();
    replyTo = null;
    replyName = null;
    suggestions = const [];
    error = null;
    _dirty = false;
    _markEditing();
    _typing?.onSent(thread);
    notifyListeners();
    await _drafts.dropWaiting();
    await _actions.send(thread, body, replyTo: reply);
    return true;
  }

  @override
  void dispose() {
    _editing.setEditing(channel, thread, editing: false);
    text.dispose();
    super.dispose();
  }
}
