// SPDX-License-Identifier: Apache-2.0

// No shortcut triggers an approval (R-PR-04; brief M4-24 Tests and fixed requirement 12):
// every binding of the table is dispatched on both platforms with the focus on an approval
// button and creates no interaction; Enter and Space on the focused button create exactly
// one. The set of actions itself holds no component action.

import 'package:client/keyboard/app_shortcuts.dart';
import 'package:client/keyboard/bindings.dart';
import 'package:client/keyboard/intents.dart';
import 'package:client/sync/agent_ops.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/agent_fakes.dart';
import '../support/card_host.dart';
import '../support/keys.dart';
import '../support/structure.dart';
import '../support/thread_host.dart' show composerField;

List<MessageComponent> approval() => [
  button(1, cardId, customId: 'allow', label: 'Allow once'),
  button(2, cardId, customId: 'deny', label: 'Deny', style: 'danger'),
];

Finder buttonOf(String customId) =>
    find.byKey(ValueKey('ds-component-$customId'));

/// Puts the keyboard focus on the button [customId] (the focus node of its ink well).
void focusButton(WidgetTester tester, String customId) {
  final inside = find.descendant(
    of: find.descendant(of: buttonOf(customId), matching: find.byType(InkWell)),
    matching: find.byType(ConstrainedBox),
  );
  Focus.of(tester.element(inside.first)).requestFocus();
}

/// Closes every dialog a shortcut opened (the in-thread search), so the next key reaches the
/// shell again.
Future<void> closeDialogs(WidgetTester tester) async {
  final navigator = tester.state<NavigatorState>(find.byType(Navigator).first);
  navigator.popUntil((route) => route is! PopupRoute);
  await tester.pump();
}

void main() {
  test('no action of the table is a component action', () {
    // The closed set of the layer: navigation, views, the thread and the run, never an
    // interaction with a component (09 11.3).
    final names = ShortcutAction.values.map((a) => a.name.toLowerCase());
    for (final name in names) {
      for (final word in ['approv', 'allow', 'deny', 'component', 'interact']) {
        expect(name, isNot(contains(word)));
      }
    }
  });

  for (final platform in ShortcutPlatform.values) {
    testWidgets(
      'no binding creates an interaction on ${platform.name}; Enter and '
      'Space on the focused button create exactly one',
      (tester) async {
        final shell = <ShortcutAction>[];
        final host = await openCard(
          tester,
          text: 'Run the tests?',
          components: approval(),
          wrap: (column) => AppShortcuts(
            platform: platform,
            handlers: {
              // The shell's own actions record only; they navigate in the app.
              for (final a in [
                ShortcutAction.quickSwitcher,
                ShortcutAction.search,
                ShortcutAction.runningNow,
                ShortcutAction.mentions,
                ShortcutAction.settings,
                ShortcutAction.nextChannel,
                ShortcutAction.previousChannel,
                ShortcutAction.nextUnreadChannel,
                ShortcutAction.previousUnreadChannel,
                ShortcutAction.toggleSessionColumn,
              ])
                a: KeyboardHandler(() => shell.add(a)),
            },
            child: column,
          ),
        );
        expect(buttonOf('allow'), findsOneWidget);
        // App and history keys with the focus on the button; the composer's keys (Enter, Shift+
        // Enter, arrow up) act in the input, so they are pressed there with the card in view.
        // Enter and Space on the focused button are the one way to apply it, checked below.
        final strokes = [
          for (final b in allShortcutBindings)
            if (b.kind != ShortcutKind.pointer)
              for (final alternative in b.on(platform))
                for (final stroke in alternative) (b.kind, stroke),
        ];
        for (final (kind, stroke) in strokes) {
          await closeDialogs(tester);
          if (kind == ShortcutKind.composer) {
            await tester.tap(composerField);
          } else {
            focusButton(tester, 'allow');
          }
          await tester.pump();
          await pressStroke(tester, stroke);
          await host.settle(tester);
          expect(
            await entriesOf(tester, host, AgentOps.createInteraction),
            isEmpty,
            reason: '$stroke must not reach the approval',
          );
        }
        // The keys did reach the layer: every shell action ran.
        expect(shell.toSet(), {
          ShortcutAction.quickSwitcher,
          ShortcutAction.search,
          ShortcutAction.runningNow,
          ShortcutAction.mentions,
          ShortcutAction.settings,
          ShortcutAction.nextChannel,
          ShortcutAction.previousChannel,
          ShortcutAction.nextUnreadChannel,
          ShortcutAction.previousUnreadChannel,
          ShortcutAction.toggleSessionColumn,
        });
        await closeDialogs(tester);
        // Only Enter or Space on the focused button applies it, once (01 6.5).
        focusButton(tester, 'allow');
        await tester.pump();
        await tester.sendKeyEvent(LogicalKeyboardKey.enter);
        await host.settle(tester);
        focusButton(tester, 'allow');
        await tester.pump();
        await tester.sendKeyEvent(LogicalKeyboardKey.space);
        await host.settle(tester);
        final entries = await entriesOf(
          tester,
          host,
          AgentOps.createInteraction,
        );
        expect(entries, hasLength(1));
        expect(entries.single.payload, contains('"custom_id":"allow"'));
        await host.dispose(tester);
      },
    );
  }
}
