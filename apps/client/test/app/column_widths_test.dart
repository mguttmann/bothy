// SPDX-License-Identifier: Apache-2.0

// Column widths (brief M4-18 Design note 5; 01 5.1 and 11): written to `layout.column_widths`
// through the outbox and restored from `user_settings`; the collapse widths come from
// AppConfig.

import 'dart:convert';

import 'package:client/app/layout/column_layout.dart';
import 'package:client/app/layout/column_widths.dart';
import 'package:client/app/providers/session.dart';
import 'package:client/app/providers/settings.dart';
import 'package:client/settings/settings_providers.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../support/engine.dart';

final AppConfig config = AppConfig.defaults();

Widget host(Widget child, double width) => MaterialApp(
  localizationsDelegates: AppLocalizations.localizationsDelegates,
  supportedLocales: AppLocalizations.supportedLocales,
  home: DsThemeScope(
    dsContext: const DsContext(
      theme: DsThemeMode.light,
      density: DsDensityMode.comfortable,
      motion: DsMotionMode.reduced,
      accent: dsDefaultAccent,
    ),
    child: Align(
      alignment: Alignment.topLeft,
      child: SizedBox(width: width, height: 600, child: child),
    ),
  ),
);

ColumnLayout layout({
  ColumnWidths widths = ColumnWidths.defaults,
  ValueChanged<ColumnWidths>? onChanged,
  bool stackShowsHistory = false,
}) => ColumnLayout(
  widths: widths,
  minWidthFourColumns: config.desktopMinWidthFourColumns.toDouble(),
  minWidthThreeColumns: config.desktopMinWidthThreeColumns.toDouble(),
  onWidthsChanged: onChanged ?? (_) {},
  stackShowsHistory: stackShowsHistory,
  sidebar: const SizedBox(),
  postList: const SizedBox(),
  history: const SizedBox(),
  session: const SizedBox(),
);

/// A window wider than every layout of the tests (the default test view is 800 wide).
void wide(WidgetTester tester) {
  tester.view
    ..physicalSize = const Size(2000, 1000)
    ..devicePixelRatio = 1;
  addTearDown(tester.view.reset);
}

Finder column(String name) => find.byKey(ValueKey('column.$name'));

