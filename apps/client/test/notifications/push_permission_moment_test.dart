// SPDX-License-Identifier: Apache-2.0

// The moment the app asks for the notification permission (15 10.1 and 10.2; briefs M5-04 Design
// note 5 and M5-05 Design note 4; docs/decisions.d/AUD-client-app-01.md AUD-CA1/4): once per
// run, when the person first opens a history. The main history of a text channel shows under
// `/c/<id>`, so it counts as much as a thread under `/t/<id>`; the post list of a forum and the
// start page are no history and ask nothing. The phone app runs over the real services against
// the fake server, with the push flow of an Apple build over the runner's host in memory.

import 'dart:io';

import 'package:client/platform/push_apple.dart';
import 'package:client/telemetry/client_metrics.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/testing.dart';

import '../phone/support/phone_device.dart';
import '../support/fake_instance.dart';
import '../support/push_fakes.dart';

void main() {
  HttpOverrides? saved;
  late FakeInstance fake;
  late FakePushHost host;
  late String forum;
  late String generalMain;
  PhoneDevice? device;

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    host = FakePushHost();
    fake = await FakeInstance.start(
      platform: 'ios',
      push: ApplePush(host: host, metrics: ClientMetrics()),
    );
    final seed = fake.seed;
    final world = fake.server.world;
    generalMain = world.addMainThread(seed.general, createdBy: seed.alex.id);
    world.addMainThread(seed.support, createdBy: seed.alex.id);
    forum = world.addForum(seed.space, 'requests', {
      seed.alex.id: 'write',
      seed.sam.id: 'write',
    });
    device = null;
  });

  tearDown(() async {
    device?.dispose();
    await fake.stop();
    HttpOverrides.global = saved;
  });

  testWidgets(
    'the main history of a text channel asks for the permission, once per run',
    (tester) async {
      final app = device = await PhoneDevice.start(tester, fake);
      final seed = fake.seed;
      // The account reached the push flow: only then a history asks (never before the login).
      await app.until(
        'the account handed over',
        () => host.settings.isNotEmpty,
      );
      expect(host.permissionRequests, 0, reason: 'the start page asks nothing');

      // The post list of a forum is no history.
      app.router.go('/c/$forum');
      await app.until(
        'the post list',
        () => find.byKey(ValueKey('postList.$forum')).evaluate().isNotEmpty,
      );
      await app.settle();
      expect(host.permissionRequests, 0, reason: 'a forum asks nothing');

      // The main history of a text channel is one.
      app.router.go('/c/${seed.general}');
      await app.until(
        'the main history',
        () => find
            .byKey(ValueKey('historyColumn.$generalMain.null'))
            .evaluate()
            .isNotEmpty,
      );
      await app.until('the question', () => host.permissionRequests == 1);

      // Once per run: another text channel and a thread ask nothing more.
      app.router.go('/c/${seed.support}');
      await app.until('support', () => app.location == '/c/${seed.support}');
      app.router.go('/t/${seed.releaseNotes}');
      await app.until(
        'the thread',
        () => app.location == '/t/${seed.releaseNotes}',
      );
      await app.settle();
      expect(host.permissionRequests, 1);
      await app.end();
    },
  );

  testWidgets('a thread still asks for the permission', (tester) async {
    final app = device = await PhoneDevice.start(tester, fake);
    await app.until('the account handed over', () => host.settings.isNotEmpty);
    app.router.go('/t/${fake.seed.releaseNotes}');
    await app.until('the question', () => host.permissionRequests == 1);
    await app.end();
  });
}
