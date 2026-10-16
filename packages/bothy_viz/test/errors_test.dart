// SPDX-License-Identifier: Apache-2.0

import 'package:api_client/api_client.dart' as api;
import 'package:bothy_viz/bothy_viz.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import 'support/views.dart';

void main() {
  const codes = ['forbidden', 'not_found', 'feature_disabled', 'rate_limited'];
  final en = lookupAppLocalizations(const Locale('en'));

  for (final code in codes) {
    testWidgets('a live error $code shows its text and stops the stream', (
      tester,
    ) async {
      final world = ViewWorld();
      await tester.runAsync(world.load);
      await pumpWorld(tester, world, world.view(VizViewKind.liveTopology));
      final live = world.live!;
      expect(live.isRunning, isTrue);
      live.simulateError(code);
      await tester.pump();
      expect(find.text(problemText(en, code)), findsOneWidget);
      expect(
        live.isRunning,
        isFalse,
        reason: 'stopped until the host restarts',
      );
      // No further frame, no stale marker after the error.
      world.clock.advance(const Duration(minutes: 5));
      await tester.pump();
      expect(find.text('No new data'), findsNothing);
    });

    for (final kind in [
      VizViewKind.backend,
      VizViewKind.delivery,
      VizViewKind.executive,
      VizViewKind.timeline,
      VizViewKind.liveTopology,
    ]) {
      testWidgets('a query error $code of ${kind.name} shows its text', (
        tester,
      ) async {
        final world = ViewWorld(withLive: false)
          ..queries.failWith = api.ApiProblem(
            status: code == 'rate_limited' ? 429 : 403,
            code: code,
          );
        await pumpWorld(tester, world, world.view(kind));
        expect(find.text(problemText(en, code)), findsOneWidget);
        final calls = world.queries.calls.length;
        world.clock.advance(const Duration(minutes: 5));
        await tester.pump();
        expect(world.queries.calls.length, calls, reason: 'no retry');
      });
    }
  }

  testWidgets('a backend view stops its live stream when the report fails', (
    tester,
  ) async {
    final world = ViewWorld()
      ..queries.failWith = const api.ApiProblem(status: 403, code: 'forbidden');
    await tester.runAsync(world.load);
    await pumpWorld(tester, world, world.view(VizViewKind.backend));
    expect(world.live!.isRunning, isFalse);
  });
}
