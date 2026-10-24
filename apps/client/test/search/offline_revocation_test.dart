// SPDX-License-Identifier: Apache-2.0

// The client row of docs/spec/14-search.md 12 (brief M4-23 Design note 5, fixed requirement
// 14): messages and thread titles of two channels are indexed through the engine's real
// appliers (`thread.created`, `message.created` in a catch-up page), then `access.changed` with
// `removed` for one channel runs through the same path. The local index keeps no row of that
// channel and the offline search finds nothing there; the other channel stays. After
// `session.revoked` the engine wipes the database and nothing is found any more (14 6).

import 'dart:io';
import 'dart:typed_data';

import 'package:api_client/api_client.dart' as rest;
import 'package:client/search/local_search.dart';
import 'package:client/search/search_port.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../support/engine.dart';
import '../support/structure.dart';

const String at = '2026-10-17T09:00:00Z';

int _events = 0;

/// A durable event envelope of [type] at [seq] (04 2.2).
Map<String, Object?> envelope(
  String type, {
  required int seq,
  required Map<String, Object?> payload,
  required String resourceKind,
  required String resourceId,
  int? version,
  Map<String, Object?> scope = const {},
  bool personal = false,
}) => {
  'id': uuid(0x70000 + ++_events),
  'seq': seq,
  'type': type,
  'schema_version': 1,
  'occurred_at': at,
  'scope': scope,
  'resource': {'kind': resourceKind, 'id': resourceId, 'version': ?version},
  'audience': ?(personal ? 'self' : null),
  'payload': payload,
  'permission_version': 3,
};

/// `thread.created` of thread [n] in channel [channelN].
Map<String, Object?> threadCreated(int seq, int n, {required int channelN}) =>
    envelope(
      'thread.created',
      seq: seq,
      payload: {
        'thread': {
          'id': threadId(n).value,
          'channel_id': channelId(channelN).value,
          'kind': 'thread',
          'title': 'Deploy plan $n',
          'title_source': 'manual',
          'state': 'open',
          'last_message_seq': 0,
          'last_activity_at': at,
          'message_count': 0,
          'created_by': sam.value,
          'created_at': at,
          'version': 1,
          'tag_ids': <Object?>[],
        },
      },
      resourceKind: 'thread',
      resourceId: threadId(n).value,
      version: 1,
      scope: {
        'channel_id': channelId(channelN).value,
        'thread_id': threadId(n).value,
      },
    );

/// `message.created` at [threadSeq] of thread [threadN] in channel [channelN].
Map<String, Object?> messageCreated(
  int seq,
  int threadSeq, {
  required int threadN,
  required int channelN,
  required String content,
}) {
  final id = uuid(0x8000 + threadN * 0x10 + threadSeq);
  return envelope(
    'message.created',
    seq: seq,
    payload: {
      'message': {
        'id': id,
        'thread_id': threadId(threadN).value,
        'channel_id': channelId(channelN).value,
        'thread_seq': threadSeq,
        'author': {
          'id': sam.value,
          'kind': 'user',
          'handle': 'sam',
          'display_name': 'Sam',
        },
        'client_msg_id': uuid(0x9000 + threadN * 0x10 + threadSeq),
        'origin': 'human',
        'kind': 'text',
        'content_md': content,
        'content_format': 1,
        'silent': false,
        'attachments': <Object?>[],
        'mention_ids': <Object?>[],
        'reactions': <Object?>[],
        'pinned': false,
        'components': <Object?>[],
        'version': 1,
        'created_at': at,
      },
    },
    resourceKind: 'message',
    resourceId: id,
    version: 1,
    scope: {
      'channel_id': channelId(channelN).value,
      'thread_id': threadId(threadN).value,
    },
  );
}

/// `access.changed` removing channel [channelN] (04 8.3).
Map<String, Object?> accessRemoved(int seq, int channelN) => envelope(
  'access.changed',
  seq: seq,
  payload: {
    'permission_version': 4,
    'added': <Object?>[],
    'removed': [
      {'kind': 'channel', 'id': channelId(channelN).value},
    ],
    'changed': <Object?>[],
  },
  resourceKind: 'principal',
  resourceId: alex.value,
  personal: true,
);

/// `session.revoked` of this device's session (04 8.5).
Map<String, Object?> sessionRevoked(int seq) => envelope(
  'session.revoked',
  seq: seq,
  payload: {'session_id': 'sid-alex-laptop', 'reason': 'admin'},
  resourceKind: 'auth_session',
  resourceId: uuid(0x5e),
  personal: true,
);

/// The connection side the catch-up listener talks to; nothing is sent anywhere.
class _Control implements SessionControl {
  @override
  void setPhase(ConnectionPhase phase) {}

  @override
  bool send(GatewayFrame frame) => false;

  @override
  set lastProcessedSeq(GlobalSeq seq) {}

  @override
  String? get syncEpoch => 'epoch-1';
}

final class _Keys implements DatabaseKeySource {
  @override
  Future<Uint8List> obtainKey() async =>
      Uint8List.fromList(List.filled(32, 0x2a));
}

/// Delivers [events] as one catch-up page that ends at the `seq` of the last one.
Future<void> page(SyncEngine engine, List<Map<String, Object?>> events) async {
  final control = _Control();
  for (final e in events) {
    await engine.catchUp.onFrame(DispatchFrame(e), control);
  }
  final last = events.last['seq']! as int;
  await engine.catchUp.onFrame(
    CursorFrame(rest.GatewayCursor((b) => b..seq = last)),
    control,
  );
}

