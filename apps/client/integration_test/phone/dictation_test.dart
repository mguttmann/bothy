// SPDX-License-Identifier: Apache-2.0

// External dictation and inserts into the composer on the phone (DICTATION-MOBILE; 01 3.3)
// against the fake server: text that an input source outside the app puts into the focused
// composer arrives through the platform's text input, not through typed keys. On the iOS
// simulator that is `insertText:` (a dictation app's keyboard extension inserts its transcript
// through `textDocumentProxy.insertText`), the same in pieces after reading the text before the
// cursor, the marked text of the system dictation ended by `unmarkText`, the edit menu's
// `paste:`, the app's own Paste button and Cmd+V of a hardware keyboard; on an Android emulator
// `commitText` of an input method, the composing text of voice typing ended by a commit or by
// `finishComposingText`, the input method's paste of the clipboard, the app's Paste button and
// Ctrl+V. Each insert must show in the field at once (no further key), match the text the
// platform holds, end any composition, enable send, be saved as the draft, and go out when
// sent; an insert over `ui.composer_paste_file_chars` becomes `paste.txt` like a long paste.
//
// The native side is the debug-only probe of support/text_input_probe.dart; the Flutter tester
// has no runner, so there the tests are skipped.

import 'dart:io';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

import '../../test/support/paste_menu.dart';
import '../../test/support/widgets.dart' show en;
import 'support/phone_flow.dart';
import 'support/text_input_probe.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  /// The send button of the thread's composer.
  final send = find.byWidgetPredicate(
    (w) => w is DsTappable && w.label == en.composerSend,
  );

  bool sendEnabled() {
    final found = send.evaluate();
    return found.isNotEmpty &&
        (found.single.widget as DsTappable).onTap != null;
  }

  TextEditingController field(WidgetTester tester) =>
      tester.widget<TextField>(composerField).controller!;

  /// The text the field paints, which is what the person sees.
  String painted(WidgetTester tester) => tester
      .state<EditableTextState>(
        find.descendant(of: composerField, matching: find.byType(EditableText)),
      )
      .renderEditable
      .text!
      .toPlainText();

  /// Pumps until [condition] holds; [condition] may ask the platform.
  Future<void> untilAsync(
    WidgetTester tester,
    String what,
    Future<bool> Function() condition,
  ) async {
    final end = DateTime.now().add(const Duration(seconds: 30));
    while (!await condition()) {
      if (DateTime.now().isAfter(end)) fail('timed out: $what');
      await tester.pump(const Duration(milliseconds: 50));
    }
  }

  Future<(TestApp, String, String)> open(WidgetTester tester) async {
    final app = await startPhone(tester);
    final world = app.fake.server.world;
    final seed = app.fake.seed;
    final forum = world.addForum(seed.space, 'notes', {seed.alex.id: 'write'});
    final thread = world.addThread(forum, 'Dictation', createdBy: seed.alex.id);
    final first = seed.post(seed.alex, thread, 'Dictate here.');
    await signIn(app);
    await openThread(app, forum, thread, first['id']!);
    await untilStill(tester, composerField);
    await tester.tap(composerField);
    await tester.pump();
    // The platform's text input is attached once the field has the focus and the platform
    // answers for it.
    await untilAsync(tester, 'the text input of the composer', () async {
      try {
        await nativeText();
        return true;
      } on PlatformException {
        return false;
      }
    });
    return (app, thread, '${seed.alex.id}|$thread');
  }

  /// Checks what an insert must do, without any further key: the field holds and paints
  /// [expected], the platform holds the same text, no composition is left, send is enabled
  /// and the draft on the server is [expected].
  Future<void> expectInserted(
    WidgetTester tester,
    TestApp app,
    String draftKey,
    String expected,
  ) async {
    await until(tester, 'the field holds "$expected"', () {
      return field(tester).text == expected && painted(tester) == expected;
    });
    await untilAsync(
      tester,
      'the platform holds "$expected"',
      () async => await nativeText() == expected,
    );
    expect(
      field(tester).value.composing.isCollapsed ||
          !field(tester).value.composing.isValid,
      isTrue,
      reason: 'the composition ended',
    );
    expect(sendEnabled(), isTrue, reason: 'send is enabled');
    final drafts = app.fake.server.world.drafts;
    await until(
      tester,
      'the draft "$expected" on the server',
      () => drafts[draftKey]?['content_md'] == expected,
    );
  }

  /// Sends the composer's text and waits for the message [expected] on the server; the field
  /// and the platform's text are empty afterwards.
  Future<void> sendAndCheck(
    WidgetTester tester,
    TestApp app,
    String thread,
    String expected,
  ) async {
    await tester.tap(send);
    await tester.pump();
    final world = app.fake.server.world;
    await until(
      tester,
      'the message "$expected" on the server',
      () => world.messagesOf(thread).any((m) => m['content_md'] == expected),
    );
    await until(tester, 'the field is empty', () => field(tester).text.isEmpty);
    await untilAsync(
      tester,
      'the platform holds no text',
      () async => (await nativeText()).isEmpty,
    );
  }

  /// A transcript over `ui.composer_paste_file_chars` (4000 characters).
  final long = [for (var i = 0; i < 700; i++) 'word$i'].join(' ');

  testWidgets(
    'iOS: insertText, a keyboard extension, dictation and the edit menu',
    (tester) async {
      final (app, thread, draft) = await open(tester);
      try {
        // A dictation app's keyboard extension inserts its whole transcript at once.
        await nativeInsert('Please restart the gateway.');
        await expectInserted(tester, app, draft, 'Please restart the gateway.');
        await sendAndCheck(tester, app, thread, 'Please restart the gateway.');

        // After typed text: the extension reads the text before the cursor and inserts in
        // pieces with a space in front.
        await nativeInsert('Note:');
        await keyboardExtensionInsert(['check the', ' logs']);
        await expectInserted(tester, app, draft, 'Note: check the logs');
        await sendAndCheck(tester, app, thread, 'Note: check the logs');

        // The system dictation: marked text that grows, then unmarked.
        await nativeDictate(['Deploy', 'Deploy the', 'Deploy the fix']);
        await expectInserted(tester, app, draft, 'Deploy the fix');
        await sendAndCheck(tester, app, thread, 'Deploy the fix');

        // The edit menu's paste: of the system.
        await nativePaste('From the menu');
        await expectInserted(tester, app, draft, 'From the menu');
        await sendAndCheck(tester, app, thread, 'From the menu');

        await _appPasteAndKeys(
          tester,
          app,
          thread,
          draft,
          expectInserted,
          sendAndCheck,
          LogicalKeyboardKey.metaLeft,
        );

        await _longInsert(tester, app, thread, long, field, untilAsync);
        expect(app.fake.server.violations, isEmpty);
      } finally {
        await app.stop();
      }
    },
    skip: !Platform.isIOS,
  );

  testWidgets('Android: commitText, voice typing and the clipboard', (
    tester,
  ) async {
    final (app, thread, draft) = await open(tester);
    try {
      // An input method commits a whole transcript.
      await nativeInsert('Please restart the gateway.');
      await expectInserted(tester, app, draft, 'Please restart the gateway.');
      await sendAndCheck(tester, app, thread, 'Please restart the gateway.');

      // Voice typing: composing text that grows, then committed as the final result.
      await nativeDictate([
        'deploy',
        'deploy the',
        'deploy the fix',
      ], commit: 'Deploy the fix.');
      await expectInserted(tester, app, draft, 'Deploy the fix.');
      await sendAndCheck(tester, app, thread, 'Deploy the fix.');

      // Voice typing that ends the composition without a commit.
      await nativeDictate(['check', 'check the logs']);
      await expectInserted(tester, app, draft, 'check the logs');
      await sendAndCheck(tester, app, thread, 'check the logs');

      // The clipboard of the input method.
      await nativePaste('From the clipboard');
      await expectInserted(tester, app, draft, 'From the clipboard');
      await sendAndCheck(tester, app, thread, 'From the clipboard');

      await _appPasteAndKeys(
        tester,
        app,
        thread,
        draft,
        expectInserted,
        sendAndCheck,
        LogicalKeyboardKey.controlLeft,
      );

      await _longInsert(tester, app, thread, long, field, untilAsync);
      expect(app.fake.server.violations, isEmpty);
    } finally {
      await app.stop();
    }
  }, skip: !Platform.isAndroid);
}

