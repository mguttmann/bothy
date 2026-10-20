// SPDX-License-Identifier: Apache-2.0

// The paging of a post list belongs to the signed-in services: after a logout and a new
// login the list loads its pages again (docs/DECISIONS.md D-M4-19-16).

import 'dart:io';

import 'package:client/forum/providers.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import '../support/fake_instance.dart';

void main() {
  // flutter_test answers every HttpClient request with 400; this test talks to the fake.
  HttpOverrides? saved;
  late FakeInstance f;
  late ProviderContainer c;

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    f = await FakeInstance.start();
    c = appContainer(f);
  });

  tearDown(() async {
    c.dispose();
    await f.stop();
    HttpOverrides.global = saved;
  });

  Future<void> signIn(OnboardingController control) async {
    await control.loginWithPassword(
      WorldSeed.alexLogin,
      WorldSeed.password,
      WorldSeed.totp,
    );
    await eventually(
      'live',
      () =>
          c.read(onboardingControllerProvider).services?.engine.state.phase ==
          ConnectionPhase.live,
    );
  }

  test('the post list pages start again after a new login', () async {
    final forum = f.server.world.addForum(f.seed.space, 'ideas', {
      f.seed.alex.id: 'write',
    });
    final pager = forumPagerProvider(forum, 'last_activity');
    c.listen(pager, (_, _) {});
    final control = await connected(c, f);
    await signIn(control);
    await c.read(pager.notifier).loadMore();
    expect(c.read(pager).hasMore, isFalse);
    expect(c.read(pager).pages, 1);

    await control.logout();
    await control.loginAgain();
    await signIn(control);
    expect(c.read(pager).hasMore, isTrue);
    expect(c.read(pager).pages, 0);
  });
}
