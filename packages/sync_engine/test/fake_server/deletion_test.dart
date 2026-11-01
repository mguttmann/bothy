// SPDX-License-Identifier: Apache-2.0

// Deleting and restoring in the fake (01 6.18, D-DEL-1 to D-DEL-6): the deletion info before
// and after, the events of D-DEL-5 with a higher version each time, the lists without the
// deleted item, `404` for a deleted thread, the undo window (`409 undo_window_passed`), a main
// thread (`409 main_thread`), missing rights (`403`) and `trash_project_folder` with the
// folder check (`409 project_folder_refused`, `trash_state` `requested`).

import 'package:api_client/api_client.dart';
import 'package:dio/dio.dart';
import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late FakeWorldHarness h;
  late FakeWorld world;
  late WorldSeed w;

  setUp(() async {
    h = await FakeWorldHarness.start();
    world = h.server.world;
    w = h.seed;
  });
  tearDown(() => h.stop());

  Future<ApiProblem?> problemOf(Future<Object?> call) async {
    try {
      await call;
      return null;
    } on DioException catch (e) {
      return ApiProblem.of(e);
    }
  }

  Map<String, Object?> payloadOf(FakeEvent e) =>
      e.envelope['payload']! as Map<String, Object?>;

  test('a chat: preview, delete, lists, 404, undo and the events', () async {
    final sam = h.device(login: WorldSeed.samLogin, keySeed: 2);
    await signedIn(h, sam);
    final threads = sam.api.getThreadsApi();
    w.post(w.sam, w.printer, 'Paper jam on floor 2');

    final preview = (await threads.getThreadDeletion(threadId: w.printer))
        .data!;
    expect(preview.state.name, 'live');
    expect(preview.contents.threads, 1);
    expect(preview.contents.messages, 1);

    final deleted = (await threads.deleteThread(threadId: w.printer)).data!;
    expect(deleted.state.name, 'pending');
    expect(deleted.undoUntil, isNotNull);
    final event = world.log.last;
    expect(event.type, 'thread.updated');
    expect(
      (payloadOf(event)['thread']! as Map<String, Object?>)['deleted_at'],
      isNotNull,
    );
    final deletedVersion =
        (event.envelope['resource']! as Map<String, Object?>)['version']!
            as int;

    final listed = (await threads.listChannelThreads(channelId: w.support))
        .data!
        .items;
    expect(listed.map((t) => t.id), isNot(contains(w.printer)));
    expect(
      (await problemOf(threads.getThread(threadId: w.printer)))?.status,
      404,
    );
    expect(
      (world.snapshot(w.sam.id)['active_threads']! as List).map(
        (t) => (t as Map)['id'],
      ),
      isNot(contains(w.printer)),
    );

    final again = (await threads.deleteThread(threadId: w.printer)).data!;
    expect(again.deletedAt, deleted.deletedAt, reason: 'the pending deletion');

    final restored = (await threads.restoreThread(threadId: w.printer)).data!;
    expect(restored.id, w.printer);
    final undo = world.log.last;
    expect(undo.type, 'thread.updated');
    final undone = payloadOf(undo)['thread']! as Map<String, Object?>;
    expect(undone.containsKey('deleted_at'), isFalse);
    expect(undone['version'], greaterThan(deletedVersion));
    expect((await threads.getThread(threadId: w.printer)).data!.id, w.printer);
    expect(h.server.violations, isEmpty);
  });

  test(
    'after the undo window the restore answers undo_window_passed',
    () async {
      final sam = h.device(login: WorldSeed.samLogin, keySeed: 2);
      await signedIn(h, sam);
      final threads = sam.api.getThreadsApi();
      world.deleteUndoWindow = Duration.zero;
      await threads.deleteThread(threadId: w.printer);
      final problem = await problemOf(
        threads.restoreThread(threadId: w.printer),
      );
      expect(problem?.status, 409);
      expect(problem?.reason, 'undo_window_passed');
      expect(
        world.deletionInfo(w.sam.id, 'thread', w.printer)['state'],
        'final',
      );
    },
  );

  test(
    'a main thread, a chat of someone else and a space need rights',
    () async {
      final sam = h.device(login: WorldSeed.samLogin, keySeed: 2);
      await signedIn(h, sam);
      final main = world.addMainThread(w.support, createdBy: w.sam.id);
      final threads = sam.api.getThreadsApi();
      final mainProblem = await problemOf(threads.deleteThread(threadId: main));
      expect(mainProblem?.status, 409);
      expect(mainProblem?.reason, 'main_thread');
      final foreign = await problemOf(
        threads.deleteThread(threadId: w.releaseNotes),
      );
      expect(foreign?.status, 403);
      final space = await problemOf(
        sam.api.getSpacesApi().deleteSpace(spaceId: w.space),
      );
      expect(space?.status, 403);
      final channel = await problemOf(
        sam.api.getChannelsApi().deleteChannel(channelId: w.support),
      );
      expect(channel?.status, 403);
      expect(world.deletions, isEmpty);
    },
  );

  test(
    'a space: deleted with its channels out of the lists, restored',
    () async {
      final alex = h.device();
      await signedIn(h, alex);
      final spaces = alex.api.getSpacesApi();
      final info = (await spaces.deleteSpace(spaceId: w.space)).data!;
      expect(info.state.name, 'pending');
      expect(info.contents.channels, 3);
      final event = world.log.last;
      expect(event.type, 'space.archived');
      final space = payloadOf(event)['space']! as Map<String, Object?>;
      expect(space['deleted_at'], isNotNull);
      expect(space['archived_at'], space['deleted_at']);
      expect((await spaces.listSpaces()).data!.items, isEmpty);
      expect(world.snapshot(w.alex.id)['channels'], isEmpty);

      final restored = (await spaces.restoreSpace(spaceId: w.space)).data!;
      expect(restored.archivedAt, isNull, reason: 'the archive the delete set');
      expect(world.log.last.type, 'space.updated');
      expect((await spaces.listSpaces()).data!.items, hasLength(1));
      expect(h.server.violations, isEmpty);
    },
  );

  test('a project forum: the folder check, the request and its undo', () async {
    final alex = h.device();
    await signedIn(h, alex);
    final forum = world.addForum(w.space, 'website', {w.alex.id: 'write'});
    world
      ..grantCapability(w.alex.id, forum, 'manage')
      ..setProjectBound(forum, bound: true)
      ..projectFolders[forum] = {
        'workspace_ref': 'website',
        'trash_check': {'ok': false, 'reason': 'uncommitted_changes'},
      };
    final channels = alex.api.getChannelsApi();
    final preview = (await channels.getChannelDeletion(channelId: forum)).data!;
    expect(preview.projectFolder!.trashCheck!.reason, 'uncommitted_changes');
    final trash = ChannelDelete((b) => b..trashProjectFolder = true);
    final refused = await problemOf(
      channels.deleteChannel(channelId: forum, channelDelete: trash),
    );
    expect(refused?.status, 409);
    expect(refused?.reason, 'project_folder_refused');

    world.projectFolders[forum]!['trash_check'] = {'ok': true};
    final info = (await channels.deleteChannel(
      channelId: forum,
      channelDelete: trash,
    )).data!;
    expect(info.projectFolder!.trashState!.name, 'requested');
    expect(world.deletions[forum]!.trashProjectFolder, isTrue);
    expect(world.log.last.type, 'channel.archived');
    expect(
      (await alex.api.getChannelsApi().listSpaceChannels(spaceId: w.space))
          .data!
          .items
          .map((c) => c.channel.id),
      isNot(contains(forum)),
    );

    await channels.restoreChannel(channelId: forum);
    expect(world.log.last.type, 'channel.updated');
    expect(world.projectFolders[forum]!.containsKey('trash_state'), isFalse);
    expect(world.deletions, isEmpty);
    expect(h.server.violations, isEmpty);
  });
}
