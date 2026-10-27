// SPDX-License-Identifier: Apache-2.0

// Accessibility of the terminal on the phone (brief M5-13 Design note 3; 01 8 row "Terminal";
// components.md 12): screen M8 as the phone app builds it, with a shell opened over the fakes of
// the desktop terminal. The line at the cursor is the label of the screen, "Read screen"
// announces the visible screen, running output announces nothing, the keys of the extra keys bar
// say their name and the state of Ctrl and Alt in words, and the glyph cells follow both the
// system text size and the terminal's own font size.

import 'dart:async';
import 'dart:convert';
import 'dart:ui' show Tristate;

import 'package:design_system/design_system.dart' show DsDensity;
import 'package:flutter/semantics.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';

import '../phone/terminal/support.dart';
import '../terminal/support.dart';

/// The semantics node of the terminal screen, found by the start of its label.
Finder _screenNode() => find.bySemanticsLabel(
  RegExp('^${RegExp.escape(en.terminalScreenLabel(''))}'),
);

/// Opens a shell in M8 and waits until the dialog "Open terminal" has gone: the shell connects
/// while the dialog's route still closes, and the screen under it is hidden from semantics
/// until then.
Future<void> _open(WidgetTester tester, PhoneTerminalHarness h) async {
  await h.openShell(tester);
  await pumpUntil(
    tester,
    () => find.text(en.terminalOpenConfirm).evaluate().isEmpty,
  );
  await tester.pump();
}

/// Sends [text] from the host and pumps until the screen shows [until].
Future<void> _print(
  WidgetTester tester,
  PhoneTerminalHarness h,
  String text, {
  required String until,
}) async {
  await h.world.host.sendData(utf8.encode(text));
  await pumpUntil(
    tester,
    () => h.tabs.current!.screen.visibleText().contains(until),
  );
  await tester.pump();
}

/// The columns and rows of the screen: a larger glyph cell fits fewer of them in M8.
(int, int) _grid(PhoneTerminalHarness h) {
  final terminal = h.tabs.current!.screen.terminal;
  return (terminal.viewWidth, terminal.viewHeight);
}

