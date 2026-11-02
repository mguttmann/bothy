// SPDX-License-Identifier: Apache-2.0

// The cursor moves of the phone keyboards in the composer (FIX-ios-floating-cursor): on iOS a
// press and hold on the space bar turns the keyboard into a trackpad, and the engine hands the
// field `TextInputClient.updateFloatingCursor` start, update and end; on Android Gboard moves
// the cursor while the finger slides on the space bar, which arrives as editing values with the
// same text and a new selection. Both drive the thread composer of the phone here through the
// text input channel, as the engine does, and expect the selection to move and to stay.

import 'package:client/thread/history_column.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import '../phone/support/phone_app.dart' show phoneWindow;
import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';

void main() {
  Future<ColumnHost> open(WidgetTester tester, String platform) async {
    phoneWindow(tester);
    final host = await ColumnHost.open(tester);
    await host.run(tester, () => seedThread(host.db, messages: [msg(1)]));
    await host.pump(
      tester,
      HistoryColumn(threadId: threadId(1)),
      location: '/t/${threadId(1).value}',
      platform: platform,
      density: DsDensityMode.touch,
      overrides: [...threadOverrides(FakeThreadReads(host.db))],
    );
    await tester.showKeyboard(composerField);
    await host.settle(tester);
    return host;
  }

  TextEditingValue fieldValue(WidgetTester tester) =>
      tester.widget<TextField>(composerField).controller!.value;

  /// The platform's editing value [text] with the cursor at [at].
  void platformValue(WidgetTester tester, String text, int at) =>
      tester.testTextInput.updateEditingValue(
        TextEditingValue(
          text: text,
          selection: TextSelection.collapsed(offset: at),
        ),
      );

  /// The engine's `TextInputClient.updateFloatingCursor` for the connected field.
  Future<void> floating(
    WidgetTester tester,
    String state, [
    Offset offset = Offset.zero,
  ]) async {
    final setClient = tester.testTextInput.log.lastWhere(
      (c) => c.method == 'TextInput.setClient',
    );
    await tester.binding.defaultBinaryMessenger.handlePlatformMessage(
      SystemChannels.textInput.name,
      SystemChannels.textInput.codec.encodeMethodCall(
        MethodCall('TextInputClient.updateFloatingCursor', <dynamic>[
          (setClient.arguments as List<dynamic>)[0],
          'FloatingCursorDragState.$state',
          <String, dynamic>{'X': offset.dx, 'Y': offset.dy},
        ]),
      ),
      (_) {},
    );
  }

  /// A press and hold on the space bar, a move by [by] in steps and the release.
  Future<void> trackpad(WidgetTester tester, Offset by) async {
    await floating(tester, 'start');
    await tester.pump();
    for (var i = 1; i <= 10; i++) {
      await floating(tester, 'update', by * (i / 10));
      await tester.pump(const Duration(milliseconds: 16));
    }
    await floating(tester, 'end');
    await tester.pumpAndSettle();
  }

  group('iOS floating cursor', () {
    testWidgets('moving left puts the cursor into the text', (tester) async {
      debugDefaultTargetPlatformOverride = TargetPlatform.iOS;
      final host = await open(tester, 'ios');
      const text = 'Please restart the gateway';
      platformValue(tester, text, text.length);
      await tester.pump();
      await trackpad(tester, const Offset(-80, 0));
      final selection = fieldValue(tester).selection;
      expect(selection.isCollapsed, isTrue);
      expect(selection.baseOffset, inExclusiveRange(0, text.length));
      expect(fieldValue(tester).text, text);
      await host.dispose(tester);
      debugDefaultTargetPlatformOverride = null;
    });

    testWidgets('moving up goes to the line above', (tester) async {
      debugDefaultTargetPlatformOverride = TargetPlatform.iOS;
      final host = await open(tester, 'ios');
      const text = 'first line\nsecond line';
      platformValue(tester, text, text.length);
      await tester.pump();
      await trackpad(tester, const Offset(0, -22));
      final selection = fieldValue(tester).selection;
      expect(selection.isCollapsed, isTrue);
      expect(selection.baseOffset, lessThanOrEqualTo('first line'.length));
      expect(fieldValue(tester).text, text);
      await host.dispose(tester);
      debugDefaultTargetPlatformOverride = null;
    });
  });

  group('Android space bar slide', () {
    testWidgets('selection-only updates move the cursor and it stays', (
      tester,
    ) async {
      debugDefaultTargetPlatformOverride = TargetPlatform.android;
      final host = await open(tester, 'android');
      const text = 'Please restart the gateway';
      platformValue(tester, text, text.length);
      await tester.pump();
      for (final at in [25, 24, 20, 14, 7]) {
        platformValue(tester, text, at);
        await tester.pump();
        expect(fieldValue(tester).selection.baseOffset, at);
      }
      await host.settle(tester);
      expect(
        fieldValue(tester).selection,
        const TextSelection.collapsed(offset: 7),
      );
      expect(fieldValue(tester).text, text);
      await host.dispose(tester);
      debugDefaultTargetPlatformOverride = null;
    });
  });
}