/// The Paste of the field's selection menu ([pasteFromMenu]: the app's own button, or the
/// native edit menu of iOS 16 and later), then the paste key of a hardware keyboard ([command]
/// with V), each with text that a dictation app copied.
Future<void> _appPasteAndKeys(
  WidgetTester tester,
  TestApp app,
  String thread,
  String draft,
  Future<void> Function(WidgetTester, TestApp, String, String) expectInserted,
  Future<void> Function(WidgetTester, TestApp, String, String) sendAndCheck,
  LogicalKeyboardKey command,
) async {
  // On iOS 16 and later the long press opens the native edit menu, whose Paste is `paste:`.
  await pasteFromMenu(
    tester,
    composerField,
    'From the toolbar',
    systemPaste: nativePaste,
  );
  await expectInserted(tester, app, draft, 'From the toolbar');
  await sendAndCheck(tester, app, thread, 'From the toolbar');

  await Clipboard.setData(const ClipboardData(text: 'From the keyboard'));
  await tester.sendKeyDownEvent(command);
  await tester.sendKeyEvent(LogicalKeyboardKey.keyV);
  await tester.sendKeyUpEvent(command);
  await tester.pump();
  await expectInserted(tester, app, draft, 'From the keyboard');
  await sendAndCheck(tester, app, thread, 'From the keyboard');
}

