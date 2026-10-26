// SPDX-License-Identifier: Apache-2.0

// The sharing glue of iOS and macOS (brief M5-06 Design notes 3, 5 and 7) against a fake Pigeon
// host: the child session of the Share Extension after a login and at a start or a return to
// the foreground when it is missing, the settings and recent targets of the App Group, the
// takeover of the share inbox, and the deletion on every end of the account. A build without
// the App Group does nothing, and every platform failure is swallowed.

import 'dart:async';

import 'package:client/share/share_apple.dart';
import 'package:client/share/share_target.dart';
import 'package:domain/domain.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/fakes.dart';
import 'share_fakes.dart';

void main() {
  late FakeShareHost host;
  late FakeSessions sessions;
  late TestClock clock;
  late AppleShare share;
  late int takeovers;
  late List<ShareTarget>? recent;

  final channel = ChannelId.parse('0192a000-0000-7000-8000-0000000000c1');
  final thread = ThreadId.parse('0192a000-0000-7000-8000-0000000000d1');
  const limits = InstanceLimits(
    messageMaxContentBytes: 65536,
    messageMaxAttachments: 10,
    fileMaxUploadBytes: 104857600,
    threadTitleMaxChars: 200,
    voiceMaxDurationMs: 600000,
  );

  ShareAccount account({InstanceLimits? instanceLimits}) => ShareAccount(
    instanceId: shareInstance,
    serverUrl: Uri.parse('https://chat.example.com'),
    sessions: sessions,
    recentTargetsMax: 8,
    limits: instanceLimits,
    recentTargets: () async => recent,
    takeOver: () async => takeovers++,
  );

  setUp(() {
    host = FakeShareHost();
    sessions = FakeSessions();
    clock = TestClock(DateTime.utc(2026, 10, 25, 12));
    share = AppleShare(host: host, clock: clock);
    takeovers = 0;
    recent = null;
  });

  test('a start without a child session creates one; '
      'with a valid one it creates none', () async {
    await share.signedIn(account(), afterLogin: false);
    expect(sessions.calls, 1);
    final stored = host.keychain[shareInstance]!;
    expect(stored.instanceId, shareInstance);
    expect(stored.serverUrl, 'https://chat.example.com');
    expect(stored.sessionId, '0192a000-0000-7000-8000-000000000001');
    expect(stored.accessToken, 'child-access-1');
    expect(stored.refreshToken, 'child-refresh-1');
    expect(
      stored.accessExpiresAtMillis,
      clock.now().add(const Duration(minutes: 15)).millisecondsSinceEpoch,
    );
    expect(
      stored.refreshExpiresAtMillis,
      DateTime.utc(2026, 11, 24).millisecondsSinceEpoch,
    );
    expect(host.asked, [clock.now().millisecondsSinceEpoch]);

    await share.signedIn(account(), afterLogin: false);
    expect(sessions.calls, 1, reason: 'the keychain holds a valid one');
  });

  test('a login always creates a new child session', () async {
    await share.signedIn(account(), afterLogin: false);
    await share.signedIn(account(), afterLogin: true);
    expect(sessions.calls, 2);
    expect(host.keychain[shareInstance]!.refreshToken, 'child-refresh-2');
  });

  test('the foreground creates the child session again when the extension '
      'deleted it, and takes the inbox over', () async {
    await share.signedIn(account(), afterLogin: false);
    expect(takeovers, 1);
    await share.foreground();
    expect(sessions.calls, 1);
    expect(takeovers, 2);
    // The extension deleted a revoked child (401 session_revoked).
    host.keychain.clear();
    await share.foreground();
    expect(sessions.calls, 2);
    expect(takeovers, 3);
  });

  test('the settings carry the instance, client.share.recent_targets and the '
      'limits once they are known', () async {
    await share.signedIn(account(), afterLogin: false);
    expect(host.settings!.instanceId, shareInstance);
    expect(host.settings!.recentTargetsMax, 8);
    expect(host.settings!.fileMaxUploadBytes, isNull);
    expect(host.settings!.messageMaxAttachments, isNull);
    await share.limitsChanged(limits);
    expect(host.settings!.fileMaxUploadBytes, 104857600);
    expect(host.settings!.messageMaxAttachments, 10);
  });

  test('the same account handed over again keeps the known limits', () async {
    final same = account();
    await share.signedIn(same, afterLogin: false);
    await share.limitsChanged(limits);
    await share.signedIn(same, afterLogin: true);
    expect(host.settings!.fileMaxUploadBytes, 104857600);
  });

  test('a login reported after the handover creates no second child session '
      'when the handover created one', () async {
    final same = account();
    await share.signedIn(same, afterLogin: false);
    expect(sessions.calls, 1, reason: 'the keychain had none');
    await share.signedIn(same, afterLogin: true);
    expect(sessions.calls, 1, reason: 'that one was created after the login');
  });

  test('a login reported while the handover creates the child session creates '
      'no second one', () async {
    final same = account();
    sessions.hold = Completer<void>();
    final handover = share.signedIn(same, afterLogin: false);
    await pumpEventQueue();
    final login = share.signedIn(same, afterLogin: true);
    await pumpEventQueue();
    sessions.hold!.complete();
    await Future.wait([handover, login]);
    expect(sessions.calls, 1);
    expect(host.keychain[shareInstance], isNotNull);
  });

  test(
    'recent targets are written when they change, never while unknown',
    () async {
      await share.signedIn(account(), afterLogin: false);
      expect(
        host.written,
        isEmpty,
        reason: 'unknown before the first snapshot',
      );
      recent = [
        ShareTarget(
          kind: ShareTargetKind.thread,
          channelId: channel,
          threadId: thread,
          path: 'general > Release notes',
        ),
      ];
      await share.refreshTargets();
      await share.refreshTargets();
      expect(host.lastPaths, ['general > Release notes']);
      expect(
        host.written,
        hasLength(1),
        reason: 'the same list is not written again',
      );
      expect(host.written.single.single.kind, 'thread');
      expect(host.written.single.single.threadId, thread.value);
      recent = [];
      await share.refreshTargets();
      expect(host.lastPaths, isEmpty);
    },
  );

  test(
    'every end of the account deletes the child session and the shared data',
    () async {
      await share.signedIn(account(), afterLogin: false);
      await share.forget(shareInstance);
      expect(host.keychain, isEmpty);
      expect(host.cleared, ['session $shareInstance', 'data']);
      // After the end nothing is written any more.
      recent = const [];
      await share.refreshTargets();
      await share.foreground();
      expect(host.written, isEmpty);
      expect(sessions.calls, 1);
    },
  );

  test('a build without the App Group does nothing', () async {
    host.available = false;
    await share.signedIn(account(), afterLogin: true);
    await share.foreground();
    await share.limitsChanged(limits);
    recent = const [];
    await share.refreshTargets();
    await share.forget(shareInstance);
    expect(sessions.calls, 0);
    expect(host.settings, isNull);
    expect(host.written, isEmpty);
    expect(host.cleared, isEmpty);
    expect(takeovers, 0);
  });

  test('platform and server failures are swallowed', () async {
    host.failing = true;
    await share.signedIn(account(), afterLogin: true);
    await share.forget(shareInstance);
    host.failing = false;
    sessions.failure = PlatformException(code: 'network');
    await share.signedIn(account(), afterLogin: true);
    expect(host.keychain, isEmpty);
    expect(host.settings, isNotNull, reason: 'the other steps still run');
  });
}
