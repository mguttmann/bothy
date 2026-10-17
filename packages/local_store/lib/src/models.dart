// SPDX-License-Identifier: Apache-2.0

/// Value types of the local store that `domain` (M4-08) does not have: the caller's access to a
/// space or channel, removed targets, outbox entries, the sync state, file cache rows, share
/// entries and search hits. They describe local storage only and hold no rules of the server.
library;

import 'package:domain/domain.dart';

/// Effective grant level on a space or channel (API `Level`, ordered `read` < `write` <
/// `trigger`).
enum AccessLevel implements Coded {
  /// Read.
  read('read'),

  /// Write.
  write('write'),

  /// Trigger agents.
  trigger('trigger'),

  /// A value of a newer server.
  unknown('');

  const AccessLevel(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static AccessLevel fromCode(String code) {
    for (final value in values) {
      if (value.code == code && code.isNotEmpty) return value;
    }
    return unknown;
  }
}

/// The caller's level and capabilities on a space or channel as the server reported them in
/// `SpaceView` or `ChannelView` (snapshot, `getSpace`, `getChannel`). The interface shows
/// controls by this answer, never by a derivation from grants (15 1 principle 1).
final class AccessView {
  /// Creates an [AccessView].
  const AccessView({required this.level, required this.capabilities});

  /// Effective level.
  final AccessLevel level;

  /// Additional capabilities.
  final List<Capability> capabilities;

  @override
  bool operator ==(Object other) =>
      other is AccessView &&
      other.level == level &&
      _sameList(other.capabilities, capabilities);

  @override
  int get hashCode => Object.hash(level, Object.hashAll(capabilities));
}

bool _sameList<T>(List<T> a, List<T> b) {
  if (a.length != b.length) return false;
  for (var i = 0; i < a.length; i++) {
    if (a[i] != b[i]) return false;
  }
  return true;
}

/// Kind of a removed target: `space`, `channel` and `host` as `AccessTargetKind` of
/// `access.changed` names them (04 8.3), plus `thread` for `thread.moved_out` (M4-14).
enum RemovedTargetKind {
  /// A space with its channels.
  space,

  /// A channel with its threads.
  channel,

  /// A thread.
  thread,

  /// A host of the terminal; ignored here, M4-36 handles its pins.
  host,
}

/// A target whose local content is deleted (04 8.3).
final class TargetRef {
  /// Creates a reference to the target [id] of [kind].
  const TargetRef(this.kind, this.id);

  /// A space.
  TargetRef.space(SpaceId id) : this(RemovedTargetKind.space, id.value);

  /// A channel.
  TargetRef.channel(ChannelId id) : this(RemovedTargetKind.channel, id.value);

  /// A thread.
  TargetRef.thread(ThreadId id) : this(RemovedTargetKind.thread, id.value);

  /// Kind.
  final RemovedTargetKind kind;

  /// Canonical UUID text of the target.
  final String id;

  @override
  bool operator ==(Object other) =>
      other is TargetRef && other.kind == kind && other.id == id;

  @override
  int get hashCode => Object.hash(kind, id);
}

/// State of an outbox entry (15 7.5).
enum OutboxState implements Coded {
  /// Waits for its predecessor in the thread.
  pending('pending'),

  /// In flight.
  sending('sending'),

  /// Backing off after a network error, timeout, `5xx` or `429`.
  waiting('waiting'),

  /// Failed for good (`400`, `404`, `410`, `412`, `413`, `422`); the text stays for copying.
  failed('failed'),

  /// Confirmed by a `2xx` or the echo; the entry leaves the outbox.
  confirmed('confirmed');

  const OutboxState(this.code);

  @override
  final String code;

  /// Whether no further send follows (`failed`, `confirmed`).
  bool get isFinal => this == failed || this == confirmed;

  /// Decodes [code]; throws a [FormatException] for a code this version never writes.
  static OutboxState fromCode(String code) {
    for (final value in values) {
      if (value.code == code) return value;
    }
    throw FormatException('unknown outbox state of ${code.length} characters');
  }
}

/// An entry of the outbox (04 13.2, 15 7.5).
final class OutboxEntry {
  /// Creates an [OutboxEntry].
  const OutboxEntry({
    required this.id,
    required this.kind,
    required this.payload,
    required this.state,
    required this.attempts,
    required this.createdAt,
    required this.stateChangedAt,
    this.threadId,
    this.dependsOn,
    this.expectedVersion,
    this.nextAttemptAt,
    this.lastErrorCode,
    this.lastErrorReason,
    this.conflictCurrent,
  });

