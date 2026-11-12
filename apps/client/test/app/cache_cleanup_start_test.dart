// SPDX-License-Identifier: Apache-2.0

// The cache cleanup of every start keeps the threads the person uses (04 13.1, 15 6.3;
// docs/decisions.d/AUD-pkg-local-store-01.md): the whole app against the fake server. Alex
// reads two threads, time passes beyond `client.cache_max_age`, Alex opens one of them again,
// quits, and starts the app the next day. The start (the real bootstrap with its cleanup)
// keeps the messages of the thread Alex opened and removes those of the thread nobody opened;
// both threads stay listed, because they are open (01 6.8).
//
// The app writes its access times with the real clock, so the test moves the stored times of
// the threads back instead of the clock: first all of them beyond the limit, and after the
// second visit all of them by one day.

import 'dart:async';
import 'dart:io';

import 'package:client/app/app.dart';
import 'package:client/app/providers/session.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart' hide ConnectionState;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import '../support/fake_instance.dart';
import '../support/widgets.dart';

/// The patience of the repository's test kits (`testkit.Patience`).
const Duration _patience = Duration(minutes: 3);

void main() {
  HttpOverrides? saved;
  late FakeInstance f;
  ProviderContainer? c;

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    f = await FakeInstance.start();
  });

  tearDown(() async {
    c?.dispose();
    await f.stop();
    HttpOverrides.global = saved;
  });

  testWidgets('the start keeps the messages of the thread the person opened '
      'yesterday and removes those of a thread nobody opened', (tester) async {
    desktopWindow(tester);
    final seed = f.seed;
    final notes = ThreadId.parse(seed.releaseNotes);
    final printer = ThreadId.parse(seed.printer);
    final maxAge = f.ports.config.cacheMaxAge;
    seed
      ..post(seed.sam, seed.releaseNotes, 'Build is green')
      ..post(seed.sam, seed.printer, 'The printer jams again');

    /// Runs [action] where the widgets live (the fake time of the test) while both clocks
    /// advance, so it shares the database with the streams the widgets watch.
    Future<T> inApp<T>(String what, Future<T> Function() action) async {
      var done = false;
      late T result;
      Object? error;
      unawaited(
        Future(() async {
          try {
            result = await action();
          } on Object catch (e) {
            error = e;
          } finally {
            done = true;
          }
        }),
      );
      final watch = Stopwatch()..start();
      while (!done) {
        if (watch.elapsed >= _patience) fail('$what did not end');
        await tester.pump(const Duration(milliseconds: 1));
        await tester.runAsync(
          () => Future<void>.delayed(const Duration(milliseconds: 1)),
        );
      }
      if (error != null) throw error!;
      return result;
    }

    /// Lets real IO run and the frames advance until [condition] holds.
    Future<void> until(String what, Future<bool> Function() condition) async {
      final watch = Stopwatch()..start();
      while (!await inApp(what, condition)) {
        if (watch.elapsed >= _patience) {
          fail('$what did not happen within $_patience');
        }
        await tester.runAsync(
          () => Future<void>.delayed(const Duration(milliseconds: 5)),
        );
        await tester.pump(const Duration(milliseconds: 5));
      }
    }

    LocalDatabase database() =>
        c!.read(onboardingControllerProvider).services!.database;
    Future<List<Message>> stored(ThreadId thread) async =>
        (await database().messagesDao.contiguous(thread, limit: 50)).messages;

    // The first run of the app: Alex signs in and reads both threads.
    final first = c = appContainer(f);
    await tester.runAsync(() async {
      final control = await connected(first, f);
      await control.loginWithPassword(
        WorldSeed.alexLogin,
        WorldSeed.password,
        WorldSeed.totp,
      );
      await eventually(
        'live',
        () =>
            first.read(syncEngineProvider)?.state.phase == ConnectionPhase.live,
      );
    });
    await tester.pumpWidget(
      UncontrolledProviderScope(container: first, child: const ClientApp()),
    );
    final sidebar = find.byKey(const ValueKey('column.sidebar'));
    await until('the desktop shell', () async => sidebar.evaluate().isNotEmpty);
    GoRouter router() => GoRouter.of(tester.element(sidebar));
    Finder history(ThreadId thread) =>
        find.byKey(ValueKey('historyColumn.${thread.value}.null'));
    Future<void> open(ThreadId thread) async {
      router().go('/t/${thread.value}');
      await until(
        'the history of ${thread.value}',
        () async =>
            history(thread).evaluate().isNotEmpty &&
            (await stored(thread)).isNotEmpty,
      );
    }

    await open(printer);
    await open(notes);
    router().go('/mentions');
    await until(
      'no thread on screen',
      () async => history(notes).evaluate().isEmpty,
    );

    // Time passes: nothing was opened for longer than client.cache_max_age.
    Future<void> age(Duration by) => inApp(
      'moving the access times back',
      () => database().customStatement(
        'UPDATE threads SET last_accessed_at = last_accessed_at - ?',
        [by.inMilliseconds],
      ),
    );
    await age(maxAge + const Duration(days: 10));

    // Alex opens "Release notes" again, and a day goes by.
    await open(notes);
    await age(const Duration(days: 1));

    // Alex quits the app.
    await tester.pumpWidget(const SizedBox());
    await inApp('closing the app', () async {
      final closing = first
          .read(onboardingControllerProvider)
          .services!
          .close();
      first.dispose();
      await closing;
    });
    await tester.pump(const Duration(minutes: 5));

    // The next start: the stored session opens the main view and the cache is cleaned.
    final second = c = appContainer(f);
    await tester.runAsync(() async {
      second.read(onboardingControllerProvider);
      await eventually(
        'the main view of the second start',
        () =>
            second.read(onboardingControllerProvider).step ==
            OnboardingStep.signedIn,
      );
      await eventually(
        'the cleanup of the start',
        () async => (await stored(printer)).isEmpty,
      );
      expect(
        await stored(notes),
        isNotEmpty,
        reason: 'opened yesterday: its messages stay in the cache',
      );
      expect(await database().threadsDao.byId(notes), isNotNull);
      expect(
        await database().threadsDao.byId(printer),
        isNotNull,
        reason: 'an open thread stays listed in the sidebar',
      );
      final closing = second
          .read(onboardingControllerProvider)
          .services!
          .close();
      second.dispose();
      c = null;
      await closing;
    });
    // The REST calls of the first run live in the fake time of the test, and the app does not
    // wait for them when it quits: the answer to a call of the thread opened last may arrive
    // as late as during the second start, which lets real IO run. The test ends in the fake
    // time, so the idle timer of that connection runs out before the test is over.
    await tester.pump(const Duration(minutes: 5));
    expect(f.server.violations, isEmpty);
  });
}
