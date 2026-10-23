// SPDX-License-Identifier: Apache-2.0

// Each fault of brief M4-17 Design note 4 has its effect: REST answers 409, 410, 412, 429 with
// Retry-After and 5xx, a lost response after the commit, a delay; on the gateway a close with
// any code, duplicate delivery, resync_required with each reason, 4012 with a revoked session,
// and held delivery.

import 'dart:io';

import 'package:api_client/api_client.dart';
import 'package:dio/dio.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late FakeWorldHarness h;
  late TestDevice alex;
  late RawClient raw;
  late String token;
  late String deviceId;

  setUp(() async {
    h = await FakeWorldHarness.start();
    alex = h.device();
    final s = await signedIn(h, alex);
    token = s.token;
    deviceId = s.deviceId;
    raw = RawClient(h.server.baseUri, alex.key);
  });
  tearDown(() async {
    raw.close();
    await h.stop();
  });

  Future<RawGateway> live({int? cursor, String? epoch}) async {
    final g = await RawGateway.connect(h.server.gatewayUri, raw);
    await g.identify(token, deviceId, cursor: cursor, epoch: epoch);
    await g.waitFor('ready');
    return g;
  }

  Matcher problem(int status, String code) => isA<DioException>()
      .having((e) => e.response?.statusCode, 'status', status)
      .having((e) => (e.response?.data as Map?)?['code'], 'code', code);

  group('REST', () {
    for (final (status, code) in const [
      (409, 'request_in_progress'),
      (410, 'gone'),
      (412, 'version_conflict'),
      (500, 'internal_error'),
      (503, 'unavailable'),
    ]) {
      test(
        'onRequest answers $status $code once, then the handler answers',
        () async {
          h.server.faults.onRequest('getMe', status: status, code: code);
          await expectLater(
            alex.api.getMeApi().getMe(),
            throwsA(problem(status, code)),
          );
          expect((await alex.api.getMeApi().getMe()).statusCode, 200);
          expect(h.server.faults.pending, isEmpty);
        },
      );
    }

    test(
      '429 carries Retry-After in seconds, for the given number of times',
      () async {
        h.server.faults.onRequest(
          'getMe',
          status: 429,
          code: 'rate_limited',
          retryAfter: const Duration(milliseconds: 1500),
          times: 2,
        );
        for (var i = 0; i < 2; i++) {
          final e = await alex.api.getMeApi().getMe().then<DioException?>(
            (_) => null,
            onError: (Object e) => e as DioException,
          );
          expect(e?.response?.statusCode, 429);
          expect(e?.response?.headers.value('retry-after'), '2');
        }
        expect((await alex.api.getMeApi().getMe()).statusCode, 200);
      },
    );

    test(
      'dropResponseAfterCommit stores the write and closes before the response',
      () async {
        h.server.faults.dropResponseAfterCommit('createMessage');
        final id = h.server.world.nextId();
        await expectLater(
          alex.api.getMessagesApi().createMessage(
            threadId: h.seed.releaseNotes,
            messageCreate: MessageCreate(
              (b) => b
                ..clientMsgId = id
                ..contentMd = 'Lost answer',
            ),
          ),
          throwsA(
            isA<DioException>().having((e) => e.response, 'response', isNull),
          ),
        );
        expect(
          h.server.world
              .messagesOf(h.seed.releaseNotes)
              .single['client_msg_id'],
          id,
        );
        expect(h.server.requestsOf('createMessage').single.status, 0);
      },
    );

    test('delay holds the response after the handler ran', () async {
      h.server.faults.delay(
        const Duration(milliseconds: 300),
        operationId: 'getMe',
      );
      final watch = Stopwatch()..start();
      await alex.api.getMeApi().getMe();
      expect(watch.elapsedMilliseconds, greaterThanOrEqualTo(300));
      final again = Stopwatch()..start();
      await alex.api.getMeApi().getMe();
      expect(again.elapsedMilliseconds, lessThan(300));
    });
  });

  group('gateway', () {
    test(
      'closeGateway closes with the chosen code and reason, also after a delay',
      () async {
        final g = await live();
        h.server.faults.closeGateway(4000, reason: 'unknown_error');
        await g.waitClosed();
        expect(g.closeCode, 4000);

        final later = await live();
        h.server.faults.closeGateway(
          4013,
          after: const Duration(milliseconds: 50),
        );
        expect(later.done, isFalse);
        await later.waitClosed();
        expect(later.closeCode, 4013);
      },
    );

    test('duplicateNextDispatch delivers the next dispatch twice', () async {
      final g = await live();
      // Sam follows the thread already, so the posts append no `notification_pref.updated`
      // of 13 3.2 between the two messages.
      h.server.world.setNotificationPref(
        h.seed.sam.id,
        'thread',
        h.seed.releaseNotes,
        'all',
      );
      h.server.faults.duplicateNextDispatch();
      h.seed.post(h.seed.sam, h.seed.releaseNotes, 'Twice');
      h.seed.post(h.seed.sam, h.seed.releaseNotes, 'Once');
      await g.waitFor('dispatch', count: 3);
      final seqs = [for (final d in g.data('dispatch')) d['seq']];
      expect(seqs, [seqs.first, seqs.first, (seqs.first! as int) + 1]);
      await g.close();
    });

    for (final reason in const [
      'epoch_changed',
      'retention',
      'too_far_behind',
      'cursor_ahead',
    ]) {
      test(
        'resyncRequired($reason) answers the next catch-up with resync_required',
        () async {
          h.server.faults.resyncRequired(reason);
          final g = await live(cursor: 0, epoch: h.server.world.syncEpoch);
          await g.waitFor('resync_required');
          final d = g.data('resync_required').single;
          expect(d['reason'], reason);
          expect(d['head_seq'], h.server.world.headSeq);
          expect(g.ops, isNot(contains('catchup.complete')));
          await g.close();
        },
      );
    }

    test('the reasons of resync_required also follow from the world', () async {
      h.seed.post(h.seed.sam, h.seed.releaseNotes, 'One');
      h.seed.post(h.seed.sam, h.seed.releaseNotes, 'Two');
      final epoch = h.server.world.syncEpoch;
      Future<String> reasonFor(int cursor, String epoch) async {
        final g = await live(cursor: cursor, epoch: epoch);
        await g.waitFor('resync_required');
        await g.close();
        return g.data('resync_required').single['reason']! as String;
      }

      expect(await reasonFor(99, epoch), 'cursor_ahead');
      h.server.world.catchupMaxEvents = 1;
      expect(await reasonFor(0, epoch), 'too_far_behind');
      h.server.world.dropEventsUpTo(1);
      expect(await reasonFor(0, epoch), 'retention');
      h.server.world.changeEpoch();
      expect(await reasonFor(2, epoch), 'epoch_changed');
    });

    test(
      'revoke closes with 4012 and the session is revoked for REST',
      () async {
        final g = await live();
        h.server.faults.revoke(deviceId: deviceId);
        await g.waitClosed();
        expect(g.closeCode, 4012);
        await expectLater(
          alex.api.getMeApi().getMe(),
          throwsA(problem(401, 'session_revoked')),
        );
      },
    );

    test('holdDispatches holds live events until releaseDispatches', () async {
      final g = await live();
      h.server.faults.holdDispatches();
      h.seed.post(h.seed.sam, h.seed.releaseNotes, 'Held');
      await Future<void>.delayed(const Duration(milliseconds: 100));
      expect(g.ops, isNot(contains('dispatch')));
      h.server.faults.releaseDispatches();
      await g.waitFor('dispatch');
      await g.close();
    });

    test('disconnect ends the connection without a close code', () async {
      final g = await live();
      h.server.disconnect(deviceId: deviceId);
      await g.waitClosed();
      expect(g.closeCode, anyOf(isNull, WebSocketStatus.noStatusReceived));
    });
  });
}