  /// UUIDv7: `client_msg_id`, `client_interaction_id` or `Idempotency-Key`.
  final String id;

  /// Name of the operation.
  final String kind;

  /// Thread of the write, if any.
  final ThreadId? threadId;

  /// Entry that must be confirmed first.
  final String? dependsOn;

  /// Request body as JSON text.
  final String payload;

  /// `If-Match` version of an edit.
  final ResourceVersion? expectedVersion;

  /// State.
  final OutboxState state;

  /// Send attempts so far.
  final int attempts;

  /// Earliest next attempt.
  final DateTime? nextAttemptAt;

  /// Problem code of the last failure.
  final String? lastErrorCode;

  /// Reason of the last failure.
  final String? lastErrorReason;

  /// JSON of the `current` revision of a `412`.
  final String? conflictCurrent;

  /// Creation time.
  final DateTime createdAt;

  /// Time of the last state change.
  final DateTime stateChangedAt;

  @override
  bool operator ==(Object other) =>
      other is OutboxEntry &&
      other.id == id &&
      other.kind == kind &&
      other.threadId == threadId &&
      other.dependsOn == dependsOn &&
      other.payload == payload &&
      other.expectedVersion == expectedVersion &&
      other.state == state &&
      other.attempts == attempts &&
      other.nextAttemptAt == nextAttemptAt &&
      other.lastErrorCode == lastErrorCode &&
      other.lastErrorReason == lastErrorReason &&
      other.conflictCurrent == conflictCurrent &&
      other.createdAt == createdAt &&
      other.stateChangedAt == stateChangedAt;

  @override
  int get hashCode => Object.hash(
    id,
    kind,
    threadId,
    dependsOn,
    payload,
    expectedVersion,
    state,
    attempts,
    nextAttemptAt,
    lastErrorCode,
    lastErrorReason,
    conflictCurrent,
    createdAt,
    stateChangedAt,
  );
}

/// The stored sync state (04 7.1, 7.5; 15 6.1 `sync_state`).
final class SyncState {
  /// Creates a [SyncState].
  const SyncState({
    this.cursor,
    this.syncEpoch,
    this.headSeq,
    this.permissionVersion,
    this.gatewaySessionId,
    this.disconnectedAt,
    this.lastSnapshotAt,
  });

  /// Nothing stored yet.
  static const SyncState empty = SyncState();

  /// Cursor; null means "connect without a cursor" (resync).
  final GlobalSeq? cursor;

  /// Sync epoch stored with the cursor.
  final String? syncEpoch;

  /// `head_seq` of the last `ready`.
  final GlobalSeq? headSeq;

  /// `permission_version` of the last `ready`.
  final PermissionVersion? permissionVersion;

  /// Gateway session for `resume`.
  final String? gatewaySessionId;

  /// Time the connection was lost.
  final DateTime? disconnectedAt;

  /// Time of the last snapshot.
  final DateTime? lastSnapshotAt;

  @override
  bool operator ==(Object other) =>
      other is SyncState &&
      other.cursor == cursor &&
      other.syncEpoch == syncEpoch &&
      other.headSeq == headSeq &&
      other.permissionVersion == permissionVersion &&
      other.gatewaySessionId == gatewaySessionId &&
      other.disconnectedAt == disconnectedAt &&
      other.lastSnapshotAt == lastSnapshotAt;

  @override
  int get hashCode => Object.hash(
    cursor,
    syncEpoch,
    headSeq,
    permissionVersion,
    gatewaySessionId,
    disconnectedAt,
    lastSnapshotAt,
  );
}

/// A row of the file cache: where the bytes of one variant of a file lie (15 6.1).
final class FileCacheEntry {
  /// Creates a [FileCacheEntry].
  const FileCacheEntry({
    required this.fileId,
    required this.variant,
    required this.path,
    required this.sizeBytes,
    required this.lastAccessedAt,
  });

  /// The file.
  final FileId fileId;

  /// `original` or a thumbnail variant code.
  final String variant;

  /// Path of the bytes.
  final String path;

  /// Size on disk.
  final int sizeBytes;

  /// Last access.
  final DateTime lastAccessedAt;

