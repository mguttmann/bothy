// SPDX-License-Identifier: Apache-2.0

/// The state of the composer of a thread (brief M4-20 Design note 3; components.md 9; 04 12,
/// 04 14): the text, the reply being prepared, the mention suggestions, the draft sync and the
/// send path with the content limit of the instance. A long paste becomes a text file and a
/// draft over the limit goes out as one message with its rest as one attachment (M4-47,
/// 01 6.13).
library;

import 'dart:async';

import 'package:domain/domain.dart';
import 'package:flutter/widgets.dart';
import 'package:sync_engine/sync_engine.dart';

import '../files/attach_controller.dart';
import '../files/upload_executor.dart' show MessageAttachments;
import '../telemetry/measurements.dart';
import '../thread/thread_actions.dart';
import 'draft_sync.dart';
import 'long_text.dart';
import 'mention_suggestions.dart';

/// Why a send was refused before it reached the outbox.
sealed class ComposerError {
  const ComposerError();
}

/// The content is larger than `message_max_content_bytes` of the instance (D-W-115) and its
/// rest could not become an attachment (no file support here, or the attachment was refused;
/// M4-47).
final class ContentTooLong extends ComposerError {
  /// Creates the error.
  const ContentTooLong(this.limit);

  /// The limit in bytes.
  final int limit;
}

/// The composer of one thread, or of the first message of a new post (brief M4-41: [thread]
/// is null, the draft is the forum's draft of a new post and [send] hands the text to
/// [sendNew]).
///
/// Every change of the text by the person ([onChanged]) marks the draft as being edited
/// (`DraftEditing`, 04 14), writes it through the [DraftWriter], sends `typing.start` through
/// the throttle (or `typing.stop` when the field is cleared, 04 12) and updates the mention
/// suggestions. A change that inserts more than [pasteRule] allows (a long paste) takes the
/// inserted text out of the field and attaches it as a file through [attach] (M4-47).
/// [send] checks the limit (a draft over it keeps its head and sends the rest as the
/// attachment [overflowFileName]), puts the message into the outbox with `clear_draft`,
/// drops the waiting draft write, sends `typing.stop` and clears the field.
///
/// Example:
/// ```dart
/// final composer = ComposerController(thread: t, channel: c, actions: a, drafts: d,
///     editing: registry, typing: engine.typing);
/// composer.onChanged('Hello @sa');
/// ```
final class ComposerController extends ChangeNotifier {
  /// Creates the controller of [thread] in [channel]; without a thread, [sendNew] sends the
  /// first message of a new post in [channel] (no typing indicator: there is no thread yet).
  ComposerController({
    required this.thread,
    required this.channel,
    required this._actions,
    required this._drafts,
    required this._editing,
    this._typing,
    this.deviceId,
    this.measurements,
    this.sendNew,
  });

  /// Sends the first message of a new post (the text, trimmed, and the attachments of
  /// [attach]); used when [thread] is null.
  final Future<void> Function(String body, MessageAttachments? attachments)?
  sendNew;

  /// The measurement points of the app metrics: a send reports `client.send_local.duration`
  /// (brief M4-31), or nothing when null.
  final AppMeasurements? measurements;

  /// This device, so the answer of an own draft write moves the base version even while the
  /// person types (04 14: `overwrote_newer` only for another device's newer write).
  final DeviceId? deviceId;

  /// The thread, or null for the first message of a new post.
  final ThreadId? thread;

  /// Its channel.
  final ChannelId channel;

  final ThreadActions? _actions;
  final DraftWriter _drafts;
  final DraftEditingRegistry _editing;
  final TypingThrottle? _typing;

  /// The text of the field.
  final TextEditingController text = TextEditingController();

  /// The limits of the instance, or null while unknown (the server checks then).
  InstanceLimits? get limits => _limits;
  set limits(InstanceLimits? value) {
    _limits = value;
    attach?.limits = value;
  }

  InstanceLimits? _limits;

  /// The attachments of the composer (M4-22), or null without file support.
  AttachController? attach;

  /// When a paste becomes a file (`ui.composer_paste_file_chars`,
  /// `ui.composer_paste_file_lines`), or null to keep every paste as text.
  PasteRule? pasteRule;

  /// The text of the field as this controller last saw it; a change is compared with it to
  /// find the inserted text of a paste.
  String _seen = '';

  /// Whether the draft is larger than one message may carry and has file support, so the
  /// send keeps the head and attaches the rest (the note before sending, M4-47).
  bool get overflows {
    final max = limits?.messageMaxContentBytes;
    return max != null && attach != null && exceedsBytes(text.text.trim(), max);
  }

  /// Sets the field to [next] with the cursor at [offset] (the end without one), as text this
  /// controller put there, never a paste.
  void _setText(String next, {int? offset}) {
    _seen = next;
    text.value = TextEditingValue(
      text: next,
      selection: TextSelection.collapsed(offset: offset ?? next.length),
    );
  }

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

