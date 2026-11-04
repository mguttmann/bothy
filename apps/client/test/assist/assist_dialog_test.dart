// SPDX-License-Identifier: Apache-2.0

// The search assistant popup (docs/spec/14-search.md 4.10, docs/design/search-assistant.md 4):
// the first question is asked on open, the hits show with place, time and the assistant's
// reason or "Full text", the fallback and the cost notes show, a tap opens the thread at the
// matching message, a follow-up carries the earlier question with its terms and threads, the
// failures have their texts, and on the phone the popup fills the screen.

import 'dart:async';
import 'dart:io';

import 'package:client/app/providers/ports.dart';
import 'package:client/assist/assist_dialog.dart';
import 'package:client/assist/assist_port.dart';
import 'package:client/assist/providers.dart';
import 'package:client/search/search_port.dart';
import 'package:client/settings/settings_providers.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import '../support/fakes.dart';
import '../support/structure.dart';
import '../support/widgets.dart';

final DateTime now = DateTime.utc(2026, 11, 4, 12);

// language-check: off

const String takeoverQuestion = 'Übernahme von Larkspur auf Hermes';

// language-check: on

/// The search assistant with scripted answers; records every question.
class FakeAssistant implements AssistPort {
  final List<({String q, List<AssistTurn> history})> calls = [];
  final List<Completer<AssistAnswer>> answers = [];

  @override
  Future<AssistAnswer> ask(String q, List<AssistTurn> history) {
    calls.add((q: q, history: history));
    final c = Completer<AssistAnswer>();
    answers.add(c);
    return c.future;
  }
}

final MessageId takeoverMessage = MessageId.parse(uuid(0x7a1));

final AssistHit takeover = AssistHit(
  threadId: threadId(1),
  messageId: takeoverMessage,
  channelId: channelId(1),
  channelName: 'larkspur',
  threadTitle: 'Takeover of Larkspur from Discord',
  spaceName: 'Projects',
  at: now.subtract(const Duration(days: 2)),
  reason: 'The thread where the steering session moved to Larkspur.',
  segments: const [
    SearchSegment(text: 'the steering session ', match: false),
    SearchSegment(text: 'moved', match: true),
  ],
);

final AssistHit release = AssistHit(
  threadId: threadId(2),
  channelId: channelId(2),
  channelName: 'larkspur',
  threadTitle: 'Release 1.4',
  spaceName: 'Projects',
  at: now.subtract(const Duration(days: 1)),
  segments: const [SearchSegment(text: 'Release 1.4', match: true)],
);

class Host {
  Host(this.tester, {this.initialQuestion = takeoverQuestion}) {
    router = GoRouter(
      initialLocation: '/',
      routes: [
        GoRoute(
          path: '/',
          builder: (context, state) => Scaffold(
            body: Builder(
              builder: (context) => TextButton(
                key: const ValueKey('open'),
                onPressed: () => showSearchAssistant(
                  context,
                  initialQuestion: initialQuestion,
                ),
                child: const Text('open'),
              ),
            ),
          ),
        ),
        GoRoute(
          path: '/t/:id',
          builder: (context, state) => const Text('thread'),
        ),
      ],
    );
    addTearDown(router.dispose);
  }

  final WidgetTester tester;
  final String initialQuestion;
  late final GoRouter router;
  final FakeAssistant assistant = FakeAssistant();

  String get location => router.routeInformationProvider.value.uri.toString();

  Future<void> open({String platform = 'macos'}) async {
    desktopWindow(tester);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          appPortsProvider.overrideWithValue(
            testPorts(
              dataRoot: Directory.systemTemp,
              clock: TestClock(now),
              platform: platform,
            ),
          ),
          personTimeZoneProvider.overrideWithValue('UTC'),
          assistPortProvider.overrideWithValue(assistant),
        ],
        child: hostRouter(router),
      ),
    );
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('open')));
    await tester.pump();
    await tester.pump();
  }

  Future<void> answer(int i, AssistAnswer a) async {
    assistant.answers[i].complete(a);
    await tester.pump();
    await tester.pump();
  }

  Future<void> fail(int i, AssistFailureKind kind) async {
    assistant.answers[i].completeError(AssistFailure(kind));
    await tester.pump();
    await tester.pump();
  }
}