void main() {
  testWidgets(
    'M8 on the phone: every key of the extra keys bar is at least the touch target of the density',
    (tester) async {
      final handle = tester.ensureSemantics();
      final h = await PhoneTerminalHarness.pump(tester);
      await _open(tester, h);
      final targetMin = DsDensity.touch.targetMin;
      for (final name in [
        en.terminalKeyEscName,
        en.terminalKeyTabName,
        en.terminalKeyCtrlName,
        en.terminalKeyAltName,
        en.terminalKeyUpName,
        en.terminalKeyDownName,
        en.terminalKeyLeftName,
        en.terminalKeyRightName,
        en.terminalKeyPipeName,
        en.terminalKeyTildeName,
        en.terminalKeySlashName,
        en.terminalKeyMinusName,
      ]) {
        // The size of the key's semantics node as laid out (the bar scrolls sideways, so a
        // key at its end is clipped on screen, not smaller).
        final size = tester.getSize(
          find.byWidgetPredicate(
            (w) => w is Semantics && w.properties.label == name,
          ),
        );
        expect(
          size.width >= targetMin && size.height >= targetMin,
          isTrue,
          reason: '$name is $size, the target minimum is $targetMin',
        );
      }
      await h.close(tester);
      handle.dispose();
    },
  );

  testWidgets(
    'M8 on the phone: the line at the cursor is the label of the screen, which is no live region',
    (tester) async {
      final handle = tester.ensureSemantics();
      final h = await PhoneTerminalHarness.pump(tester);
      await _open(tester, h);
      await _print(
        tester,
        h,
        'first line\r\nsecond line\r\nalex@build-host:~/src \$ ',
        until: '~/src',
      );

      final data = tester.getSemantics(_screenNode()).getSemanticsData();
      expect(
        data.label,
        en.terminalScreenLabel('alex@build-host:~/src \$'),
        reason: 'the label is the line at the cursor, not the whole screen',
      );
      expect(data.flagsCollection.isLiveRegion, isFalse);

      // A new prompt moves the cursor: the label follows it.
      await _print(tester, h, 'ls\r\nREADME.md\r\n\$ ', until: 'README.md');
      expect(
        tester.getSemantics(_screenNode()).getSemanticsData().label,
        en.terminalScreenLabel('\$'),
      );
      await h.close(tester);
      handle.dispose();
    },
  );

  testWidgets(
    'M8 on the phone: the action Read screen announces the visible screen',
    (tester) async {
      final handle = tester.ensureSemantics();
      final h = await PhoneTerminalHarness.pump(tester);
      await _open(tester, h);
      await _print(
        tester,
        h,
        'first line\r\nsecond line\r\n\$ ',
        until: 'second line',
      );
      tester.takeAnnouncements();

      final node = tester.getSemantics(_screenNode());
      final actions = node.getSemanticsData().customSemanticsActionIds!;
      expect(actions, hasLength(1));
      expect(
        CustomSemanticsAction.getAction(actions.single)!.label,
        en.terminalReadScreen,
      );
      node.owner!.performAction(
        node.id,
        SemanticsAction.customAction,
        actions.single,
      );
      await tester.pump();

      final said = tester.takeAnnouncements();
      expect(said, hasLength(1));
      expect(said.single.message, contains('first line'));
      expect(said.single.message, contains('second line'));
      await h.close(tester);
      handle.dispose();
    },
  );

  testWidgets('M8 on the phone: running output triggers no announcement', (
    tester,
  ) async {
    final handle = tester.ensureSemantics();
    final h = await PhoneTerminalHarness.pump(tester);
    await _open(tester, h);
    tester.takeAnnouncements();
    // A program that prints line after line, as a build log does.
    for (var i = 1; i <= 20; i++) {
      await _print(tester, h, 'building step $i\r\n', until: 'step $i');
    }
    expect(
      tester.takeAnnouncements(),
      isEmpty,
      reason: 'output is read on request, never aloud by itself',
    );
    expect(
      tester
          .getSemantics(_screenNode())
          .getSemanticsData()
          .flagsCollection
          .isLiveRegion,
      isFalse,
    );
    await h.close(tester);
    handle.dispose();
  });

  testWidgets(
    'M8 on the phone: the keys of the extra keys bar have labels; Ctrl and Alt say their state',
    (tester) async {
      final handle = tester.ensureSemantics();
      final h = await PhoneTerminalHarness.pump(tester);
      await _open(tester, h);

      expect(find.bySemanticsLabel(en.terminalExtraKeys), findsOneWidget);
      for (final name in [
        en.terminalKeyEscName,
        en.terminalKeyTabName,
        en.terminalKeyCtrlName,
        en.terminalKeyAltName,
        en.terminalKeyUpName,
        en.terminalKeyDownName,
        en.terminalKeyLeftName,
        en.terminalKeyRightName,
        en.terminalKeyPipeName,
        en.terminalKeyTildeName,
        en.terminalKeySlashName,
        en.terminalKeyMinusName,
      ]) {
        final key = find.bySemanticsLabel(name);
        expect(key, findsOneWidget, reason: name);
        final flags = tester
            .getSemantics(key)
            .getSemanticsData()
            .flagsCollection;
        expect(flags.isButton, isTrue, reason: name);
      }

      for (final (visible, name) in [
        (en.terminalKeyCtrl, en.terminalKeyCtrlName),
        (en.terminalKeyAlt, en.terminalKeyAltName),
      ]) {
        // One tap: one-shot, for the next key only.
        await tester.tap(find.text(visible));
        await tester.pump();
        _expectModifier(tester, en.terminalKeyOneShot(name), on: true);
        // The next key of the bar uses it up. (A second tap on the modifier is not used here:
        // within the double-tap interval of the wall clock it latches instead.)
        await tester.tap(find.text(en.terminalKeyEsc));
        await tester.pump();
        _expectModifier(tester, name, on: false);
        // A long press latches it until it is tapped again; a tap on a latched modifier turns
        // it off whatever the time.
        await tester.longPress(find.text(visible));
        await tester.pump();
        _expectModifier(tester, en.terminalKeyLatched(name), on: true);
        await tester.tap(find.text(visible));
        await tester.pump();
        _expectModifier(tester, name, on: false);
      }
      await h.close(tester);
      handle.dispose();
    },
  );

  testWidgets(
    'M8 on the phone: a screen reader presses the keys of the extra keys bar and latches Ctrl and Alt',
    (tester) async {
      final handle = tester.ensureSemantics();
      final h = await PhoneTerminalHarness.pump(tester);
      await _open(tester, h);
      final sent = h.world.host.input.length;

      // VoiceOver's double tap and TalkBack's double tap are the tap action of the node; the
      // long press of a latch is the long press action (a double tap and hold).
      await _act(tester, en.terminalKeyEscName, SemanticsAction.tap);
      await pumpUntil(tester, () => h.world.host.input.length > sent);
      await _act(tester, en.terminalKeyCtrlName, SemanticsAction.tap);
      _expectModifier(
        tester,
        en.terminalKeyOneShot(en.terminalKeyCtrlName),
        on: true,
      );
      await _act(tester, en.terminalKeyAltName, SemanticsAction.longPress);
      _expectModifier(
        tester,
        en.terminalKeyLatched(en.terminalKeyAltName),
        on: true,
      );
      await h.close(tester);
      handle.dispose();
    },
  );

  testWidgets(
    'M8 on the phone: a screen reader opens the session switcher; New session is a button of its own with a full target',
    (tester) async {
      final handle = tester.ensureSemantics();
      final h = await PhoneTerminalHarness.pump(tester);

      // Without a session M8 offers "New session": one button of its own, not the screen.
      final add = find.bySemanticsLabel(en.terminalNewSession);
      expect(add, findsOneWidget);
      final data = tester.getSemantics(add).getSemanticsData();
      expect(data.flagsCollection.isButton, isTrue);
      expect(data.hasAction(SemanticsAction.tap), isTrue);
      expect(
        tester.getSemantics(add).rect.height,
        lessThan(tester.view.physicalSize.height / 4),
        reason: 'the node of "New session" covers more than the button',
      );
      await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
      await expectLater(tester, meetsGuideline(iOSTapTargetGuideline));

      // With two sessions the switcher can be opened by a screen reader.
      await _open(tester, h);
      await _act(tester, en.terminalDetach, SemanticsAction.tap);
      await pumpUntil(
        tester,
        () => h.tabs.current!.state.phase == TerminalPhase.detached,
      );
      unawaited(h.tabs.newSession('shell'));
      await pumpUntil(
        tester,
        () => find.text(en.terminalOpenConfirm).evaluate().isNotEmpty,
      );
      await tester.tap(find.text(en.terminalOpenConfirm));
      await tester.pump();
      await pumpUntil(
        tester,
        () =>
            h.tabs.tabs.length == 2 &&
            h.tabs.current!.state.phase == TerminalPhase.connected,
      );
      await pumpUntil(
        tester,
        () => find.text(en.terminalOpenConfirm).evaluate().isEmpty,
      );
      final switcher = find.bySemanticsLabel(
        en.terminalSessionSwitcher(en.terminalTabLabel('shell', 2)),
      );
      expect(switcher, findsOneWidget);
      await _act(
        tester,
        en.terminalSessionSwitcher(en.terminalTabLabel('shell', 2)),
        SemanticsAction.tap,
      );
      await tester.pumpAndSettle();
      expect(
        find.byKey(const ValueKey('terminal.switcher.0')),
        findsOneWidget,
        reason: 'the menu of the sessions is open',
      );
      await tester.tap(find.byKey(const ValueKey('terminal.switcher.0')));
      await tester.pumpAndSettle();
      await h.close(tester);
      handle.dispose();
    },
  );

  testWidgets('M8 on the phone: the glyph cells follow the system text size', (
    tester,
  ) async {
    final h = await PhoneTerminalHarness.pump(tester);
    await _open(tester, h);
    final (columns, rows) = _grid(h);

    tester.platformDispatcher.textScaleFactorTestValue = 2;
    addTearDown(tester.platformDispatcher.clearTextScaleFactorTestValue);
    await tester.pump();
    await tester.pump();
    final (scaledColumns, scaledRows) = _grid(h);
    // Twice the text size: about half the cells fit (the bar grows too, so fewer rows).
    expect(scaledColumns, lessThan(columns * 0.6));
    expect(scaledRows, lessThan(rows * 0.6));
    await h.close(tester);
  });

  testWidgets(
    'M8 on the phone: the glyph cells follow the terminal font size setting',
    (tester) async {
      final h = await PhoneTerminalHarness.pump(tester);
      await _open(tester, h);
      final (columns, rows) = _grid(h);
      final size = h.tabs.fontSize;

      h.tabs.changeFont(size);
      await tester.pump();
      await tester.pump();
      expect(h.tabs.fontSize, greaterThan(size));
      final (largerColumns, largerRows) = _grid(h);
      expect(largerColumns, lessThan(columns));
      expect(largerRows, lessThan(rows));
      await h.close(tester);
    },
  );
}

/// Performs [action] on the one semantics node labelled [label], as a screen reader does.
Future<void> _act(
  WidgetTester tester,
  String label,
  SemanticsAction action,
) async {
  final node = tester.getSemantics(find.bySemanticsLabel(label));
  expect(
    node.getSemanticsData().hasAction(action),
    isTrue,
    reason: '"$label" offers no ${action.name} to a screen reader',
  );
  node.owner!.performAction(node.id, action);
  await tester.pump();
}

/// Expects the modifier key named [label] to be one toggle button whose state is [on].
void _expectModifier(WidgetTester tester, String label, {required bool on}) {
  final key = find.bySemanticsLabel(label);
  expect(key, findsOneWidget, reason: label);
  final flags = tester.getSemantics(key).getSemanticsData().flagsCollection;
  expect(flags.isButton, isTrue, reason: label);
  expect(
    flags.isToggled,
    on ? Tristate.isTrue : Tristate.isFalse,
    reason: label,
  );
}
