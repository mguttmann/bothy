// SPDX-License-Identifier: Apache-2.0

// The keyboard of a phone closes as on iOS and Android (FIX-keyboard-dismiss-focus-ring): a tap
// anywhere outside the focused text field unfocuses it and hides the keyboard, and dragging a
// list hides it too; the text stays. The composer's own buttons (attach, send, the caller's
// control) and its suggestion list belong to the field, so they work without closing it first.
// Before DsKeyboardDismiss a touch outside the field did nothing on iOS and Android (Flutter
// unfocuses on a touch outside only on the web and the desktop).

import 'package:design_system/design_system.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import 'support.dart';

const _ds = DsContext(
  theme: DsThemeMode.light,
  density: DsDensityMode.touch,
  motion: DsMotionMode.reduced,
  accent: dsDefaultAccent,
);

/// A phone screen as the app builds it: [DsKeyboardDismiss] around the navigator, a header,
/// a message list and the composer.
Widget _screen({
  required TextEditingController text,
  required List<String> sent,
  required List<String> attached,
  List<DsSuggestion> suggestions = const [],
  List<DsSuggestion>? accepted,
}) => MaterialApp(
  theme: buildDsTheme(_ds),
  localizationsDelegates: AppLocalizations.localizationsDelegates,
  supportedLocales: AppLocalizations.supportedLocales,
  locale: const Locale('en'),
  builder: (context, app) => DsThemeScope(
    dsContext: _ds,
    child: DsKeyboardDismiss(child: app!),
  ),
  home: Scaffold(
    appBar: AppBar(title: const Text('general', key: ValueKey('header'))),
    body: Column(
      children: [
        Expanded(
          child: ListView(
            key: const ValueKey('messages'),
            children: [
              for (var i = 0; i < 40; i++)
                SizedBox(height: 60, child: Text('Message $i')),
            ],
          ),
        ),
        DsComposer(
          targetName: 'general',
          maxLines: 4,
          enterSends: true,
          controller: text,
          onSend: sent.add,
          onAttach: () => attached.add('tap'),
          suggestions: suggestions,
          onAcceptSuggestion: accepted?.add,
        ),
      ],
    ),
  ),
);

Finder get _field => find.byType(EditableText);

bool _focused(WidgetTester tester) =>
    tester.widget<EditableText>(_field).focusNode.hasFocus;

/// A phone test on iOS and on Android.
void _phone(String name, Future<void> Function(WidgetTester tester) body) {
  for (final platform in const [TargetPlatform.iOS, TargetPlatform.android]) {
    testWidgets('$name on ${platform.name}', (tester) async {
      debugDefaultTargetPlatformOverride = platform;
      try {
        tester.view
          ..physicalSize = const Size(390, 844) * 3
          ..devicePixelRatio = 3;
        addTearDown(tester.view.reset);
        await body(tester);
      } finally {
        debugDefaultTargetPlatformOverride = null;
      }
    });
  }
}

void main() {
  _phone(
    'a tap on the message list unfocuses the composer and keeps the draft',
    (tester) async {
      final text = TextEditingController();
      addTearDown(text.dispose);
      await tester.pumpWidget(_screen(text: text, sent: [], attached: []));
      await tester.tap(_field);
      await tester.pump();
      await tester.enterText(_field, 'Half a thought');
      expect(_focused(tester), isTrue);
      expect(tester.testTextInput.isVisible, isTrue);

      await tester.tap(find.text('Message 3'));
      await tester.pumpAndSettle();
      expect(_focused(tester), isFalse);
      expect(tester.testTextInput.isVisible, isFalse);
      expect(text.text, 'Half a thought');
    },
  );

  _phone('a tap on the header unfocuses the composer', (tester) async {
    final text = TextEditingController();
    addTearDown(text.dispose);
    await tester.pumpWidget(_screen(text: text, sent: [], attached: []));
    await tester.tap(_field);
    await tester.pump();
    expect(_focused(tester), isTrue);

    await tester.tap(find.byKey(const ValueKey('header')));
    await tester.pumpAndSettle();
    expect(_focused(tester), isFalse);
  });

  _phone('dragging the message list hides the keyboard', (tester) async {
    final text = TextEditingController();
    addTearDown(text.dispose);
    await tester.pumpWidget(_screen(text: text, sent: [], attached: []));
    await tester.tap(_field);
    await tester.pump();
    expect(_focused(tester), isTrue);

    await tester.drag(
      find.byKey(const ValueKey('messages')),
      const Offset(0, 200),
    );
    await tester.pumpAndSettle();
    expect(_focused(tester), isFalse);
    expect(tester.testTextInput.isVisible, isFalse);
  });

  _phone('send and attach work without closing the keyboard first', (
    tester,
  ) async {
    final text = TextEditingController();
    addTearDown(text.dispose);
    final sent = <String>[];
    final attached = <String>[];
    await tester.pumpWidget(
      _screen(text: text, sent: sent, attached: attached),
    );
    final l = await en();
    await tester.tap(_field);
    await tester.pump();
    await tester.enterText(_field, 'Restart the gateway');

    await tester.tap(find.bySemanticsLabel(l.composerAttach));
    await tester.pump();
    expect(attached, ['tap']);
    expect(_focused(tester), isTrue);

    await tester.tap(find.bySemanticsLabel(l.composerSend));
    await tester.pump();
    expect(sent, ['Restart the gateway']);
    expect(_focused(tester), isTrue);
  });

  _phone('a suggestion is accepted with the keyboard open', (tester) async {
    final text = TextEditingController(text: '/');
    addTearDown(text.dispose);
    final accepted = <DsSuggestion>[];
    await tester.pumpWidget(
      _screen(
        text: text,
        sent: [],
        attached: [],
        suggestions: const [
          DsSuggestion(id: '1', command: '/deploy', description: 'Deploy'),
        ],
        accepted: accepted,
      ),
    );
    await tester.tap(_field);
    await tester.pump();
    await tester.tap(find.text('/deploy'));
    await tester.pump();
    expect(accepted.map((s) => s.id), ['1']);
    expect(_focused(tester), isTrue);
  });

  testWidgets('on the desktop a scrolled list keeps the focus (trackpad)', (
    tester,
  ) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.macOS;
    try {
      final text = TextEditingController();
      addTearDown(text.dispose);
      await tester.pumpWidget(_screen(text: text, sent: [], attached: []));
      final list = tester.widget<ListView>(
        find.byKey(const ValueKey('messages')),
      );
      final context = tester.element(find.byKey(const ValueKey('messages')));
      expect(
        list.keyboardDismissBehavior ??
            ScrollConfiguration.of(context).getKeyboardDismissBehavior(context),
        ScrollViewKeyboardDismissBehavior.manual,
      );
    } finally {
      debugDefaultTargetPlatformOverride = null;
    }
  });
}
