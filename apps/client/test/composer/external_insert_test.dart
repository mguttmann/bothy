// SPDX-License-Identifier: Apache-2.0

// Text that an input source outside the app puts into the composer (DICTATION-MOBILE; 01 3.3):
// a dictation app, a keyboard extension, the system dictation or voice typing, and a paste of
// the system. The platform's text input hands the framework a new editing value, often with a
// composing range while the dictation runs; no key event comes with it. Such an insert does
// what typing does: the field shows it at once, send is enabled and the draft is saved; while
// a dictation grows in small steps it stays text as typing does, and an insert over
// `ui.composer_paste_file_chars` at once becomes `paste.txt` like a long paste (M4-47). The
// paste keys of a hardware keyboard (Cmd+V on iOS, Ctrl+V on Android) paste the text of the
// clipboard. The same paths on the iOS simulator and an Android emulator are driven from the
// native side in integration_test/phone/dictation_test.dart.

import 'package:client/files/upload_executor.dart';
import 'package:client/thread/history_column.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../support/column_host.dart';
import '../support/file_fakes.dart';
import '../support/file_widgets.dart';
import '../support/paste_menu.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';
import '../support/widgets.dart';

void main() {
  Future<List<OutboxEntry>> entries(
    WidgetTester tester,
    ColumnHost host,
    String kind,
  ) => host.run(tester, () async {
    final all = await host.db.outboxDao.allEntries();
    return [
      for (final e in all)
        if (e.kind == kind) e,
    ];
  });

  Future<ColumnHost> open(WidgetTester tester) async {
    final host = await ColumnHost.open(tester);
    final services = FileTestServices(host.db, host.root, fakeFilePorts());
    await host.run(
      tester,
      () => seedThread(host.db, maxBytes: 262144, messages: [msg(1)]),
    );
    await host.pump(
      tester,
      HistoryColumn(threadId: threadId(1)),
      location: '/t/${threadId(1).value}',
      overrides: [
        ...threadOverrides(FakeThreadReads(host.db)),
        ...services.overrides,
      ],
    );
    await tester.showKeyboard(composerField);
    await host.settle(tester);
    return host;
  }

  TextEditingValue fieldValue(WidgetTester tester) =>
      tester.widget<TextField>(composerField).controller!.value;

  bool sendEnabled(WidgetTester tester) =>
      tester
          .widget<DsTappable>(
            find.byWidgetPredicate(
              (w) => w is DsTappable && w.label == en.composerSend,
            ),
          )
          .onTap !=
      null;

  /// The platform's new editing value: [text] with the cursor at its end, [composing] the
  /// range an input method still composes.
  void platformValue(
    WidgetTester tester,
    String text, {
    TextRange composing = TextRange.empty,
  }) => tester.testTextInput.updateEditingValue(
    TextEditingValue(
      text: text,
      selection: TextSelection.collapsed(offset: text.length),
      composing: composing,
    ),
  );

  Future<String?> draftText(WidgetTester tester, ColumnHost host) async {
    final drafts = await entries(tester, host, PersonalOps.putThreadDraft);
    if (drafts.isEmpty) return null;
    return decodePayload(drafts.last.payload)['content_md'] as String?;
  }

  testWidgets('an insert of the platform shows, enables send and is saved', (
    tester,
  ) async {
    final host = await open(tester);
    expect(sendEnabled(tester), isFalse);
    platformValue(tester, 'Please restart the gateway.');
    // No key and no further event: one frame shows it.
    await tester.pump();
    expect(fieldValue(tester).text, 'Please restart the gateway.');
    expect(find.text('Please restart the gateway.'), findsOneWidget);
    expect(sendEnabled(tester), isTrue);
    await host.settle(tester);
    expect(await draftText(tester, host), 'Please restart the gateway.');
    await host.dispose(tester);
  });

  testWidgets('a dictation in marked or composing text ends as the text', (
    tester,
  ) async {
    final host = await open(tester);
    platformValue(tester, 'Note: ');
    await tester.pump();
    for (final partial in ['deploy', 'deploy the', 'deploy the fix']) {
      platformValue(
        tester,
        'Note: $partial',
        composing: TextRange(start: 6, end: 6 + partial.length),
      );
      await tester.pump();
      expect(fieldValue(tester).text, 'Note: $partial');
      expect(sendEnabled(tester), isTrue);
    }
    // `unmarkText` on iOS, `finishComposingText` on Android: the same text, no composition.
    platformValue(tester, 'Note: deploy the fix');
    await tester.pump();
    await host.settle(tester);
    expect(fieldValue(tester).text, 'Note: deploy the fix');
    expect(fieldValue(tester).composing, TextRange.empty);
    expect(await draftText(tester, host), 'Note: deploy the fix');
    expect(await entries(tester, host, UploadOps.upload), isEmpty);
    await host.dispose(tester);
  });

  testWidgets(
    'a dictation that grows in steps stays text past the paste limit',
    (tester) async {
      final host = await open(tester);
      var text = '';
      for (var i = 0; i < 100; i++) {
        final start = text.length;
        final sentence = 'Sentence $i of a long dictation goes on here. ';
        text = '$text$sentence';
        platformValue(
          tester,
          text,
          composing: TextRange(start: start, end: text.length),
        );
        await tester.pump();
      }
      platformValue(tester, text);
      await tester.pump();
      await host.settle(tester);
      expect(text.length, greaterThan(4000));
      expect(fieldValue(tester).text, text);
      expect(await entries(tester, host, UploadOps.upload), isEmpty);
      await host.dispose(tester);
    },
  );

  testWidgets('a long insert at once becomes paste.txt, also while composed', (
    tester,
  ) async {
    final host = await open(tester);
    platformValue(tester, 'See ');
    await tester.pump();
    final long = [for (var i = 0; i < 800; i++) 'word$i'].join(' ');
    // A keyboard extension's insertText, or an input method that sets it as composing text.
    platformValue(
      tester,
      'See $long',
      composing: TextRange(start: 4, end: 4 + long.length),
    );
    await settleUntil(
      tester,
      host,
      find.byKey(const ValueKey('composer.attachments')),
    );
    expect(fieldValue(tester).text, 'See ');
    expect(fieldValue(tester).composing, TextRange.empty);
    final up = (await entries(tester, host, UploadOps.upload)).single;
    expect(decodePayload(up.payload)['name'], 'paste.txt');
    expect(sendEnabled(tester), isTrue);
    await host.dispose(tester);
  });

  /// How late `Clipboard.hasStrings` of [mockClipboard] answers, as on a device under load.
  var hasStringsDelay = Duration.zero;

  /// The clipboard of the platform.
  void mockClipboard(WidgetTester tester) {
    String? clip;
    tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
      SystemChannels.platform,
      (call) async {
        switch (call.method) {
          case 'Clipboard.setData':
            clip = (call.arguments as Map)['text'] as String?;
            return null;
          case 'Clipboard.getData':
            return {'text': clip};
          case 'Clipboard.hasStrings':
            final has = clip?.isNotEmpty ?? false;
            if (hasStringsDelay > Duration.zero) {
              await Future<void>.delayed(hasStringsDelay);
            }
            return {'value': has};
        }
        return null;
      },
    );
  }

  void unmockClipboard(WidgetTester tester) => tester
      .binding
      .defaultBinaryMessenger
      .setMockMethodCallHandler(SystemChannels.platform, null);

  testWidgets('the Paste of the native edit menu on iOS 16 and later', (
    tester,
  ) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.iOS;
    tester.platformDispatcher.supportsShowingSystemContextMenu = true;
    mockClipboard(tester);
    try {
      final host = await open(tester);
      final pasted = <String>[];
      final menu = await pasteFromMenu(
        tester,
        composerField,
        'From the dictation app',
        patience: const Duration(seconds: 10),
        // The engine's `paste:` inserts the text of the pasteboard at the cursor.
        systemPaste: (text) async {
          pasted.add(text);
          platformValue(tester, text);
        },
      );
      expect(menu, PasteMenu.system);
      expect(pasted, ['From the dictation app']);
      expect(find.text('Paste'), findsNothing);
      await host.settle(tester);
      expect(fieldValue(tester).text, 'From the dictation app');
      expect(sendEnabled(tester), isTrue);
      expect(await draftText(tester, host), 'From the dictation app');
      await host.dispose(tester);
    } finally {
      debugDefaultTargetPlatformOverride = null;
      tester.platformDispatcher.resetSupportsShowingSystemContextMenu();
      unmockClipboard(tester);
    }
  });

  testWidgets('the app Paste button when the clipboard answers late', (
    tester,
  ) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.android;
    mockClipboard(tester);
    hasStringsDelay = const Duration(milliseconds: 300);
    try {
      final host = await open(tester);
      final menu = await pasteFromMenu(
        tester,
        composerField,
        'From the dictation app',
        patience: const Duration(seconds: 10),
        systemPaste: (_) async => fail('no native menu on Android'),
      );
      hasStringsDelay = Duration.zero;
      // The answers asked for while it was late.
      await tester.pump(const Duration(milliseconds: 300));
      expect(menu, PasteMenu.app);
      await host.settle(tester);
      expect(fieldValue(tester).text, 'From the dictation app');
      expect(sendEnabled(tester), isTrue);
      expect(await draftText(tester, host), 'From the dictation app');
      await host.dispose(tester);
    } finally {
      hasStringsDelay = Duration.zero;
      debugDefaultTargetPlatformOverride = null;
      unmockClipboard(tester);
    }
  });

  for (final (platform, command) in [
    (TargetPlatform.iOS, LogicalKeyboardKey.metaLeft),
    (TargetPlatform.android, LogicalKeyboardKey.controlLeft),
  ]) {
    testWidgets('the paste key of a hardware keyboard on ${platform.name}', (
      tester,
    ) async {
      debugDefaultTargetPlatformOverride = platform;
      tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
        SystemChannels.platform,
        (call) async => switch (call.method) {
          'Clipboard.getData' => {'text': 'From the dictation app'},
          'Clipboard.hasStrings' => {'value': true},
          _ => null,
        },
      );
      try {
        final host = await open(tester);
        await tester.sendKeyDownEvent(command);
        await tester.sendKeyEvent(LogicalKeyboardKey.keyV);
        await tester.sendKeyUpEvent(command);
        await host.settle(tester);
        expect(fieldValue(tester).text, 'From the dictation app');
        expect(sendEnabled(tester), isTrue);
        expect(await draftText(tester, host), 'From the dictation app');
        await host.dispose(tester);
      } finally {
        debugDefaultTargetPlatformOverride = null;
        tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
          SystemChannels.platform,
          null,
        );
      }
    });
  }
}