void main() {
  testWidgets('the first question runs on open; picks with reasons, full-text hits, the cost; '
      'a tap opens the message', (tester) async {
    final h = Host(tester);
    await h.open();
    expect(find.byKey(const ValueKey('assist.dialog')), findsOneWidget);
    expect(h.assistant.calls.single.q, takeoverQuestion);
    expect(h.assistant.calls.single.history, isEmpty);
    expect(find.byKey(const ValueKey('assist.question.0')), findsOneWidget);
    expect(find.byKey(const ValueKey('assist.pending')), findsOneWidget);

    await h.answer(
      0,
      AssistAnswer(
        hits: [takeover, release],
        terms: const ['larkspur', 'hermes', 'migration'],
        costMicros: 1200,
        currency: 'USD',
      ),
    );
    expect(find.byKey(const ValueKey('assist.pending')), findsNothing);
    expect(
      find.textContaining('Takeover of Larkspur from Discord'),
      findsOneWidget,
    );
    expect(find.textContaining(takeover.reason!), findsOneWidget);
    expect(find.text(en.assistFulltext), findsOneWidget);
    expect(find.byKey(const ValueKey('assist.fallback.0')), findsNothing);
    expect(find.text(en.assistCost('0.0012 USD')), findsOneWidget);

    await tester.tap(find.byKey(ValueKey('assist.hit.0.${threadId(1).value}')));
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('assist.dialog')), findsNothing);
    expect(h.location, '/t/${threadId(1).value}?m=${takeoverMessage.value}');
  });

  testWidgets('a follow-up carries the earlier question; a full-text answer names its reason; '
      'a hit without a message opens the thread', (tester) async {
    final h = Host(tester);
    await h.open();
    await h.answer(
      0,
      AssistAnswer(hits: [release], terms: const ['larkspur', 'hermes']),
    );
    await tester.enterText(
      find.byKey(const ValueKey('assist.field')),
      'no, the older one',
    );
    await tester.tap(find.byKey(const ValueKey('assist.send')));
    await tester.pump();
    final follow = h.assistant.calls.last;
    expect(follow.q, 'no, the older one');
    expect(follow.history.single.q, takeoverQuestion);
    expect(follow.history.single.terms, ['larkspur', 'hermes']);
    expect(follow.history.single.threadIds, [threadId(2)]);

    await h.answer(
      1,
      AssistAnswer(
        hits: [release],
        terms: const ['older'],
        fallback: AssistFallback.timeout,
      ),
    );
    expect(find.byKey(const ValueKey('assist.fallback.1')), findsOneWidget);
    expect(find.text(en.assistFallbackTimeout), findsOneWidget);

    await tester.tap(find.byKey(ValueKey('assist.hit.1.${threadId(2).value}')));
    await tester.pumpAndSettle();
    expect(h.location, '/t/${threadId(2).value}');
  });

  testWidgets('failures and an empty answer have their texts', (tester) async {
    final h = Host(tester);
    await h.open();
    await h.fail(0, AssistFailureKind.queryEmpty);
    expect(find.byKey(const ValueKey('assist.failure.0')), findsOneWidget);
    expect(find.text(en.assistQueryEmpty), findsOneWidget);

    await tester.enterText(find.byKey(const ValueKey('assist.field')), 'x y');
    await tester.testTextInput.receiveAction(TextInputAction.search);
    await tester.pump();
    await h.fail(1, AssistFailureKind.offline);
    expect(find.text(en.assistOffline), findsOneWidget);

    await tester.enterText(find.byKey(const ValueKey('assist.field')), 'z');
    await tester.testTextInput.receiveAction(TextInputAction.search);
    await tester.pump();
    await h.answer(
      2,
      const AssistAnswer(
        hits: [],
        terms: ['z'],
        fallback: AssistFallback.notConfigured,
      ),
    );
    expect(find.byKey(const ValueKey('assist.nothing.2')), findsOneWidget);
    expect(find.text(en.assistFallbackNotConfigured), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('assist.close')));
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('assist.dialog')), findsNothing);
    expect(h.location, '/');
  });

  testWidgets('without a first question the popup waits for one', (
    tester,
  ) async {
    final h = Host(tester, initialQuestion: '  ');
    await h.open();
    expect(h.assistant.calls, isEmpty);
    expect(find.byKey(const ValueKey('assist.intro')), findsOneWidget);
  });

  testWidgets('on the phone the popup fills the screen', (tester) async {
    final h = Host(tester);
    await h.open(platform: 'ios');
    final dialog = tester.widget<Dialog>(
      find.byKey(const ValueKey('assist.dialog')),
    );
    expect(dialog.insetPadding, EdgeInsets.zero);
    await h.answer(0, AssistAnswer(hits: [takeover], terms: const []));
    await tester.tap(find.byKey(ValueKey('assist.hit.0.${threadId(1).value}')));
    await tester.pumpAndSettle();
    expect(h.location, '/t/${threadId(1).value}?m=${takeoverMessage.value}');
  });
}
