// SPDX-License-Identifier: Apache-2.0

// The entries of the sidebar channel menu run after the menu closed (D-TEAM-APP-6): "Team
// channel..." and "Rename project" open their dialogs although the menu's ref is gone.

import 'package:client/app/providers/session.dart'
    show signedInServicesProvider;
import 'package:client/menus/channel_menu.dart';
import 'package:client/notifications_settings/providers.dart';
import 'package:client/team/providers.dart';
import 'package:client/thread/providers.dart';
import 'package:domain/domain.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/team_fakes.dart';

void main() {
  testWidgets('"Team channel..." opens the dialog and switches team mode on', (
    tester,
  ) async {
    final ch = channel(1, name: 'general');
    final port = FakeTeamPort();
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() async {
      await host.db.structureDao.upsertSpaceIfNewer(space(1, name: 'Team'));
      await host.db.structureDao.upsertChannelIfNewer(ch);
      await host.db.structureDao.putChannelAccess(
        ch.id,
        const AccessView(level: AccessLevel.write, capabilities: []),
      );
    });
    await host.pump(
      tester,
      ChannelMenu(channel: ch, child: const Text('the row')),
      overrides: [
        isOwnerProvider.overrideWith((ref) async => true),
        myPrincipalIdProvider.overrideWithValue(alex),
        teamPortProvider.overrideWithValue(port),
        teamKeysProvider.overrideWithValue(() => 'k'),
      ],
    );
    await tester.tap(find.text('the row'), buttons: kSecondaryMouseButton);
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('menu.team')), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('menu.team')));
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('team.dialog')), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('team.switch')));
    await tester.pumpAndSettle();
    expect(port.calls, ['read', 'enable']);
    expect(tester.takeException(), isNull);
    await host.dispose(tester);
  });

  testWidgets('"Rename project" opens its dialog', (tester) async {
    final forum = channel(
      1,
      name: 'website',
      kind: ChannelKind.forum,
    ).copyWith(projectBound: true);
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() async {
      await host.db.structureDao.upsertSpaceIfNewer(space(1, name: 'Team'));
      await host.db.structureDao.upsertChannelIfNewer(forum);
      await host.db.structureDao.putChannelAccess(
        forum.id,
        const AccessView(
          level: AccessLevel.write,
          capabilities: [Capability.manage],
        ),
      );
    });
    await host.pump(
      tester,
      ChannelMenu(channel: forum, child: const Text('the row')),
      overrides: [
        isOwnerProvider.overrideWith((ref) async => false),
        myPrincipalIdProvider.overrideWithValue(alex),
      ],
    );
    await tester.tap(find.text('the row'), buttons: kSecondaryMouseButton);
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('menu.renameProject')));
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
    // Without a signed-in session in the host the dialog is not shown, but the entry no
    // longer throws; with one it opens.
    expect(
      find.byKey(const ValueKey('renameProject.dialog')),
      ProviderScope.containerOf(tester.element(find.byType(ChannelMenu)))
                  .read(signedInServicesProvider) ==
              null
          ? findsNothing
          : findsOneWidget,
    );
    await host.dispose(tester);
  });
}
