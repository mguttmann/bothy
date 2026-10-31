// SPDX-License-Identifier: Apache-2.0

// The paste through a text field's selection menu, as a person does it after another app (a
// dictation app) copied text: a long press on the field, then Paste (DICTATION-MOBILE). Used by
// the widget tests and by the phone integration test on the iOS simulator and an Android
// emulator.

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import 'until.dart' show testPatience;

/// The menu the long press opened.
enum PasteMenu {
  /// The menu Flutter draws: on Android and on iOS before 16; Paste is the app's own button.
  app,

  /// The native edit menu of iOS 16 and later ([SystemContextMenu]); Flutter draws no button.
  system,
}

/// Puts [text] on the clipboard and pastes it into [field] through the field's selection menu:
/// a long press, then Paste.
///
/// Where the field shows the native menu of iOS 16 and later, no Paste button exists in the
/// widget tree; that menu's Paste is the action `paste:` of the platform's text input, which
/// [systemPaste] performs (the integration test sends it to the first responder). Before the
/// long press the field must know that the clipboard holds text: Flutter builds the menu with
/// the clipboard status it has at that moment and does not add Paste to an open menu when
/// `Clipboard.hasStrings` answers later, as it does on a device under load.
///
/// Example:
/// ```dart
/// await pasteFromMenu(tester, composerField, 'From the toolbar', systemPaste: nativePaste);
/// ```
Future<PasteMenu> pasteFromMenu(
  WidgetTester tester,
  Finder field,
  String text, {
  required Future<void> Function(String text) systemPaste,
  Duration patience = testPatience,
}) async {
  final watch = Stopwatch()..start();
  Future<void> until(String what, bool Function() done) async {
    while (!done()) {
      if (watch.elapsed > patience) fail('timed out: $what');
      await tester.pump(const Duration(milliseconds: 50));
    }
  }

  await Clipboard.setData(ClipboardData(text: text));
  final editable = tester.state<EditableTextState>(
    find.descendant(of: field, matching: find.byType(EditableText)),
  );
  final status = editable.clipboardStatus;
  // Asks the platform again; the status may be from before [text] was copied.
  status.update().ignore();
  await until(
    'the field knows the clipboard holds text',
    () => status.value == ClipboardStatus.pasteable,
  );
  final system = SystemContextMenu.isSupportedByField(editable);
  await tester.longPress(field);
  await tester.pump();
  if (system) {
    await until(
      'the native edit menu',
      () => find.byType(SystemContextMenu).evaluate().isNotEmpty,
    );
    await systemPaste(text);
    editable.hideToolbar();
    await tester.pump();
    return PasteMenu.system;
  }
  final paste = find.text('Paste');
  await until('the Paste button', () => paste.evaluate().isNotEmpty);
  await tester.tap(paste.last);
  await tester.pump();
  return PasteMenu.app;
}
