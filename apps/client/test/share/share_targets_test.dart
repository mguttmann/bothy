// SPDX-License-Identifier: Apache-2.0

// The targets of the share sheet from the local database (brief M5-06 Design notes 1 and 7; 01
// 14 `client.share.recent_targets`): the threads the person last wrote into, newest first, a
// main thread as its channel; only channels the server reports as writable (level `write` or
// `trigger`, not read-only), no permission logic of the app's own; the search over channel
// names and thread titles under the same rule.

import 'dart:async';

import 'package:client/share/share_target.dart';
import 'package:client/share/share_targets.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/structure.dart';
import '../support/thread_fixtures.dart';

/// The thread titles the server would find.
class _Titles implements ThreadTitleSearch {
  List<ThreadTitleHit> hits = [];
  final List<String> queries = [];

  @override
  Future<List<ThreadTitleHit>> search(String query) async {
    queries.add(query);
    return hits;
  }
}

Channel _channel(
  int n,
  String name, {
  ChannelKind kind = ChannelKind.text,
  bool readOnly = false,
}) {
  final c = channel(n, kind: kind, name: name);
  return Channel(
    id: c.id,
    spaceId: c.spaceId,
    kind: c.kind,
    name: c.name,
    topic: c.topic,
    sortKey: c.sortKey,
    operator: c.operator,
    readOnly: readOnly,
    mainThreadId: c.mainThreadId,
    dmParticipantIds: c.dmParticipantIds,
    createdAt: c.createdAt,
    version: c.version,
    settingsVersion: c.settingsVersion,
  );
}

Thread _main(int channelN) {
  final t = thread(0x80 + channelN, channelN: channelN, title: 'main');
  return Thread(
    id: t.id,
    channelId: t.channelId,
    kind: ThreadKind.main,
    title: t.title,
    titleSource: t.titleSource,
    state: t.state,
    lastMessageSeq: t.lastMessageSeq,
    unreadFloorSeq: t.unreadFloorSeq,
    lastActivityAt: t.lastActivityAt,
    messageCount: t.messageCount,
    createdBy: t.createdBy,
    createdAt: t.createdAt,
    version: t.version,
    tagIds: t.tagIds,
  );
}

