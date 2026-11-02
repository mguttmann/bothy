// SPDX-License-Identifier: Apache-2.0

// The keyboard on the phone (FIX-keyboard-dismiss-focus-ring): a tap outside the focused text
// field closes it on every screen, as iOS and Android do, and the text stays. Before the fix a
// person on an iPhone could close the keyboard only by leaving the screen. The app installs
// DsKeyboardDismiss once around the navigator; the composer's send button still sends with the
// keyboard open.

import 'dart:io';

import 'package:client/onboarding/onboarding_controller.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/column_host.dart';
import '../support/new_chat_fixtures.dart';
import '../support/widgets.dart';
import 'support/phone_app.dart';
import 'support/phone_threads.dart';

/// The send button of the new chat view's composer.
final Finder _send = find.descendant(
  of: find.byKey(const ValueKey('newChat.composer')),
  matching: find.byWidgetPredicate(
    (w) => w is DsTappable && w.label == en.composerSend,
  ),
);

void main() {
  for (final platform in const ['ios', 'android']) {
    testPhone(
      'the app closes the keyboard on a tap outside the field',
      platform,
      (tester) async {
        final root = Directory.systemTemp.createTempSync('keyboard_dismiss');
        addTearDown(() => root.deleteSync(recursive: true));
        await PhoneApp.start(
          tester,
          root: root,
          step: OnboardingStep.server,
          platform: platform,
        );
        final field = find.byType(EditableText).first;
        await tester.tap(field);
        await tester.pump();
        await tester.enterText(field, 'chat.example.com');
        expect(tester.widget<EditableText>(field).focusNode.hasFocus, isTrue);
        expect(tester.testTextInput.isVisible, isTrue);

        // Empty space at the top of the screen, away from every control.
        await tester.tapAt(const Offset(195, 4));
        await tester.pumpAndSettle();
        expect(tester.widget<EditableText>(field).focusNode.hasFocus, isFalse);
        expect(tester.testTextInput.isVisible, isFalse);
        expect(
          tester.widget<EditableText>(field).controller.text,
          'chat.example.com',
        );
      },
    );

    testPhone(
      'a tap outside the composer closes its keyboard and keeps the draft',
      platform,
      (tester) async {
        final host = await ColumnHost.open(tester);
        await tester.runAsync(() => fillThreads(host.db));
        await pumpPhone(tester, host, platform: platform);
        await tester.tap(addAction(server, en));
        await settlePhone(tester, host);
        expect(composerFocused(tester), isTrue);
        await tester.enterText(composerField, 'Half a thought');
        await settlePhone(tester, host);
        expect(tester.testTextInput.isVisible, isTrue);

        // The empty space of the view between the title and the composer.
        final view = tester.getRect(find.byKey(const ValueKey('newChat.view')));
        await tester.tapAt(view.center);
        await settlePhone(tester, host);
        expect(composerFocused(tester), isFalse);
        expect(tester.testTextInput.isVisible, isFalse);
        expect(composerText(tester), 'Half a thought');
        await host.dispose(tester);
      },
    );

    testPhone('send works with one tap while the keyboard is open', platform, (
      tester,
    ) async {
      final host = await ColumnHost.open(tester);
      await tester.runAsync(() => fillThreads(host.db));
      await pumpPhone(tester, host, platform: platform);
      await tester.tap(addAction(server, en));
      await settlePhone(tester, host);
      await tester.enterText(composerField, 'Restart the gateway');
      await settlePhone(tester, host);
      expect(composerFocused(tester), isTrue);
      await tester.tap(_send);
      await settlePhone(tester, host);
      final sent = creates(
        await host.run(tester, host.db.outboxDao.allEntries),
      );
      expect(sent, hasLength(1));
      await host.dispose(tester);
    });
  }
}
