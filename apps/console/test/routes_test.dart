// SPDX-License-Identifier: Apache-2.0

import 'package:console/console.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

void main() {
  // ADR 0028 "Confirmation": the console has no route for channels, threads or messages.
  test('no route for channels, threads, messages or spaces', () {
    const forbidden = [
      'channel',
      'thread',
      'message',
      'space',
      '/t/',
      '/c/',
      '/s/',
    ];
    for (final page in ConsolePage.values) {
      final path = '${page.fullPath}/';
      for (final word in forbidden) {
        expect(path.contains(word), isFalse, reason: '$path contains $word');
      }
    }
  });

  test('the route table has exactly the paths of the brief', () {
    expect(ConsolePage.values.map((p) => p.fullPath), [
      '/console/login',
      '/console/login/callback',
      '/console/step-up',
      '/console/step-up/callback',
      '/console/logged-out',
      '/console/live',
      '/console/stats',
      '/console/hosts',
      '/console/people',
      '/console/groups',
      '/console/devices',
      '/console/agents',
      '/console/apps',
      '/console/audit',
      '/console/import',
      '/console/instance',
      '/console/status',
    ]);
  });

  test(
    'every navigation entry has a route and a text in both languages',
    () async {
      expect(consoleNavigation, hasLength(12));
      for (final locale in AppLocalizations.supportedLocales) {
        final l = await AppLocalizations.delegate.load(locale);
        final labels = {
          for (final p in consoleNavigation) navigationLabel(l, p),
        };
        expect(labels, hasLength(consoleNavigation.length));
      }
      for (final page in consoleNavigation) {
        expect(page.isAdministration, isTrue);
        expect(ConsolePage.fromPath(page.path), page);
      }
    },
  );

  test('a path outside the table resolves to no page', () {
    for (final path in [
      '/threads/1',
      '/t/1',
      '/channels',
      '/messages/2',
      '/',
    ]) {
      expect(ConsolePage.fromPath(path), isNull, reason: path);
    }
  });
}
