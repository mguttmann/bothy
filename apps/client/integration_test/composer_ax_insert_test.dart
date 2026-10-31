// SPDX-License-Identifier: Apache-2.0

// Dictation and text tools that write through the macOS Accessibility API reach the composer
// (FIX-macos-ax-text-insert; docs/decisions.d/FIX-macos-ax-text-insert.md). Such a tool sets the
// selected text (AXSelectedText) or the value (AXValue) of the focused element, which is the
// engine's text input plugin; the debug runner's channel `bothy/a11y_probe` makes the same
// calls on the window's first responder, through the setters of NSAccessibility and through
// the legacy `accessibilitySetValue:forAttribute:` that AppKit's accessibility server calls for
// other processes. The text must land in Flutter's text model, so the composer shows it with
// the cursor after it; without a focused text field nothing changes. macOS only.

import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

import 'support/agent_flow.dart';

const MethodChannel _probe = MethodChannel('bothy/a11y_probe');

/// Whether tool/run_composer_ax_insert_test.sh started the writer of another process
/// (macos/AxWriteProbe/ax_write_probe.swift), which it does only with the Accessibility permission.
const bool _crossProcess = bool.fromEnvironment('BOTHY_AX_CROSS_PROCESS');

/// The composer text the writer of another process waits for, with the cursor before `|`.
const String axCrossProcessMarker = 'axprobe|';

/// The text that writer sets as the selected text.
const String axCrossProcessText = 'Hallo';

/// The text and selection of the focused text input of the engine as AppKit holds them, or
/// null when the window's first responder is not the engine's text input.
Future<({String text, int start, int length})?> _textInput() async {
  final raw = await _probe.invokeMapMethod<String, Object?>('textInput');
  if (raw == null) return null;
  return (
    text: raw['text']! as String,
    start: raw['start']! as int,
    length: raw['length']! as int,
  );
}

/// Writes [text] as the selected text (or with [value] as the whole value) of the window's
/// first responder; [legacy] through `accessibilitySetValue:forAttribute:`. Returns
/// `inserted`, or `noTextInput` when the first responder is not the engine's text input.
Future<String?> _axInsert(
  String text, {
  bool value = false,
  bool legacy = false,
}) => _probe.invokeMethod<String>('axInsert', {
  'text': text,
  'attribute': value ? 'value' : 'selectedText',
  'legacy': legacy,
});

/// Pumps until the native [condition] holds, with the bound of [waitUntil].
Future<void> _untilNative(
  WidgetTester tester,
  String what,
  Future<bool> Function() condition,
) async {
  final end = DateTime.now().add(const Duration(seconds: 20));
  while (!await condition()) {
    if (DateTime.now().isAfter(end)) fail('timed out: $what');
    await tester.pump(const Duration(milliseconds: 50));
  }
}

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets(
    'text written through the Accessibility API reaches the composer',
    (tester) async {
      if (!Platform.isMacOS) {
        markTestSkipped('the Accessibility API is the macOS one');
        return;
      }
      final opened = await openAgentThread(tester);
      final app = opened.app;
      try {
        await tester.tap(composerText);
        await tester.pump();
        final controller = tester.widget<TextField>(composerText).controller!;

        /// Types "ab" with the cursor between both letters and waits until the engine's text
        /// input holds that state.
        Future<void> typeAb() async {
          controller.value = const TextEditingValue(
            text: 'ab',
            selection: TextSelection.collapsed(offset: 1),
          );
          await tester.pump();
          await _untilNative(
            tester,
            'the engine holds "ab" with the cursor at 1',
            () async {
              final native = await _textInput();
              return native?.text == 'ab' && native?.start == 1;
            },
          );
        }

        Future<void> expectComposer(String text, int cursor) async {
          final end = DateTime.now().add(const Duration(seconds: 20));
          while (controller.text != text && DateTime.now().isBefore(end)) {
            await tester.pump(const Duration(milliseconds: 50));
          }
          expect(controller.text, text);
          expect(controller.selection.baseOffset, cursor);
          expect(controller.selection.extentOffset, cursor);
        }

        // AXSelectedText, as dictation tools insert at the cursor.
        await typeAb();
        expect(await _axInsert('Hallo'), 'inserted');
        await expectComposer('aHallob', 6);
        // What the tool reads back as AXValue is what the composer shows.
        expect((await _textInput())?.text, 'aHallob');

        // AXValue replaces the whole text.
        expect(await _axInsert('X', value: true), 'inserted');
        await expectComposer('X', 1);

        // The same two writes through the legacy setter of the accessibility server.
        await typeAb();
        expect(await _axInsert('Hallo', legacy: true), 'inserted');
        await expectComposer('aHallob', 6);
        expect(await _axInsert('Y', value: true, legacy: true), 'inserted');
        await expectComposer('Y', 1);

        // Without a focused text field there is no text input to write to, and the composer
        // keeps its text.
        FocusManager.instance.primaryFocus?.unfocus();
        await tester.pump();
        await _untilNative(
          tester,
          'the engine text input loses focus',
          () async => await _textInput() == null,
        );
        expect(await _axInsert('Z'), 'noTextInput');
        expect(await _axInsert('Z', value: true, legacy: true), 'noTextInput');
        await tester.pump(const Duration(milliseconds: 300));
        expect(controller.text, 'Y');
      } finally {
        await app.stop();
      }
    },
  );

  testWidgets(
    'a write of another process through the Accessibility API reaches the composer',
    (tester) async {
      if (!Platform.isMacOS || !_crossProcess) {
        markTestSkipped(
          'unproven here: no writer of another process with the Accessibility permission '
          '(tool/run_composer_ax_insert_test.sh)',
        );
        return;
      }
      final opened = await openAgentThread(tester);
      final app = opened.app;
      try {
        // The writer asks the app for its focused element, which needs the app in front.
        await _probe.invokeMethod<void>('activate');
        await tester.tap(composerText);
        await tester.pump();
        final controller = tester.widget<TextField>(composerText).controller!;
        const cursor = axCrossProcessMarker.length - 1;
        controller.value = const TextEditingValue(
          text: axCrossProcessMarker,
          selection: TextSelection.collapsed(offset: cursor),
        );
        await tester.pump();
        const expected = 'axprobe$axCrossProcessText|';
        final end = DateTime.now().add(const Duration(minutes: 3));
        while (controller.text != expected) {
          if (DateTime.now().isAfter(end)) {
            fail(
              'the write of macos/AxWriteProbe/ax_write_probe.swift did not reach the composer: '
              '"${controller.text}", native ${await _textInput()}',
            );
          }
          await tester.pump(const Duration(milliseconds: 100));
        }
        expect(
          controller.selection.baseOffset,
          cursor + axCrossProcessText.length,
        );
      } finally {
        await app.stop();
      }
    },
  );
}
