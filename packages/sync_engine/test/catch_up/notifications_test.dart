// SPDX-License-Identifier: Apache-2.0

// The hint stream of the gateway (13 6.3, brief M4-30): a live `notification.created` reaches
// `CatchUpListener.notifications` with every field parsed and is never stored; a malformed one
// is skipped; one inside a catch-up page comes after the commit of the page; an unknown kind
// decodes to `HintKind.unknown`.

import 'package:domain/domain.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late World w;
  late List<NotificationCreated> received;
  setUp(() {
    w = World();
    received = [];
    w.catchUp.notifications.listen(received.add);
  });
  tearDown(() => w.dispose());

  final nid = id(0x801);

  Map<String, Object?> hint(Map<String, Object?> payload) => envelope(
    'notification.created',
    payload: payload,
    resourceKind: 'thread',
    resourceId: threadId(1),
    scope: {'channel_id': channelId(1), 'thread_id': threadId(1)},
    personal: true,
  );

  Map<String, Object?> fullPayload({String kind = 'mention'}) => {
    'nid': nid,
    'kind': kind,
    'thread_id': threadId(1),
    'message_id': messageId(1, 4),
    'quiet': true,
    'count': 3,
    'space_name': 'Team',
    'channel_name': 'general',
    'thread_title': 'Release notes',
    'sender_display_name': 'Sam',
    'excerpt': 'Alex, the build is ready',
    'attachment_count': 2,
  };

  Future<void> live() async {
    await w.connect(cursor: 30);
    await w.push('catchup.complete', {'head_seq': 30});
  }

  test(
    'a live notification.created reaches the stream, fully parsed',
    () async {
      await live();
      await w.dispatch([hint(fullPayload())]);
      final n = received.single;
      expect(n.nid, nid);
      expect(n.kind, HintKind.mention);
      expect(n.threadId, ThreadId.parse(threadId(1)));
      expect(n.messageId, MessageId.parse(messageId(1, 4)));
      expect(n.targetId, isNull);
      expect(n.quiet, isTrue);
      expect(n.count, 3);
      expect(n.spaceName, 'Team');
      expect(n.channelName, 'general');
      expect(n.threadTitle, 'Release notes');
      expect(n.senderDisplayName, 'Sam');
      expect(n.excerpt, 'Alex, the build is ready');
      expect(n.attachmentCount, 2);
      expect(n.toString(), isNot(contains('build is ready')));
      expect(n.toString(), isNot(contains('Sam')));
      expect(
        await w.cursor(),
        30,
        reason: 'an ephemeral event moves no cursor',
      );
      expect(
        await w.db.threadsDao.byId(ThreadId.parse(threadId(1))),
        isNull,
        reason: 'nothing of the hint is stored',
      );
      expect(
        await w.db.messagesDao.pageBefore(
          ThreadId.parse(threadId(1)),
          null,
          10,
        ),
        isEmpty,
      );
    },
  );

  test('a malformed notification.created is skipped without content', () async {
    await live();
    await w.dispatch([
      hint({...fullPayload(), 'thread_id': 'not-a-thread'}),
    ]);
    expect(received, isEmpty);
    expect(
      w.log.where((l) => l.contains('notification.created malformed')),
      hasLength(1),
    );
    expect(w.log.where((l) => l.contains('build is ready')), isEmpty);
    // The stream still works after a bad one.
    await w.dispatch([hint(fullPayload())]);
    expect(received, hasLength(1));
  });

  test('a hint inside a catch-up page comes after the commit', () async {
    await w.connect(cursor: 30);
    await w.dispatch([
      threadCreated(31, 1),
      hint(fullPayload(kind: 'dm')),
      messageCreated(32, 1),
    ]);
    expect(received, isEmpty, reason: 'the page is not committed yet');
    await w.push('catchup.complete', {'head_seq': 32});
    expect(received.single.kind, HintKind.dm);
    expect(await w.cursor(), 32);
  });

  test('an unknown kind decodes to HintKind.unknown', () async {
    await live();
    await w.dispatch([
      hint({'nid': nid, 'kind': 'future_kind', 'quiet': false, 'count': 1}),
    ]);
    final n = received.single;
    expect(n.kind, HintKind.unknown);
    expect(n.threadId, isNull);
    expect(n.messageId, isNull);
    expect(n.spaceName, isNull);
  });

  group('NotificationCreated.fromPayload', () {
    test('reads the required members and the defaults', () {
      final target = id(0x802);
      final n = NotificationCreated.fromPayload({
        'nid': nid,
        'kind': 'security',
        'target_id': target,
        'quiet': false,
        'count': 1,
      });
      expect(n.kind, HintKind.security);
      expect(n.targetId, target);
      expect(n.excerpt, isNull);
      expect(n.attachmentCount, isNull);
    });

    for (final (name, payload) in <(String, Map<String, Object?>)>[
      ('no nid', {'kind': 'dm', 'quiet': false, 'count': 1}),
      (
        'a nid that is no UUID',
        {'nid': 'x', 'kind': 'dm', 'quiet': false, 'count': 1},
      ),
      ('no kind', {'nid': nid, 'quiet': false, 'count': 1}),
      (
        'a quiet that is no boolean',
        {'nid': nid, 'kind': 'dm', 'quiet': 'no', 'count': 1},
      ),
      ('no count', {'nid': nid, 'kind': 'dm', 'quiet': false}),
      (
        'a bad message_id',
        {
          'nid': nid,
          'kind': 'dm',
          'quiet': false,
          'count': 1,
          'message_id': 'm',
        },
      ),
      (
        'a bad target_id',
        {
          'nid': nid,
          'kind': 'admin',
          'quiet': false,
          'count': 1,
          'target_id': 't',
        },
      ),
      (
        'an excerpt that is no string',
        {'nid': nid, 'kind': 'dm', 'quiet': false, 'count': 1, 'excerpt': 3},
      ),
    ]) {
      test('throws a FormatException for $name', () {
        expect(
          () => NotificationCreated.fromPayload(payload),
          throwsFormatException,
        );
      });
    }
  });

  test('every wire code maps both ways', () {
    for (final kind in HintKind.values) {
      if (kind == HintKind.unknown) continue;
      expect(HintKind.fromCode(kind.code), kind);
    }
    expect(HintKind.runDone.code, 'run_done');
    expect(HintKind.inputNeeded.code, 'input_needed');
    expect(HintKind.fromCode('banner'), HintKind.unknown);
  });

  test('events after dispose are dropped silently', () async {
    await live();
    await w.catchUp.dispose();
    await w.dispatch([hint(fullPayload())]);
    expect(received, isEmpty);
    expect(w.log.where((l) => l.contains('failed')), isEmpty);
  });
}
