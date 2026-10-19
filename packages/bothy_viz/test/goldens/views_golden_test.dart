// SPDX-License-Identifier: Apache-2.0

import 'package:alchemist/alchemist.dart';
import 'package:api_client/api_client.dart' as api;
import 'package:bothy_viz/bothy_viz.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import '../support/views.dart';

/// The golden matrix of every view (VZ-T11; brief M6-12, Tests): light and dark, the three
/// densities, text scale 1 and 2, de and en, 24 cells per file.
/// [themes] narrows a file to one theme where the full matrix would pass `repo.max_file_kb`.
GoldenTestGroup matrix(
  String name,
  Widget Function() builder, {
  Iterable<DsThemeMode> themes = DsThemeMode.values,
}) => dsGoldenMatrix(
  name: name,
  builder: () => _Localized(child: builder()),
  themes: themes,
  densities: DsDensityMode.values,
  textScales: const [1.0, 2.0],
  locales: AppLocalizations.supportedLocales,
);

/// Adds the `AppLocalizations` delegate for the locale of the cell; the alchemist app carries
/// only the Material delegates.
class _Localized extends StatelessWidget {
  const _Localized({required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) => Localizations(
    locale: Localizations.localeOf(context),
    delegates: AppLocalizations.localizationsDelegates,
    child: SizedBox(width: DsLayout.overlayWidth, child: child),
  );
}

/// The live frames of every state, decoded once before the goldens.
final Map<String, api.StatsLiveFrame> _frames = {};

/// One view on its own fixture world; the first frame (and, for `offline`, the stale
/// marker) arrives after the first build, as from a server.
class _ViewFixture extends StatefulWidget {
  const _ViewFixture({
    required this.kind,
    required this.state,
    this.scope = const VizScope.instance(),
    this.sloForbidden = false,
  });

  final VizViewKind kind;
  final String state;
  final VizScope scope;
  final bool sloForbidden;

  @override
  State<_ViewFixture> createState() => _ViewFixtureState();
}

class _ViewFixtureState extends State<_ViewFixture> {
  late final ViewWorld world = ViewWorld(
    state: widget.state,
    scope: widget.scope,
  );

  @override
  void initState() {
    super.initState();
    world.useFrame(_frames[world.frameState]!);
    if (widget.sloForbidden) {
      world.queries.failOn['slo'] = const api.ApiProblem(
        status: 403,
        code: 'forbidden',
      );
    }
    WidgetsBinding.instance.addPostFrameCallback((_) {
      world.clock.advance(
        widget.state == 'offline'
            ? AppConfig.defaults().staleAfter
            : Duration.zero,
      );
    });
  }

  @override
  Widget build(BuildContext context) => widget.kind == VizViewKind.tvMode
      ? SizedBox(
          height: DsLayout.overlayWidth,
          child: TvMode(
            views: const [VizViewKind.liveTopology],
            options: world.options,
            live: world.live,
            queries: world.queries,
            onExit: () {},
          ),
        )
      : world.view(widget.kind);
}

Future<void> _settle(WidgetTester tester) async {
  for (var i = 0; i < 3; i++) {
    await tester.pump();
  }
  await dsPumpIcons(tester);
}

/// Registers [body] with platform goldens only when [platform] is true: the full matrix of
/// every fixture stays in the CI goldens, the macOS goldens keep one file per view (brief
/// M6-11, design note 8; docs/DECISIONS.md D-M6-12-7).
///
/// With [perTheme] the matrix is split into one file per theme (`<file>_light`,
/// `<file>_dark`): the minute areas of the timeline make one file of both themes larger than
/// `repo.max_file_kb`.
void _golden(
  String description,
  String file,
  Widget Function() builder, {
  bool platform = false,
  bool perTheme = false,
}) {
  void register() {
    if (!perTheme) {
      goldenTest(
        description,
        fileName: file,
        pumpBeforeTest: _settle,
        builder: () => matrix(description, builder),
      );
      return;
    }
    for (final theme in DsThemeMode.values) {
      goldenTest(
        '$description, ${theme.name}',
        fileName: '${file}_${theme.name}',
        pumpBeforeTest: _settle,
        builder: () => matrix(description, builder, themes: [theme]),
      );
    }
  }

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
    for (final name in fixtureFiles.keys) {
      if (!name.startsWith('StatsLiveFrame.')) continue;
      final state = name.split('.')[1];
      _frames[state] = await memoryFixtures.liveFrame(state);
    }
  });

  for (final MapEntry(key: kind, value: states) in viewStates.entries) {
    for (final state in states) {
      _golden(
        '${kind.name} view with the $state fixtures',
        'view_${kind.name}_$state',
        () => _ViewFixture(kind: kind, state: state),
        platform: state == 'normal',
        perTheme: kind == VizViewKind.timeline,
      );
    }
  }

  _golden(
    'delivery view in a space scope',
    'view_delivery_space_scope',
    () => const _ViewFixture(
      kind: VizViewKind.delivery,
      state: 'normal',
      scope: VizScope.space('0192f4a0-7b1c-7000-8a00-000000000002'),
    ),
  );

  _golden(
    'executive page of a space manager',
    'view_executive_slo_forbidden',
    () => const _ViewFixture(
      kind: VizViewKind.executive,
      state: 'normal',
      sloForbidden: true,
    ),
  );

  _golden(
    'TV mode with the live topology',
    'view_tvMode_normal',
    () => const _ViewFixture(kind: VizViewKind.tvMode, state: 'normal'),
    platform: true,
  );
}
