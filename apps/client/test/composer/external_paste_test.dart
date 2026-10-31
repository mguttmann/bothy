// SPDX-License-Identifier: Apache-2.0

// Text that a dictation tool inserts with Ctrl+V (FIX-windows-dictation;
// docs/decisions.d/FIX-windows-dictation.md). The tool sends the keys through SendInput with a
// virtual key only, so Windows delivers scan code 0, and the Flutter embedder maps Ctrl and V to
// the same unknown physical key: it sends a synthesized Ctrl up before V down. This test feeds
// the composer that sequence and shows that nothing is pasted, while the same keys with their
// real physical keys paste the clipboard. The Windows runner fills the scan code before the
// engine sees the message (windows/runner/key_scan_code.h, tested by
// windows/runner/test/key_scan_code_test.cpp), so the app gets the second sequence.

import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/column_host.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';

/// The physical key the Windows embedder gives a key message with scan code 0: scan code 0 in
/// the Windows plane.
const _noScanCode = PhysicalKeyboardKey(0x01600000000);

const _vkControl = 0x11;
const _vkV = 0x56;
const _modifierControl = 1 << 3;

/// The method channel of package pasteboard.
const _pasteboard = MethodChannel('pasteboard');

/// One key message as the Windows embedder delivers it: the key data events of the message
/// (with the synthesized ones before it), then the legacy raw message with the virtual key,
/// which releases them to the framework (KeyDataTransitMode.keyDataThenRawKeyData).
Future<void> _send(
  WidgetTester tester, {
  required bool down,
  required int vk,
  required int scanCode,
  required int modifiers,
  required List<ui.KeyData> data,
}) async {
  // The engine's route for key data: PlatformDispatcher.onKeyData.
  for (final d in data) {
    tester.binding.platformDispatcher.onKeyData!(d);
  }
  await tester.binding.defaultBinaryMessenger.handlePlatformMessage(
    SystemChannels.keyEvent.name,
    SystemChannels.keyEvent.codec.encodeMessage(<String, Object?>{
      'type': down ? 'keydown' : 'keyup',
      'keymap': 'windows',
      'keyCode': vk,
      'scanCode': scanCode,
      'characterCodePoint': 0,
      'modifiers': modifiers,
    }),
    (_) {},
  );
}

ui.KeyData _key(
  ui.KeyEventType type,
  PhysicalKeyboardKey physical,
  LogicalKeyboardKey logical, {
  bool synthesized = false,
}) => ui.KeyData(
  type: type,
  physical: physical.usbHidUsage,
  logical: logical.keyId,
  timeStamp: Duration.zero,
  character: null,
  synthesized: synthesized,
);

/// Ctrl down, V down, V up, Ctrl up with the physical keys the embedder derives from
/// [ctrlScan] and [vScan].
Future<void> _ctrlV(
  WidgetTester tester, {
  required int ctrlScan,
  required int vScan,
}) async {
  PhysicalKeyboardKey physical(int scan, PhysicalKeyboardKey known) =>
      scan == 0 ? _noScanCode : known;
  final ctrl = physical(ctrlScan, PhysicalKeyboardKey.controlLeft);
  final v = physical(vScan, PhysicalKeyboardKey.keyV);
  const down = ui.KeyEventType.down;
  const up = ui.KeyEventType.up;
  const ctrlKey = LogicalKeyboardKey.controlLeft;
  const vKey = LogicalKeyboardKey.keyV;
  await _send(
    tester,
    down: true,
    vk: _vkControl,
    scanCode: ctrlScan,
    modifiers: _modifierControl,
    data: [_key(down, ctrl, ctrlKey)],
  );
  await _send(
    tester,
    down: true,
    vk: _vkV,
    scanCode: vScan,
    modifiers: _modifierControl,
    // V down on the physical key that Ctrl still holds: the engine releases it first
    // (keyboard_key_embedder_handler.cc, "a new down event without a matching up event").
    data: [
      if (ctrl == v) _key(up, ctrl, ctrlKey, synthesized: true),
      _key(down, v, vKey),
    ],
  );
  await _send(
    tester,
    down: false,
    vk: _vkV,
    scanCode: vScan,
    modifiers: _modifierControl,
    data: [_key(up, v, vKey)],
  );
  await _send(
    tester,
    down: false,
    vk: _vkControl,
    scanCode: ctrlScan,
    modifiers: 0,
    // An up event for a physical key that is no longer held is dropped by the engine.
    data: [if (ctrl != v) _key(up, ctrl, ctrlKey)],
  );
}

void main() {
  late List<MethodCall> reads;

  setUp(() {
    reads = [];
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(SystemChannels.platform, (call) async {
          if (call.method == 'Clipboard.getData') {
            reads.add(call);
            return <String, Object?>{'text': 'Hallo'};
          }
          if (call.method == 'Clipboard.hasStrings') {
            return <String, Object?>{'value': true};
          }
          return null;
        });
    // The attachment paste beside the field (AttachPasteShortcut) asks package pasteboard for
    // copied files and images on Ctrl+V outside macOS: the clipboard holds neither.
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(_pasteboard, (call) async {
          if (call.method == 'files') return <Object?>[];
          return null;
        });
  });

  tearDown(() {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(SystemChannels.platform, null);
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(_pasteboard, null);
  });

  Future<ColumnHost> open(WidgetTester tester) async {
    final host = await ColumnHost.open(tester);
    await host.run(tester, () => seedThread(host.db));
    await pumpThread(
      tester,
      host,
      FakeThreadReads(host.db),
      platform: 'windows',
    );
    await tester.showKeyboard(composerField);
    await host.settle(tester);
    return host;
  }

  String fieldText(WidgetTester tester) =>
      tester.widget<TextField>(composerField).controller!.text;

  testWidgets(
    'Ctrl+V with the real physical keys pastes the clipboard into the composer',
    (tester) async {
      final host = await open(tester);
      await _ctrlV(tester, ctrlScan: 0x1D, vScan: 0x2F);
      await host.settle(tester);
      expect(reads, isNotEmpty);
      expect(fieldText(tester), 'Hallo');
      await host.dispose(tester);
    },
    variant: TargetPlatformVariant.only(TargetPlatform.windows),
  );

  testWidgets(
    'Ctrl+V of keys without a scan code reaches the app without Ctrl and pastes nothing',
    (tester) async {
      final host = await open(tester);
      // A tool's SendInput with virtual keys only: scan code 0 for both keys.
      await _ctrlV(tester, ctrlScan: 0, vScan: 0);
      await host.settle(tester);
      expect(reads, isEmpty);
      expect(fieldText(tester), isEmpty);
      await host.dispose(tester);
    },
    variant: TargetPlatformVariant.only(TargetPlatform.windows),
  );
}
