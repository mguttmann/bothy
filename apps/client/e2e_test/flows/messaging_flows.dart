// SPDX-License-Identifier: Apache-2.0

// Sending and receiving between Alex in the app and Sam through the API, online and offline with
// the outbox (brief M4-34 product requirement 13, Design note 2; 04 5.1, 15 7.5): with the
// network of the app's host cut, two messages wait as "Sending"; after the network is back each
// reaches the server exactly once.

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/sync_engine.dart' show ConnectionPhase;

import '../support/e2e_app.dart';
import '../support/e2e_context.dart';
import '../support/e2e_keys.dart';

final AppLocalizations _en = lookupAppLocalizations(const Locale('en'));

/// The text field of the composer.
final Finder composerText = find.descendant(
  of: find.byKey(const ValueKey('composer')),
  matching: find.byType(TextField),
);

/// Types [text] into the composer and sends it with Enter.
Future<void> sendText(WidgetTester tester, String text) async {
  await tester.enterText(composerText, text);
  await tester.pump();
  await tester.sendKeyEvent(LogicalKeyboardKey.enter);
  await tester.pump();
}

/// Registers the messaging flows.
void messagingFlows(E2eContext c) {
  final world = c.world;

  testWidgets('send and receive online between Alex and Sam', (tester) async {
    final app = await E2eApp.start(tester, world);
    final nonce = randomNonce().substring(0, 8);
    try {
      await app.signIn(await c.alexCode());
      await app.openChannel(world.generalId);
      await app.waitFor(find.byKey(const ValueKey('composer')));

      await sendText(tester, 'Hello Sam $nonce');
      await app.untilAsync(
        'Sam reads the message',
        () async =>
            (await c.sam.messages(world.generalThreadId))
                .any((m) => m.contentMd == 'Hello Sam $nonce'),
      );
      final answer = (await tester.runAsync(
        () => c.sam.post(world.generalThreadId, 'Hello Alex $nonce'),
      ))!;
      await app.waitFor(find.byKey(ValueKey('message.${answer.id}')));
      expect(find.text('Hello Alex $nonce'), findsOneWidget);
    } finally {
      await app.stop();
    }
  });

  testWidgets('offline send through the outbox, delivered exactly once', (
    tester,
  ) async {
    final app = await E2eApp.start(tester, world);
    final nonce = randomNonce().substring(0, 8);
    final texts = ['First offline $nonce', 'Second offline $nonce'];
    try {
      await app.signIn(await c.alexCode());
      await app.openChannel(world.generalId);
      await app.waitFor(find.byKey(const ValueKey('composer')));

      c.network.cut();
      await app.until(
        'offline',
        () => app.state.services!.engine.state.phase != ConnectionPhase.live,
      );
      for (final text in texts) {
        await sendText(tester, text);
      }
      await app.until(
        'two waiting messages',
        () => find.text(_en.messageSending).evaluate().length >= 2,
      );
      final before = (await tester.runAsync(
        () => c.sam.messages(world.generalThreadId),
      ))!;
      expect(before.where((m) => texts.contains(m.contentMd)), isEmpty);

      c.network.restore();
      await app.waitLive();
      await app.until(
        'nothing waits any more',
        () => find.text(_en.messageSending).evaluate().isEmpty,
      );
      final after = (await tester.runAsync(
        () => c.sam.messages(world.generalThreadId),
      ))!;
      for (final text in texts) {
        expect(
          after.where((m) => m.contentMd == text),
          hasLength(1),
          reason: '$text exactly once',
        );
      }
      final order = [
        for (final m in after)
          if (texts.contains(m.contentMd)) m.contentMd,
      ];
      expect(order, texts, reason: 'the outbox keeps the order');
    } finally {
      c.network.restore();
      await app.stop();
    }
  });
}
