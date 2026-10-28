// SPDX-License-Identifier: Apache-2.0

// "New project" at a space row and in the space menu, for owners on the desktop only (01 6.9
// item 1; docs/decisions.d/M4-43.md D-M4-43-7, D-M4-43-12).

import 'package:client/notifications_settings/providers.dart';
import 'package:client/sidebar/sidebar_column.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import '../support/column_host.dart';
import '../support/structure.dart';

Future<void> fill(ColumnHost host) =>
    host.db.structureDao.upsertSpaceIfNewer(space(1, name: 'Engineering'));

Future<void> hoverSpace(WidgetTester tester) async {
  final mouse = await tester.createGesture(kind: PointerDeviceKind.mouse);
  await mouse.addPointer(location: Offset.zero);
  addTearDown(mouse.removePointer);
  await mouse.moveTo(
    tester.getCenter(find.byKey(ValueKey('sidebar.space.${spaceId(1).value}'))),
  );
  await tester.pump();
}

void main() {
  testWidgets('an owner gets "+" at the space row and the menu entry', (
    tester,
  ) async {
    final handle = tester.ensureSemantics();
    final l = await AppLocalizations.delegate.load(const Locale('en'));
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() => fill(host));
    await host.pump(
      tester,
      const SidebarColumn(),
      overrides: [isOwnerProvider.overrideWith((ref) async => true)],
    );
    await hoverSpace(tester);
    expect(
      find.bySemanticsLabel(l.newProjectIn('Engineering')),
      findsOneWidget,
    );
    await tester.tap(
      find.byKey(ValueKey('sidebar.space.${spaceId(1).value}')),
      buttons: kSecondaryButton,
    );
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('menu.newProject')), findsOneWidget);
    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    await host.dispose(tester);
    handle.dispose();
  });

  testWidgets('no entry for a person who is not an owner, nor on the phone', (
    tester,
  ) async {
    final handle = tester.ensureSemantics();
    final l = await AppLocalizations.delegate.load(const Locale('en'));
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() => fill(host));
    await host.pump(
      tester,
      const SidebarColumn(),
      overrides: [isOwnerProvider.overrideWith((ref) async => false)],
    );
    await hoverSpace(tester);
    expect(find.bySemanticsLabel(l.newProjectIn('Engineering')), findsNothing);
    await host.pump(
      tester,
      const SidebarColumn(phone: true),
      platform: 'ios',
      overrides: [isOwnerProvider.overrideWith((ref) async => true)],
    );
    expect(find.bySemanticsLabel(l.newProjectIn('Engineering')), findsNothing);
    await host.dispose(tester);
    handle.dispose();
  });
}
