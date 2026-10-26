// SPDX-License-Identifier: Apache-2.0

// Fakes of sharing (brief M5-06): the Pigeon hosts of the Apple and Android runners and the
// REST call that creates the child session of the Share Extension.

// The fakes implement the generated host APIs, whose fields carry Pigeon's names.
// ignore_for_file: non_constant_identifier_names

import 'dart:async';
import 'dart:io';

import 'package:client/share/share_android.dart';
import 'package:client/share/share_apple.dart';
import 'package:flutter/services.dart';

/// The instance of the share tests.
const String shareInstance = '0192a000-0000-7000-8000-00000000c0de';

/// The runner's share host in memory: the shared keychain, the App Group values and the share
/// inbox, with switches for the failures the test sets.
class FakeShareHost implements ShareAppleHostApi {
  bool available = true;

  /// Every call that fails with a `PlatformException` when set.
  bool failing = false;

  /// The child sessions of the shared keychain by instance.
  final Map<String, ExtensionSessionRecord> keychain = {};

  /// The last settings written, or null.
  ShareSettingsRecord? settings;

  /// Every list of recent targets written, in order.
  final List<List<ShareTargetRecord>> written = [];

  /// The calls that ended the account's data, in order.
  final List<String> cleared = [];

  /// The entries of the share inbox by `client_msg_id`.
  final Map<String, ShareInboxEntryRecord> inbox = {};

  /// The inbox entries deleted, in order.
  final List<String> deletedEntries = [];

  /// The times [hasExtensionSession] was asked with.
  final List<int> asked = [];

  /// The next [deleteInboxEntry] fails once when set.
  bool failDeleteOnce = false;

  @override
  BinaryMessenger? get pigeonVar_binaryMessenger => null;

  @override
  String get pigeonVar_messageChannelSuffix => '';

  void _check() {
    if (failing) throw PlatformException(code: 'unavailable');
  }

  @override
  Future<bool> isAvailable() async {
    _check();
    return available;
  }

  @override
  Future<void> saveExtensionSession(ExtensionSessionRecord record) async {
    _check();
    keychain[record.instanceId] = record;
  }

  @override
  Future<bool> hasExtensionSession(String instanceId, int nowMillis) async {
    _check();
    asked.add(nowMillis);
    final record = keychain[instanceId];
    return record != null && record.refreshExpiresAtMillis > nowMillis;
  }

  @override
  Future<void> deleteExtensionSession(String instanceId) async {
    _check();
    keychain.remove(instanceId);
    cleared.add('session $instanceId');
  }

  @override
  Future<void> writeShareSettings(ShareSettingsRecord settings) async {
    _check();
    this.settings = settings;
  }

  @override
  Future<void> writeRecentTargets(List<ShareTargetRecord> targets) async {
    _check();
    written.add(List.of(targets));
  }

  @override
  Future<void> clearShareData() async {
    _check();
    settings = null;
    cleared.add('data');
  }

  @override
  Future<List<ShareInboxEntryRecord>> readInbox() async {
    _check();
    final entries = inbox.values.toList()
      ..sort((a, b) => a.createdAtMillis.compareTo(b.createdAtMillis));
    return entries;
  }

  @override
  Future<void> deleteInboxEntry(String clientMsgId) async {
    _check();
    if (failDeleteOnce) {
      failDeleteOnce = false;
      throw PlatformException(code: 'io');
    }
    // The runner deletes the entry's folder with its files.
    for (final f
        in inbox.remove(clientMsgId)?.files ?? const <ShareInboxFileRecord>[]) {
      final file = File(f.path);
      if (file.existsSync()) file.deleteSync();
    }
    deletedEntries.add(clientMsgId);
  }

  /// The paths of the recent targets written last, or null before the first write.
  List<String>? get lastPaths =>
      written.isEmpty ? null : [for (final t in written.last) t.path];
}

/// `createExtensionSession` that records its calls and hands out numbered tokens.
class FakeSessions implements ExtensionSessions {
  int calls = 0;

  /// Throws on the next call when set.
  Object? failure;

  /// Holds every call until it completes, when set.
  Completer<void>? hold;

  @override
  Future<ExtensionSessionGrant> create() async {
    final error = failure;
    if (error != null) throw error;
    calls++;
    await hold?.future;
    return ExtensionSessionGrant(
      sessionId:
          '0192a000-0000-7000-8000-0000000000${calls.toString().padLeft(2, '0')}',
      accessToken: 'child-access-$calls',
      accessExpiresIn: const Duration(minutes: 15),
      refreshToken: 'child-refresh-$calls',
      refreshExpiresAt: DateTime.utc(2026, 11, 24),
    );
  }
}

/// The Android runner's share host: one pending share, taken once.
class FakeAndroidShareHost implements ShareAndroidHostApi {
  SharedContentRecord? pending;
  int takes = 0;

  @override
  BinaryMessenger? get pigeonVar_binaryMessenger => null;

  @override
  String get pigeonVar_messageChannelSuffix => '';

  @override
  Future<SharedContentRecord?> takePendingShare() async {
    takes++;
    final taken = pending;
    pending = null;
    return taken;
  }
}
