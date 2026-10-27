// SPDX-License-Identifier: Apache-2.0

// Offline on the phone (brief M5-02 Tests; 01 5.3 M7, 6.6): the fake drops the connection and
// the network goes; the banner appears with the waiting entries, a message waits with "sending"
// and is sent after the reconnect, and the banner goes.

import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

import '../../test/support/fakes.dart' show SwitchNetwork;
import '../../test/support/widgets.dart' show en;
import 'support/phone_flow.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('banner, sending, sent after the reconnect', (tester) async {
    final app = await startPhone(tester);
    final world = app.fake.server.world;
    final seed = app.fake.seed;
    final forum = world.addForum(seed.space, 'requests', {
      seed.alex.id: 'write',
    });
    final thread = world.addThread(forum, 'Gateway', createdBy: seed.alex.id);
    final first = seed.post(seed.alex, thread, 'The gateway drops.');
    try {
      await signIn(app);
      await openThread(app, forum, thread, first['id']!);

      final network = app.fake.ports.network as SwitchNetwork;
      network.set(false);
      app.fake.server.disconnect(deviceId: app.deviceId);
      await app.waitFor(find.byKey(const ValueKey('shell.offline')));

      await sendText(tester, 'And summarize afterwards.');
      await app.waitFor(find.text(en.messageSending));
      await app.waitFor(find.text(en.phoneOfflineWaiting(1)));
      expect(
        world
            .messagesOf(thread)
            .where((m) => m['content_md'] == 'And summarize afterwards.'),
        isEmpty,
      );

      network.set(true);
      await until(
        tester,
        'sent after the reconnect',
        () => world
            .messagesOf(thread)
            .any((m) => m['content_md'] == 'And summarize afterwards.'),
      );
      await until(
        tester,
        'banner gone',
        () => find.byKey(const ValueKey('shell.offline')).evaluate().isEmpty,
      );
      expect(app.fake.server.violations, isEmpty);
    } finally {
      await app.stop();
    }
  });
}
