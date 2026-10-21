// SPDX-License-Identifier: Apache-2.0

// The Notifications settings of M4-30 (brief M4-30 Scope and requirements 17 and 18): the
// sentence that notifications arrive only while the app runs, and "Start at login", a device
// setting that is off by default, written to the system only when the person turns it on.

import 'dart:io';

import 'package:client/app/providers/ports.dart';
import 'package:client/desktop/start_at_login.dart';
import 'package:client/settings/notifications_entry.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/fakes.dart';
import '../support/widgets.dart';

Widget entry(StartAtLogin login) => ProviderScope(
  overrides: [
    appPortsProvider.overrideWithValue(
      testPorts(
        dataRoot: Directory.systemTemp,
        startAtLogin: () async => login,
      ),
    ),
  ],
  child: hostScreen(const Material(child: NotificationsEntry())),
);

/// A login start kept in memory that counts the writes.
class MemoryStartAtLogin implements StartAtLogin {
  MemoryStartAtLogin({this.supported = true});

  @override
  final bool supported;

  bool on = false;
  final List<bool> writes = [];

  @override
  Future<bool> isEnabled() async => on;

  @override
  Future<void> setEnabled(bool value) async {
    writes.add(value);
    on = value;
  }
}

void main() {
  testWidgets(
    'the sentence and the switch, off by default and nothing written',
    (tester) async {
      final login = MemoryStartAtLogin();
      await tester.pumpWidget(entry(login));
      await tester.pumpAndSettle();
      expect(find.text(en.settingsNotificationsWhileRunning), findsOneWidget);
      expect(find.text(en.settingsStartAtLogin), findsOneWidget);
      final sw = tester.widget<Switch>(
        find.byKey(const ValueKey('settings.startAtLogin')),
      );
      expect(sw.value, isFalse);
      expect(login.writes, isEmpty);
    },
  );

  testWidgets('turning it on writes to the system and reads it back', (
    tester,
  ) async {
    final login = MemoryStartAtLogin();
    await tester.pumpWidget(entry(login));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('settings.startAtLogin')));
    await tester.pumpAndSettle();
    expect(login.writes, [true]);
    expect(
      tester
          .widget<Switch>(find.byKey(const ValueKey('settings.startAtLogin')))
          .value,
      isTrue,
    );
  });

  testWidgets('without a login start on the system: a sentence, no switch', (
    tester,
  ) async {
    final login = MemoryStartAtLogin(supported: false);
    await tester.pumpWidget(entry(login));
    await tester.pumpAndSettle();
    expect(find.text(en.settingsNotificationsWhileRunning), findsOneWidget);
    expect(find.text(en.settingsStartAtLoginUnavailable), findsOneWidget);
    expect(find.byKey(const ValueKey('settings.startAtLogin')), findsNothing);
  });
}
