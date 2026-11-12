// SPDX-License-Identifier: Apache-2.0

// The server search (brief M4-34 product requirement 13; 14 4): Sam writes a message with a word
// no other message has, and Alex finds it in D7. Alex attaches a file whose name has such a
// word, finds the file with the type "Files" and opens it: the hit leads to the message the
// file is attached to (audit finding fn-search-1: against the real server a file hit named no
// place and could not be opened).

import 'dart:io';

import 'package:client/files/file_ports.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';

import '../../test/support/file_fakes.dart';
import '../support/e2e_app.dart';
import '../support/e2e_context.dart';
import '../support/e2e_keys.dart';
import 'messaging_flows.dart' show composerText;

final AppLocalizations _en = lookupAppLocalizations(const Locale('en'));

/// A word no other object of the stack has. Letters only: the search splits words at digits as
/// well.
String _uniqueWord(String stem) =>
    '$stem${randomNonce().substring(0, 6).replaceAll(RegExp('[0-9]'), 'q')}';

/// Registers the search flows.
void searchFlows(E2eContext c) {
  final world = c.world;

  testWidgets('search finds a file and opens the message it is attached to', (
    tester,
  ) async {
    final word = _uniqueWord('ketch');
    // A space after the word: the server's parser reads "name.txt" as one token (14 3.3).
    final name = '$word plan.txt';
    final picker = FakePicker();
    final dir = await Directory.systemTemp.createTemp('e2e-search');
    final original = File('${dir.path}/$name')
      ..writeAsStringSync('The plan of the harbour\n');
    picker.next = [PickedFile(path: original.path, name: name)];
    final app = await E2eApp.start(
      tester,
      world,
      files: fakeFilePorts(picker: picker),
    );
    String route() =>
        GoRouter.of(tester.element(find.byType(Navigator).first))
            .routerDelegate
            .currentConfiguration
            .uri
            .toString();
    try {
      await app.signIn(await c.alexCode());
      await app.openChannel(world.generalId);
      await app.waitFor(find.byKey(const ValueKey('composer')));
      await app.tapFinder(
        find.byWidgetPredicate(
          (w) => w is DsTappable && w.label == _en.composerAttach,
        ),
      );
      await app.waitFor(
        find.descendant(
          of: find.byKey(const ValueKey('composer.attachments')),
          matching: find.textContaining(name),
        ),
      );
      final text = 'The harbour plan ${randomNonce().substring(0, 8)}';
      await tester.enterText(composerText, text);
      await tester.pump();
      await tester.sendKeyEvent(
        LogicalKeyboardKey.enter,
        physicalKey: PhysicalKeyboardKey.enter,
      );
      await tester.pump();

      String? fileId;
      String? messageId;
      await app.untilAsync('Sam sees the attachment', () async {
        final messages = await c.sam.messages(world.generalThreadId);
        final m = messages.where((m) => m.contentMd == text).firstOrNull;
        if (m == null || m.attachments.isEmpty) return false;
        fileId = m.attachments.first.file.id;
        messageId = m.id;
        return true;
      });

      GoRouter.of(tester.element(find.byKey(const ValueKey('column.sidebar'))))
          .go('/search');
      await app.waitFor(find.byKey(const ValueKey('search.field')));
      // The type "Files": a filter chosen before the first search starts none.
      await app.tap('search.filter.type');
      await app.tapSettled(find.text(_en.searchTypeFiles).last);
      final hit = find.byKey(ValueKey('search.hit.$fileId'));
      // The server indexes asynchronously (14 4): ask again, every half second, until the
      // answer holds the hit.
      final watch = Stopwatch()..start();
      while (hit.evaluate().isEmpty) {
        if (watch.elapsed >= patience) {
          fail('timed out waiting for the file hit');
        }
        await tester.enterText(
          find.byKey(const ValueKey('search.field')),
          word,
        );
        await tester.pump();
        await app.tap('search.submit');
        await tester.pump(const Duration(milliseconds: 500));
      }

      await app.tapFinder(hit);
      final target = '/t/${world.generalThreadId}?m=$messageId';
      await app.until('the message of the file', () => route() == target);
      // The thread shows the message with its attachment.
      await app.waitFor(find.byKey(ValueKey('file.$fileId.save')));
    } finally {
      await app.stop();
      await dir.delete(recursive: true);
    }
  });

  testWidgets('search finds a message of Sam', (tester) async {
    final word = _uniqueWord('zeppelin');
    final app = await E2eApp.start(tester, world);
    try {
      await app.signIn(await c.alexCode());
      final message = (await tester.runAsync(
        () => c.sam.post(world.generalThreadId, 'The $word lands at noon'),
      ))!;
      GoRouter.of(tester.element(find.byKey(const ValueKey('column.sidebar'))))
          .go('/search');
      await app.waitFor(find.byKey(const ValueKey('search.field')));
      final hit = find.byKey(ValueKey('search.hit.${message.id}'));
      // The server indexes asynchronously (14 4): ask again, every half second, until the
      // answer holds the hit.
      final watch = Stopwatch()..start();
      while (hit.evaluate().isEmpty) {
        if (watch.elapsed >= patience) fail('timed out waiting for the hit');
        await tester.enterText(
          find.byKey(const ValueKey('search.field')),
          word,
        );
        await tester.pump();
        await app.tap('search.submit');
        await tester.pump(const Duration(milliseconds: 500));
      }
    } finally {
      await app.stop();
    }
  });
}