/// The rows of `search_index` of [channel], counted in the table itself.
Future<int> indexRows(LocalDatabase db, ChannelId channel) async {
  final row = await db
      .customSelect(
        "SELECT COUNT(*) AS n FROM search_index WHERE channel_id = '${channel.value}'",
      )
      .getSingle();
  return row.read<int>('n');
}

/// Every row of `search_index`.
Future<int> allIndexRows(LocalDatabase db) async {
  final row = await db
      .customSelect('SELECT COUNT(*) AS n FROM search_index')
      .getSingle();
  return row.read<int>('n');
}

/// Two channels of space 1 with a thread and two messages each, through the engine.
Future<SyncEngine> seeded(LocalDatabase db) async {
  final engine = idleEngine(db).engine;
  addTearDown(engine.dispose);
  await db.structureDao.upsertSpaceIfNewer(space(1));
  await db.structureDao.upsertChannelIfNewer(channel(1, name: 'general'));
  await db.structureDao.upsertChannelIfNewer(channel(2, name: 'server'));
  await page(engine, [
    threadCreated(11, 1, channelN: 1),
    messageCreated(12, 1, threadN: 1, channelN: 1, content: 'deploy at nine'),
    messageCreated(13, 2, threadN: 1, channelN: 1, content: 'deploy moved'),
    threadCreated(14, 2, channelN: 2),
    messageCreated(15, 1, threadN: 2, channelN: 2, content: 'deploy done'),
    messageCreated(16, 2, threadN: 2, channelN: 2, content: 'deploy notes'),
  ]);
  expect(
    (await db.syncStateDao.read()).cursor?.value,
    16,
    reason: 'the page committed, so the appliers ran',
  );
  return engine;
}

void main() {
  test('access.changed with removed deletes every index row of the channel', () async {
    final db = openInMemoryForTests();
    addTearDown(db.close);
    final engine = await seeded(db);
    // A transcript row of the channel, as the transcript feed writes it (14 6).
    await db.searchDao.indexTranscript(
      fileId: FileId.parse(uuid(0x9f1)),
      threadId: threadId(1),
      channelId: channelId(1),
      spaceId: spaceId(1),
      text: 'deploy checklist read aloud',
    );
    final search = LocalSearch(db);
    Future<Set<ChannelId?>> channelsOf(SearchType type) async => {
      for (final h in await search.search(
        SearchRequest(q: 'deploy', type: type),
        limit: 50,
      ))
        h.channelId,
    };

    final before = await indexRows(db, channelId(1));
    final otherBefore = await indexRows(db, channelId(2));
    // Thread title, two messages and the transcript of channel 1.
    expect(before, 4);
    expect(otherBefore, 3);
    expect(await channelsOf(SearchType.messages), {channelId(1), channelId(2)});
    expect(await channelsOf(SearchType.threads), {channelId(1), channelId(2)});
    expect(await channelsOf(SearchType.files), {channelId(1)});

    await page(engine, [accessRemoved(17, 1)]);

    final after = await indexRows(db, channelId(1));
    expect(after, 0);
    expect(await indexRows(db, channelId(2)), otherBefore);
    expect(await channelsOf(SearchType.messages), {channelId(2)});
    expect(await channelsOf(SearchType.threads), {channelId(2)});
    expect(await channelsOf(SearchType.files), isEmpty);
    expect(
      await search.search(
        SearchRequest(q: 'deploy', channelIds: [channelId(1)]),
        limit: 50,
      ),
      isEmpty,
    );
    printOnFailure('index rows of the removed channel: $before -> $after');
  });

  test('after the wipe of session.revoked nothing is found', () async {
    final dir = await Directory.systemTemp.createTemp('client_search_wipe_');
    addTearDown(() => dir.delete(recursive: true));
    final file = File('${dir.path}/store.db');
    final db = await openLocalStore(
      file: file,
      keys: _Keys(),
      inBackground: false,
    );
    final engine = await seeded(db);
    expect(await allIndexRows(db), 6);
    expect(
      await LocalSearch(db).search(const SearchRequest(q: 'deploy'), limit: 50),
      hasLength(4),
    );

    await page(engine, [sessionRevoked(17)]);
    expect(engine.catchUp.wiped, isTrue);
    expect(file.existsSync(), isFalse, reason: 'the wipe deletes the file');

    final reopened = await openLocalStore(
      file: file,
      keys: _Keys(),
      inBackground: false,
    );
    addTearDown(reopened.close);
    expect(await allIndexRows(reopened), 0);
    expect(
      await LocalSearch(reopened)
          .search(const SearchRequest(q: 'deploy'), limit: 50),
      isEmpty,
    );
  });

  test(
    'local snippets become segments; operators stay out of the local query',
    () {
      expect(segmentsOfSnippet('a \u0002deploy\u0003 b'), const [
        SearchSegment(text: 'a ', match: false),
        SearchSegment(text: 'deploy', match: true),
        SearchSegment(text: ' b', match: false),
      ]);
      expect(segmentsOfSnippet(''), isEmpty);
      expect(segmentsOfSnippet('<b>x</b>'), const [
        SearchSegment(text: '<b>x</b>', match: false),
      ]);
      expect(
        localQueryOf('"resume window" -draft OR heartbeat'),
        '"resume window" heartbeat',
      );
    },
  );
}
