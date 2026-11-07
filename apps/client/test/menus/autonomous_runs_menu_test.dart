// SPDX-License-Identifier: Apache-2.0

// The channel switch "Let agents continue on their own" (01 6.21 item 5, 07 2.14 item 1; brief
// AR-08) on the desktop apps (macOS, Windows) and the mobile apps (iOS, Android): the channel
// menu has it for a person with `manage`, with the sentence that owners always may and every
// other person's wake-ups need the switch; a change puts one setChannelAutonomousRuns with the
// channel's version as If-Match into the outbox; it is hidden without `manage`, in a direct
// message and on a server before stage AR (the channel carries no switch).

import 'package:client/menus/autonomous_runs_dialog.dart';
import 'package:client/menus/channel_menu.dart';
import 'package:client/notifications_settings/providers.dart';
import 'package:client/sync/wake_ops.dart';
import 'package:client/thread/providers.dart';
import 'package:domain/domain.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/widgets.dart';

const String rowText = 'the row';

Future<void> openMenu(WidgetTester tester, String platform) async {
  if (platform == 'ios' || platform == 'android') {
    await tester.longPress(find.text(rowText));
  } else {
    await tester.tap(find.text(rowText), buttons: kSecondaryMouseButton);
  }
  await tester.pumpAndSettle();
}

const AccessView manage = AccessView(
  level: AccessLevel.write,
  capabilities: [Capability.manage],
);
const AccessView write = AccessView(level: AccessLevel.write, capabilities: []);

Future<ColumnHost> pumpMenu(
  WidgetTester tester,
  Channel channel, {
  AccessView access = manage,
  String platform = 'macos',
  bool? ofServer,
}) async {
  final host = await ColumnHost.open(tester);
  await tester.runAsync(() async {
    await host.db.structureDao.upsertSpaceIfNewer(space(1, name: 'Team'));
    await host.db.structureDao.upsertChannelIfNewer(channel);
    await host.db.structureDao.putChannelAccess(channel.id, access);
  });
  await host.pump(
    tester,
    ChannelMenu(channel: channel, child: const Text(rowText)),
    platform: platform,
    overrides: [
      isOwnerProvider.overrideWith((ref) async => false),
      myPrincipalIdProvider.overrideWithValue(alex),
      // What getChannel reports; the snapshot and the channel events carry no switch.
      autonomousRunsOfServerProvider(channel.id.value)
          .overrideWith((ref) async => ofServer),
    ],
  );
  return host;
}

final Finder entry = find.byKey(const ValueKey('menu.autonomousRuns'));
final Finder toggle = find.byKey(const ValueKey('autonomousRuns.switch'));

void main() {
  final general = channel(
    1,
    name: 'general',
  ).copyWith(allowAutonomousRuns: false);

  for (final platform in ['macos', 'windows', 'ios', 'android']) {
    testWidgets('$platform: manage switches it on with the sentence and '
        'If-Match', (tester) async {
      final host = await pumpMenu(tester, general, platform: platform);
      await openMenu(tester, platform);
      expect(entry, findsOneWidget);
      await tester.tap(entry);
      await tester.pumpAndSettle();
      expect(find.text(en.autonomousRunsExplanation), findsOneWidget);
      expect(
        tester
            .widget<Switch>(
              find.descendant(of: toggle, matching: find.byType(Switch)),
            )
            .value,
        isFalse,
      );
      await tester.tap(toggle);
      await host.settle(tester);
      final all = await host.run(tester, host.db.outboxDao.allEntries);
      final sent = [
        for (final e in all)
          if (e.kind == WakeOps.setChannelAutonomousRuns) e,
      ];
      expect(sent, hasLength(1));
      expect(sent.single.payload, contains('"allow_autonomous_runs":true'));
      expect(sent.single.payload, contains(general.id.value));
      expect(sent.single.expectedVersion, general.version);
      await host.dispose(tester);
    });
  }

  testWidgets('hidden without manage', (tester) async {
    final host = await pumpMenu(tester, general, access: write);
    await openMenu(tester, 'macos');
    expect(entry, findsNothing);
    await host.dispose(tester);
  });

  testWidgets('hidden on a server before stage AR', (tester) async {
    final host = await pumpMenu(tester, channel(1, name: 'general'));
    await openMenu(tester, 'macos');
    expect(entry, findsNothing);
    await host.dispose(tester);
  });

  for (final platform in ['macos', 'windows', 'ios', 'android']) {
    testWidgets('$platform: offered for a channel stored from the snapshot '
        'once getChannel carries the switch', (tester) async {
      final host = await pumpMenu(
        tester,
        channel(1, name: 'general'),
        platform: platform,
        ofServer: false,
      );
      await openMenu(tester, platform);
      expect(entry, findsOneWidget);
      await tester.tap(entry);
      await tester.pumpAndSettle();
      expect(
        tester
            .widget<Switch>(
              find.descendant(of: toggle, matching: find.byType(Switch)),
            )
            .value,
        isFalse,
      );
      await host.dispose(tester);
    });
  }

  testWidgets('hidden in a direct message', (tester) async {
    final dm = general.copyWith(kind: ChannelKind.dm, name: '');
    final host = await pumpMenu(tester, dm);
    await tester.tap(find.text(rowText), buttons: kSecondaryMouseButton);
    await tester.pumpAndSettle();
    expect(entry, findsNothing);
    await host.dispose(tester);
  });

  testWidgets('a switched-on channel shows the switch on', (tester) async {
    final host = await pumpMenu(
      tester,
      general.copyWith(allowAutonomousRuns: true),
    );
    await openMenu(tester, 'macos');
    await tester.tap(entry);
    await tester.pumpAndSettle();
    expect(
      tester
          .widget<Switch>(
            find.descendant(of: toggle, matching: find.byType(Switch)),
          )
          .value,
      isTrue,
    );
    await host.dispose(tester);
  });
}
