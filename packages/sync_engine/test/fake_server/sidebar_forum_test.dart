// SPDX-License-Identifier: Apache-2.0

// The handlers M4-19 added to the fake (brief M4-19 Scope): the personal sidebar with the
// `fracindex` check of the server, forum tags, forum posts with their first message, closing
// and reopening by a new message, paged thread lists, and the instance logo.

import 'dart:typed_data';

import 'package:api_client/api_client.dart';
import 'package:built_collection/built_collection.dart';
import 'package:dio/dio.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late FakeWorldHarness h;
  late String forum;
  late String bug;
  late String idea;

  setUp(() async {
    h = await FakeWorldHarness.start();
    final w = h.seed;
    forum = h.server.world.addForum(w.space, 'questions', {
      w.alex.id: 'write',
      w.sam.id: 'write',
    });
    bug = h.server.world.addForumTag(forum, 'bug', color: 'rowan');
    idea = h.server.world.addForumTag(forum, 'idea');
  });
  tearDown(() => h.stop());

  Future<DioException?> failure(Future<Object?> call) =>
      call.then((_) => null, onError: (Object e) => e as DioException);

  test('the sidebar state of a space: put, list, event and snapshot', () async {
    final alex = h.device();
    await signedIn(h, alex);
    final prefs = alex.api.getPreferencesApi();
    final put = (await prefs.putMySidebarSpace(
      spaceId: h.seed.space,
      sidebarStatePut: SidebarStatePut(
        (b) => b
          ..collapsed = true
          ..sortKey = 'a0V',
      ),
    )).data!;
    expect(put.collapsed, isTrue);
    expect(put.sortKey, 'a0V');
    expect(put.version, 1);

    // Collapse alone keeps the order.
    final again = (await prefs.putMySidebarSpace(
      spaceId: h.seed.space,
      sidebarStatePut: SidebarStatePut((b) => b..collapsed = false),
    )).data!;
    expect(again.sortKey, 'a0V');
    expect(again.version, 2);

    final listed = (await prefs.getMySidebar()).data!.items;
    expect(listed.single.spaceId, h.seed.space);
    final event = h.server.world.log.lastWhere(
      (e) => e.type == 'sidebar.updated',
    );
    expect(event.audience, h.seed.alex.id);
    expect(
      (event.envelope['payload']! as Map<String, Object?>)['state'],
      isA<Map<String, Object?>>(),
    );
    expect(
      (h.server.world.snapshot(h.seed.alex.id)['sidebar']! as List).single,
      containsPair('sort_key', 'a0V'),
    );
    expect(h.server.violations, isEmpty);
  });

  test('a key outside the fractional index is refused with 400', () async {
    final alex = h.device();
    await signedIn(h, alex);
    final e = await failure(
      alex.api.getPreferencesApi().putMySidebarSpace(
        spaceId: h.seed.space,
        sidebarStatePut: SidebarStatePut(
          (b) => b
            ..collapsed = false
            ..sortKey = 'a00',
        ),
      ),
    );
    expect(e?.response?.statusCode, 400);
    expect(
      ((e?.response?.data as Map)['errors'] as List).single,
      containsPair('pointer', '/sort_key'),
    );
  });

  test('forum tags and a post with its first message', () async {
    final alex = h.device();
    await signedIn(h, alex);
    final tags = (await alex.api.getChannelsApi().listForumTags(
      channelId: forum,
    )).data!.items;
    expect([for (final t in tags) t.name], ['bug', 'idea']);

    final threads = alex.api.getThreadsApi();
    final refused = await failure(
      threads.createThread(
        channelId: forum,
        threadCreate: ThreadCreate((b) => b..title = 'No first message'),
      ),
    );
    expect(refused?.response?.statusCode, 400);
    expect(
      ((refused?.response?.data as Map)['errors'] as List).single,
      containsPair('code', 'required_for_forum_post'),
    );

    final created = (await threads.createThread(
      channelId: forum,
      threadCreate: ThreadCreate(
        (b) => b
          ..title = 'Printer jams'
          ..tagIds = ListBuilder([bug])
          ..firstMessage.clientMsgId = h.server.world.nextId()
          ..firstMessage.contentMd = 'Since Monday',
      ),
    ));
    expect(created.statusCode, 201);
    final thread = created.data!.thread;
    expect(thread.title, 'Printer jams');
    expect(thread.tagIds, [bug]);
    expect(created.data!.firstMessage!.contentMd, 'Since Monday');
    expect(
      h.server.world.log.map((e) => e.type),
      containsAllInOrder(['thread.created', 'message.created']),
    );
    expect(h.server.violations, isEmpty);
  });

  test('closing a post and reopening it by a new message', () async {
    final w = h.seed;
    final post = h.server.world.addThread(
      forum,
      'Login loop',
      createdBy: w.sam.id,
    );
    h.server.world.closeThread(post, actor: w.alex.id);
    expect(h.server.world.threads[post]!['state'], 'closed');
    expect(h.server.world.log.last.type, 'thread.closed');
    w.post(w.sam, post, 'Still broken');
    expect(h.server.world.threads[post]!['state'], 'open');
    expect(
      h.server.world.log.map((e) => e.type),
      containsAllInOrder([
        'thread.closed',
        'thread.reopened',
        'message.created',
      ]),
    );
  });

  test('threads come in pages, filtered and sorted', () async {
    final w = h.seed;
    final world = h.server.world;
    final ids = [
      for (var i = 0; i < 5; i++)
        world.addThread(forum, 'Post $i', createdBy: w.sam.id),
    ];
    world.closeThread(ids[1]);
    world.threads[ids[0]]!['tag_ids'] = [idea];
    final alex = h.device();
    await signedIn(h, alex);
    final api = alex.api.getThreadsApi();
    final first = (await api.listChannelThreads(
      channelId: forum,
      limit: 2,
      sort: ForumSort.created,
    )).data!;
    expect([for (final t in first.items) t.title], ['Post 4', 'Post 3']);
    final second = (await api.listChannelThreads(
      channelId: forum,
      limit: 2,
      sort: ForumSort.created,
      cursor: first.nextCursor,
    )).data!;
    expect([for (final t in second.items) t.title], ['Post 2', 'Post 1']);
    final last = (await api.listChannelThreads(
      channelId: forum,
      limit: 2,
      sort: ForumSort.created,
      cursor: second.nextCursor,
    )).data!;
    expect(last.nextCursor, isNull);
    final closed = (await api.listChannelThreads(
      channelId: forum,
      state: ThreadState.closed,
    )).data!;
    expect([for (final t in closed.items) t.title], ['Post 1']);
    final tagged = (await api.listChannelThreads(
      channelId: forum,
      tagId: idea,
    )).data!;
    expect([for (final t in tagged.items) t.title], ['Post 0']);
  });

  test('the logo of the instance', () async {
    final alex = h.device();
    final discovery = alex.api.getDiscoveryApi();
    expect(
      (await failure(discovery.getBrandingLogo()))?.response?.statusCode,
      404,
    );
    expect((await discovery.getDiscovery()).data!.branding.logoUrl, isNull);
    h.server.world.logo = Uint8List.fromList([137, 80, 78, 71]);
    expect((await discovery.getBrandingLogo()).data, [137, 80, 78, 71]);
    expect((await discovery.getDiscovery()).data!.branding.logoUrl, isNotNull);
    expect(h.server.violations, isEmpty);
  });
}
