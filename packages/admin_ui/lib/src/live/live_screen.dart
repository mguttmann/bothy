// SPDX-License-Identifier: Apache-2.0

/// The page "Live" of the desktop app (D12, TV mode D13), the mobile app (M10) and the console
/// (01 5.5): the views of `bothy_viz` as tabs, the scope, TV mode and the link "Open in
/// Grafana" (docs/spec/25-visualization.md 6, 7.1, 9.1; brief M6-16 Design notes 2 to 4).
library;

import 'dart:async';

import 'package:bothy_viz/bothy_viz.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart' show AppConfig;
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../stats/stats_source.dart';

/// The hours of the REST queries of the views: the last day up to the time the page opened or
/// changed its scope (the longest timeline range is `stats.timeline_max_range`, a day by
/// default; the server answers `validation_failed` above it).
const Duration liveQueryRange = Duration(hours: 24);

/// The views of `bothy_viz` with one tab per view, a scope selector (all spaces or one space)
/// and, where the host allows it, TV mode over the whole window.
///
/// The host passes its transport: [live] (the gateway subscription in the apps, polling in the
/// console, D-M6-16-3 and D-M6-16-4) and [queries] (REST). The page knows no transport and no
/// permission: the server answers what the person may see, and a refusal shows its text in the
/// view (15 1 principle 1). [directory] supplies the names of spaces and agents and the
/// telemetry link of `getInstance`; every one of these reads may fail, and the page then shows
/// short IDs and no link. "Open in Grafana" appears only when the server names
/// `telemetry_dashboard_url` and the host can open it outside the app ([openExternal]: the
/// system browser on the desktop and mobile, a new tab in the console; never a web view or an
/// iframe, 25 9.1).
///
/// TV mode (25 6.6) needs [VizFormFactor.desktop] or [VizFormFactor.web]: it is pushed on the
/// root navigator as an opaque route over the whole window and rotates through the views of
/// the scope (instance-only views only in the instance scope); any key or click ends it. While
/// it runs, the tab view is taken down, so the one live source belongs to TV mode, and it is
/// started again afterwards.
///
/// Example:
/// ```dart
/// LiveScreen(
///   formFactor: VizFormFactor.desktop,
///   live: engineSource,
///   queries: ApiStatsQuerySource(session.api),
///   directory: ApiAdminStatsSource(session.api),
///   onOpenThread: (id) => context.go('/t/$id'),
///   openExternal: browser.open,
/// )
/// ```
class LiveScreen extends StatefulWidget {
  /// Creates the page.
  const LiveScreen({
    super.key,
    required this.formFactor,
    required this.live,
    required this.queries,
    required this.onOpenThread,
    this.directory,
    this.openExternal,
    this.config,
    this.timeZone = 'UTC',
    this.clock = const SystemVizClock(),
  });

  /// Desktop, mobile or web; mobile has no TV mode.
  final VizFormFactor formFactor;

  /// The live stream of the host, or null (the views read one frame through [queries]).
  final LiveFrameSource? live;

  /// The REST queries of the host.
  final StatsQuerySource queries;

  /// Opens the thread of a run card: the deep link `/t/{thread_id}` in the apps, the note
  /// "Open in the app" in the console (ADR 0028).
  final void Function(String threadId) onOpenThread;

  /// The spaces of the scope selector, the agent names and the telemetry link, or null.
  final AdminStatsSource? directory;

  /// Opens a URL outside the app, or null (no link "Open in Grafana").
  final void Function(Uri url)? openExternal;

  /// The app configuration (`viz.*`); null takes the defaults.
  final AppConfig? config;

  /// The IANA zone of the person for clock times.
  final String timeZone;

  /// The clock of the views, the playback and the TV rotation.
  final VizClock clock;

  @override
  State<LiveScreen> createState() => _LiveScreenState();
}

class _LiveScreenState extends State<LiveScreen> {
  VizViewKind _kind = VizViewKind.liveTopology;
  String? _spaceId;
  late VizRange _range = _rangeNow();
  List<StatsSpace> _spaces = const [];
  Map<String, String> _agents = const {};
  Uri? _dashboard;
  bool _tv = false;
  int _generation = 0;

  @override
  void initState() {
    super.initState();
    unawaited(_loadDirectory());
  }

  VizRange _rangeNow() {
    final now = widget.clock.now();
    return VizRange(from: now.subtract(liveQueryRange), to: now);
  }

  Future<void> _loadDirectory() async {
    final directory = widget.directory;
    if (directory == null) return;
    // Each read may be refused (a person without `stats.read` on the instance gets no
    // telemetry link); the page works without it.
    Future<T?> optional<T>(Future<T> read) =>
        read.then<T?>((v) => v, onError: (Object _) => null);
    final spaces = await optional(directory.statsSpaces());
    final agents = await optional(directory.statsAgentNames());
    final instance = await optional(directory.statsInstance());
    if (!mounted) return;
    setState(() {
      _spaces = spaces ?? const [];
      _agents = agents ?? const {};
      _dashboard = instance?.telemetryDashboardUrl;
    });
  }