void main() {
  group('ColumnWidths', () {
    test(
      'defaults are the layout tokens; a stored value replaces its default',
      () {
        expect(
          ColumnWidths.defaults.of(AppColumn.sidebar),
          DsLayout.sidebarWidth,
        );
        expect(
          ColumnWidths.defaults.of(AppColumn.postList),
          DsLayout.listWidth,
        );
        expect(
          ColumnWidths.defaults.of(AppColumn.session),
          DsLayout.sessionWidth,
        );
        final stored = ColumnWidths.fromSetting({
          'sidebar': 300,
          'session': 320,
        });
        expect(stored.of(AppColumn.sidebar), 300);
        expect(stored.of(AppColumn.postList), DsLayout.listWidth);
        expect(stored.toSetting(), {'sidebar': 300, 'session': 320});
      },
    );

    test('unknown names, negative and non-numeric values are skipped; widths have a floor', () {
      final w = ColumnWidths.fromSetting({
        'sidebar': -3,
        'post_list': 'wide',
        'later_column': 400,
        'session': 10,
      });
      expect(w.toSetting(), {'session': DsLayout.columnMin.round()});
      expect(ColumnWidths.fromSetting('x'), ColumnWidths.defaults);
      expect(
        ColumnWidths.defaults
            .withWidth(AppColumn.sidebar, 1)
            .of(AppColumn.sidebar),
        DsLayout.columnMin,
      );
    });

    test(
      'the writer puts and deletes layout.column_widths through the outbox',
      () async {
        final db = openInMemoryForTests();
        final idle = idleEngine(db);
        final writer = ColumnWidthsWriter(SettingsRepository(db, idle.engine));
        await writer.save(
          ColumnWidths.defaults.withWidth(AppColumn.sidebar, 312),
        );
        // The local row changes at once, so Settings offers "Reset" (M4-25 Design note 1).
        expect(
          (await db.personalDao.watchSettings().first).map((s) => s.key),
          contains('layout.column_widths'),
        );
        await writer.reset();
        expect(await db.personalDao.watchSettings().first, isEmpty);
        final entries = await db.outboxDao.allEntries();
        expect(entries.map((e) => e.kind), [
          PersonalOps.putMySetting,
          PersonalOps.deleteMySetting,
        ]);
        expect(jsonDecode(entries.first.payload), {
          'key': 'layout.column_widths',
          'value': {'sidebar': 312},
        });
        expect(jsonDecode(entries.last.payload), {
          'key': 'layout.column_widths',
        });
        await idle.engine.dispose();
        await idle.auth.dispose();
        await db.close();
      },
    );

    test('the stored setting is restored from user_settings', () async {
      final db = openInMemoryForTests();
      final container = ProviderContainer(
        overrides: [localDatabaseProvider.overrideWithValue(db)],
      );
      final sub = container.listen(columnWidthsProvider, (_, _) {});
      await db.personalDao.upsertSettingIfNewer(
        const UserSetting(
          key: 'layout.column_widths',
          version: ResourceVersion(1),
          value: {'post_list': 410},
        ),
      );
      await Future<void>.delayed(const Duration(milliseconds: 50));
      expect(container.read(columnWidthsProvider).of(AppColumn.postList), 410);
      sub.close();
      container.dispose();
      await db.close();
    });
  });

  group('ColumnLayout', () {
    testWidgets('four columns at ui.desktop_min_width_four_columns', (
      tester,
    ) async {
      wide(tester);
      await tester.pumpWidget(
        host(layout(), config.desktopMinWidthFourColumns.toDouble()),
      );
      for (final c in ['sidebar', 'post_list', 'history', 'session']) {
        expect(column(c), findsOneWidget, reason: c);
      }
    });

    testWidgets('below it the session column collapses', (tester) async {
      wide(tester);
      await tester.pumpWidget(
        host(layout(), config.desktopMinWidthFourColumns - 1),
      );
      expect(column('session'), findsNothing);
      expect(column('post_list'), findsOneWidget);
    });

    testWidgets(
      'below ui.desktop_min_width_three_columns the post list becomes a stack',
      (tester) async {
        wide(tester);
        final width = config.desktopMinWidthThreeColumns - 1.0;
        await tester.pumpWidget(host(layout(), width));
        expect(column('post_list'), findsOneWidget);
        expect(column('history'), findsNothing);
        await tester.pumpWidget(host(layout(stackShowsHistory: true), width));
        expect(column('post_list'), findsNothing);
        expect(column('history'), findsOneWidget);
      },
    );

    testWidgets(
      'a drag changes the width at once and reports it once at the end',
      (tester) async {
        wide(tester);
        final reported = <ColumnWidths>[];
        await tester.pumpWidget(
          host(
            layout(onChanged: reported.add),
            config.desktopMinWidthFourColumns.toDouble(),
          ),
        );
        final before = tester.getSize(column('sidebar')).width;
        await tester.drag(
          find.byKey(const ValueKey('column.handle.sidebar')),
          const Offset(40, 0),
        );
        await tester.pump();
        expect(tester.getSize(column('sidebar')).width, greaterThan(before));
        expect(reported, hasLength(1));
        expect(reported.single.of(AppColumn.sidebar), greaterThan(before));
        final session = tester.getSize(column('session')).width;
        await tester.drag(
          find.byKey(const ValueKey('column.handle.session')),
          const Offset(-30, 0),
        );
        await tester.pump();
        expect(tester.getSize(column('session')).width, greaterThan(session));
      },
    );

    testWidgets('a drag never squeezes the history below its minimum', (
      tester,
    ) async {
      wide(tester);
      final reported = <ColumnWidths>[];
      final width = config.desktopMinWidthFourColumns.toDouble();
      await tester.pumpWidget(host(layout(onChanged: reported.add), width));
      await tester.drag(
        find.byKey(const ValueKey('column.handle.post_list')),
        const Offset(5000, 0),
      );
      await tester.pump();
      expect(
        tester.getSize(column('history')).width,
        greaterThanOrEqualTo(DsLayout.columnMin - 1),
      );
    });

    testWidgets(
      'stored widths wider than the window are fitted; history keeps its minimum',
      (tester) async {
        wide(tester);
        final stored = ColumnWidths.fromSetting({
          'post_list': 1164,
          'sidebar': 600,
        });
        for (final width in [
          config.desktopMinWidthFourColumns.toDouble(),
          config.desktopMinWidthThreeColumns.toDouble(),
        ]) {
          await tester.pumpWidget(host(layout(widths: stored), width));
          expect(tester.takeException(), isNull, reason: '$width');
          expect(
            tester.getSize(column('history')).width,
            greaterThanOrEqualTo(DsLayout.columnMin - 1),
            reason: '$width',
          );
        }
      },
    );
  });
}
