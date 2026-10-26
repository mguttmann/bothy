// SPDX-License-Identifier: Apache-2.0

/// The targets and the content of a share (brief M5-06 Design notes 1, 5 and 7; the target
/// JSON of the App Group, docs/decisions.d/M5-06.md).
library;

import 'package:domain/domain.dart';

import '../src/generated/share_apple_api.g.dart' show ShareTargetRecord;

/// What a share goes to.
enum ShareTargetKind {
  /// A thread or a forum post: the message goes into the thread.
  thread('thread'),

  /// The main timeline of a text channel: the message goes into its `main_thread_id`.
  channel('channel'),

  /// A new post in a forum channel (`createThread` with `first_message`).
  newPost('new_post');

  const ShareTargetKind(this.code);

  /// The code of the target JSON (`kind`).
  final String code;

  /// The kind of [code]; throws a [FormatException] for an unknown code.
  static ShareTargetKind fromCode(String code) => values.firstWhere(
    (k) => k.code == code,
    orElse: () =>
        throw FormatException('unknown share target kind ${code.length}'),
  );
}

/// One target of the share sheet: the kind, the channel, the thread messages go to and the
/// display path.
///
/// The display path holds only names the person can see: `<channel name> > <thread title>`
/// for a thread, `<channel name>` for a channel and a new post; the sheet adds the glyph and
/// the "(new post)" suffix by [kind] (docs/decisions.d/M5-06.md).
///
/// Example:
/// ```dart
/// final target = ShareTarget(kind: ShareTargetKind.thread, channelId: c,
///     threadId: t, path: 'general > Release notes');
/// ```
final class ShareTarget {
  /// Creates the target; [threadId] is null exactly for [ShareTargetKind.newPost].
  const ShareTarget({
    required this.kind,
    required this.channelId,
    required this.path,
    this.threadId,
  });

  /// The kind.
  final ShareTargetKind kind;

  /// The channel; uploads go to its `/channels/{channel_id}/uploads`.
  final ChannelId channelId;

  /// The thread the message goes to: the thread itself, or the channel's main thread; null
  /// for a new post.
  final ThreadId? threadId;

  /// The display path.
  final String path;

  /// The target as the Pigeon record of the App Group.
  ShareTargetRecord toRecord() => ShareTargetRecord(
    kind: kind.code,
    channelId: channelId.value,
    threadId: threadId?.value,
    path: path,
  );

  /// The target of [record]; throws a [FormatException] for an unknown kind, an ID that is no
  /// UUID or a thread that does not match the kind.
  static ShareTarget fromRecord(ShareTargetRecord record) {
    final kind = ShareTargetKind.fromCode(record.kind);
    final thread = record.threadId;
    if ((kind == ShareTargetKind.newPost) != (thread == null)) {
      throw const FormatException(
        'share target thread does not match its kind',
      );
    }
    return ShareTarget(
      kind: kind,
      channelId: ChannelId.parse(record.channelId),
      threadId: thread == null ? null : ThreadId.parse(thread),
      path: record.path,
    );
  }

  /// The members of the target JSON (`kind`, `channel_id`, `thread_id`, `path`).
  Map<String, Object?> toJson() => {
    'kind': kind.code,
    'channel_id': channelId.value,
    'thread_id': ?threadId?.value,
    'path': path,
  };

  @override
  bool operator ==(Object other) =>
      other is ShareTarget &&
      other.kind == kind &&
      other.channelId == channelId &&
      other.threadId == threadId &&
      other.path == path;

  @override
  int get hashCode => Object.hash(kind, channelId, threadId, path);

  @override
  String toString() => 'ShareTarget(${kind.code}, ${channelId.value})';
}

/// One file of a share, copied already by the extension or the Android runner.
final class ShareFile {
  /// Creates the file.
  const ShareFile({
    required this.path,
    required this.name,
    required this.mimeType,
    required this.sizeBytes,
    this.completedFileId,
  });

  /// The absolute path of the copy (App Group container or the app's cache).
  final String path;

  /// The original file name (a foreign name).
  final String name;

  /// The declared type.
  final String mimeType;

  /// The size in bytes.
  final int sizeBytes;

  /// The file ID of an upload the extension completed already, else null.
  final FileId? completedFileId;
}

/// A share as the app sends it through its outbox: the extension's inbox entry, or what the
/// person chose in the Flutter sheet on Android.
final class ShareDraft {
  /// Creates the draft; [clientMsgId] is the UUIDv7 of the message (or of the post's first
  /// message), which the outbox entry takes as its ID.
  const ShareDraft({
    required this.clientMsgId,
    required this.target,
    required this.comment,
    required this.files,
    this.title,
  });

  /// The `client_msg_id`, also the ID of the outbox entry (04 5.1).
  final String clientMsgId;

  /// The target.
  final ShareTarget target;

  /// The text of the message: the comment and shared text or links.
  final String comment;

  /// The title of a new post; null for the other kinds.
  final String? title;

  /// The files in their order.
  final List<ShareFile> files;
}