  @override
  bool operator ==(Object other) =>
      other is FileCacheEntry &&
      other.fileId == fileId &&
      other.variant == variant &&
      other.path == path &&
      other.sizeBytes == sizeBytes &&
      other.lastAccessedAt == lastAccessedAt;

  @override
  int get hashCode =>
      Object.hash(fileId, variant, path, sizeBytes, lastAccessedAt);
}

/// A share entry taken over from an extension (15 12).
final class ShareInboxEntry {
  /// Creates a [ShareInboxEntry].
  const ShareInboxEntry({
    required this.id,
    required this.payload,
    required this.filePaths,
    required this.createdAt,
    this.targetThreadId,
  });

  /// The `client_msg_id` the extension created.
  final String id;

  /// Thread the person chose, if any.
  final ThreadId? targetThreadId;

  /// Shared text and metadata as JSON text.
  final String payload;

  /// Paths of the shared files.
  final List<String> filePaths;

  /// Creation time.
  final DateTime createdAt;

  @override
  bool operator ==(Object other) =>
      other is ShareInboxEntry &&
      other.id == id &&
      other.targetThreadId == targetThreadId &&
      other.payload == payload &&
      _sameList(other.filePaths, filePaths) &&
      other.createdAt == createdAt;

  @override
  int get hashCode => Object.hash(
    id,
    targetThreadId,
    payload,
    Object.hashAll(filePaths),
    createdAt,
  );
}

/// What a row of the offline search index stands for (14 6).
enum SearchKind implements Coded {
  /// A message.
  message('message'),

  /// The title of a thread.
  threadTitle('thread_title'),

  /// The transcript of a voice message.
  transcript('transcript');

  const SearchKind(this.code);

  @override
  final String code;

  /// Decodes [code]; throws a [FormatException] for a code this version never writes.
  static SearchKind fromCode(String code) {
    for (final value in values) {
      if (value.code == code) return value;
    }
    throw FormatException('unknown search kind of ${code.length} characters');
  }
}

/// A hit of the offline search (14 6).
final class SearchHit {
  /// Creates a [SearchHit].
  const SearchHit({
    required this.kind,
    required this.targetId,
    required this.snippet,
    required this.rank,
    this.threadId,
    this.channelId,
    this.spaceId,
  });

  /// What was found.
  final SearchKind kind;

  /// Message, thread or file ID.
  final String targetId;

  /// Thread of the hit.
  final ThreadId? threadId;

  /// Channel of the hit.
  final ChannelId? channelId;

  /// Space of the hit.
  final SpaceId? spaceId;

  /// Part of the text with each match between [SearchDaoMarks.open] and [SearchDaoMarks.close].
  final String snippet;

  /// `bm25` score; lower is more relevant.
  final double rank;
}

/// The markers of a [SearchHit.snippet]: control characters that never occur in Markdown, so
/// the interface can split the text without escaping.
abstract final class SearchDaoMarks {
  /// Start of a match.
  static const String open = '\u0002';

  /// End of a match.
  static const String close = '\u0003';

  /// Text left out.
  static const String ellipsis = '…';
}

/// The contiguous window of a thread (15 7.3 item 4): the messages of the loaded range that ends
/// at the newest loaded message, and the gaps next to it, where M4-14 shows a loading row.
final class ContiguousWindow {
  /// Creates a [ContiguousWindow].
  const ContiguousWindow({
    required this.messages,
    this.gapBefore,
    this.gapAfter,
  });

  /// Messages in ascending `thread_seq`.
  final List<Message> messages;

  /// Missing positions directly before the window, if any.
  final SeqRange? gapBefore;

  /// Missing positions between the window and the thread's `last_message_seq`, if any.
  final SeqRange? gapAfter;
}

/// The key of the key source does not open the database file. The message never contains the
/// key.
final class LocalStoreKeyError implements Exception {
  /// Creates the error.
  const LocalStoreKeyError();

  @override
  String toString() =>
      'LocalStoreKeyError: the key does not open the local database';
}

/// The SQLite library lacks a feature the store needs (encryption or FTS5), or the key source
/// returned a key of the wrong length. The message never contains the key.
final class LocalStoreSetupError implements Exception {
  /// Creates the error with a [message] that names the missing feature.
  const LocalStoreSetupError(this.message);

  /// What is missing.
  final String message;

  @override
  String toString() => 'LocalStoreSetupError: $message';
}
