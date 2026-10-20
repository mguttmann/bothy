// SPDX-License-Identifier: Apache-2.0

// The session flow of 04 6.3: identify or resume after hello, the gateway proof, the protocol
// version check, ready and resumed, and reauth (brief M4-13, Tests).

import 'package:auth/auth.dart';
import 'package:domain/domain.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import '../support/fakes.dart';

void main() {
  late Harness h;
  setUp(() => h = Harness());
  tearDown(() => h.dispose());

  test(
    'the first connection sends identify without cursor and epoch',
    () async {
      final ch = await h.open();
      expect(h.phase, ConnectionPhase.connecting);
      expect(h.transport.urls.single, gatewayUrl);
      final sent = await h.answerHello();
      expect(sent['op'], 'identify');
      expect(h.phase, ConnectionPhase.identifying);
      final d = sent['d']! as Map<String, Object?>;
      expect(d.keys.toSet(), {
        'protocol_version',
        'access_token',
        'dpop',
        'device_id',
        'client',
      });
      expect(d['protocol_version'], 1);
      expect(tokenTag(d['access_token']! as String), 'A1');
      expect(d['device_id'], deviceId);
      expect(d['client'], {'platform': 'macos', 'version': '1.0.0'});
      expect(ch.sent, hasLength(1));
    },
  );

  test('a later identify carries cursor and epoch', () async {
    await h.storeCursor(17, epoch: 'epoch-7');
    await h.open();
    final sent = await h.answerHello();
    expect(sent['op'], 'identify');
    final d = sent['d']! as Map<String, Object?>;
    expect(d['cursor'], 17);
    expect(d['sync_epoch'], 'epoch-7');
  });

  test(
    'the gateway proof uses the hello nonce, GET and the https form',
    () async {
      await h.open();
      final sent = await h.answerHello(nonce: 'nonce-abc');
      final d = sent['d']! as Map<String, Object?>;
      final proof = jwsPayload(d['dpop']! as String);
      expect(proof['htm'], 'GET');
      expect(proof['htu'], 'https://chat.example.com/api/v1/gateway');
      expect(proof['nonce'], 'nonce-abc');
      expect(proof['ath'], accessTokenHash(d['access_token']! as String));
      expect(h.auth.nonces.current, 'nonce-abc');
    },
  );

  test('ready stores the session and goes live without a listener', () async {
    await h.goLive();
    expect(h.phase, ConnectionPhase.live);
    final s = await h.db.syncStateDao.read();
    expect(s.gatewaySessionId, 'gs-1');
    expect(s.headSeq, const GlobalSeq(40));
    expect(s.permissionVersion, const PermissionVersion(3));
    expect(s.cursor, const GlobalSeq(30), reason: 'the cursor is M4-14');
    expect(h.connection.syncEpoch, 'epoch-1');
  });

  test(
    'with a listener ready leads to catchingUp and the listener sets live',
    () async {
      final listener = RecordingListener();
      await h.dispose();
      h = Harness(listener: listener);
      final ch = await h.goLive();
      expect(listener.calls, ['ready gs-1']);
      expect(h.phase, ConnectionPhase.catchingUp);
      listener.control!.setPhase(ConnectionPhase.live);
      await settle();
      expect(h.phase, ConnectionPhase.live);
      ch.push('cursor', {'seq': 41});
      ch.push('catchup.complete', {'head_seq': 41});
      ch.push('dispatch', {'type': 'message.created', 'seq': 42});
      await settle();
      expect(listener.calls.skip(1), [
        'frame cursor',
        'frame catchup.complete',
        'frame dispatch',
      ]);
      final dispatch = listener.frames.last as DispatchFrame;
      expect(dispatch.envelope['seq'], 42);
      ch.serverClose(1001);
      await settle();
      expect(listener.calls.last, 'disconnected');
    },
  );

  test('within the resume window resume, after it identify', () async {
    var ch = await h.goLive();
    ch.serverClose(1001);
    await settle();
    await h.clock.elapse(const Duration(seconds: 5));
    var sent = await h.answerHello();
    expect(sent['op'], 'resume');
    final d = sent['d']! as Map<String, Object?>;
    expect(d['gateway_session_id'], 'gs-1');
    expect(d['cursor'], 30);
    expect(d['sync_epoch'], 'epoch-1');
    expect(tokenTag(d['access_token']! as String), 'A1');
    expect(jwsPayload(d['dpop']! as String)['nonce'], 'nonce-2');
    ch = h.transport.last;
    ch.push('resumed', resumedData());
    await settle();
    expect(h.phase, ConnectionPhase.live);

    // A disconnect followed by a wait beyond resume_window_ms: identify.
    h.network.set(false);
    ch.serverClose(null);
    await settle();
    expect(h.phase, ConnectionPhase.offline);
    expect(h.connection.state.reason, ConnectionReason.networkUnavailable);
    await h.clock.elapse(resumeWindow + const Duration(seconds: 1));
    h.network.set(true);
    await settle();
    sent = await h.answerHello();
    expect(sent['op'], 'identify');
  });

  test('4006 after resume leads to identify', () async {
    final ch = await h.goLive();
    ch.serverClose(1001);
    await h.clock.elapse(const Duration(seconds: 5));
    expect((await h.answerHello())['op'], 'resume');
    h.transport.last.serverClose(4006);
    // Step until the reconnect opens; a longer wait would pass client.sync.connect_timeout.
    for (var i = 0; i < 600 && h.transport.channels.length < 3; i++) {
      await h.clock.elapse(const Duration(milliseconds: 100));
    }
    expect(h.transport.channels, hasLength(3));
    expect((await h.answerHello())['op'], 'identify');
  });

  test('a hello without the client version leads to updateRequired', () async {
    final ch = await h.open();
    ch.push('hello', helloData(versions: [2, 3]));
    await settle();
    expect(h.phase, ConnectionPhase.updateRequired);
    expect(h.connection.state.reason, ConnectionReason.protocolUnsupported);
    expect(ch.sent, isEmpty);
    expect(ch.isClosed, isTrue);
    await h.clock.elapse(const Duration(minutes: 5));
    expect(h.transport.channels, hasLength(1));
  });

  test('reauth_required refreshes and sends reauth', () async {
    final ch = await h.goLive();
    ch.push('reauth_required', {
      'expires_at': '2026-10-15T11:00:00.000Z',
      'dpop_nonce': 'nonce-re',
    });
    await settle();
    expect(h.refreshes, 1);
    final d = ch.lastData('reauth')!;
    expect(tokenTag(d['access_token']! as String), 'A2');
    final proof = jwsPayload(d['dpop']! as String);
    expect(proof['nonce'], 'nonce-re');
    expect(proof['htu'], 'https://chat.example.com/api/v1/gateway');
    expect(h.phase, ConnectionPhase.live);
  });

  test('reauth_required with a rejected refresh signs out', () async {
    h.api.routes['POST /auth/token/refresh'] = (_) =>
        FakeReply.problem(401, 'unauthenticated');
    final ch = await h.goLive();
    ch.push('reauth_required', {
      'expires_at': '2026-10-15T11:00:00.000Z',
      'dpop_nonce': 'nonce-re',
    });
    await settle();
    expect(h.phase, ConnectionPhase.signedOut);
    expect(ch.clientCloseCode, 1000);
    await h.clock.elapse(const Duration(minutes: 5));
    expect(h.transport.channels, hasLength(1));
  });

  test('a disconnect stores disconnected_at', () async {
    final ch = await h.goLive();
    await h.clock.elapse(const Duration(seconds: 3));
    final at = h.clock.now();
    ch.serverClose(1001);
    await settle();
    expect((await h.db.syncStateDao.read()).disconnectedAt, at);
  });

  test('only one connection is open at a time', () async {
    await h.open();
    await h.connection.connectNow();
    h.network.set(true);
    h.lifecycle.set(AppActivity.foreground);
    await settle();
    expect(h.transport.attempts, 1);
  });

  test('stop closes with 1000 and does not reconnect', () async {
    final ch = await h.goLive();
    await h.connection.stop();
    await settle();
    expect(ch.clientCloseCode, 1000);
    expect(h.phase, ConnectionPhase.offline);
    await h.clock.elapse(const Duration(minutes: 5));
    h.network.set(true);
    await settle();
    expect(h.transport.channels, hasLength(1));
  });

  test(
    'mobile closes with 1000 in the background and resumes in the foreground',
    () async {
      final ch = await h.goLive();
      h.lifecycle.set(AppActivity.background);
      await settle();
      expect(ch.clientCloseCode, 1000);
      expect(h.phase, ConnectionPhase.suspended);
      await h.clock.elapse(const Duration(seconds: 20));
      expect(h.transport.channels, hasLength(1));
      h.lifecycle.set(AppActivity.foreground);
      await settle();
      expect(h.transport.channels, hasLength(2));
      expect((await h.answerHello())['op'], 'resume');
    },
  );

  test('a start in the background on mobile stays suspended', () async {
    await h.dispose();
    h = Harness(activity: AppActivity.background);
    await h.signIn();
    h.connection.start();
    await settle();
    expect(h.phase, ConnectionPhase.suspended);
    expect(h.transport.attempts, 0);
  });

  test('a return to the foreground before the background close is handled reconnects', () async {
    await h.goLive();
    h.lifecycle.set(AppActivity.background);
    h.lifecycle.set(AppActivity.foreground);
    await settle();
    expect(h.transport.channels, hasLength(2));
    expect(h.phase, ConnectionPhase.connecting);
  });

  test('no socket opens in the background after a token renewal', () async {
    final ch = await h.goLive();
    ch.serverClose(4011);
    h.lifecycle.set(AppActivity.background);
    await settle();
    expect(h.refreshes, 1);
    expect(h.transport.channels, hasLength(1));
    expect(h.phase, ConnectionPhase.suspended);
    h.lifecycle.set(AppActivity.foreground);
    await settle();
    expect(h.transport.channels, hasLength(2));
  });

  test(
    'no socket opens in the background after the pending device is approved',
    () async {
      var polls = 0;
      h.api.routes['GET /devices/$deviceId'] = (_) {
        polls++;
        return FakeReply(200, deviceBody(polls < 2 ? 'pending' : 'active'));
      };
      final ch = await h.goLive();
      ch.serverClose(4016);
      await settle();
      h.lifecycle.set(AppActivity.background);
      await h.clock.elapse(const Duration(seconds: 10));
      expect(polls, 2);
      expect(h.transport.channels, hasLength(1));
      expect(h.phase, ConnectionPhase.suspended);
      h.lifecycle.set(AppActivity.foreground);
      await settle();
      expect(h.transport.channels, hasLength(2));
    },
  );

  test('a server close that arrives before the background close keeps its reaction', () async {
    final ch = await h.goLive();
    ch.serverClose(4012);
    h.lifecycle.set(AppActivity.background);
    await settle();
    expect(h.phase, ConnectionPhase.signedOut);
  });

  test('dispose does not wait for a paused listener of the states', () async {
    await h.goLive();
    // A Riverpod provider that nobody watches pauses its stream (15 7.2 status line).
    final sub = h.connection.states.listen((_) {})..pause();
    await h.connection.dispose().timeout(const Duration(seconds: 5));
    await sub.cancel();
  });
}
