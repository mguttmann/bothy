// SPDX-License-Identifier: Apache-2.0

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

Widget _composer({
  TextEditingController? controller,
  bool enterSends = true,
  int maxLines = 4,
  void Function(String)? onSend,
  VoidCallback? onEditLast,
  DsComposerCommand? command,
  bool offline = false,
  bool readOnly = false,
  String? followupAgentName,
  List<String> typingNames = const [],
  List<DsSuggestion> suggestions = const [],
  ValueChanged<DsSuggestion>? onAcceptSuggestion,
  VoidCallback? onDismissSuggestions,
  bool hasAttachments = false,
}) => DsComposer(
  targetName: 'general',
  followupAgentName: followupAgentName,
  controller: controller,
  maxLines: maxLines,
  enterSends: enterSends,
  onSend: onSend ?? (_) {},
  onEditLast: onEditLast,
  command: command,
  offline: offline,
  readOnly: readOnly,
  typingNames: typingNames,
  suggestions: suggestions,
  onAcceptSuggestion: onAcceptSuggestion,
  onDismissSuggestions: onDismissSuggestions,
  hasAttachments: hasAttachments,
);

Future<void> _focus(WidgetTester tester) async {
  await tester.tap(find.byType(TextField));
  await tester.pump();
}

const _suggestions = [
  DsSuggestion(
    id: '1',
    command: '/deploy',
    description: 'Deploy a branch',
    originName: 'Robin',
  ),
  DsSuggestion(id: '2', command: '/stop', control: true),
];

