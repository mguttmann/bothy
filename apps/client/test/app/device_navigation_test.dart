// SPDX-License-Identifier: Apache-2.0

// Navigation stays on its device (01 6.7, R-PR-24; 04 14, R-SY-29): two devices of Alex against
// one fake server. Device A is the desktop app with its router; device B opens a thread the way
// the app does (focus, thread read, read position) while Sam writes there, then as the phone app
// next to A. Data reaches A (the read state), but A keeps its route, its selected row and shows
// no history of the thread B opened.

import 'dart:io';

import 'package:client/app/app.dart';
import 'package:client/app/providers/session.dart';
import 'package:client/sidebar/providers.dart';
import 'package:client/thread/providers.dart';
import 'package:client/thread/thread_actions.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart' hide ConnectionState;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import '../support/fake_instance.dart';
import '../support/widgets.dart';

/// The patience of the repository's test kits (`testkit.Patience`).
const Duration _patience = Duration(minutes: 3);

/// Lets real IO run and advances the fake clock of the test zone a little each round, until
/// [condition] holds; bounded by the patience.
Future<void> _until(
  WidgetTester tester,
  String what,
  bool Function() condition,
) async {
  final watch = Stopwatch()..start();
  while (!condition()) {
    if (watch.elapsed >= _patience) {
      fail('$what did not happen within $_patience');
    }
    await tester.runAsync(
      () => Future<void>.delayed(const Duration(milliseconds: 5)),
    );
    await tester.pump(const Duration(milliseconds: 5));
  }
}

/// Lets real IO and the app's timers run for about [rounds] rounds without a condition.
Future<void> _settle(WidgetTester tester, {int rounds = 100}) async {
  for (var i = 0; i < rounds; i++) {
    await tester.runAsync(
      () => Future<void>.delayed(const Duration(milliseconds: 5)),
    );
    await tester.pump(const Duration(milliseconds: 5));
  }
}

