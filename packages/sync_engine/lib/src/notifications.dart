// SPDX-License-Identifier: Apache-2.0

/// The hints of the gateway (13 6.3): the payload of the ephemeral event
/// `notification.created`, which the server sends to the live connections of the recipient
/// instead of a push. The client shows it and decides nothing about who is notified (13 5.3,
/// R-PN-01); nothing of it is stored.
library;

import 'package:domain/domain.dart';

import 'wire.dart';

/// The kind of a hint (13 2, API `NotificationKind`).
///
/// Example:
/// ```dart
/// final kind = HintKind.fromCode('run_done'); // HintKind.runDone
/// assert(kind.code == 'run_done');
/// ```
enum HintKind {
  /// A new message in a thread the person follows with level `all`.
  message('message'),

  /// A message that mentions the person.
  mention('mention'),

  /// A message in a direct message channel.
  dm('dm'),

  /// A run the person started completed.
  runDone('run_done'),

  /// A run the person started failed.
  runFailed('run_failed'),

  /// A run the person started waits for an answer or an approval.
  inputNeeded('input_needed'),

  /// A new device of the person waits for approval.
  security('security'),

  /// A reminder of the administration.
  admin('admin'),

  /// A kind this client does not know (04 6.1: unknown values are tolerated).
  unknown('unknown');

  const HintKind(this._code);

  final String _code;

  /// The wire code of the kind; [unknown] gives `unknown`, which the server never sends.
  String get code => _code;

  /// The kind of the wire [code]; any code this client does not know gives [unknown].
  static HintKind fromCode(String code) {
    for (final k in values) {
      if (k != unknown && k._code == code) return k;
    }
    return unknown;
  }
}

/// One hint of `notification.created` (13 6.3, API `NotificationCreatedPayload`).
///
/// [nid] identifies the hint; [kind] says why it came; [threadId] and [messageId] point at
/// what it is about (absent for hints without a thread, which name [targetId] instead:
/// the waiting device of [HintKind.security], the grant or group of [HintKind.admin]).
/// [quiet] means inside quiet hours: show without sound. [count] is 1, or the unread count
/// of a bundled hint (13 4.5). The preview fields follow the recipient's
/// `notify.preview_mode` (13 6.4) and are null when the server sent none.
///
/// [toString] names only [nid] and [kind]: logs never carry content.
///
/// Example:
/// ```dart
/// engine.notifications.listen((n) {
///   if (n.kind == HintKind.mention) showBanner(n.senderDisplayName, n.excerpt);
/// });
/// ```
final class NotificationCreated {
  /// Creates a hint from its members.
  const NotificationCreated({
    required this.nid,
    required this.kind,
    this.threadId,
    this.messageId,
    this.targetId,
    this.quiet = false,
    this.count = 1,
    this.spaceName,
    this.channelName,
    this.threadTitle,
    this.senderDisplayName,
    this.excerpt,
    this.attachmentCount,
  });

  /// Reads the `payload` of a `notification.created` envelope.
  ///
  /// Returns the hint; an unknown `kind` gives [HintKind.unknown]. Throws a
  /// [FormatException] when `nid`, `kind`, `quiet` or `count` is missing or of the wrong
  /// type, when an optional member is of the wrong type, or when an ID is not a UUID.
  ///
  /// Example:
  /// ```dart
  /// final n = NotificationCreated.fromPayload({
  ///   'nid': nid,
  ///   'kind': 'dm',
  ///   'quiet': false,
  ///   'count': 1,
  /// });
  /// ```
  static NotificationCreated fromPayload(Map<String, Object?> payload) {
    final threadId = optStr(payload, 'thread_id');
    final messageId = optStr(payload, 'message_id');
    final targetId = optStr(payload, 'target_id');
    final quiet = payload['quiet'];
    if (quiet is! bool) {
      throw const FormatException('member quiet is not a boolean');
    }
    return NotificationCreated(
      nid: canonicalUuid(str(payload, 'nid'), 'hint ID'),
      kind: HintKind.fromCode(str(payload, 'kind')),
      threadId: threadId == null ? null : ThreadId.parse(threadId),
      messageId: messageId == null ? null : MessageId.parse(messageId),
      targetId: targetId == null ? null : canonicalUuid(targetId, 'target ID'),
      quiet: quiet,
      count: integer(payload, 'count'),
      spaceName: optStr(payload, 'space_name'),
      channelName: optStr(payload, 'channel_name'),
      threadTitle: optStr(payload, 'thread_title'),
      senderDisplayName: optStr(payload, 'sender_display_name'),
      excerpt: optStr(payload, 'excerpt'),
      attachmentCount: optInt(payload, 'attachment_count'),
    );
  }

  /// The ID of the hint.
  final String nid;

  /// Why the hint came.
  final HintKind kind;

  /// The thread of the hint, or null for a hint without a thread.
  final ThreadId? threadId;

  /// The message the hint points at, or null.
  final MessageId? messageId;

  /// The target of a hint without a thread, or null.
  final String? targetId;

  /// Inside quiet hours: show without sound.
  final bool quiet;

  /// 1, or the unread count of a bundled hint.
  final int count;

  /// The name of the space, or null (direct messages, preview mode `none`).
  final String? spaceName;

  /// The name of the channel, or null.
  final String? channelName;

  /// The title of the thread, or null.
  final String? threadTitle;

  /// The display name of the sender, or null.
  final String? senderDisplayName;

  /// Plain text of the message, at most `notify.preview_max_chars`; only with preview mode
  /// `full`.
  final String? excerpt;

  /// The number of attachments, or null.
  final int? attachmentCount;

  @override
  String toString() => 'NotificationCreated($nid, ${kind.code})';
}