/// A transcript longer than `ui.composer_paste_file_chars` inserted at once becomes
/// `paste.txt`, as a long paste does: the field and the platform keep the text around it, and
/// the message goes out with the file.
Future<void> _longInsert(
  WidgetTester tester,
  TestApp app,
  String thread,
  String long,
  TextEditingController Function(WidgetTester) field,
  Future<void> Function(WidgetTester, String, Future<bool> Function())
  untilAsync,
) async {
  expect(long.length, greaterThan(4000));
  await nativeInsert('See ');
  await nativeInsert(long);
  await app.waitFor(find.byKey(const ValueKey('composer.attachments')));
  await until(
    tester,
    'the field keeps "See "',
    () => field(tester).text == 'See ',
  );
  await untilAsync(
    tester,
    'the platform holds "See "',
    () async => await nativeText() == 'See ',
  );
  final send = find.byWidgetPredicate(
    (w) => w is DsTappable && w.label == en.composerSend,
  );
  await until(
    tester,
    'send is enabled',
    () => tester.widget<DsTappable>(send).onTap != null,
  );
  await tester.tap(send);
  await tester.pump();
  // The tap sent: the composer gave up its text and the file.
  await until(
    tester,
    'the composer is empty after send',
    () =>
        field(tester).text.isEmpty &&
        find.byKey(const ValueKey('composer.attachments')).evaluate().isEmpty,
  );
  final world = app.fake.server.world;
  await until(tester, 'the message with paste.txt', () {
    for (final m in world.messagesOf(thread)) {
      if (m['content_md'] != 'See') continue;
      final attachments = m['attachments']! as List;
      if (attachments.length != 1) return false;
      final file = ((attachments.single as Map)['file']! as Map)
          .cast<String, Object?>();
      return world.files['${file['id']}']?.json['original_name'] == 'paste.txt';
    }
    return false;
  });
}