void main() {
  HttpOverrides? saved;
  late FakeInstance first;
  late FakeInstance second;
  late ProviderContainer a;
  late ProviderContainer b;
  var disposed = false;

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    first = await FakeInstance.start();
    second = await first.sibling(platform: 'ios');
    a = appContainer(first);
    b = appContainer(second);
    disposed = false;
  });

  tearDown(() async {
    if (!disposed) {
      a.dispose();
      b.dispose();
    }
    await second.stopSibling();
    await first.stop();
    HttpOverrides.global = saved;
  });

  Future<void> signIn(ProviderContainer c, FakeInstance f) async {
    final control = await connected(c, f);
    await control.loginWithPassword(
      WorldSeed.alexLogin,
      WorldSeed.password,
      WorldSeed.totp,
    );
    await eventually(
      'live',
      () => c.read(syncEngineProvider)?.state.phase == ConnectionPhase.live,
    );
  }

  /// Ends both devices inside the test zone, where the apps' periodic timers live.
  Future<void> end(WidgetTester tester) async {
    await tester.pumpWidget(const SizedBox());
    a.dispose();
    b.dispose();
    disposed = true;
    await tester.pump(const Duration(minutes: 5));
  }

  testWidgets(
    'device B opens a thread: device A keeps its route, data still syncs',
    (tester) async {
      desktopWindow(tester);
      final seed = first.seed;
      final thread = ThreadId.parse(seed.releaseNotes);
      await tester.runAsync(() async {
        await signIn(a, first);
        await signIn(b, second);
      });
      await tester.pumpWidget(
        UncontrolledProviderScope(container: a, child: const ClientApp()),
      );
      final sidebar = find.byKey(const ValueKey('column.sidebar'));
      await _until(
        tester,
        'the desktop shell',
        () => sidebar.evaluate().isNotEmpty,
      );
      GoRouter router() => GoRouter.of(tester.element(sidebar));

      // Device A shows the channel `support`.
      router().go('/c/${seed.support}');
      await _until(
        tester,
        'A on support',
        () => a.read(lastRouteSelectionProvider).channel?.value == seed.support,
      );
      final before = router().routerDelegate.currentConfiguration.uri;

      // Device B opens "Release notes" as its history column does: the thread is visible (the
      // gateway focus), its messages load and the newest one is marked read.
      final message = seed.post(seed.sam, seed.releaseNotes, 'Build is green');
      final seq = ThreadSeq(message['thread_seq']! as int);
      await tester.runAsync(() async {
        b.read(visibleThreadsProvider.notifier).add(thread);
        await b.read(threadReadsProvider).loadThread(thread);
        await ThreadActions(b.read(syncEngineProvider)!).markRead(thread, seq);
      });

      // The read reaches device A (data syncs) ...
      await _until(
        tester,
        'the read state of B on A',
        () =>
            (a.read(threadReadStateProvider(seed.releaseNotes))?.lastReadSeq ??
                const ThreadSeq(0)) >=
            seq,
      );
      await _settle(tester);

      // ... but device A stays where it was: same route, no thread selected, no history column
      // of the thread B opened.
      expect(router().routerDelegate.currentConfiguration.uri, before);
      expect(a.read(lastRouteSelectionProvider).thread, isNull);
      expect(a.read(lastRouteSelectionProvider).channel?.value, seed.support);
      expect(
        find.byKey(ValueKey('historyColumn.${seed.releaseNotes}.null')),
        findsNothing,
      );
      expect(first.server.violations, isEmpty);

      await end(tester);
    },
  );

  testWidgets(
    'the phone app opens a thread: the desktop app beside it stays where it was',
    (tester) async {
      tester.view
        ..physicalSize = const Size(1440 + 390, 1000)
        ..devicePixelRatio = 1;
      addTearDown(tester.view.reset);
      final seed = first.seed;
      await tester.runAsync(() async {
        await signIn(a, first);
        await signIn(b, second);
      });
      // Two processes in one test: the desktop app of device A and the phone app of device B.
      await tester.pumpWidget(
        Directionality(
          textDirection: TextDirection.ltr,
          child: Row(
            children: [
              SizedBox(
                width: 1440,
                child: UncontrolledProviderScope(
                  container: a,
                  child: const ClientApp(),
                ),
              ),
              SizedBox(
                width: 390,
                child: UncontrolledProviderScope(
                  container: b,
                  child: const ClientApp(),
                ),
              ),
            ],
          ),
        ),
      );
      final sidebar = find.byKey(const ValueKey('column.sidebar'));
      final phone = find.byKey(const ValueKey('phone.shell'));
      await _until(
        tester,
        'both shells',
        () => sidebar.evaluate().isNotEmpty && phone.evaluate().isNotEmpty,
      );
      GoRouter desktop() => GoRouter.of(tester.element(sidebar));
      GoRouter mobile() => GoRouter.of(tester.element(phone));
      String where(GoRouter r) =>
          r.routerDelegate.currentConfiguration.uri.toString();

      desktop().go('/c/${seed.support}');
      await _until(
        tester,
        'A on support',
        () => a.read(lastRouteSelectionProvider).channel?.value == seed.support,
      );
      final before = where(desktop());

      // The phone opens "Release notes" while Sam writes there.
      seed.post(seed.sam, seed.releaseNotes, 'Build is green');
      mobile().go('/t/${seed.releaseNotes}');
      await _until(
        tester,
        'the phone on the thread',
        () => where(mobile()) == '/t/${seed.releaseNotes}',
      );
      await _settle(tester, rounds: 300);

      final history = find.byKey(
        ValueKey('historyColumn.${seed.releaseNotes}.null'),
      );
      Finder on(ProviderContainer c) => find.descendant(
        of: find.byWidgetPredicate(
          (w) => w is UncontrolledProviderScope && w.container == c,
        ),
        matching: history,
      );
      // The phone shows the thread; the desktop stays where it was.
      expect(on(b), findsOneWidget);
      expect(where(desktop()), before);
      expect(a.read(lastRouteSelectionProvider).thread, isNull);
      expect(on(a), findsNothing);
      expect(first.server.violations, isEmpty);

      await end(tester);
    },
  );
}
