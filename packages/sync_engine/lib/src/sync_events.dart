// SPDX-License-Identifier: Apache-2.0

/// The notices of the catch-up (brief M4-14 Scope): what the app must react to beyond the rows
/// of the local database, which the interface reads on its own (15 1 principle 2).
library;

import 'package:auth/auth.dart';
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';

/// One notice of the sync engine to the app, delivered on `CatchUpListener.notices`.
///
/// Example:
/// ```dart
/// catchUp.notices.listen((notice) => switch (notice) {
///   TargetsRemoved(:final targets) => closeViews(targets, 'No access anymore'),
///   DraftOverwritten() => showDraftNotice(),
///   Resynced() => null,
///   SignedOutNotice(:final deleteLocalData) => showLogin(deleteLocalData),
/// });
/// ```
sealed class SyncNotice {
  const SyncNotice();
}

/// `access.changed` removed [targets]; their rows, search entries and drafts are gone and the
/// app closes their views with "No access anymore" (04 8.3 item 3).
final class TargetsRemoved extends SyncNotice {
  /// Creates the notice.
  const TargetsRemoved(this.targets);

  /// The removed spaces, channels and threads (hosts are M4-36's).
  final List<TargetRef> targets;
}

/// A draft write of this device overwrote a newer draft of another device
/// (`overwrote_newer`, 04 14); the app shows a notice.
final class DraftOverwritten extends SyncNotice {
  /// Creates the notice.
  const DraftOverwritten(this.draft);

  /// The draft as the server stored it.
  final Draft draft;
}

/// A `resync_required` was handled: the mirror holds the new snapshot and the cursor is its
/// `head_seq` (04 7.3).
final class Resynced extends SyncNotice {
  /// Creates the notice.
  const Resynced(this.reason);

  /// The reason the server named (`retention`, `too_far_behind`, `epoch_changed`,
  /// `cursor_ahead`).
  final String reason;
}

/// The session was revoked (`4012` or `session.revoked`); the local database is wiped and the
/// app signs out (04 8.5, R-SY-14).
final class SignedOutNotice extends SyncNotice {
  /// Creates the notice.
  const SignedOutNotice({required this.deleteLocalData});

  /// Whether the app deletes the rest of the account's local data (file cache, keys, tokens);
  /// always true for a revocation.
  final bool deleteLocalData;
}

/// Who this device is, for the echo rule (04 14) and the scope of `session.revoked`.
///
/// The app wires [AuthIdentity] of this package over the `AuthSession`; tests use a fixed
/// value.
abstract interface class LocalIdentity {
  /// This device, or null when unknown.
  DeviceId? get deviceId;

  /// The session ID (`sid`) of this device, or null when unknown.
  String? get sessionId;
}

/// A [LocalIdentity] with fixed values.
final class FixedIdentity implements LocalIdentity {
  /// Creates the identity.
  const FixedIdentity({this.deviceId, this.sessionId});

  @override
  final DeviceId? deviceId;

  @override
  final String? sessionId;
}

/// The [LocalIdentity] of the signed-in [AuthSession]: its device and its `sid`.
final class AuthIdentity implements LocalIdentity {
  /// Reads the identity from [auth] on every call, so a new session is seen at once.
  const AuthIdentity(this._auth);

  final AuthSession _auth;

  @override
  DeviceId? get deviceId => _auth.session?.deviceId;

  @override
  String? get sessionId => _auth.session?.sid;
}

/// Deletes the bytes of cached files after their rows were removed (M4-22 implements it;
/// 04 8.3 item 3).
abstract interface class FileCacheEraser {
  /// Deletes the files at [paths]; a missing file is no error.
  Future<void> erase(List<String> paths);
}

/// Whether the person is typing in a draft on this device right now (04 14: an incoming change
/// of another device does not overwrite it). The app sets it from the composer.
abstract interface class DraftEditing {
  /// Whether the draft of [threadId] (or of a new post in [channelId] when [threadId] is null)
  /// is being edited.
  bool isEditing({required ChannelId channelId, ThreadId? threadId});
}
