// SPDX-License-Identifier: Apache-2.0

// D9 says when the build receives no push (brief M5-05 Design note 4): an Android build without
// a Firebase configuration, or an Apple build without the shared groups; a build without a push
// platform (Windows) and one with push show nothing.

import 'dart:io';

import 'package:client/app/providers/ports.dart';
import 'package:client/notifications/push_provider.dart';
import 'package:client/notifications_settings/notification_settings_page.dart';
import 'package:client/notifications_settings/providers.dart';
import 'package:client/settings/settings_providers.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/fakes.dart';
import '../support/widgets.dart';

Future<void> pumpPage(WidgetTester tester, {bool? available}) async {
  desktopWindow(tester);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        appPortsProvider.overrideWithValue(
          testPorts(dataRoot: Directory.systemTemp),
        ),
        settingsRepositoryProvider.overrideWithValue(null),
        notifyWriterProvider.overrideWithValue(null),
        notifyRefreshProvider.overrideWith((ref) async {}),
        isOwnerProvider.overrideWith((ref) async => false),
        levelTreeProvider.overrideWith((ref) async => const []),
        quietHoursRulesProvider.overrideWith((ref) => Stream.value(const [])),
        pushAvailableProvider.overrideWith((ref) async => available),
      ],
      child: hostScreen(
        const Material(
          child: SingleChildScrollView(child: NotificationSettingsPage()),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('a build without push says so', (tester) async {
    await pumpPage(tester, available: false);
    expect(find.text(en.notifySettingsPushUnavailable), findsOneWidget);
  });

  testWidgets('a build with push or without a push platform says nothing', (
    tester,
  ) async {
    await pumpPage(tester, available: true);
    expect(find.text(en.notifySettingsPushUnavailable), findsNothing);
    await pumpPage(tester);
    expect(find.text(en.notifySettingsPushUnavailable), findsNothing);
  });
}
