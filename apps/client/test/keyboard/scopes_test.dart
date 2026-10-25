// SPDX-License-Identifier: Apache-2.0

// The scopes of the shortcut layer (brief M4-24 Tests; Design note 2; 01 7; R-PR-10): a
// focused text field keeps Enter, Shift+Enter and arrow up while app shortcuts with a modifier
// still work in it; an action without a target does not consume its key; the pass-through
// region lets plain Ctrl and Alt keys through to the platform and only the platform-modifier
// shortcuts reach the app.

import 'package:client/keyboard/app_shortcuts.dart';
import 'package:client/keyboard/bindings.dart';
import 'package:client/keyboard/intents.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/keys.dart';
import '../support/widgets.dart';

const KeyStroke ctrlK = KeyStroke(LogicalKeyboardKey.keyK, control: true);
const KeyStroke cmdK = KeyStroke(LogicalKeyboardKey.keyK, meta: true);

/// [child] under the layer for [platform] with [handlers], and a listener above the layer that
/// records every key that the layer leaves to its ancestors.
Widget layer(
  ShortcutPlatform platform,
  Map<ShortcutAction, KeyboardHandler> handlers,
  List<LogicalKeyboardKey> passed,
  Widget child,
) => hostScreen(
  Focus(
    onKeyEvent: (_, event) {
      if (event is KeyDownEvent) passed.add(event.logicalKey);
      return KeyEventResult.ignored;
    },
    child: AppShortcuts(
      platform: platform,
      handlers: handlers,
      child: Scaffold(body: child),
    ),
  ),
);

