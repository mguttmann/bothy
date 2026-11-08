// SPDX-License-Identifier: Apache-2.0

// The personal archive of a channel follows the person to every device (01 6.22 item 4,
// PROJ-ARCHIVE; 04 14): two devices of Alex against the fake server. Alex's phone archives the
// forum (`putMySidebarChannel` with `hidden`), and the app on the laptop moves it into the
// section "Archived" through the engine's real appliers; the laptop restores it through its own
// writer and the outbox, and the phone reads the restored state from the server. Nothing of the
// channel changes for another person. Headless against the fake server through the app's
// providers.

import 'dart:io';

import 'package:api_client/api_client.dart';
import 'package:client/app/providers/ports.dart';
import 'package:client/app/providers/session.dart';
import 'package:client/sidebar/providers.dart';
import 'package:client/sidebar/sidebar_model.dart';
import 'package:client/sync/sidebar_ops.dart';
import 'package:domain/domain.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/app_scenario_client.dart';
import '../support/fake_instance.dart';
import '../support/fakes.dart';
import '../support/other_device.dart';

void main() {
  test('an archive on one device moves the forum on the other, and a restore '
      'there reaches the first', () async {
    final saved = HttpOverrides.current;
    HttpOverrides.global = null;
    final f = await FakeInstance.start();
    final phone = OtherDevice(f.server);
    final c = ProviderContainer(
      overrides: [appPortsProvider.overrideWithValue(f.ports)],
    );
    final laptop = AppScenarioClient(
      c,
      f.server,
      f.ports.network as SwitchNetwork,
    );
    final w = f.seed;
    final tree = c.listen(sidebarTreeProvider, (_, _) {});
    SidebarSpaceNode? team() =>
        tree.read().where((n) => n.entry.space.id.value == w.space).firstOrNull;
    List<String> shown() => [
      for (final ch in team()?.channels ?? const <SidebarChannel>[])
        ch.channel.id.value,
    ];
    List<String> archived() => [
      for (final ch in team()?.archived ?? const <SidebarChannel>[])
        ch.channel.id.value,
    ];
    try {
      await phone.signIn();
      await laptop.start();
      await eventually(
        'the tree with both channels',
        () => shown().contains(w.general) && shown().contains(w.support),
      );
      final order = shown();

      // The phone archives the support forum.
      final put = (await phone.auth.api.getPreferencesApi().putMySidebarChannel(
        channelId: w.support,
        sidebarChannelPut: SidebarChannelPut((b) => b..hidden = true),
      )).data!;
      expect(put.hiddenAt, isNotNull);
      await eventually(
        'the forum in the section Archived on the laptop',
        () => !shown().contains(w.support) && archived().contains(w.support),
      );
      expect(shown(), [
        for (final id in order)
          if (id != w.support) id,
      ]);

      // The laptop restores it through its writer and the outbox.
      await SidebarWriter(
        c.read(syncEngineProvider)!,
        c.read(localDatabaseProvider)!,
      ).setHidden(ChannelId.parse(w.support), hidden: false);
      expect(shown(), order, reason: 'back in its place at once');
      await eventually('the phone reads the restored forum', () async {
        final rows = (await phone.auth.api.getPreferencesApi().getMySidebar())
            .data!
            .channels!;
        final row = rows.where((r) => r.channelId == w.support).firstOrNull;
        return row != null && row.hiddenAt == null && row.version == 2;
      });
      expect(archived(), isEmpty);
      expect(
        f.server.world.snapshot(w.sam.id)['sidebar_channels'],
        isEmpty,
        reason: 'another person sees nothing of it',
      );
      expect(f.server.violations, isEmpty);
    } finally {
      tree.close();
      await laptop.stop();
      c.dispose();
      await phone.dispose();
      await f.stop();
      HttpOverrides.global = saved;
    }
  });
}