  VizHostOptions _options() {
    final url = _dashboard;
    final open = widget.openExternal;
    final spaceId = _spaceId;
    return VizHostOptions(
      formFactor: widget.formFactor,
      scope: spaceId == null
          ? const VizScope.instance()
          : VizScope.space(spaceId),
      range: _range,
      allowTvMode: widget.formFactor != VizFormFactor.mobile,
      onOpenGrafana: url == null || open == null ? null : () => open(url),
      names: VizNames(
        agents: _agents,
        spaces: {for (final s in _spaces) s.id: s.name},
      ),
      timeZone: widget.timeZone,
      clock: widget.clock,
      config: widget.config,
    );
  }

  void _setScope(String? spaceId) {
    if (spaceId == _spaceId) return;
    setState(() {
      _spaceId = spaceId;
      _range = _rangeNow();
    });
  }

  Future<void> _startTv(VizHostOptions options) async {
    final views = [
      for (final spec in vizViewsFor(options))
        if (spec.kind != VizViewKind.tvMode &&
            (options.scope.isInstance || !spec.instanceOnly))
          spec.kind,
    ];
    setState(() => _tv = true);
    await Navigator.of(context, rootNavigator: true).push(
      PageRouteBuilder<void>(
        opaque: true,
        settings: const RouteSettings(name: 'live.tv'),
        pageBuilder: (routeContext, _, _) => TvMode(
          key: const ValueKey('live.tv.mode'),
          views: views,
          options: options,
          live: widget.live,
          queries: widget.queries,
          onExit: () => Navigator.of(routeContext).pop(),
        ),
      ),
    );
    if (!mounted) return;
    // A new key starts the tab view again: TV mode held the live source.
    setState(() {
      _tv = false;
      _generation++;
    });
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final options = _options();
    final views = [
      for (final spec in vizViewsFor(options))
        if (spec.kind != VizViewKind.tvMode) spec,
    ];
    final tvAllowed = vizViewsFor(options)
        .any((s) => s.kind == VizViewKind.tvMode);
    final kind = views.any((s) => s.kind == _kind) ? _kind : views.first.kind;
    final touch = widget.formFactor == VizFormFactor.mobile;
    final padding = touch ? DsSpace.s12 : DsSpace.s24;
    final header = Wrap(
      spacing: DsSpace.s12,
      runSpacing: DsSpace.s8,
      crossAxisAlignment: WrapCrossAlignment.center,
      children: [
        SingleChildScrollView(
          scrollDirection: Axis.horizontal,
          child: SegmentedButton<VizViewKind>(
            key: const ValueKey('live.tabs'),
            showSelectedIcon: false,
            segments: [
              for (final spec in views)
                ButtonSegment(
                  value: spec.kind,
                  label: Text(
                    spec.title(l),
                    key: ValueKey('live.tab.${spec.kind.name}'),
                  ),
                ),
            ],
            selected: {kind},
            onSelectionChanged: (s) => setState(() => _kind = s.first),
          ),
        ),
        if (_spaces.isNotEmpty)
          Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                l.statsScopeLabel,
                style: dsTextStyle(DsType.label)
                    .copyWith(color: colors.textSecondary),
              ),
              const SizedBox(width: DsSpace.s8),
              DropdownButton<String?>(
                key: const ValueKey('live.scope'),
                value: _spaceId,
                onChanged: _setScope,
                items: [
                  DropdownMenuItem<String?>(child: Text(l.statsScopeAll)),
                  for (final s in _spaces)
                    DropdownMenuItem<String?>(value: s.id, child: Text(s.name)),
                ],
              ),
            ],
          ),
        if (tvAllowed)
          OutlinedButton(
            key: const ValueKey('live.tv'),
            onPressed: _tv ? null : () => unawaited(_startTv(options)),
            child: Text(l.vizViewTv),
          ),
      ],
    );
    return ColoredBox(
      color: colors.surfaceCanvas,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Padding(
            padding: EdgeInsets.fromLTRB(padding, padding, padding, 0),
            child: header,
          ),
          Expanded(
            child: _tv
                // TV mode holds the live source; the tab view comes back when it ends.
                ? const SizedBox.shrink()
                : SingleChildScrollView(
                    key: ValueKey('live.scroll.${kind.name}'),
                    padding: EdgeInsets.all(padding),
                    child: buildVizView(
                      kind,
                      key: ValueKey(
                        'live.view.${kind.name}.${_spaceId ?? 'instance'}.$_generation',
                      ),
                      live: widget.live,
                      queries: widget.queries,
                      options: options,
                      onOpenThread: widget.onOpenThread,
                    ),
                  ),
          ),
        ],
      ),
    );
  }
}