void main() {
  testWidgets('Enter sends, Shift+Enter makes a new line', (tester) async {
    final sent = <String>[];
    final text = TextEditingController();
    await pumpDs(tester, _composer(controller: text, onSend: sent.add));
    await _focus(tester);
    await tester.enterText(find.byType(TextField), 'hello');
    await tester.sendKeyDownEvent(LogicalKeyboardKey.shift);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.sendKeyUpEvent(LogicalKeyboardKey.shift);
    expect(text.text, 'hello\n');
    expect(sent, isEmpty);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    expect(sent, ['hello\n']);
  });

  testWidgets(
    'an own controller is cleared after a send, so Enter does not send twice',
    (tester) async {
      final sent = <String>[];
      await pumpDs(tester, _composer(onSend: sent.add));
      await _focus(tester);
      await tester.enterText(find.byType(TextField), 'hello');
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      await tester.pump();
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      expect(sent, ['hello']);
      expect(
        tester.widget<TextField>(find.byType(TextField)).controller!.text,
        isEmpty,
      );
    },
  );

  testWidgets('a controller of the caller keeps its text after a send', (
    tester,
  ) async {
    final sent = <String>[];
    final text = TextEditingController();
    await pumpDs(tester, _composer(controller: text, onSend: sent.add));
    await _focus(tester);
    await tester.enterText(find.byType(TextField), 'hello');
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    expect(sent, ['hello']);
    expect(text.text, 'hello');
  });

  testWidgets('onChanged also reports the changes the composer makes itself', (
    tester,
  ) async {
    final changes = <String>[];
    await pumpDs(
      tester,
      DsComposer(
        targetName: 'general',
        maxLines: 4,
        enterSends: true,
        onSend: (_) {},
        onChanged: changes.add,
      ),
    );
    await _focus(tester);
    await tester.enterText(find.byType(TextField), 'hi');
    await tester.sendKeyDownEvent(LogicalKeyboardKey.shift);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.sendKeyUpEvent(LogicalKeyboardKey.shift);
    expect(changes.last, 'hi\n');
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    expect(changes.last, isEmpty);
  });

  testWidgets('with enterSends false the keys swap', (tester) async {
    final sent = <String>[];
    final text = TextEditingController();
    await pumpDs(
      tester,
      _composer(controller: text, enterSends: false, onSend: sent.add),
    );
    await _focus(tester);
    await tester.enterText(find.byType(TextField), 'hi');
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    expect(text.text, 'hi\n');
    expect(sent, isEmpty);
    await tester.sendKeyDownEvent(LogicalKeyboardKey.shift);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.sendKeyUpEvent(LogicalKeyboardKey.shift);
    expect(sent, ['hi\n']);
  });

  testWidgets('arrow up in the empty field edits the last message', (
    tester,
  ) async {
    var edits = 0;
    final text = TextEditingController();
    await pumpDs(
      tester,
      _composer(controller: text, onEditLast: () => edits++),
    );
    await _focus(tester);
    await tester.sendKeyEvent(LogicalKeyboardKey.arrowUp);
    expect(edits, 1);
    await tester.enterText(find.byType(TextField), 'x');
    await tester.sendKeyEvent(LogicalKeyboardKey.arrowUp);
    expect(edits, 1);
  });

  testWidgets('the field grows up to maxLines and then stops', (tester) async {
    final text = TextEditingController();
    await pumpDs(tester, _composer(controller: text, maxLines: 3));
    final h1 = tester.getSize(find.byType(TextField)).height;
    text.text = 'a\nb';
    await tester.pump();
    final h2 = tester.getSize(find.byType(TextField)).height;
    text.text = 'a\nb\nc';
    await tester.pump();
    final h3 = tester.getSize(find.byType(TextField)).height;
    text.text = 'a\nb\nc\nd\ne\nf\ng';
    await tester.pump();
    final h7 = tester.getSize(find.byType(TextField)).height;
    expect(h2, greaterThan(h1));
    expect(h3, greaterThan(h2));
    expect(h7, h3);
    expect(tester.widget<TextField>(find.byType(TextField)).maxLines, 3);
  });

  testWidgets('placeholder, follow-up, typing line and empty send', (
    tester,
  ) async {
    final l = await en();
    await pumpDs(tester, _composer(typingNames: ['Sam']));
    expect(
      find.text(l.composerMessageTo('\u2068general\u2069')),
      findsOneWidget,
    );
    expect(find.text(l.composerTyping('\u2068Sam\u2069')), findsOneWidget);
    final send = tester.getSemantics(find.bySemanticsLabel(l.composerSend));
    expect(
      send,
      isSemantics(isButton: true, hasEnabledState: true, isEnabled: false),
    );

    await pumpDs(
      tester,
      _composer(followupAgentName: 'Robin', typingNames: ['Sam', 'Alex']),
    );
    expect(
      find.text(l.composerFollowupTo('\u2068Robin\u2069')),
      findsOneWidget,
    );
    expect(find.text(l.composerTypingSeveral), findsOneWidget);
  });

  testWidgets('attachments allow sending without text (M4-22)', (tester) async {
    final l = await en();
    final sent = <String>[];
    await pumpDs(tester, _composer(onSend: sent.add, hasAttachments: true));
    expect(
      tester.getSemantics(find.bySemanticsLabel(l.composerSend)),
      isSemantics(isButton: true, hasEnabledState: true, isEnabled: true),
    );
    await tester.tap(find.bySemanticsLabel(l.composerSend));
    expect(sent, ['']);
  });

  testWidgets('incomplete options disable send with the reason', (
    tester,
  ) async {
    final l = await en();
    final sent = <String>[];
    final text = TextEditingController(text: '/deploy');
    await pumpDs(
      tester,
      _composer(
        controller: text,
        onSend: sent.add,
        command: const DsComposerCommand(
          name: '/deploy',
          options: [DsCommandOption(name: 'branch', required: true)],
        ),
      ),
    );
    expect(find.text(l.composerOptionMissing('branch')), findsOneWidget);
    expect(find.text('branch'), findsOneWidget);
    await _focus(tester);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    expect(sent, isEmpty);

    await pumpDs(
      tester,
      _composer(
        controller: text,
        onSend: sent.add,
        command: const DsComposerCommand(
          name: '/deploy',
          options: [
            DsCommandOption(name: 'branch', required: true, value: 'main'),
          ],
        ),
      ),
    );
    expect(find.text(l.composerOptionMissing('branch')), findsNothing);
    await tester.tap(find.bySemanticsLabel(l.composerSend));
    expect(sent, ['/deploy']);
  });

  testWidgets('offline shows the queue hint; read only replaces the field', (
    tester,
  ) async {
    final l = await en();
    await pumpDs(tester, _composer(offline: true));
    expect(find.text(l.composerOffline), findsOneWidget);
    expect(find.byType(TextField), findsOneWidget);

    await pumpDs(tester, _composer(readOnly: true));
    expect(find.text(l.composerReadOnly), findsOneWidget);
    expect(find.byType(TextField), findsNothing);
  });

  testWidgets('suggestions: count, arrows, Enter accepts, Escape closes', (
    tester,
  ) async {
    final l = await en();
    final accepted = <String>[];
    var dismissed = 0;
    final text = TextEditingController(text: '/');
    await pumpDs(
      tester,
      _composer(
        controller: text,
        suggestions: _suggestions,
        onAcceptSuggestion: (s) => accepted.add(s.command),
        onDismissSuggestions: () => dismissed++,
      ),
    );
    expect(find.bySemanticsLabel(l.composerSuggestions(2)), findsOneWidget);
    expect(find.text('/deploy'), findsOneWidget);
    expect(find.text('Deploy a branch'), findsOneWidget);
    expect(find.text(l.composerControlCommand), findsOneWidget);
    var list = tester.widget<DsSuggestionList>(find.byType(DsSuggestionList));
    expect(list.activeIndex, 0);
    await _focus(tester);
    await tester.sendKeyEvent(LogicalKeyboardKey.arrowDown);
    await tester.pump();
    list = tester.widget<DsSuggestionList>(find.byType(DsSuggestionList));
    expect(list.activeIndex, 1);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    expect(accepted, ['/stop']);
    await tester.sendKeyEvent(LogicalKeyboardKey.arrowUp);
    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    expect(accepted, ['/stop', '/deploy']);
    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    expect(dismissed, 1);
    await tester.tap(find.text('/stop'));
    expect(accepted.last, '/stop');
  });

  testWidgets('Enter during an IME composition does not send', (tester) async {
    final sent = <String>[];
    final text = TextEditingController();
    await pumpDs(tester, _composer(controller: text, onSend: sent.add));
    await _focus(tester);
    text.value = const TextEditingValue(
      text: 'nihon',
      selection: TextSelection.collapsed(offset: 5),
      composing: TextRange(start: 0, end: 5),
    );
    await tester.pump();
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    expect(sent, isEmpty);
    text.value = const TextEditingValue(
      text: 'nihon',
      selection: TextSelection.collapsed(offset: 5),
    );
    await tester.pump();
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    expect(sent, ['nihon']);
  });
}
