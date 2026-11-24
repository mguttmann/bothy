// SPDX-License-Identifier: Apache-2.0

// The views follow their inputs (audit package AUD-PKG-P04-client-packages): another range or
// scope loads again, equal options load nothing, names that arrive later relabel the timeline,
// a defect in a view reaches FlutterError instead of vanishing behind the general text, and a
// sorted tree table keeps every tree row under its run.

import 'package:bothy_viz/bothy_viz.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support/fixtures.dart' show app, names;
import 'support/views.dart';

VizHostOptions _options(
  ViewWorld world, {
  VizScope scope = const VizScope.instance(),
  VizRange? range,
  VizNames names = const VizNames(),
}) => VizHostOptions(
  formFactor: VizFormFactor.desktop,
  scope: scope,
  range: range ?? fixtureRange,
  names: names,
  clock: world.clock,
);

void main() {
  testWidgets('a new range loads again, equal options load nothing', (
    tester,
  ) async {
    final world = ViewWorld(withLive: false);
    Widget view(VizHostOptions options) => ExecutiveView(
      live: null,
      queries: world.queries,
      options: options,
      onOpenThread: (_) {},
    );
    await pumpWorld(tester, world, view(_options(world)));
    final first = world.queries.calls.length;
    expect(first, greaterThan(0));

    // The host builds its options anew on every build: equal values change nothing.
    await pumpWorld(tester, world, view(_options(world)));
    expect(world.queries.calls.length, first);

    final next = VizRange(
      from: fixtureRange.from.add(const Duration(days: 1)),
      to: fixtureRange.to.add(const Duration(days: 1)),
    );
    await pumpWorld(tester, world, view(_options(world, range: next)));
    expect(world.queries.calls.length, 2 * first);
    expect(
      world.queries.rangeQueries.last.from.isAfter(fixtureRange.from),
      isTrue,
    );
  });

  testWidgets('another scope loads the run history of that space', (
    tester,
  ) async {
    final world = ViewWorld(withLive: false);
    Widget view(VizScope scope) => RunHistoryView(
      live: null,
      queries: world.queries,
      options: _options(world, scope: scope),
      onOpenThread: (_) {},
    );
    await pumpWorld(tester, world, view(const VizScope.instance()));
    final before = world.queries.calls.where((c) => c == 'runHistory').length;
    await pumpWorld(
      tester,
      world,
      view(const VizScope.space('0192f4a0-7b1c-7000-8a00-000000000002')),
    );
    expect(
      world.queries.calls.where((c) => c == 'runHistory').length,
      before + 1,
    );
  });

  testWidgets('names that arrive after the timeline relabel its bars', (
    tester,
  ) async {
    final world = ViewWorld(withLive: false);
    Widget view(VizNames n) => TimelineView(
      live: null,
      queries: world.queries,
      options: _options(world, names: n),
      onOpenThread: (_) {},
    );
    await pumpWorld(tester, world, view(const VizNames()));
    expect(renderedTexts(tester).join(' '), isNot(contains('support-agent')));
    final calls = world.queries.calls.length;
    await pumpWorld(tester, world, view(names));
    expect(renderedTexts(tester).join(' '), contains('support-agent'));
    expect(world.queries.calls.length, calls, reason: 'names never reload');
  });

  testWidgets('a defect while loading reaches FlutterError with its stack', (
    tester,
  ) async {
    final world = ViewWorld(withLive: false);
    final reported = <FlutterErrorDetails>[];
    final previous = FlutterError.onError;
    FlutterError.onError = reported.add;
    addTearDown(() => FlutterError.onError = previous);
    await pumpWorld(
      tester,
      world,
      TimelineView(
        live: null,
        queries: _BrokenQueries(),
        options: _options(world),
        onOpenThread: (_) {},
      ),
    );
    FlutterError.onError = previous;
    expect(reported.single.exception, isA<StateError>());
    expect(reported.single.library, 'bothy_viz');
    expect(find.byType(DsBanner), findsOneWidget);
  });

  testWidgets('a sorted tree table keeps the tree rows under their run', (
    tester,
  ) async {
    VizCell top(String text) => VizCell(text);
    VizCell sub(String text) => VizCell(text, level: 1);
    final data = VizTableData(
      caption: 'Runs',
      columns: const ['Name'],
      rows: [
        [top('b run')],
        [sub('b step')],
        [top('a run')],
        [sub('a step')],
      ],
    );
    await tester.pumpWidget(app(VizTable(data: data)));
    await tester.tap(find.text('Name'));
    await tester.pump();
    final order = [
      for (final t in tester.widgetList<Text>(find.byType(Text)))
        if (t.data != 'Name') t.data,
    ];
    expect(order, ['a run', 'a step', 'b run', 'b step']);
  });
}

/// The timeline throws a defect, not a problem of the server.
final class _BrokenQueries implements StatsQuerySource {
  @override
  dynamic noSuchMethod(Invocation invocation) =>
      invocation.memberName == #timeline
      ? Future<Never>.error(StateError('broken decoder'))
      : super.noSuchMethod(invocation);
}
