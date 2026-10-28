// SPDX-License-Identifier: Apache-2.0

/// The state of the new chat view of a forum (01 6.8 item 8; brief M4-41 Design notes 1 and 2):
/// the title and tags of the view, the composer of M4-20 on the person's draft of a new post in
/// the forum, the title rule and the send through the outbox.
library;

import 'package:domain/domain.dart';
import 'package:flutter/widgets.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../composer/composer_controller.dart';
import '../composer/draft_sync.dart';
import '../sync/thread_ops.dart';

/// The new chat of one forum.
///
/// The text is the person's draft of a new post in [forum] (03 `drafts` with `thread_id` NULL):
/// the [composer] writes it through the outbox as the composer of a thread writes a thread's
/// draft, so it survives a restart and reaches the person's other devices (04 14). Title and
/// tags live only here until the send (the server's draft has neither; D-M4-41-2).
///
/// [titleRequired] is the title rule of 09 12: with an operator agent the title may stay empty
/// (the server keeps `title_source` `auto` and the agent names the chat), without one the
/// view sends only with a title. [send] puts one `createThread` with `clear_draft` into the
/// outbox (the server deletes the forum's draft with the first message, D-M4-41-1) and
/// returns its entry ID; [leave] deletes an empty draft when the person leaves the view.
///
/// Example:
/// ```dart
/// final chat = NewChatController(forum: id, outbox: engine.outbox, dao: db.outboxDao,
///     editing: registry, dropStoredDraft: () async {});
/// chat.composer.text.text = 'Fix the login page';
/// final entry = await chat.send(chat.composer.text.text);
/// ```
final class NewChatController extends ChangeNotifier {
  /// Creates the controller of [forum] on [outbox] and [dao].
  ///
  /// [editing] is the registry of drafts being typed (04 14), [deviceId] this device, and
  /// [dropStoredDraft] deletes the stored draft of the forum from the local database, so the
  /// draft row leaves at once and offline too.
  NewChatController({
    required this.forum,
    required this._outbox,
    required OutboxDao dao,
    required DraftEditingRegistry editing,
    required this._dropStoredDraft,
    DeviceId? deviceId,
  }) : _drafts = DraftWriter.forum(outbox: _outbox, dao: dao, forum: forum) {
    composer = ComposerController(
      thread: null,
      channel: forum,
      actions: null,
      drafts: _drafts,
      editing: editing,
      deviceId: deviceId,
      sendNew: _sendNew,
    )..addListener(notifyListeners);
    title.addListener(notifyListeners);
  }

  /// The forum.
  final ChannelId forum;

  final Outbox _outbox;
  final DraftWriter _drafts;
  final Future<void> Function() _dropStoredDraft;

  /// The composer of the first message, on the forum's draft of a new post.
  late final ComposerController composer;

  /// The title field.
  final TextEditingController title = TextEditingController();

  /// The chosen tags.
  Set<ForumTagId> get tags => Set.unmodifiable(_tags);
  final Set<ForumTagId> _tags = {};

  /// Whether the forum has no operator agent, so the title is required (09 12).
  bool get titleRequired => _titleRequired;
  set titleRequired(bool value) {
    if (value == _titleRequired) return;
    _titleRequired = value;
    notifyListeners();
  }

  bool _titleRequired = false;

  /// The outbox entry of the sent post, or null before the send.
  String? get sentEntry => _sentEntry;
  String? _sentEntry;

  List<ForumTagId> _order = const [];

  /// Whether the title rule lets the person send: with [titleRequired] only with a title.
  bool get canSend => !_titleRequired || title.text.trim().isNotEmpty;

  /// Starts with [title] and [tags] (the preset of a refused post being edited).
  void preset(String title, Iterable<ForumTagId> tags) {
    this.title.text = title;
    _tags
      ..clear()
      ..addAll(tags);
    notifyListeners();
  }

  /// Chooses or drops [tag]; [order] is the order of the forum's tags, which the send keeps.
  void toggleTag(ForumTagId tag, {required List<ForumTagId> order}) {
    _order = order;
    if (!_tags.remove(tag)) _tags.add(tag);
    notifyListeners();
  }

  /// Sends [text] as the first message of a new post; returns the outbox entry, or null when
  /// nothing was sent (no text, the title rule, or the content limit of the composer).
  Future<String?> send(String text) async {
    if (!canSend || _sentEntry != null) return null;
    final sent = await composer.send(text);
    return sent ? _sentEntry : null;
  }

  Future<void> _sendNew(String body) async {
    final ordered = [
      for (final t in _order)
        if (_tags.contains(t)) t,
      for (final t in _tags)
        if (!_order.contains(t)) t,
    ];
    final entry = await _outbox.enqueue(
      ThreadOps.postRequest(
        forum: forum,
        // An empty title stays empty: the agent names the chat (09 12 rule 2).
        title: title.text.trim(),
        contentMd: body,
        tagIds: ordered,
        clearDraft: true,
      ),
    );
    _sentEntry = entry.id;
    // The server deletes its draft with the first message; the stored copy goes now, so the
    // draft row becomes the row of the sent post (01 6.8 item 8).
    await _dropStoredDraft();
  }

  /// The person leaves the view without sending: an empty draft is deleted (01 6.8 item 8);
  /// a draft with text stays for the next "+". Does nothing after a send.
  Future<void> leave({required bool hadStoredDraft}) async {
    if (_sentEntry != null || composer.text.text.trim().isNotEmpty) return;
    if (!hadStoredDraft && !await _drafts.hasUnsent()) return;
    // An empty write deletes the draft on the server (04 14) and replaces a waiting write.
    await _drafts.write('');
    await _dropStoredDraft();
  }

  @override
  void dispose() {
    composer
      ..removeListener(notifyListeners)
      ..dispose();
    title.dispose();
    super.dispose();
  }
}
