// SPDX-License-Identifier: Apache-2.0

// The fake answers as the server where a difference would hide a client defect (audit package
// AUD-PKG-P04-client-packages): a failing handler is a 500 and a violation, not an empty 200;
// a move is 409 on another operator, writes thread.moved_out and drops the tags; a refused
// first message leaves no thread; Idempotency-Key follows the server's stage; a proof over the
// previous nonce gets the current one on a success; a problem names one correlation ID; a
// session with a half-open connection resumes and replaces it with 4017.

import 'dart:async';

import 'package:api_client/api_client.dart';
import 'package:built_collection/built_collection.dart';
import 'package:dio/dio.dart';
import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late FakeWorldHarness h;
  late FakeWorld world;
  late TestDevice alex;
  late RawClient raw;
  late String token;
  late String deviceId;

  setUp(() async {
    h = await FakeWorldHarness.start();
    world = h.server.world;
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

  Future<RawResponse> rawSend(
    String method,
    String path, {
    Map<String, String> headers = const {},
    Object? body,
    String? nonce,
  }) async => raw.send(
    method,
    path,
    headers: {
      'Authorization': 'DPoP $token',
      'DPoP': await raw.proof(
        method: method,
        htu: '${h.server.baseUri.origin}$path',
        accessToken: token,
        nonce: nonce ?? h.server.verifier.currentNonce,
      ),
      ...headers,
    },
    body: body,
  );

  test('a handler that throws answers 500 and records a violation', () async {
    h.server.handle('getMe', (r) => throw StateError('broken handler'));
    final r = await rawSend('GET', '/api/v1/me');
    expect(r.status, 500);
    expect(r.code, 'internal_error');
    expect(h.server.violations.single, contains('getMe'));
    expect(h.server.requests.last.status, 500);
  });

  test('a problem carries the correlation ID of its header', () async {
    final r = await rawSend('GET', '/api/v1/threads/${world.nextId()}');
    expect(r.status, 404);
    expect(
      (r.body! as Map)['correlation_id'],
      r.headers.value('correlation-id'),
    );
  });

  test(
    'a proof over the previous nonce gets DPoP-Nonce on a success',
    () async {
      final previous = h.server.verifier.currentNonce;
      h.server.verifier.rotateNonce();
      final r = await rawSend('GET', '/api/v1/me', nonce: previous);
      expect(r.status, 200);
      expect(r.headers.value('dpop-nonce'), h.server.verifier.currentNonce);
      final current = await rawSend('GET', '/api/v1/me');
      expect(current.status, 200);
      expect(current.headers.value('dpop-nonce'), isNull);
    },
  );

  group('Idempotency-Key', () {
    Future<RawResponse> create(String key) => rawSend(
      'POST',
      '/api/v1/me/templates',
      headers: {'Idempotency-Key': key},
      body: {'name': 'Greeting', 'content_md': 'Hello Sam'},
    );

    test('a key that is not a UUIDv7 is 400 validation_failed', () async {
      final r = await create('not-a-uuid');
      expect(r.status, 400);
      expect(r.code, 'validation_failed');
      expect(
        ((r.body! as Map)['errors'] as List).single,
        containsPair('pointer', '#header/Idempotency-Key'),
      );
    });

    test('an answer outside the stored statuses is not repeated', () async {
      final key = world.nextId();
      var calls = 0;
      h.server.handle('createPersonalTemplate', (r) {
        if (++calls == 1) {
          throw const FakeProblem(428, 'precondition_required');
        }
        return FakeResponse(
          201,
          world.addTemplate(
            scope: 'personal',
            ownerId: r.person.id,
            name: 'Greeting',
            contentMd: 'Hello Sam',
          ),
        );
      });
      expect((await create(key)).status, 428);
      final r = await create(key);
      expect(r.status, 201, reason: 'the 428 was not stored');
      expect(r.headers.value('idempotent-replay'), isNull);
    });
  });

  group('moveThread', () {
    late String other;
    late ThreadsApi threads;

    setUp(() {
      threads = alex.api.getThreadsApi();
      other = world.addChannel(h.seed.space, 'ops', {h.seed.alex.id: 'write'});
      world
        ..grantCapability(h.seed.alex.id, h.seed.general, 'manage')
        ..grantCapability(h.seed.alex.id, other, 'manage');
    });

    ThreadMove to(String channel) => ThreadMove(
      (b) => b
        ..targetChannelId = channel
        ..reason = 'Belongs to ops',
    );

    test('another operator is 409 operator_mismatch', () async {
      world.setOperator(other, h.seed.robin.id);
      await expectLater(
        threads.moveThread(
          threadId: h.seed.releaseNotes,
          threadMove: to(other),
        ),
        throwsA(
          isA<DioException>()
              .having((e) => e.response?.statusCode, 'status', 409)
              .having(
                (e) => (e.response?.data as Map?)?['code'],
                'code',
                'operator_mismatch',
              ),
        ),
      );
      expect(h.server.violations, isEmpty);
    });

    test('writes thread.moved to the target before thread.moved_out and drops the tags', () async {
      world.threads[h.seed.releaseNotes]!['tag_ids'] = ['tag-1'];
      final head = world.headSeq;
      final moved = (await threads.moveThread(
        threadId: h.seed.releaseNotes,
        threadMove: to(other),
      )).data!;
      expect(moved.tagIds, isEmpty);
      final events = world.eventsAfter(head).toList();
      expect(
        [for (final e in events) e.type],
        ['thread.moved', 'thread.moved_out'],
      );
      final out = events.last;
      expect(out.scope['channel_id'], h.seed.general);
      expect(out.envelope['resource'], {
        'kind': 'channel',
        'id': h.seed.general,
      });
      expect(out.envelope['payload'], {
        'thread_id': h.seed.releaseNotes,
        'channel_id': h.seed.general,
      });
      // Sam sees only the source channel: he gets thread.moved_out, not thread.moved.
      expect(world.mayReceive(h.seed.sam.id, out), isTrue);
      expect(world.mayReceive(h.seed.sam.id, events.first), isFalse);
      expect(h.server.violations, isEmpty);
    });
  });

  test('a refused first message leaves no forum post and no event', () async {
    final forum = world.addForum(h.seed.space, 'questions', {
      h.seed.alex.id: 'write',
    });
    world.channels[forum]!.json['read_only'] = true;
    final threadsBefore = world.threads.length;
    final head = world.headSeq;
    await expectLater(
      alex.api.getThreadsApi().createThread(
        channelId: forum,
        threadCreate: ThreadCreate(
          (b) => b
            ..title = 'Printer jams'
            ..tagIds = ListBuilder<String>()
            ..firstMessage.clientMsgId = world.nextId()
            ..firstMessage.contentMd = 'Since Monday',
        ),
      ),
      throwsA(
        isA<DioException>().having(
          (e) => (e.response?.data as Map?)?['code'],
          'code',
          'channel_read_only',
        ),
      ),
    );
    expect(world.threads.length, threadsBefore);
    expect(world.headSeq, head);
    expect(h.server.violations, isEmpty);
  });

  test(
    'a thread of a text channel without a parent message names the server code',
    () {
      expect(
        () => world.createThread(h.seed.alex.id, h.seed.general, {
          'title': 'No parent',
          'first_message': {'client_msg_id': world.nextId(), 'content_md': 'x'},
        }),
        throwsA(
          isA<FakeProblem>().having(
            (p) => p.errors?.single['code'],
            'code',
            'required_in_text_channel',
          ),
        ),
      );
    },
  );

  test('resume of a session whose connection still looks open takes it over with 4017', () async {
    final first = await RawGateway.connect(h.server.gatewayUri, raw);
    await first.identify(token, deviceId);
    await first.waitFor('ready');
    final session = first.data('ready').single['gateway_session_id']! as String;
    final second = await RawGateway.connect(h.server.gatewayUri, raw);
    second.send('resume', {
      'gateway_session_id': session,
      'access_token': token,
      'dpop': await raw.proof(
        method: 'GET',
        htu: '${h.server.baseUri.origin}/api/v1/gateway',
        accessToken: token,
        nonce: second.nonce,
      ),
      'cursor': world.headSeq,
      'sync_epoch': world.syncEpoch,
    });
    await second.waitFor('resumed');
    await first.waitClosed();
    expect(first.closeCode, 4017);
    await second.close();
  });

  test(
    'done of a scripted agent waits for the newest of two overlapping runs',
    () async {
      final agent = h.server.addAgent(
        h.seed.robin.id,
        const AgentScript([
          AgentStep.queue(),
          AgentStep.awaitFollowup(),
          AgentStep.absorbFollowup(),
        ], pause: Duration(milliseconds: 5)),
      );
      final channel =
          world.threads[h.seed.releaseNotes]!['channel_id']! as String;
      final second = world.addThread(
        channel,
        'Second',
        createdBy: h.seed.alex.id,
      );
      void post(String thread, String text, {bool mention = false}) =>
          world.createMessage(h.seed.alex.id, thread, {
            'client_msg_id': world.nextId(),
            'content_md': text,
            if (mention) 'mention_ids': [h.seed.robin.id],
          });
      post(h.seed.releaseNotes, 'Robin, first', mention: true);
      post(second, 'Robin, second', mention: true);
      final done = agent.done;
      var finished = false;
      unawaited(done.then((_) => finished = true));
      // The follow-up ends the first run only; the second still waits for its own.
      post(h.seed.releaseNotes, 'And this too');
      final first = agent.runIds.first;
      while (world.runs[first]!['followups'] is! Map ||
          ((world.runs[first]!['followups']! as Map).values.first
                  as Map)['state'] !=
              'absorbed') {
        await Future<void>.delayed(const Duration(milliseconds: 5));
      }
      await Future<void>.delayed(const Duration(milliseconds: 50));
      expect(finished, isFalse);
      post(second, 'And for you');
      await done;
    },
  );
}
