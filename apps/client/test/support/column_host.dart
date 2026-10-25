// SPDX-License-Identifier: Apache-2.0

// A host for the sidebar and post list columns over an in-memory local database and an idle
// engine (its outbox only; nothing connects), with a fake clock and a router.

import 'dart:async';
import 'dart:io';

import 'package:client/app/providers/ports.dart';
import 'package:client/app/providers/session.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import 'engine.dart';
import 'fakes.dart';
import 'structure.dart';
import 'widgets.dart';

/// The database, engine and route of a hosted column.
final class ColumnHost {
  ColumnHost._(this.db, this.engine, this.root, this._dispose);

  /// The local database.
  final LocalDatabase db;

  /// The idle engine on [db].
  final SyncEngine engine;

  /// The data folder of the test ports.
  final Directory root;

  final Future<void> Function() _dispose;

  /// The paths the router went to.
  final List<String> visited = [];

  /// Opens an in-memory database (or uses [database], which the caller closes) and an idle
  /// engine on it.
  static Future<ColumnHost> open(
    WidgetTester tester, {
    LocalDatabase? database,
  }) async {
    // The database and the engine live in the real zone: their timers and streams must run
    // outside the fake time of the widget test, or closing them never completes.
    final (db, idle) = (await tester.runAsync(() async {
      final db = database ?? openInMemoryForTests(clock: () => t0);
      return (db, idleEngine(db));
    }))!;
    final root = Directory.systemTemp.createTempSync('column');
    return ColumnHost._(db, idle.engine, root, () async {
      await idle.engine.dispose();
      await idle.auth.dispose();
      if (database == null) await db.close();
    });
  }

  /// Pumps [child] at [location] with the provider [overrides]; every path the child goes to
  /// is recorded in [visited]. [platform] `ios` gives the ports of a phone build in a phone
  /// window (brief M5-02); [lifecycle] replaces the foreground of the test ports.
  Future<void> pump(
    WidgetTester tester,
    Widget child, {
    String location = '/',
    Locale locale = const Locale('en'),
    List<Override> overrides = const [],
    String platform = 'macos',
    AppLifecycle? lifecycle,
  }) async {
    if (platform == 'ios') {
      tester.view
        ..physicalSize = const Size(390, 844) * 3
        ..devicePixelRatio = 3;
      addTearDown(tester.view.reset);
    } else {
      desktopWindow(tester);
    }
    final router = GoRouter(
      initialLocation: location,
      routes: [
        GoRoute(
          path: '/:rest(.*)',
          builder: (context, state) {
            visited.add(state.uri.toString());
            return Scaffold(body: child);
          },
        ),
      ],
    );
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          appPortsProvider.overrideWithValue(
            testPorts(
              dataRoot: root,
              clock: TestClock(t0),
              platform: platform,
              lifecycle: lifecycle,
            ),
          ),
          localDatabaseProvider.overrideWithValue(db),
          syncEngineProvider.overrideWithValue(engine),
          ...overrides,
        ],
        child: hostRouter(router, locale: locale),
      ),
    );
    await settle(tester);
  }

  /// Lets the database streams deliver and the frames settle.
  Future<void> settle(WidgetTester tester) async {
    for (var i = 0; i < 20; i++) {
      await tester.runAsync(
        () => Future<void>.delayed(const Duration(milliseconds: 5)),
      );
      await tester.pump();
    }
  }

  /// Runs [action] in fake time while both clocks advance, for writes while the widgets watch
  /// the database; returns its result.
  Future<T> run<T>(WidgetTester tester, Future<T> Function() action) async {
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
    for (var i = 0; i < 400 && !done; i++) {
      await tester.pump(const Duration(milliseconds: 1));
      await tester.runAsync(
        () => Future<void>.delayed(const Duration(milliseconds: 1)),
      );
    }
    if (!done) fail('the database write did not complete');
    if (error != null) throw error!;
    return result;
  }

  /// Unmounts the widgets and closes everything.
  ///
  /// Streams the widgets listened to were opened in fake time; closing runs while the test
  /// pumps, so their pending callbacks run on both clocks until the close completes.
  Future<void> dispose(WidgetTester tester) async {
    await tester.pumpWidget(const SizedBox());
    var done = false;
    unawaited(_dispose().whenComplete(() => done = true));
    for (var i = 0; i < 400 && !done; i++) {
      await tester.pump(const Duration(milliseconds: 1));
      await tester.runAsync(
        () => Future<void>.delayed(const Duration(milliseconds: 1)),
      );
    }
    if (!done) fail('the database and the engine did not close');
    root.deleteSync(recursive: true);
  }
}
