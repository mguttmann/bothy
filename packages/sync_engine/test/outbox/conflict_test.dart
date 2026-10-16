// SPDX-License-Identifier: Apache-2.0

// `412 version_conflict` keeps both revisions; `410 message_deleted` keeps the text (04 13.3).

import 'dart:convert';

import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late Harness h;
  setUp(() => h = Harness.open());
  tearDown(() => h.close());

  const messageId = '0192f4a0-7b1c-7000-8a00-0000000000c1';

  Future<OutboxEntry> update(String text, {int version = 4}) =>
      h.outbox.enqueue(
        OutboxRequest(
          kind: 'updateMessage',
          threadId: threadA,
          payload: {
            'message_id': messageId,
            'version': version,
            'content_md': text,
          },
        ),
      );

  test(
    '412 version_conflict keeps the current revision for the person',
    () async {
      final exec = h.fake('updateMessage')
        ..then([
          problem(
            412,
            'version_conflict',
            current: {
              'id': messageId,
              'version': 5,
              'content_md': 'Edited by Sam',
            },
          ),
        ]);
      final e = await update('Edited by Alex');
      await h.outbox.start();
      await settle();
      final failed = (await h.entry(e.id))!;
      expect(failed.state, OutboxState.failed);
      expect(failed.lastErrorCode, 'version_conflict');
      expect(jsonDecode(failed.conflictCurrent!), {
        'id': messageId,
        'version': 5,
        'content_md': 'Edited by Sam',
      });
      expect(
        failed.payload,
        contains('Edited by Alex'),
        reason: 'both revisions stay',
      );
      expect(exec.calls, hasLength(1), reason: 'no automatic merge');

      // The person chooses: the edit targets the same message with the version they saw.
      final chosen = await h.outbox.edit(e.id, {
        'message_id': messageId,
        'version': 4,
        'content_md': 'Edited by Alex, again',
      });
      expect(chosen, isNot(e.id));
      expect(await h.entry(e.id), isNull);
      final next = (await h.entry(chosen))!;
      expect(next.expectedVersion?.value, 5);
      expect(jsonDecode(next.payload), {
        'message_id': messageId,
        'version': 5,
        'content_md': 'Edited by Alex, again',
      });
      await settle();
      expect(exec.calls.last.id, chosen);
      expect(exec.calls.last.payload['version'], 5);
    },
  );

  test(
    '410 message_deleted discards the edit and keeps the text for copying',
    () async {
      final exec = h.fake('updateMessage')
        ..then([problem(410, 'message_deleted')]);
      final e = await update('A long text Alex wrote');
      await h.outbox.start();
      await settle();
      final failed = (await h.entry(e.id))!;
      expect(failed.state, OutboxState.failed);
      expect(failed.lastErrorCode, 'message_deleted');
      expect(
        jsonDecode(failed.payload)['content_md'],
        'A long text Alex wrote',
      );
      await h.time.advance(const Duration(hours: 1));
      expect(exec.calls, hasLength(1));
      final views = await h.outbox.watchThread(threadA).first;
      expect(views.single.stateLabel, OutboxStateLabel.failed);
      expect(views.single.failureCode, 'message_deleted');
    },
  );
}
