// SPDX-License-Identifier: Apache-2.0

import 'package:api_client/api_client.dart' as api;
import 'package:bothy_viz/bothy_viz.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support/views.dart';

/// A live source that only counts its starts.
final class _CountingLive implements LiveFrameSource {
  int starts = 0;

  @override
  Stream<LiveFrameEvent> start({String? spaceId}) {
    starts++;
    return const Stream.empty();
  }

  @override
  Future<void> stop() async {}
}

void main() {
  const space = VizScope.space('0192f4a0-7b1c-7000-8a00-000000000002');

  testWidgets('delivery and sync live in a space scope says "Only for the owner" and never calls a '
      'source', (tester) async {
    final world = ViewWorld(scope: space, withLive: false);
    final live = _CountingLive();
    await pumpWorld(
      tester,
      world,
      DeliveryView(
        live: live,
        queries: world.queries,
        options: world.options,
        onOpenThread: (_) {},
      ),
    );
    expect(find.text('Only for the owner'), findsOneWidget);
    expect(world.queries.calls, isEmpty);
    expect(live.starts, 0);
    expect(find.byKey(const ValueKey('viz-as-table')), findsNothing);
  });

  test('the registry marks delivery and sync live as instance only', () {
    expect(
      [
        for (final s in vizViewRegistry)
          if (s.instanceOnly) s.kind,
      ],
      [VizViewKind.delivery],
    );
  });

  testWidgets(
    'the executive page of a space manager shows "Only for the owner" '
    'in the traffic light tile',
    (tester) async {
      final world = ViewWorld(scope: space)
        ..queries.failOn['slo'] = const api.ApiProblem(
          status: 403,
          code: 'forbidden',
        );
      await pumpWorld(tester, world, world.view(VizViewKind.executive));
      expect(find.text('Only for the owner'), findsOneWidget);
      expect(find.byType(TrafficLightMark), findsNothing);
      expect(find.byType(MetricTileMark), findsWidgets);
    },
  );

  testWidgets('the other views pass the space to their sources', (
    tester,
  ) async {
    final world = ViewWorld(scope: space);
    final live = _CountingLive();
    await pumpWorld(
      tester,
      world,
      LiveTopologyView(
        live: live,
        queries: world.queries,
        options: world.options,
        onOpenThread: (_) {},
      ),
    );
    expect(live.starts, 1);
  });
}
