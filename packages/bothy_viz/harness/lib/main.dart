// SPDX-License-Identifier: Apache-2.0

/// The harness of `bothy_viz`: every mark from the fixtures, without a server and without
/// network (docs/spec/25-visualization.md 7.1 row `harness/`).
///
/// Run it with `flutter run -d macos`, `-d windows` or `-d chrome` from this folder after
/// `flutter create --platforms=macos,windows,web .` (the platform folders are not checked in,
/// see the README). The toolbar switches theme, density, text size, language and fixture
/// state; the time-lapse slider moves the fake clock of the fixture live source, so frames
/// and the stale state appear as they would over time. M6-12 adds the views to [harnessMarks].
library;

import 'package:bothy_viz/bothy_viz.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:l10n/l10n.dart';

/// Starts the harness on the fixtures of the asset bundle.
void main() => runApp(const HarnessApp());

/// One entry of the left list: its label and the widget it renders from the [HarnessData].
typedef HarnessEntry = ({
  String Function(AppLocalizations l10n) label,
  Widget Function(HarnessData data) build,
});

/// The marks of the harness in the order of the list.
final List<HarnessEntry> harnessMarks = [
  (
    label: (l10n) => l10n.vizMarkLane,
    build: (data) => Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (data.live.runsTruncated) _Note(data.l10n.vizMoreRunsThanShown),
        if (data.stale) _Note(data.l10n.vizStale),
        for (final lane in data.live.lanes)
          LaneMark(model: lane, onTapRun: data.onTapRun),
      ],
    ),
  ),
  (
    label: (l10n) => l10n.vizMarkRunCard,
    build: (data) => Wrap(
      spacing: DsSpace.s8,
      runSpacing: DsSpace.s8,
      children: [
        for (final card in data.live.cards)
          RunCardMark(model: card, onTap: data.onTapRun),
      ],
    ),
  ),
  (
    label: (l10n) => l10n.vizMarkArea,
    build: (data) =>
        AreaMark(title: data.l10n.vizChartRates, series: data.executive.series),
  ),
  (
    label: (l10n) => l10n.vizMarkBar,
    build: (data) => BarMark(model: data.timeline.bars),
  ),
  (
    label: (l10n) => l10n.vizMarkTrafficLight,
    build: (data) => Wrap(
      spacing: DsSpace.s8,
      runSpacing: DsSpace.s8,
      children: [
        for (final light in data.executive.trafficLights)
          TrafficLightMark(model: light),
      ],
    ),
  ),
  (
    label: (l10n) => l10n.vizMarkMetricTile,
    build: (data) => Wrap(
      spacing: DsSpace.s8,
      runSpacing: DsSpace.s8,
      children: [
        for (final tile in data.executive.tiles) MetricTileMark(model: tile),
        for (final tile
            in data.executive.costTiles ?? const <MetricTileModel>[])
          MetricTileMark(model: tile),
      ],
    ),
  ),
];

/// Fixture states the toolbar offers; a schema without a file of the state shows `normal`.
const List<String> harnessStates = [
  'normal',
  'empty',
  'truncated',
  'no_cost_permission',
  'offline',
  'load',
];

/// Text scales of the toolbar.
const List<double> harnessTextScales = [1.0, 1.5, 2.0];

/// Upper end of the time-lapse slider, in frame intervals (`interval_ms`) of the live fixture.
const int harnessTimeLapseSteps = 20;

/// The names of the test world (07 10) for the IDs of the fixtures.
const VizNames harnessNames = VizNames(
  agents: {
    '0192f4a0-7b1c-7000-8a00-000000000201': 'support-agent',
    '0192f4a0-7b1c-7000-8a00-000000000202': 'dev-runner',
  },
  spaces: {
    '0192f4a0-7b1c-7000-8a00-000000000001': 'Support',
    '0192f4a0-7b1c-7000-8a00-000000000002': 'Engineering',
  },
  channels: {
    '0192f4a0-7b1c-7000-8a00-000000000101': '#tickets',
    '0192f4a0-7b1c-7000-8a00-000000000102': '#backend',
    '0192f4a0-7b1c-7000-8a00-000000000103': '#release',
  },
);

/// What a harness entry renders: the models of the current fixture state.
final class HarnessData {
  /// Creates the data.
  const HarnessData({
    required this.l10n,
    required this.live,
    required this.stale,
    required this.timeline,
    required this.executive,
    required this.onTapRun,
  });

  /// Texts of the current language.
  final AppLocalizations l10n;

  /// The latest frame of the live source.
  final LiveTopologyModel live;

  /// The live source reported `stale`.
  final bool stale;

  /// The timeline of the state.
  final TimelineModel timeline;

  /// SLO report, usage series and costs of the state.
  final ExecutiveModel executive;

  /// Shows the thread of a tapped run card.
  final RunTapCallback onTapRun;
}