  /// The person changed the text to [value]; a long paste in it becomes a file (M4-47).
  void onChanged(String changed) {
    final previous = _seen;
    var value = _seen = changed;
    final rule = pasteRule;
    final files = attach;
    if (rule != null && files != null) {
      final paste = insertionOf(previous, value);
      if (paste != null && rule.isLong(paste.inserted)) {
        value = value.replaceRange(
          paste.start,
          paste.start + paste.inserted.length,
          '',
        );
        _setText(value, offset: paste.start);
        unawaited(_attachPaste(files, paste));
      }
    }
    _dirty = true;
    error = null;
    _markEditing();
    final typing = _typing;
    final thread = this.thread;
    if (typing != null && thread != null) {
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

  /// Attaches the long [paste] as a file; a refused file (count or size) goes back into the
  /// field as text, and the refusal shows its reason.
  Future<void> _attachPaste(AttachController files, TextInsertion paste) async {
    final file = pasteFileOf(paste.inserted);
    final ok = await files.addText(
      file.name,
      file.content,
      mimeType: file.mimeType,
      pastedText: paste.inserted,
    );
    if (!ok) _insertAt(paste.inserted, paste.start);
  }

  /// "Insert as text" on the chip of the long paste [key]: the file goes and its text is
  /// inserted at the cursor (M4-47).
  Future<void> insertPasteAsText(String key) async {
    final files = attach;
    if (files == null) return;
    String? pasted;
    for (final a in files.attachments) {
      if (a.key == key) pasted = a.pastedText;
    }
    if (pasted == null) return;
    await files.remove(key);
    insert(pasted);
  }

  void _insertAt(String content, int at) {
    final current = text.text;
    final where = at.clamp(0, current.length);
    final next = current.replaceRange(where, where, content);
    _setText(next, offset: where + content.length);
    onChanged(next);
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
    // The answer of this device's own write: the next write builds on its version even while
    // the person types; the text is the person's own and stays as it is.
    final own = draft != null && _isOwn(draft);
    if (own) _draftVersion = draft.version;
    if (_typingNow) return;
    // An own write that is not confirmed is newer than any stored draft; its answer, which can
    // arrive after a send, must not bring the sent text back.
    if (await _drafts.hasUnsent() || call != _applyCalls || _typingNow) return;
    // Nothing unconfirmed and nobody typing: the stored draft, own or not, is the text (a
    // composer opened again shows the draft this device saved).
    final next = draft?.contentMd ?? '';
    if (draft != null) _draftVersion = draft.version;
    if (next == text.text) return;
    _setText(next);
    replyTo = draft?.replyToMessageId;
    notifyListeners();
  }

  int _applyCalls = 0;

  bool _isOwn(Draft draft) {
    final own = deviceId;
    return own != null && draft.updatedByDeviceId == own;
  }

  bool get _typingNow =>
      _dirty || _editing.isEditing(channelId: channel, threadId: thread);

  /// Puts the text of a draft write that was not confirmed back into the field (the composer
  /// opened again, or the app started again while offline, 04 13); the stored draft is older.
  Future<void> restoreUnsent() async {
    final unsent = await _drafts.restore();
    if (unsent == null || _typingNow || unsent == text.text) return;
    _setText(unsent);
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
    _setText(next, offset: at);
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
    // A template, a quote or the text of a paste the person chose to keep: never a file.
    _setText(next, offset: atStart ? next.length : at + content.length);
    onChanged(next);
  }

  /// Whether [send] writes the rest of a long text as [overflowFileName] (M4-47).
  bool _overflowing = false;

  /// Sends [value] with the attachments of [attach]. Over the limit of the instance
  /// (D-W-115) the message keeps the head and the rest becomes the attachment
  /// [overflowFileName] of the same message (M4-47, D-M4-47-3); only when that attachment
  /// cannot be made it returns false and sets [error]. A message of attachments only has no
  /// text.
  Future<bool> send(String value) async {
    // A second send while the rest of a long text is written would send it twice.
    if (_overflowing) return false;
    var body = value.trim();
    final hasFiles = attach?.attachments.isNotEmpty ?? false;
    if (body.isEmpty && !hasFiles) return false;
    final started = measurements?.sendStarted();
    final max = limits?.messageMaxContentBytes;
    if (max != null && exceedsBytes(body, max)) {
      final files = attach;
      final cut = splitForLimit(body, max);
      final bool attached;
      _overflowing = true;
      try {
        attached =
            files != null &&
            await files.addText(
              overflowFileName,
              cut.overflow!,
              mimeType: 'text/plain',
            );
      } finally {
        _overflowing = false;
      }
      if (!attached) {
        error = ContentTooLong(max);
        notifyListeners();
        return false;
      }
      // Head and attachment together are exactly the text (D-M4-47-3).
      body = cut.head.trim().isEmpty ? '' : cut.head;
    }
    final reply = replyTo;
    _seen = '';
    text.clear();
    replyTo = null;
    replyName = null;
    suggestions = const [];
    error = null;
    _dirty = false;
    _markEditing();
    final thread = this.thread;
    if (thread != null) _typing?.onSent(thread);
    notifyListeners();
    final files = attach?.take();
    await _drafts.dropWaiting();
    if (thread == null) {
      await sendNew?.call(body, files);
    } else {
      await _actions?.send(thread, body, replyTo: reply, attachments: files);
    }
    // The outbox write is done; the history shows it in the next frame.
    if (started != null) measurements!.sendShown(started);
    return true;
  }

  /// Empties the field after a slash command went out (M4-21): a command is no message, so
  /// the server does not clear the draft with it; the empty text is written as the draft and
  /// the typing indicator ends.
  void clearAfterCommand() {
    _seen = '';
    text.clear();
    suggestions = const [];
    error = null;
    onChanged('');
    _dirty = false;
    _markEditing();
    notifyListeners();
  }

  @override
  void dispose() {
    _editing.setEditing(channel, thread, editing: false);
    text.dispose();
    super.dispose();
  }
}
