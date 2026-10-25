// SPDX-License-Identifier: Apache-2.0

// Goldens of the run graph and the control room cards (brief M4-39 Tests; 01 D10, D14, 8
// "Motion"): every `Run.<state>.json` fixture, light and dark, `comfortable` and `compact`,
// text scale 1 and 2, de and en; every cell renders in the context `reduced` of the golden
// matrix.

import 'package:alchemist/alchemist.dart';
import 'package:api_client/api_client.dart' as api;
import 'package:bothy_viz/bothy_viz.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import '../support/fixtures.dart';

/// The fixture states of the run graph.
const runStates = ['flat', 'subagents', 'summary', 'stalled', 'load'];

/// The time of the goldens: 4:12 after the start of every fixture run.
final DateTime graphNow = DateTime.utc(2026, 10, 24, 9, 4, 12);

final Map<String, api.Run> _runs = {};

GoldenTestGroup _matrix(String name, Widget Function() builder) =>
    dsGoldenMatrix(
      name: name,
      builder: () => _Localized(child: builder()),
      densities: const [DsDensityMode.comfortable, DsDensityMode.compact],
      textScales: const [1.0, 2.0],
      locales: AppLocalizations.supportedLocales,
    );

/// Adds the `AppLocalizations` delegate for the locale of the cell.
class _Localized extends StatelessWidget {
  const _Localized({required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) => Localizations(
    locale: Localizations.localeOf(context),
    delegates: AppLocalizations.localizationsDelegates,
    child: child,
  );
}

Widget _graph(String state, {bool asList = false}) => SizedBox(
  width: DsLayout.overlayWidth,
  height: DsLayout.overlayWidth * 0.75,
  child: RunGraphView(
    graph: RunGraphModel.fromRun(
      _runs[state]!,
      graphNow,
      agentName: 'dev-runner',
    ),
    visibleSteps: AppConfig.defaults().liveboxVisibleSteps,
    announceMinInterval: AppConfig.defaults().liveAnnounceMinInterval,
    clock: () => graphNow,
    initiallyAsList: asList,
  ),
);

Widget _cards() => SizedBox(
  width: DsLayout.listWidth * 2 + DsSpace.s8,
  child: Wrap(
    spacing: DsSpace.s8,
    runSpacing: DsSpace.s8,
    children: [
      for (final state in ['flat', 'subagents', 'stalled'])
        for (final live in [true, false])
          if (live || state != 'subagents')
            RunControlCard(
              model: RunControlCardModel.from(
                runSummaryOf(_runs[state]!),
                document: live ? _runs[state] : null,
                now: graphNow,
                agentName: 'dev-runner',
                path: const ['Engineering', '#backend', 'Gateway resume'],
                canStop: state != 'stalled',
              ),
              onOpenGraph: () {},
              onStop: () {},
              onOpen: () {},
            ),
    ],
  ),
);

Future<void> _settle(WidgetTester tester) => dsPumpIcons(tester);

/// Registers [register] with platform goldens only when [platform] is true: the full matrix
/// stays in the CI goldens, the macOS goldens keep one file (as the views, D-M6-12-7).
void _withPlatform(bool platform, void Function() register) {
  if (platform) {
    register();
    return;
  }
  AlchemistConfig.runWithConfig(
    config: AlchemistConfig.current().merge(
      const AlchemistConfig(
        platformGoldensConfig: PlatformGoldensConfig(enabled: false),
      ),
    ),
    run: register,
  );
}

void main() {
  setUpAll(() async {
    for (final state in runStates) {
      _runs[state] = await fixtures.run(state);
    }
  });

  for (final state in runStates) {
    _withPlatform(
      state == 'subagents',
      () => goldenTest(
        'run graph trace view of Run.$state.json',
        fileName: 'run_graph_$state',
        pumpBeforeTest: _settle,
        builder: () => _matrix('run graph $state', () => _graph(state)),
      ),
    );
  }

  _withPlatform(
    false,
    () => goldenTest(
      'run graph list form of Run.subagents.json',
      fileName: 'run_graph_subagents_list',
      pumpBeforeTest: _settle,
      builder: () =>
          _matrix('run graph list', () => _graph('subagents', asList: true)),
    ),
  );

  _withPlatform(
    true,
    () => goldenTest(
      'control room cards, live and paused',
      fileName: 'run_cards',
      pumpBeforeTest: _settle,
      builder: () => _matrix('run cards', _cards),
    ),
  );
}