void main() {
  late LocalDatabase db;
  late ShareTargetSource source;
  late _Titles titles;

  Future<void> put(Channel c, AccessLevel level) async {
    await db.structureDao.upsertChannelIfNewer(c);
    await db.structureDao.putChannelAccess(
      c.id,
      AccessView(level: level, capabilities: const []),
    );
  }

  Future<void> say(
    int threadN,
    int channelN,
    int seq,
    int minutes, {
    PrincipalId? by,
  }) => db.messagesDao.upsertIfNewer(
    msg(
      seq,
      threadN: threadN,
      channelN: channelN,
      author: by ?? alex,
      minutes: minutes,
    ),
  );

  setUp(() async {
    db = openInMemoryForTests();
    titles = _Titles();
    source = ShareTargetSource(db);
    await db.structureDao.upsertSpaceIfNewer(space(1));
    await put(_channel(1, 'general'), AccessLevel.trigger);
    await put(_channel(2, 'ideas', kind: ChannelKind.forum), AccessLevel.write);
    await put(_channel(3, 'news'), AccessLevel.read);
    await put(_channel(4, 'archive', readOnly: true), AccessLevel.write);
    await db.threadsDao.upsertIfNewer(_main(1));
    await db.threadsDao.upsertIfNewer(
      thread(1, channelN: 1, title: 'Release notes'),
    );
    await db.threadsDao.upsertIfNewer(
      thread(2, channelN: 2, title: 'Printer jam'),
    );
    await db.threadsDao.upsertIfNewer(thread(3, channelN: 3, title: 'Updates'));
    await db.threadsDao.upsertIfNewer(thread(4, channelN: 4, title: 'Old'));
    await say(1, 1, 1, 1);
    await say(2, 2, 1, 2);
    await say(0x81, 1, 1, 3);
    await say(3, 3, 1, 4);
    await say(4, 4, 1, 5);
    await say(1, 1, 2, 6, by: sam);
  });

  tearDown(() => db.close());

  test('the recent targets are the writable threads Alex last wrote into, '
      'newest first, a main thread as its channel', () async {
    final recent = await source.recent(me: alex, max: 8);
    expect(recent, [
      ShareTarget(
        kind: ShareTargetKind.channel,
        channelId: channelId(1),
        threadId: threadId(0x81),
        path: 'general',
      ),
      ShareTarget(
        kind: ShareTargetKind.thread,
        channelId: channelId(2),
        threadId: threadId(2),
        path: 'ideas > Printer jam',
      ),
      ShareTarget(
        kind: ShareTargetKind.thread,
        channelId: channelId(1),
        threadId: threadId(1),
        path: 'general > Release notes',
      ),
    ]);
    expect(await source.recent(me: alex, max: 2), hasLength(2));
    expect(await source.recent(me: sam, max: 8), hasLength(1));
  });

  test(
    'a second message in the same thread moves it to the top once',
    () async {
      await say(1, 1, 3, 10);
      final recent = await source.recent(me: alex, max: 8);
      expect(
        [for (final t in recent) t.path],
        ['general > Release notes', 'general', 'ideas > Printer jam'],
      );
    },
  );

  test('a thread without a stored row is loaded for its title; one that cannot '
      'be loaded is left out and not asked again', () async {
    // Alex wrote into threads 5 and 6 of general, which this device never stored.
    await say(5, 1, 1, 7);
    await say(6, 1, 1, 8);
    final loaded = <ThreadId>[];
    final withLoad = ShareTargetSource(
      db,
      loadThread: (id) async {
        loaded.add(id);
        if (id == threadId(6)) throw StateError('not found');
        await db.threadsDao.upsertIfNewer(
          thread(5, channelN: 1, title: 'Invoices'),
        );
      },
    );
    final recent = await withLoad.recent(me: alex, max: 8);
    expect(recent.first.path, 'general > Invoices');
    expect(recent.where((t) => t.threadId == threadId(6)), isEmpty);
    expect(loaded, [threadId(6), threadId(5)]);
    await withLoad.recent(me: alex, max: 8);
    expect(loaded, [threadId(6), threadId(5)]);
  });

  test('a lost write level removes the targets of the channel and the change '
      'is reported', () async {
    final changes = StreamIterator(source.changes());
    expect(await changes.moveNext(), isTrue, reason: 'the first value');
    await db.structureDao.putChannelAccess(
      channelId(1),
      const AccessView(level: AccessLevel.read, capabilities: []),
    );
    expect(await changes.moveNext(), isTrue);
    await changes.cancel();
    expect(
      [for (final t in await source.recent(me: alex, max: 8)) t.path],
      ['ideas > Printer jam'],
    );
  });

  test(
    'channels by name: a text channel as its main timeline, a forum as a new '
    'post, only writable ones',
    () async {
      expect(await source.channelsMatching('GEN'), [
        ShareTarget(
          kind: ShareTargetKind.channel,
          channelId: channelId(1),
          threadId: threadId(0x81),
          path: 'general',
        ),
      ]);
      expect(await source.channelsMatching('ide'), [
        ShareTarget(
          kind: ShareTargetKind.newPost,
          channelId: channelId(2),
          path: 'ideas',
        ),
      ]);
      expect(await source.channelsMatching('news'), isEmpty);
      expect(await source.channelsMatching('archive'), isEmpty);
      expect(await source.channelsMatching('  '), isEmpty);
    },
  );

  test(
    'the search offers channels, then thread titles of writable channels',
    () async {
      titles.hits = [
        ThreadTitleHit(
          threadId: threadId(2),
          channelId: channelId(2),
          title: 'Printer jam',
        ),
        ThreadTitleHit(
          threadId: threadId(3),
          channelId: channelId(3),
          title: 'Updates',
        ),
        ThreadTitleHit(
          threadId: threadId(9),
          channelId: channelId(9),
          title: 'Unknown',
        ),
        ThreadTitleHit(
          threadId: threadId(0x81),
          channelId: channelId(1),
          title: 'main',
        ),
      ];
      final search = ShareTargetSearch(source: source, titles: titles);
      final found = await search.search(' i ');
      expect(titles.queries, ['i']);
      expect(
        [for (final t in found) '${t.kind.code} ${t.path}'],
        ['new_post ideas', 'thread ideas > Printer jam', 'channel general'],
      );
      expect(await search.search(''), isEmpty);
      expect(titles.queries, ['i'], reason: 'an empty query asks nobody');
    },
  );

  test('a failing title search still gives the channels', () async {
    final search = ShareTargetSearch(source: source, titles: _Failing());
    expect([for (final t in await search.search('gen')) t.path], ['general']);
  });
}

class _Failing implements ThreadTitleSearch {
  @override
  Future<List<ThreadTitleHit>> search(String query) =>
      Future.error(StateError('offline'));
}