/// The harness app; [reader] replaces the asset bundle (tests read the files directly).
class HarnessApp extends StatefulWidget {
  /// Creates the app.
  const HarnessApp({super.key, this.reader, this.motion = DsMotionMode.full});

  /// Reads a fixture by file name; defaults to the asset bundle of `bothy_viz`.
  final FixtureReader? reader;

  /// Motion context; tests use `reduced`, so the spinning run icon lets the tree settle.
  final DsMotionMode motion;

  @override
  State<HarnessApp> createState() => HarnessAppState();
}

/// State of [HarnessApp]: the toolbar values, the fake clock and the loaded models.
class HarnessAppState extends State<HarnessApp> {
  /// Light or dark.
  DsThemeMode theme = DsThemeMode.light;

  /// Density context.
  DsDensityMode density = DsDensityMode.comfortable;

  /// Text scale.
  double textScale = harnessTextScales.first;

  /// Language.
  Locale locale = AppLocalizations.supportedLocales.last;

  /// Fixture state.
  String fixtureState = harnessStates.first;

  /// Position of the time-lapse slider in replay steps.
  int timeLapse = 0;

  /// Index of the selected entry of [harnessMarks].
  int selected = 0;

  late final VizFixtures _fixtures = VizFixtures(
    widget.reader ??
        (name) => rootBundle.loadString('packages/bothy_viz/fixtures/$name'),
  );
  final Duration _staleAfter = AppConfig.defaults().staleAfter;
  FakeVizClock? _clock;
  Duration _step = Duration.zero;
  FixtureLiveFrameSource? _source;
  LiveTopologyModel? _live;
  bool _stale = false;
  TimelineModel? _timeline;
  ExecutiveModel? _executive;
  String? _tapped;
  int _generation = 0;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _source?.stop();
    super.dispose();
  }

  Future<String?> _pick(String schema) async {
    for (final state in [fixtureState, 'normal']) {
      try {
        await _fixtures.read('$schema.$state.json');
        return state;
      } on Object {
        continue;
      }
    }
    return null;
  }

  Future<void> _load() async {
    final generation = ++_generation;
    final frameState = (await _pick('StatsLiveFrame'))!;
    final frame = await _fixtures.liveFrame(frameState);
    final empty = await _fixtures.liveFrame('empty');
    final timeline = await _fixtures.timeline((await _pick('StatsTimeline'))!);
    final executive = ExecutiveModel.from(
      slo: await _fixtures.sloReport((await _pick('StatsSloReport'))!),
      usage: fixtureState == 'empty'
          ? await _fixtures.series('empty')
          : await _fixtures.series('normal', query: 'usage'),
      costs: fixtureState == 'no_cost_permission'
          ? null
          : await _fixtures.costSeries((await _pick('CostSeries'))!),
      names: harnessNames,
    );
    if (!mounted || generation != _generation) return;
    await _source?.stop();
    final clock = FakeVizClock(frame.at);
    final step = Duration(milliseconds: frame.intervalMs);
    final source = FixtureLiveFrameSource(
      clock: clock,
      staleAfter: _staleAfter,
      names: harnessNames,
      // Two frames, then silence: the time-lapse shows the frames and then the stale state.
      steps: [
        (after: Duration.zero, frame: frame),
        (after: step, frame: empty),
        (after: step, frame: frame),
      ],
    );
    source.start().listen((event) {
      if (!mounted) return;
      setState(() {
        switch (event) {
          case LiveFrameFrame(:final model):
            _live = model;
            _stale = false;
          case LiveFrameStale():
            _stale = true;
          case LiveFrameError():
            break;
        }
      });
    });
    clock.advance(Duration.zero);
    setState(() {
      _clock = clock;
      _step = step;
      _source = source;
      _timeline = TimelineModel.fromTimeline(timeline, names: harnessNames);
      _executive = executive;
      timeLapse = 0;
    });
  }

  void _timeLapseTo(int value) {
    final clock = _clock;
    if (clock == null) return;
    if (value < timeLapse) {
      // The fake clock moves only forward: going back replays from the start.
      _load();
      return;
    }
    clock.advance(_step * (value - timeLapse));
    setState(() => timeLapse = value);
  }

  @override
  Widget build(BuildContext context) {
    final dsContext = DsContext(
      theme: theme,
      density: density,
      motion: widget.motion,
      accent: dsDefaultAccent,
    );
    return MaterialApp(
      locale: locale,
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      theme: buildDsTheme(dsContext),
      builder: (context, child) => MediaQuery(
        data: MediaQuery.of(context)
            .copyWith(textScaler: TextScaler.linear(textScale)),
        child: child!,
      ),
      home: Builder(builder: _page),
    );
  }

  Widget _page(BuildContext context) {
    final l10n = AppLocalizations.of(context);
    final live = _live;
    final timeline = _timeline;
    final executive = _executive;
    return Scaffold(
      appBar: AppBar(title: Text(l10n.vizHarnessTitle)),
      body: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: DsLayout.sidebarWidth,
            child: ListView(
              children: [
                for (var i = 0; i < harnessMarks.length; i++)
                  ListTile(
                    key: ValueKey('mark-$i'),
                    selected: i == selected,
                    title: Text(harnessMarks[i].label(l10n)),
                    onTap: () => setState(() => selected = i),
                  ),
              ],
            ),
          ),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                _toolbar(l10n),
                if (_tapped != null) _Note(_tapped!),
                Expanded(
                  child: SingleChildScrollView(
                    padding: const EdgeInsets.all(DsSpace.s16),
                    child: live == null || timeline == null || executive == null
                        ? const SizedBox.shrink()
                        : KeyedSubtree(
                            key: const ValueKey('harness-mark'),
                            child: harnessMarks[selected].build(
                              HarnessData(
                                l10n: l10n,
                                live: live,
                                stale: _stale,
                                timeline: timeline,
                                executive: executive,
                                onTapRun: (thread) =>
                                    setState(() => _tapped = thread),
                              ),
                            ),
                          ),
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _toolbar(AppLocalizations l10n) => Padding(
    padding: const EdgeInsets.all(DsSpace.s8),
    child: Wrap(
      spacing: DsSpace.s16,
      runSpacing: DsSpace.s8,
      crossAxisAlignment: WrapCrossAlignment.center,
      children: [
        _Choice<DsThemeMode>(
          id: 'theme',
          label: l10n.vizHarnessTheme,
          value: theme,
          values: DsThemeMode.values,
          text: (v) => v == DsThemeMode.light
              ? l10n.vizHarnessThemeLight
              : l10n.vizHarnessThemeDark,
          onChanged: (v) => setState(() => theme = v),
        ),
        _Choice<DsDensityMode>(
          id: 'density',
          label: l10n.vizHarnessDensity,
          value: density,
          values: DsDensityMode.values,
          text: (v) => switch (v) {
            DsDensityMode.comfortable => l10n.vizHarnessDensityComfortable,
            DsDensityMode.compact => l10n.vizHarnessDensityCompact,
            DsDensityMode.touch => l10n.vizHarnessDensityTouch,
          },
          onChanged: (v) => setState(() => density = v),
        ),
        _Choice<double>(
          id: 'textScale',
          label: l10n.vizHarnessTextScale,
          value: textScale,
          values: harnessTextScales,
          text: (v) => '${v}x',
          onChanged: (v) => setState(() => textScale = v),
        ),
        _Choice<Locale>(
          id: 'language',
          label: l10n.vizHarnessLanguage,
          value: locale,
          values: AppLocalizations.supportedLocales,
          text: (v) => v.toLanguageTag(),
          onChanged: (v) => setState(() => locale = v),
        ),
        _Choice<String>(
          id: 'fixtureState',
          label: l10n.vizHarnessFixtureState,
          value: fixtureState,
          values: harnessStates,
          text: (v) => v,
          onChanged: (v) {
            setState(() => fixtureState = v);
            _load();
          },
        ),
        Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(l10n.vizHarnessTimeLapse),
            SizedBox(
              width: DsLayout.columnMin,
              child: Slider(
                key: const ValueKey('timeLapse'),
                value: timeLapse.toDouble(),
                max: harnessTimeLapseSteps.toDouble(),
                divisions: harnessTimeLapseSteps,
                label: '$timeLapse',
                onChanged: (v) => _timeLapseTo(v.round()),
              ),
            ),
          ],
        ),
      ],
    ),
  );
}

class _Choice<T> extends StatelessWidget {
  const _Choice({
    required this.id,
    required this.label,
    required this.value,
    required this.values,
    required this.text,
    required this.onChanged,
  });

  /// Test handle of the dropdown: `<id>-button`.
  final String id;
  final String label;
  final T value;
  final List<T> values;
  final String Function(T value) text;
  final ValueChanged<T> onChanged;

  @override
  Widget build(BuildContext context) => Row(
    mainAxisSize: MainAxisSize.min,
    children: [
      Text(label),
      const SizedBox(width: DsSpace.s8),
      DropdownButton<T>(
        key: ValueKey('$id-button'),
        value: value,
        items: [
          for (final v in values)
            DropdownMenuItem(value: v, child: Text(text(v))),
        ],
        onChanged: (v) {
          if (v != null) onChanged(v);
        },
      ),
    ],
  );
}

class _Note extends StatelessWidget {
  const _Note(this.text);

  final String text;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.symmetric(
      vertical: DsSpace.s4,
      horizontal: DsSpace.s16,
    ),
    child: Text(
      text,
      style: dsTextStyle(DsType.label)
          .copyWith(color: context.dsColors.textSecondary),
    ),
  );
}
