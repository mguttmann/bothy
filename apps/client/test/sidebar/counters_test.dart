// SPDX-License-Identifier: Apache-2.0

// The unread and mention counters of a channel row (04 14; brief M4-19 Tests): a message from
// another person raises unread, one mentioning the reader raises mentions, and
// `read_state.updated` resets them. Headless against the fake server through the app's
// providers.

import 'dart:io';

import 'package:client/app/providers/ports.dart';
import 'package:client/sidebar/providers.dart';
import 'package:client/sidebar/sidebar_model.dart';
import 'package:domain/domain.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/app_scenario_client.dart';
import '../support/fake_instance.dart';
import '../support/fakes.dart';

void main() {
  test('message.created raises unread and mentions; read_state.updated resets them', () async {
    final saved = HttpOverrides.current;
    HttpOverrides.global = null;
    final f = await FakeInstance.start();
    final c = ProviderContainer(
      overrides: [appPortsProvider.overrideWithValue(f.ports)],
    );
    final client = AppScenarioClient(
      c,
      f.server,
      f.ports.network as SwitchNetwork,
    );
    final w = f.seed;
    try {
      await client.start();
      final sub = c.listen(channelsOfSpaceProvider(w.space), (_, _) {});
      SidebarChannel general() => sub.read().singleWhere(
        (e) => e.channel.id == ChannelId.parse(w.general),
      );
      await eventually('the channel row', () {
        try {
          general();
          return true;
        } on StateError {
          return false;
        }
      });
      expect(general().counts, ThreadCounts.zero);

      w.post(w.sam, w.releaseNotes, 'Build is green');
      await eventually('one unread', () => general().counts.unread == 1);
      expect(general().counts.mentions, 0);
      expect(general().hasUnread, isTrue);

      final mention = f.server.world.createMessage(w.sam.id, w.releaseNotes, {
        'client_msg_id': f.server.world.nextId(),
        'content_md': '@alex please check',
        'mention_ids': [w.alex.id],
      }).message;
      await eventually(
        'two unread, one mention',
        () => general().counts.unread == 2 && general().counts.mentions == 1,
      );

      // Alex reads the thread on another device: the server's read state resets the counters.
      f.server.world.putReadState(
        w.alex.id,
        w.releaseNotes,
        mention['thread_seq']! as int,
      );
      await eventually(
        'read on another device',
        () => general().counts == ThreadCounts.zero,
      );
      expect(general().hasUnread, isFalse);
      sub.close();
    } finally {
      await client.stop();
      c.dispose();
      await f.stop();
      HttpOverrides.global = saved;
    }
  });
}
