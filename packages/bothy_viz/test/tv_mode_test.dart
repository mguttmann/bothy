// SPDX-License-Identifier: Apache-2.0

import 'package:bothy_viz/bothy_viz.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support/views.dart';

void main() {
  final rotation = AppConfig.defaults().tvRotationInterval;
  const views = [
    VizViewKind.liveTopology,
    VizViewKind.backend,
    VizViewKind.executive,
  ];

  Future<(ViewWorld, List<int>)> pumpTv(
    WidgetTester tester, {
    bool? showCosts,
    DsMotionMode motion = DsMotionMode.reduced,
    List<VizViewKind> kinds = views,
    bool scaffold = true,
  }) async {
    final world = ViewWorld(showCostsInTv: showCosts);
    await tester.runAsync(world.load);
    final exits = <int>[];
    tester.view.physicalSize = const Size(1600, 1200);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    await pumpWorld(
      tester,
      world,
      SizedBox(
        height: 1100,
        child: TvMode(
          views: kinds,
          options: world.options,
          live: world.live,
          queries: world.queries,
          onExit: () => exits.add(1),
        ),
      ),
      motion: motion,
      scaffold: scaffold,
    );
    return (world, exits);
  }

  testWidgets(
    'TV mode as a route of its own gives its texts the design text style (no fallback)',
    (tester) async {
      // live_screen.dart pushes TV mode as a page without a Material ancestor; the text style
      // of the app's root is then Flutter's fallback with the yellow double underline.
      await pumpTv(tester, scaffold: false);
      final texts = find.descendant(
        of: find.byType(TvMode),
        matching: find.byType(Text),
      );
      expect(texts, findsWidgets);
      for (final e in texts.evaluate()) {
        final style = DefaultTextStyle.of(e).style
            .merge((e.widget as Text).style);
        expect(
          style.decoration ?? TextDecoration.none,
          TextDecoration.none,
          reason: '"${(e.widget as Text).data}" has the fallback text style',
        );
      }
    },
  );

  Future<void> rotate(WidgetTester tester, ViewWorld world) async {
    world.clock.advance(rotation);
    await tester.pump();
    await settleWorld(tester, world);
  }

  testWidgets('rotates through the views every viz.tv_rotation_interval', (
    tester,
  ) async {
    final (world, _) = await pumpTv(tester);
    expect(find.byType(LiveTopologyView), findsOneWidget);
    expect(find.text('Live topology (1 of 3)'), findsOneWidget);
    world.clock.advance(rotation - const Duration(milliseconds: 1));
    await tester.pump();
    expect(find.byType(LiveTopologyView), findsOneWidget);
    await rotate(tester, world);
    expect(find.byType(BackendView), findsOneWidget);
    expect(find.text('Backend view (2 of 3)'), findsOneWidget);
    await rotate(tester, world);
    expect(find.byType(ExecutiveView), findsOneWidget);
    await rotate(tester, world);
    expect(find.byType(LiveTopologyView), findsOneWidget);
  });

  testWidgets(
    'the live source keeps running when TV mode rotates between live views',
    (tester) async {
      final (world, _) = await pumpTv(
        tester,
        kinds: const [VizViewKind.liveTopology, VizViewKind.backend],
      );
      final live = world.live!;
      expect(live.isRunning, isTrue);
      await rotate(tester, world);
      expect(find.byType(BackendView), findsOneWidget);
      expect(live.isRunning, isTrue, reason: 'the old view must not stop it');
      expect(
        find.text('Online'),
        findsOneWidget,
        reason: 'presence from the frame',
      );
      await rotate(tester, world);
      expect(live.isRunning, isTrue);
      expect(find.byType(RunCardMark), findsWidgets);
    },
  );

  testWidgets('the timeline in TV mode shows the whole period', (tester) async {
    final (world, _) = await pumpTv(
      tester,
      kinds: const [VizViewKind.timeline, VizViewKind.backend],
    );
    String playhead() =>
        tester.widget<Text>(find.textContaining('Playhead')).data!;
    final first = playhead();
    world.clock.advance(rotation - const Duration(seconds: 1));
    await tester.pump();
    expect(playhead(), first, reason: 'the playhead stays at the end');
    expect(first, contains('11:00'));
  });

  testWidgets('dark, comfortable, one font step larger and no controls', (
    tester,
  ) async {
    await pumpTv(tester);
    final inside = tester.element(find.byType(LiveTopologyView));
    expect(Theme.of(inside).brightness, Brightness.dark);
    expect(inside.dsDensity, DsDensity.comfortable);
    expect(
      MediaQuery.textScalerOf(inside).scale(DsType.bodySm.fontSize!),
      closeTo(DsType.body.fontSize!, 0.001),
    );
    expect(find.byKey(const ValueKey('viz-as-table')), findsNothing);
  });

  testWidgets('costs are hidden by default', (tester) async {
    await pumpTv(tester);
    expect(find.textContaining('USD'), findsNothing);
    expect(find.bySemanticsLabel(RegExp('USD')), findsNothing);
  });

  testWidgets('costs are shown with viz.tv_show_costs', (tester) async {
    await pumpTv(tester, showCosts: true);
    expect(find.textContaining('USD'), findsWidgets);
  });

  testWidgets('a key press ends it', (tester) async {
    final (_, exits) = await pumpTv(tester);
    await tester.sendKeyEvent(LogicalKeyboardKey.space);
    await tester.pump();
    expect(exits, [1]);
    await tester.sendKeyEvent(LogicalKeyboardKey.keyA);
    expect(exits, [1], reason: 'ends once');
  });

  testWidgets('a key press ends it as an overlay over a focused host button', (
    tester,
  ) async {
    // Design note 4: the host places TV mode as a route-less overlay over its screen, where
    // a host control may already hold the focus (red team of M6-12).
    final world = ViewWorld();
    await tester.runAsync(world.load);
    final exits = <int>[];
    var opened = 0;
    OverlayEntry? entry;
    await pumpWorld(
      tester,
      world,
      Builder(
        builder: (context) => TextButton(
          autofocus: true,
          onPressed: () {
            opened++;
            entry = OverlayEntry(
              builder: (_) => TvMode(
                views: const [VizViewKind.backend],
                options: world.options,
                live: world.live,
                queries: world.queries,
                onExit: () {
                  exits.add(1);
                  entry?.remove();
                },
              ),
            );
            Overlay.of(context).insert(entry!);
          },
          child: const Text('Open TV'),
        ),
      ),
    );
    await tester.tap(find.text('Open TV'));
    await settleWorld(tester, world);
    expect(find.byType(TvMode), findsOneWidget);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.pump();
    expect(exits, [1]);
    expect(
      opened,
      1,
      reason: 'Enter must not reach the host button behind TV mode',
    );
    expect(find.byType(TvMode), findsNothing);
  });

  testWidgets('a click ends it', (tester) async {
    final (_, exits) = await pumpTv(tester);
    await tester.tapAt(const Offset(800, 600));
    await tester.pump();
    expect(exits, [1]);
  });

  // Views without a running run, so no spinning run icon keeps the tree busy.
  const still = [VizViewKind.executive, VizViewKind.backend];

  for (final motion in DsMotionMode.values) {
    testWidgets('the switch follows motion.* in the ${motion.name} context', (
      tester,
    ) async {
      final (world, _) = await pumpTv(tester, motion: motion, kinds: still);
      world.clock.advance(rotation);
      await tester.pump();
      await tester.pump();
      final expected = motion == DsMotionMode.full
          ? DsMotion.full.durationMoveSlow
          : Duration.zero;
      expect(
        tester.widget<AnimatedSwitcher>(find.byType(AnimatedSwitcher)).duration,
        expected,
      );
      expect(tester.hasRunningAnimations, motion == DsMotionMode.full);
      await tester.pumpAndSettle();
      expect(find.byType(BackendView), findsOneWidget);
    });
  }

  test('the registry hides TV mode on mobile and without allowTvMode', () {
    VizHostOptions options(VizFormFactor form, {required bool allow}) =>
        VizHostOptions(
          formFactor: form,
          scope: const VizScope.instance(),
          range: fixtureRange,
          allowTvMode: allow,
        );
    bool hasTv(VizHostOptions o) =>
        vizViewsFor(o).any((s) => s.kind == VizViewKind.tvMode);
    expect(hasTv(options(VizFormFactor.desktop, allow: true)), isTrue);
    expect(hasTv(options(VizFormFactor.web, allow: true)), isTrue);
    expect(hasTv(options(VizFormFactor.desktop, allow: false)), isFalse);
    expect(hasTv(options(VizFormFactor.mobile, allow: true)), isFalse);
    expect(
      vizViewsFor(options(VizFormFactor.mobile, allow: false))
          .map((s) => s.kind),
      [
        VizViewKind.liveTopology,
        VizViewKind.backend,
        VizViewKind.delivery,
        VizViewKind.executive,
        VizViewKind.timeline,
      ],
    );
  });

  test('a direct construction without allowTvMode asserts', () {
    final world = ViewWorld(allowTvMode: false);
    expect(
      () => TvMode(
        views: views,
        options: world.options,
        live: null,
        queries: world.queries,
        onExit: () {},
      ),
      throwsAssertionError,
    );
    final mobile = ViewWorld(formFactor: VizFormFactor.mobile);
    expect(
      () => TvMode(
        views: views,
        options: mobile.options,
        live: null,
        queries: mobile.queries,
        onExit: () {},
      ),
      throwsAssertionError,
    );
    expect(
      () => buildVizView(
        VizViewKind.tvMode,
        live: null,
        queries: world.queries,
        options: world.options,
        onOpenThread: (_) {},
      ),
      throwsArgumentError,
    );
  });
}