void main() {
  testWidgets(
    'a focused input keeps Enter, Shift+Enter and arrow up; Ctrl+K still '
    'opens the switcher from it',
    (tester) async {
      final sent = <String>[];
      var editLast = 0;
      var switcher = 0;
      final text = TextEditingController();
      addTearDown(text.dispose);
      await tester.pumpWidget(
        layer(
          ShortcutPlatform.windows,
          {ShortcutAction.quickSwitcher: KeyboardHandler(() => switcher++)},
          [],
          DsComposer(
            targetName: 'general',
            maxLines: 6,
            enterSends: true,
            controller: text,
            onSend: sent.add,
            onEditLast: () => editLast++,
          ),
        ),
      );
      await tester.tap(find.byType(TextField));
      await tester.pump();
      await pressStroke(tester, const KeyStroke(LogicalKeyboardKey.arrowUp));
      expect(editLast, 1, reason: 'arrow up in the empty field edits');
      await tester.enterText(find.byType(TextField), 'hello');
      await pressStroke(
        tester,
        const KeyStroke(LogicalKeyboardKey.enter, shift: true),
      );
      expect(text.text, 'hello\n', reason: 'Shift+Enter makes a new line');
      expect(sent, isEmpty);
      await pressStroke(tester, const KeyStroke(LogicalKeyboardKey.enter));
      expect(sent, ['hello\n'], reason: 'Enter sends');
      await pressStroke(tester, ctrlK);
      expect(switcher, 1);
    },
  );

  testWidgets('an action without a target does not consume its key', (
    tester,
  ) async {
    final passed = <LogicalKeyboardKey>[];
    var stops = 0;
    final handlers = <ShortcutAction, KeyboardHandler>{
      ShortcutAction.stopRun: KeyboardHandler(() => stops++, enabled: false),
    };
    await tester.pumpWidget(
      layer(
        ShortcutPlatform.windows,
        handlers,
        passed,
        const Focus(autofocus: true, child: SizedBox.expand()),
      ),
    );
    const stop = KeyStroke(LogicalKeyboardKey.period, control: true);
    expect(await pressStroke(tester, stop), isFalse);
    expect(stops, 0);
    expect(passed, contains(LogicalKeyboardKey.period));
    // Not bound at all (Open hosts and terminal until M4-36): passes the same way.
    passed.clear();
    const hosts = KeyStroke(
      LogicalKeyboardKey.keyT,
      control: true,
      shift: true,
    );
    expect(await pressStroke(tester, hosts), isFalse);
    expect(passed, contains(LogicalKeyboardKey.keyT));
  });

  testWidgets('an enabled action consumes its key', (tester) async {
    final passed = <LogicalKeyboardKey>[];
    var stops = 0;
    await tester.pumpWidget(
      layer(
        ShortcutPlatform.windows,
        {ShortcutAction.stopRun: KeyboardHandler(() => stops++)},
        passed,
        const Focus(autofocus: true, child: SizedBox.expand()),
      ),
    );
    final handled = await pressStroke(
      tester,
      const KeyStroke(LogicalKeyboardKey.period, control: true),
    );
    expect(handled, isTrue);
    expect(stops, 1);
    expect(passed, isNot(contains(LogicalKeyboardKey.period)));
  });

  testWidgets('a focus-scoped handler runs only with the focus in its view', (
    tester,
  ) async {
    var inside = 0;
    final outer = FocusNode();
    final inner = FocusNode();
    addTearDown(outer.dispose);
    addTearDown(inner.dispose);
    await tester.pumpWidget(
      layer(
        ShortcutPlatform.windows,
        const {},
        [],
        Column(
          children: [
            Focus(focusNode: outer, child: const SizedBox(height: 10)),
            KeyboardActions(
              handlers: {
                ShortcutAction.stopRun: KeyboardHandler(
                  () => inside++,
                  focusScoped: true,
                ),
              },
              child: Focus(focusNode: inner, child: const SizedBox(height: 10)),
            ),
          ],
        ),
      ),
    );
    const stop = KeyStroke(LogicalKeyboardKey.period, control: true);
    outer.requestFocus();
    await tester.pump();
    expect(await pressStroke(tester, stop), isFalse);
    expect(inside, 0);
    inner.requestFocus();
    await tester.pump();
    expect(await pressStroke(tester, stop), isTrue);
    expect(inside, 1);
  });

  testWidgets(
    'when a focused field gives up the focus, the keys still reach the layer',
    (tester) async {
      var switcher = 0;
      await tester.pumpWidget(
        layer(
          ShortcutPlatform.windows,
          {ShortcutAction.quickSwitcher: KeyboardHandler(() => switcher++)},
          [],
          const TextField(autofocus: true),
        ),
      );
      await tester.pump();
      // What a desktop field does on a click outside it (TextField.onTapOutside): the focus
      // goes to the enclosing focus scope.
      FocusManager.instance.primaryFocus!.unfocus();
      await tester.pump();
      expect(await pressStroke(tester, ctrlK), isTrue);
      expect(switcher, 1);
    },
  );

  for (final platform in ShortcutPlatform.values) {
    testWidgets('the pass-through region lets plain Ctrl and Alt keys through on '
        '${platform.name}', (tester) async {
      final passed = <LogicalKeyboardKey>[];
      final calls = <ShortcutAction>[];
      KeyboardHandler record(ShortcutAction a) =>
          KeyboardHandler(() => calls.add(a));
      final terminal = FocusNode();
      addTearDown(terminal.dispose);
      await tester.pumpWidget(
        layer(
          platform,
          {
            for (final a in [
              ShortcutAction.quickSwitcher,
              ShortcutAction.search,
              ShortcutAction.nextChannel,
              ShortcutAction.nextColumn,
            ])
              a: record(a),
          },
          passed,
          KeyboardScope.passThrough(
            child: Focus(focusNode: terminal, child: const SizedBox.expand()),
          ),
        ),
      );
      terminal.requestFocus();
      await tester.pump();
      // What a shell or a coding agent uses: none reaches an app shortcut or the app
      // above the region; the platform gets them (the result is "not handled").
      for (final stroke in const [
        KeyStroke(LogicalKeyboardKey.keyC, control: true),
        KeyStroke(LogicalKeyboardKey.keyD, control: true),
        ctrlK,
        KeyStroke(LogicalKeyboardKey.keyL, control: true),
        KeyStroke(LogicalKeyboardKey.keyF, alt: true),
        KeyStroke(LogicalKeyboardKey.arrowDown, alt: true),
        KeyStroke(LogicalKeyboardKey.escape),
        KeyStroke(LogicalKeyboardKey.f6),
      ]) {
        expect(await pressStroke(tester, stroke), isFalse, reason: '$stroke');
      }
      expect(calls, isEmpty);
      expect(passed, isEmpty);
      // The platform modifier reaches the app: Cmd on macOS, Ctrl+Shift on Windows.
      final search = platform == ShortcutPlatform.mac
          ? const KeyStroke(LogicalKeyboardKey.keyF, meta: true, shift: true)
          : const KeyStroke(
              LogicalKeyboardKey.keyF,
              control: true,
              shift: true,
            );
      expect(await pressStroke(tester, search), isTrue);
      expect(calls, [ShortcutAction.search]);
      if (platform == ShortcutPlatform.mac) {
        expect(await pressStroke(tester, cmdK), isTrue);
        expect(calls.last, ShortcutAction.quickSwitcher);
      }
    });
  }
}
