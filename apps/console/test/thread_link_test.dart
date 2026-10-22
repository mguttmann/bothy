// SPDX-License-Identifier: Apache-2.0

import 'package:console/console.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

const _thread = '0192f4a0-7b1c-78f9-8074-fa94e84bab7d';

Widget _host(Widget child) => MaterialApp(
  locale: const Locale('en'),
  localizationsDelegates: AppLocalizations.localizationsDelegates,
  supportedLocales: AppLocalizations.supportedLocales,
  home: Scaffold(body: child),
);

void main() {
  testWidgets(
    'a thread shows its ID with a copy action, because discovery names no deep link scheme',
    (tester) async {
      String? copied;
      tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
        SystemChannels.platform,
        (call) async {
          if (call.method == 'Clipboard.setData') {
            copied = (call.arguments as Map)['text'] as String;
          }
          return null;
        },
      );
      await tester.pumpWidget(_host(const ThreadLink(threadId: _thread)));
      expect(
        find.text(
          'Thread $_thread: there is no chat in the browser. Open it in the app.',
        ),
        findsOneWidget,
      );
      await tester.tap(find.text('Copy thread ID'));
      await tester.pump();
      expect(copied, _thread);
      expect(find.text('Copied'), findsOneWidget);
    },
  );
}
