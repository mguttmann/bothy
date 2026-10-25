// SPDX-License-Identifier: Apache-2.0

// The channel menu against the real server (brief M4-34 product requirement 13; M4-33; 13 3.1,
// 07 2.4): Alex mutes general from its menu in the sidebar and the server stores the level; then
// "Export history" with a step-up writes the server's export, which holds Sam's message.

import 'dart:convert';
import 'dart:io';

import 'package:client/files/file_ports.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../test/support/file_fakes.dart';
import '../support/e2e_app.dart';
import '../support/e2e_context.dart';
import '../support/e2e_keys.dart';

/// Registers the channel menu flow.
void channelFlows(E2eContext c) {
  final world = c.world;

  testWidgets('channel menu: notification level and export', (tester) async {
    final nonce = randomNonce().substring(0, 8);
    final picker = FakePicker();
    final out = await Directory.systemTemp.createTemp('e2e-export');
    picker.saveTo = '${out.path}/general.ndjson';
    final app = await E2eApp.start(
      tester,
      world,
      files: FilePorts(picker: picker),
    );
    Future<void> openMenu() => app.tapFinder(
      find.byKey(ValueKey('sidebar.channel.${world.generalId}')),
      buttons: kSecondaryMouseButton,
    );

    try {
      final message = (await tester.runAsync(
        () => c.sam.post(world.generalThreadId, 'For the export $nonce'),
      ))!;
      await app.signIn(await c.alexCode());

      await openMenu();
      await app.tap('menu.notifications');
      await app.tap('menu.level.mute');
      await app.untilAsync(
        'the server has general muted',
        () async => await c.owner.levelOf('channel', world.generalId) == 'mute',
      );

      // The menu of the level closed with the choice; the export starts from a new one.
      await app.until(
        'the menu closed',
        () => find.byKey(const ValueKey('menu.level.mute')).evaluate().isEmpty,
      );
      await openMenu();
      await app.tap('menu.export');
      await app.tap('export.start');
      await app.waitFor(find.byKey(const ValueKey('stepUp.dialog')));
      await app.tap('stepUp.continue');
      await app.waitFor(find.byKey(const ValueKey('export.close')));
      final ids = [
        for (final line in const LineSplitter().convert(
          File(picker.saveTo!).readAsStringSync(),
        ))
          (jsonDecode(line) as Map)['id'],
      ];
      expect(ids, contains(message.id));
      await app.tap('export.close');
    } finally {
      await tester.runAsync(
        () => c.owner.setLevel('channel', world.generalId, 'mentions'),
      );
      await app.stop();
      await out.delete(recursive: true);
    }
  });
}
