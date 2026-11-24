// SPDX-License-Identifier: Apache-2.0

// The fake imitates the replay rules of 04 5 (brief M4-17, Tests): a repeated client_msg_id
// returns the original with `Idempotent-Replay: true` and no new event; a different body gives
// 422 idempotency_key_reuse; an `Idempotency-Key` write repeats its stored answer.

import 'package:api_client/api_client.dart';
import 'package:dio/dio.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late FakeWorldHarness h;
  late TestDevice alex;

  setUp(() async {
    h = await FakeWorldHarness.start();
    alex = h.device();
    await signedIn(h, alex);
  });
  tearDown(() => h.stop());

  MessageCreate create(String id, String text) => MessageCreate(
    (b) => b
      ..clientMsgId = id
      ..contentMd = text,
  );

  test(
    'a replay returns the original with Idempotent-Replay and appends nothing',
    () async {
      final messages = alex.api.getMessagesApi();
      final id = h.server.world.nextId();
      final first = await messages.createMessage(
        threadId: h.seed.releaseNotes,
        messageCreate: create(id, 'Hello Sam'),
      );
      final head = h.server.world.headSeq;
      final second = await messages.createMessage(
        threadId: h.seed.releaseNotes,
        messageCreate: create(id, 'Hello Sam'),
      );
      expect(first.statusCode, 201);
      expect(second.statusCode, 200);
      expect(second.headers.value('idempotent-replay'), 'true');
      expect(second.data!.id, first.data!.id);
      expect(h.server.world.headSeq, head, reason: 'no new event');
      expect(h.server.world.messagesOf(h.seed.releaseNotes), hasLength(1));
      expect(h.server.violations, isEmpty);
    },
  );

  test(
    'the same client_msg_id with another body gives 422 idempotency_key_reuse',
    () async {
      final messages = alex.api.getMessagesApi();
      final id = h.server.world.nextId();
      await messages.createMessage(
        threadId: h.seed.releaseNotes,
        messageCreate: create(id, 'Hello Sam'),
      );
      await expectLater(
        messages.createMessage(
          threadId: h.seed.releaseNotes,
          messageCreate: create(id, 'Hello Robin'),
        ),
        throwsA(
          isA<DioException>()
              .having((e) => e.response?.statusCode, 'status', 422)
              .having(
                (e) => (e.response?.data as Map?)?['code'],
                'code',
                'idempotency_key_reuse',
              ),
        ),
      );
      expect(h.server.world.messagesOf(h.seed.releaseNotes), hasLength(1));
    },
  );

  test(
    'an Idempotency-Key write repeats its answer; another body gives 422',
    () async {
      final key = h.server.world.nextId();
      final me = alex.api.getMeApi();
      Future<Response<Template>> create(String text) =>
          me.createPersonalTemplate(
            templateCreate: TemplateCreate(
              (b) => b
                ..name = 'Greeting'
                ..contentMd = text,
            ),
            extra: {idempotencyKeyExtra: key},
          );
      final first = await create('Hello Sam');
      final again = await create('Hello Sam');
      expect(again.headers.value('idempotent-replay'), 'true');
      expect(again.data!.id, first.data!.id);
      await expectLater(
        create('Hello Robin'),
        throwsA(
          isA<DioException>().having(
            (e) => (e.response?.data as Map?)?['code'],
            'code',
            'idempotency_key_reuse',
          ),
        ),
      );
    },
  );

  test(
    'the key has no effect on an operation that does not declare it',
    () async {
      final key = h.server.world.nextId();
      final prefs = alex.api.getPreferencesApi();
      h.seed.post(h.seed.sam, h.seed.releaseNotes, 'one');
      Future<Response<ReadState>> put(int seq) => prefs.putReadState(
        threadId: h.seed.releaseNotes,
        readStatePut: ReadStatePut((b) => b..lastReadSeq = seq),
        extra: {idempotencyKeyExtra: key},
      );
      await put(0);
      // As the server: putReadState declares no Idempotency-Key, so the same key with
      // another body runs the operation instead of answering 422.
      expect((await put(1)).data!.lastReadSeq, 1);
    },
  );
}
