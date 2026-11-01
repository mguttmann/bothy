// SPDX-License-Identifier: Apache-2.0

// M10 "Live" on the phone against the fake server with the fixtures of bothy_viz (brief M6-16
// Tests; 01 5.3 M10; 25 6, 7.1; 15 7.2): Live under Me shows every view with its table view and
// no TV mode; the stream runs only in the foreground: in the background the gateway closes and
// the fake holds no subscription on an open connection, back in the foreground `resume`
// restores it and frames arrive again; a run card opens its thread.

import 'dart:convert';

import 'package:bothy_viz/bothy_viz.dart';
import 'package:client/platform/app_lifecycle.dart';
import 'package:flutter/services.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:sync_engine/sync_engine.dart';

import 'support/phone_flow.dart';

Future<Map<String, Object?>> _fixture(String name) async =>
    jsonDecode(await rootBundle.loadString('packages/bothy_viz/fixtures/$name'))
        as Map<String, Object?>;

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('M10: every view with its table, foreground only, a card opens '
      'its thread', (tester) async {
    await asPhone(() async {
      final lifecycle = FlutterAppLifecycle(initial: AppLifecycleState.resumed);
      final app = await startPhone(tester, lifecycle: lifecycle);
      final world = app.fake.server.world;
      final seed = app.fake.seed;
      final gateway = app.fake.server.gateway;
      world.stats['getStatsLive'] = await _fixture(
        'StatsLiveFrame.normal.json',
      );
      world.stats['getStatsTimeline'] = await _fixture(
        'StatsTimeline.normal.json',
      );
      world.stats['getStatsSlo'] = await _fixture('StatsSloReport.normal.json');
      world.stats['getStatsBackends'] = await _fixture(
        'StatsBackendReport.normal.json',
      );
      // The first run of the frame belongs to a thread Alex sees, and the server named its chat,
      // so its card opens it.
      final frame = await _fixture('StatsLiveFrame.normal.json');
      final runs = (frame['runs']! as List<Object?>)
          .cast<Map<String, Object?>>();
      frame['runs'] = [
        {
          ...runs.first,
          'thread_id': seed.releaseNotes,
          'channel_id': seed.general,
          'space_id': seed.space,
          'chat': {
            ...(runs.first['chat']! as Map<String, Object?>),
            'thread_id': seed.releaseNotes,
          },
        },
      ];
      bool subscribed() =>
          gateway.connections.any((c) => c.open && c.statsSubscription != null);

      Future<void> tap(Finder finder) async {
        await app.waitFor(finder);
        await tester.ensureVisible(finder.first);
        await tester.pump();
        await tester.tap(finder.first);
        await tester.pump();
      }

      try {
        await signIn(app);
        await tester.tap(find.byKey(const ValueKey('phone.tab.me')));
        await tester.pumpAndSettle();
        await tap(find.byKey(const ValueKey('phone.me.live')));
        await app.waitFor(find.byKey(const ValueKey('live.tabs')));
        expect(find.byKey(const ValueKey('live.tv')), findsNothing);
        await until(tester, 'the subscription', subscribed);
        expect(gateway.sendStatsLive(frame), 1);
        await app.waitFor(find.byType(RunCardMark));

        // Every view of M10 with its table view; a live view waits for the next frame, which
        // the fake sends only when the test says so.
        const live = {
          VizViewKind.liveTopology,
          VizViewKind.backend,
          VizViewKind.delivery,
        };
        for (final kind in [
          VizViewKind.liveTopology,
          VizViewKind.backend,
          VizViewKind.delivery,
          VizViewKind.executive,
          VizViewKind.timeline,
        ]) {
          await tap(find.byKey(ValueKey('live.tab.${kind.name}')));
          if (live.contains(kind)) {
            await until(tester, 'the subscription of ${kind.name}', subscribed);
            gateway.sendStatsLive(frame);
          }
          await tap(find.byKey(const ValueKey('viz-as-table')));
          await app.waitFor(find.byType(VizTable));
          expect(find.byType(VizTable), findsWidgets, reason: kind.name);
        }
        await tap(find.byKey(const ValueKey('live.tab.liveTopology')));
        await until(tester, 'the subscription of the topology', subscribed);

        // The background closes the gateway: no open connection holds a subscription.
        final engine = app.state.services!.engine;
        tester.binding.handleAppLifecycleStateChanged(
          AppLifecycleState.inactive,
        );
        tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.hidden);
        tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.paused);
        await untilInBackground(
          'suspended',
          () => engine.state.phase == ConnectionPhase.suspended,
        );
        await untilInBackground(
          'the subscription ended with the connection',
          () => !subscribed(),
        );
        expect(gateway.sendStatsLive(frame), 0);

        // Back in the foreground `resume` restores it and frames arrive again.
        tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.hidden);
        tester.binding.handleAppLifecycleStateChanged(
          AppLifecycleState.inactive,
        );
        tester.binding.handleAppLifecycleStateChanged(
          AppLifecycleState.resumed,
        );
        await app.waitLive();
        await until(tester, 'the subscription after resume', subscribed);
        expect(gateway.sendStatsLive(frame), 1);
        await app.waitFor(find.byType(RunCardMark));

        // A run card opens its thread.
        await tap(find.byType(RunCardMark));
        await app.waitFor(
          find.byKey(ValueKey('historyColumn.${seed.releaseNotes}.null')),
        );
        expect(app.fake.server.violations, isEmpty);
      } finally {
        await app.stop();
        await lifecycle.dispose();
      }
    });
  });
}
