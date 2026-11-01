// SPDX-License-Identifier: Apache-2.0

// "Arrange chats automatically" moves a post up the moment a message arrives (brief
// FIX-autoarrange-delay). Owner report of 2026-11-01: the post he had just written into moved to
// the top of its forum only about three minutes later. The server moves the thread's
// `last_activity_at` with every message but writes only `message.created` (no `thread.updated`),
// and the client did not apply that time to its stored thread, so the order waited for a later
// REST read of the thread. Two devices of Alex (the desktop app and the phone app) against one
// fake server: the order of the forum group follows an own message and a message of Sam at once,
// on both devices, without a refresh.

import 'dart:io';

import 'package:client/app/providers/session.dart';
import 'package:client/sidebar/providers.dart';
import 'package:client/sidebar/sidebar_threads.dart';
import 'package:client/thread/thread_actions.dart';
import 'package:domain/domain.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import '../support/fake_instance.dart';

void main() {
  HttpOverrides? saved;
  late FakeInstance first;
  late FakeInstance second;
  late ProviderContainer desktop;
  late ProviderContainer phone;

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    first = await FakeInstance.start();
    second = await first.sibling(platform: 'ios');
    desktop = appContainer(first);
    phone = appContainer(second);
  });

  tearDown(() async {
    desktop.dispose();
    phone.dispose();
    await second.stopSibling();
    await first.stop();
    HttpOverrides.global = saved;
  });

  Future<void> signIn(ProviderContainer c, FakeInstance f) async {
    final control = await connected(c, f);
    await control.loginWithPassword(
      WorldSeed.alexLogin,
      WorldSeed.password,
      WorldSeed.totp,
    );
    await eventually(
      'live',
      () => c.read(syncEngineProvider)?.state.phase == ConnectionPhase.live,
    );
  }

  test('an own message and a message of Sam move their post to the top of '
      'the forum at once, on both devices', () async {
    final seed = first.seed;
    final world = first.server.world;
    final forum = world.addForum(seed.space, 'deploys', {
      seed.alex.id: 'write',
      seed.sam.id: 'write',
    });
    final older = world.addThread(forum, 'Older', createdBy: seed.sam.id);
    final newer = world.addThread(forum, 'Newer', createdBy: seed.sam.id);
    final now = world.now();
    world.threads[older]!['last_activity_at'] = world.stamp(
      now.subtract(const Duration(hours: 2)),
    );
    world.threads[newer]!['last_activity_at'] = world.stamp(
      now.subtract(const Duration(hours: 1)),
    );

    final group = channelThreadsProvider(forum);
    for (final c in [desktop, phone]) {
      c.listen(group, (_, _) {});
    }
    await signIn(desktop, first);
    await signIn(phone, second);

    List<String> rows(ProviderContainer c) => [
      for (final r in c.read(group)?.rows ?? const <SidebarThread>[])
        r.thread.id.value,
    ];
    Future<void> bothShow(String what, List<String> order) => eventually(
      what,
      () =>
          rows(desktop).join() == order.join() &&
          rows(phone).join() == order.join(),
    );
    // The reads that replace a stored thread: before the fix only they moved a post.
    int threadReads() =>
        first.server.requestsOf('getThread').length +
        first.server.requestsOf('listChannelThreads').length;

    await bothShow('both posts by activity', [newer, older]);
    final reads = threadReads();

    // Alex writes into the older post on the desktop.
    await ThreadActions(desktop.read(syncEngineProvider)!)
        .send(ThreadId.parse(older), 'Ship it');
    await bothShow('the own message moved its post up', [older, newer]);

    // Sam writes into the other post; the stream brings it to both devices.
    seed.post(seed.sam, newer, 'Done here');
    await bothShow("Sam's message moved its post up", [newer, older]);
    expect(threadReads(), reads, reason: 'the stream alone moves the posts');
    expect(first.server.violations, isEmpty);
  });
}
