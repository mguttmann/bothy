// SPDX-License-Identifier: Apache-2.0

// TM-T18 as a widget test (docs/spec/24-terminal.md 5.5 and 14; 15 23.3): a pinned old key and
// a new key from getHost show the warning with both fingerprints, "Do not connect" opens
// nothing, "Accept new key" opens (or attaches) with pin_changed = true and stores the new pin
// only after the handshake, and a higher key_generation alone still warns.

import 'package:domain/domain.dart' show fingerprintDisplay;
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';
import 'package:terminal_ui/testing.dart';

import 'support/fakes.dart';
import 'support/widgets.dart';

HostKeyPin _oldPin(String key) => HostKeyPin(
  hostId: hostIdText,
  noisePublicKey: key,
  keyGeneration: 1,
  fingerprint: oldFingerprint,
  firstSeen: DateTime.utc(2026, 9),
);

Future<Harness> _setUp(WidgetTester tester, {required bool pinned}) async {
  final nav = GlobalKey<NavigatorState>();
  final pins = FakePins();
  final h = (await tester.runAsync(
    () => Harness.create(
      pins: pins,
      prompts: DialogTerminalPrompts(
        () => nav.currentContext,
        selfUserId: alexId,
      ),
    ),
  ))!;
  if (pinned) {
    final old = (await tester.runAsync(FakeHostKey.other))!;
    pins.pins[hostIdText] = _oldPin(old);
  }
  await pumpApp(tester, const SizedBox.expand(), navigatorKey: nav);
  return h;
}

Finder _text(String s) => find.text(s);

Future<void> _confirmOpen(WidgetTester tester) async {
  await pumpUntil(tester, () => _text('Open terminal').evaluate().isNotEmpty);
  await tester.tap(_text('Confirm with passkey or device key'));
  await tester.pump();
}

void main() {
  testWidgets(
    'open: the warning shows both fingerprints; Do not connect opens nothing',
    (tester) async {
      final h = await _setUp(tester, pinned: true);
      final done = h.open();
      await pumpUntil(
        tester,
        () => _text("The host's key has changed").evaluate().isNotEmpty,
      );
      expect(_text(fingerprintDisplay(oldFingerprint)), findsOneWidget);
      expect(_text(fingerprintDisplay(newFingerprint)), findsOneWidget);
      await tester.tap(_text('Do not connect'));
      await tester.pumpAndSettle();
      await done;
      expect(h.api.opens, isEmpty);
      expect(h.stepUps, isEmpty);
      expect(h.transport.connected, isEmpty);
      expect(h.pins.writes, isEmpty);
      expect(h.controller.state.phase, TerminalPhase.idle);
    },
  );

  testWidgets('open: Accept new key opens with pin_changed and pins the new key after the '
      'handshake', (tester) async {
    final h = await _setUp(tester, pinned: true);
    h.host.handshake = FakeHostHandshake.silent;
    final done = h.open();
    await pumpUntil(
      tester,
      () => _text('Accept new key').evaluate().isNotEmpty,
    );
    await tester.tap(_text('Accept new key'));
    await tester.pump();
    await _confirmOpen(tester);
    await pumpUntil(tester, () => h.host.connections == 1);
    await tester.pump(const Duration(milliseconds: 50));
    expect(h.api.opens.single.pinChanged, isTrue);
    expect(h.pins.writes, isEmpty, reason: 'no pin before message 2');

    h.host.handshake = FakeHostHandshake.normal;
    await h.controller.end();
    await done;

    final g = await _setUp(tester, pinned: true);
    final opening = g.open();
    await pumpUntil(
      tester,
      () => _text('Accept new key').evaluate().isNotEmpty,
    );
    await tester.tap(_text('Accept new key'));
    await tester.pump();
    await _confirmOpen(tester);
    await pumpUntil(
      tester,
      () => g.controller.state.phase == TerminalPhase.connected,
    );
    await opening;
    final pin = g.pins.pins[hostIdText]!;
    expect(pin.noisePublicKey, g.host.noisePublicKey);
    expect(pin.fingerprint, newFingerprint);
    expect(g.api.opens.single.pinChanged, isTrue);
  });

  testWidgets('a higher key_generation alone still warns', (tester) async {
    final h = await _setUp(tester, pinned: true);
    h.api.keyGeneration = 2;
    final done = h.open();
    await pumpUntil(
      tester,
      () => _text("The host's key has changed").evaluate().isNotEmpty,
    );
    await tester.tap(_text('Do not connect'));
    await tester.pumpAndSettle();
    await done;
    expect(h.api.opens, isEmpty);
  });

  testWidgets('the same key opens without a warning and writes no pin', (
    tester,
  ) async {
    final h = await _setUp(tester, pinned: false);
    h.pins.pins[hostIdText] = _oldPin(h.host.noisePublicKey);
    final done = h.open();
    await _confirmOpen(tester);
    await pumpUntil(
      tester,
      () => h.controller.state.phase == TerminalPhase.connected,
    );
    await done;
    expect(_text("The host's key has changed"), findsNothing);
    expect(h.pins.writes, isEmpty);
    expect(h.api.opens.single.pinChanged, isNull);
  });

  testWidgets('attach: a changed key warns; Do not connect stays detached, Accept attaches '
      'with pin_changed and pins after the handshake', (tester) async {
    final h = await _setUp(tester, pinned: false);
    final opening = h.open();
    await _confirmOpen(tester);
    await pumpUntil(
      tester,
      () => h.controller.state.phase == TerminalPhase.connected,
    );
    await opening;
    await h.controller.detach();
    final old = (await tester.runAsync(FakeHostKey.other))!;
    h.pins.pins[hostIdText] = _oldPin(old);
    h.pins.writes.clear();

    var attaching = h.controller.attach();
    await pumpUntil(
      tester,
      () => _text('Do not connect').evaluate().isNotEmpty,
    );
    expect(_text(fingerprintDisplay(oldFingerprint)), findsOneWidget);
    await tester.tap(_text('Do not connect'));
    await tester.pumpAndSettle();
    await attaching;
    expect(h.api.attaches, isEmpty);
    expect(h.controller.state.phase, TerminalPhase.detached);

    attaching = h.controller.attach();
    await pumpUntil(
      tester,
      () => _text('Accept new key').evaluate().isNotEmpty,
    );
    await tester.tap(_text('Accept new key'));
    await tester.pump();
    await pumpUntil(
      tester,
      () => h.controller.state.phase == TerminalPhase.connected,
    );
    await attaching;
    expect(h.api.attaches.single.pinChanged, isTrue);
    expect(h.pins.writes.single.noisePublicKey, h.host.noisePublicKey);
  });
}
