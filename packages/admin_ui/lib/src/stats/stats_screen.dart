// SPDX-License-Identifier: Apache-2.0

/// The statistics screen A7 of the administration (docs/spec/01-product.md 5.2, 5.5;
/// docs/observability/admin-statistics.md; brief M6-06).
library;

import 'dart:async';

import 'package:api_client/api_client.dart' as api;
import 'package:bothy_viz/bothy_viz.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/problem_banner.dart';
import 'stats_controller.dart';
import 'stats_range.dart';
import 'stats_sections.dart';
import 'stats_source.dart';

/// A7 "Statistics": the time range (presets and a custom range; the server bounds it with
/// `stats.max_range` and answers a longer one with its reason), the scope ("All spaces" and the
/// spaces; an entry the server refuses leaves with a note), the tiles (runs with the change
/// against the previous range, costs per billing mode with "Estimate" or "information only")
/// and the sections runs by result, run duration per agent, failure reasons, tool calls, error
/// rates, costs per agent and per space and usage per space, each with "As table".
///
/// Every number comes from the statistics operations of [source]; no section groups by person,
/// and a count of people the server withheld reads "fewer than the minimum group". The link
/// "Open telemetry dashboard" appears only when `getInstance` delivers
/// `telemetry_dashboard_url` (owners) and [openExternal] is set; the host opens it in the system
/// browser or a new tab, never in a WebView (25 9.1).
///
/// Parameters: [timeZone] and [clock] format the periods and end the preset ranges.
///
/// Example:
/// ```dart
/// StatsScreen(
///   source: repository,
///   timeZone: 'Europe/Berlin',
///   openExternal: (url) => launchUrl(url, mode: LaunchMode.externalApplication),
/// );
/// ```
class StatsScreen extends StatefulWidget {
  /// Creates the screen.
  const StatsScreen({
    super.key,
    required this.source,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
    this.openExternal,
  });

  /// The statistics calls.
  final AdminStatsSource source;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// Opens a link outside the app, or null for no telemetry link.
  final ValueChanged<Uri>? openExternal;

  @override
  State<StatsScreen> createState() => _StatsScreenState();
}

class _StatsScreenState extends State<StatsScreen> {
  late final StatsController _c = StatsController(
    source: widget.source,
    clock: widget.clock,
  );

  @override
  void initState() {
    super.initState();
    unawaited(_c.start());
  }

  @override
  void dispose() {
    _c.dispose();
    super.dispose();
  }

  Future<void> _pickCustom() async {
    final now = widget.clock();
    final picked = await showDateRangePicker(
      context: context,
      // The server bounds the range with stats.max_range and answers a longer one with its
      // reason; the picker only keeps the dates in the past.
      firstDate: DateTime(now.year - 10),
      lastDate: now,
      initialDateRange: DateTimeRange(
        start: _c.range.from.toLocal(),
        end: _c.range.to.toLocal(),
      ),
    );
    if (picked == null) return;
    await _c.setRange(
      StatsRange(
        from: DateTime.utc(
          picked.start.year,
          picked.start.month,
          picked.start.day,
        ),
        // The last chosen day is included, so the range ends at the start of the next day.
        to: DateTime.utc(picked.end.year, picked.end.month, picked.end.day + 1),
        preset: StatsPreset.custom,
      ),
    );
  }

  @override
  Widget build(BuildContext context) => ListenableBuilder(
    listenable: _c,
    builder: (context, _) {
      final l = AppLocalizations.of(context);
      final format = L10nFormat(Localizations.localeOf(context));
      final texts = StatsTexts(
        l10n: l,
        format: format,
        timeZone: widget.timeZone,
        now: widget.clock(),
        agentNames: _c.agentNames,
        spaceName: _c.spaceName,
      );
      final data = _c.data;
      final left = _c.nothingLeft;
      return Column(
        key: const ValueKey('stats.screen'),
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          _header(context, l),
          const SizedBox(height: DsSpace.s12),
          _controls(context, l, texts),
          for (final r in _c.removed)
            Padding(
              padding: const EdgeInsets.only(top: DsSpace.s8),
              child: DsBanner(
                key: ValueKey('stats.removed.${r.spaceId ?? 'instance'}'),
                tone: DsTone.info,
                text: l.statsScopeRemoved(
                  r.spaceId == null ? l.statsScopeAll : r.name,
                  r.problem == null
                      ? l.selfOffline
                      : problemText(
                          l,
                          r.problem!.code,
                          reason: r.problem!.reason,
                        ),
                ),
              ),
            ),
          if (_c.currencies.isNotEmpty)
            Padding(
              padding: const EdgeInsets.only(top: DsSpace.s8),
              child: DsBanner(tone: DsTone.info, text: l.statsCurrencyRequired),
            ),
          const SizedBox(height: DsSpace.s16),
          if (left != null)
            ProblemBanner(problem: left.problem)
          else if (data == null)
            DsBanner(
              key: const ValueKey('stats.loading'),
              tone: DsTone.info,
              text: l.statsLoading,
              liveRegion: true,
            )
          else ...[
            _tiles(data),
            const SizedBox(height: DsSpace.s16),
            ..._sections(texts, data),
          ],
        ],
      );
    },
  );

  Widget _header(BuildContext context, AppLocalizations l) {
    final colors = context.dsColors;
    final url = _c.telemetryUrl;
    final open = widget.openExternal;
    return Wrap(
      spacing: DsSpace.s16,
      runSpacing: DsSpace.s8,
      crossAxisAlignment: WrapCrossAlignment.center,
      children: [
        Semantics(
          header: true,
          child: Text(
            l.statsTitle,
            style: dsTextStyle(DsType.title)
                .copyWith(color: colors.textPrimary),
          ),
        ),
        if (url != null && open != null)
          TextButton(
            key: const ValueKey('stats.telemetry'),
            onPressed: () => open(url),
            child: Text(l.statsOpenTelemetry),
          ),
      ],
    );
  }

  Widget _controls(BuildContext context, AppLocalizations l, StatsTexts t) {
    final range = _c.range;
    final presetLabel = <StatsPreset, String>{
      StatsPreset.day: l.statsRangeDay,
      StatsPreset.week: l.statsRangeWeek,
      StatsPreset.month: l.statsRangeMonth,
      StatsPreset.quarter: l.statsRangeQuarter,
      StatsPreset.custom: l.statsRangeCustom,
    };
    final spaces = _c.spaces ?? const <StatsSpace>[];
    final scopeValue = _c.spaceId ?? '';
    final currencies = _c.currencies;
    return Wrap(
      spacing: DsSpace.s16,
      runSpacing: DsSpace.s8,
      crossAxisAlignment: WrapCrossAlignment.center,
      children: [
        _labelled(
          context,
          l.statsRangeLabel,
          DropdownButton<StatsPreset>(
            key: const ValueKey('stats.range'),
            value: range.preset,
            items: [
              for (final p in StatsPreset.values)
                DropdownMenuItem(
                  key: ValueKey('stats.range.${p.name}'),
                  value: p,
                  child: Text(presetLabel[p]!),
                ),
            ],
            onChanged: (p) {
              if (p == null) return;
              if (p == StatsPreset.custom) {
                unawaited(_pickCustom());
              } else {
                unawaited(_c.setRange(StatsRange.of(p, now: widget.clock())));
              }
            },
          ),
        ),
        if (range.preset == StatsPreset.custom)
          Text(
            key: const ValueKey('stats.range.custom.value'),
            l.statsRangeCustomValue(
              t.bucket(range.from, StatsBucketSize.day),
              t.bucket(
                range.to.subtract(const Duration(days: 1)),
                StatsBucketSize.day,
              ),
            ),
            style: dsTextStyle(DsType.bodySm)
                .copyWith(color: context.dsColors.textSecondary),
          ),
        _labelled(
          context,
          l.statsScopeLabel,
          DropdownButton<String>(
            key: const ValueKey('stats.scope'),
            value:
                (scopeValue.isEmpty && _c.offersInstance) ||
                    spaces.any((s) => s.id == scopeValue)
                ? scopeValue
                : null,
            items: [
              if (_c.offersInstance)
                DropdownMenuItem(
                  key: const ValueKey('stats.scope.instance'),
                  value: '',
                  child: Text(l.statsScopeAll),
                ),
              for (final s in spaces)
                DropdownMenuItem(
                  key: ValueKey('stats.scope.${s.id}'),
                  value: s.id,
                  child: Text(s.name),
                ),
            ],
            onChanged: (id) {
              if (id == null) return;
              final next = id.isEmpty ? null : id;
              if (next != _c.spaceId) unawaited(_c.setScope(next));
            },
          ),
        ),
        if (currencies.isNotEmpty)
          _labelled(
            context,
            l.statsCurrencyLabel,
            DropdownButton<String>(
              key: const ValueKey('stats.currency'),
              value: _c.currency,
              items: [
                for (final c in currencies)
                  DropdownMenuItem(
                    key: ValueKey('stats.currency.$c'),
                    value: c,
                    child: Text(c),
                  ),
              ],
              onChanged: (c) {
                if (c != null && c != _c.currency) unawaited(_c.setCurrency(c));
              },
            ),
          ),
      ],
    );
  }

  static Widget _labelled(BuildContext context, String label, Widget field) =>
      Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            label,
            style: dsTextStyle(DsType.label)
                .copyWith(color: context.dsColors.textSecondary),
          ),
          const SizedBox(width: DsSpace.s8),
          field,
        ],
      );

  Widget _tiles(StatsData data) {
    final runs = data.runs.value;
    final previous = data.runsPrevious.value;
    final tiles = <MetricTileModel>[
      MetricTileModel(
        metric: api.StatsMetric.runsFinished,
        value: runs == null ? null : runTotal(runs),
        previous: previous == null ? null : runTotal(previous),
        unit: TileUnit.count,
      ),
    ];
    final costs = data.costsByAgent.value;
    if (costs == null) {
      // No permission or no answer: "not shown", never zero (25 4 rule 5).
      tiles.add(const MetricTileModel(value: null, unit: TileUnit.money));
    } else {
      final micros = <api.BillingMode, int>{};
      final estimate = <api.BillingMode, bool>{};
      for (final s in costSums(costs)) {
        micros[s.mode] = (micros[s.mode] ?? 0) + s.micros;
        estimate[s.mode] = (estimate[s.mode] ?? false) || s.estimate;
      }
      for (final mode in const [
        api.BillingMode.organization,
        api.BillingMode.personal,
      ]) {
        final sum = micros[mode];
        if (sum == null) continue;
        tiles.add(
          MetricTileModel.cost(
            VizCost.of(
              micros: sum,
              currency: costs.currency,
              mode: mode,
              isEstimate: estimate[mode]!,
            ),
          ),
        );
      }
    }
    return Wrap(
      key: const ValueKey('stats.tiles'),
      spacing: DsSpace.s8,
      runSpacing: DsSpace.s8,
      children: [
        for (final (i, t) in tiles.indexed)
          KeyedSubtree(
            key: ValueKey('stats.tile.$i'),
            child: MetricTileMark(model: t),
          ),
      ],
    );
  }

  List<Widget> _sections(StatsTexts t, StatsData data) {
    Widget part<T>(
      String key,
      String title,
      StatsPart<T> answer,
      StatsSectionData Function(T value) build,
    ) {
      final value = answer.value;
      if (value == null) {
        return _FailedSection(
          key: ValueKey('stats.section.$key'),
          title: title,
          problem: answer.problem,
          onRetry: () => unawaited(_c.load()),
        );
      }
      final section = build(value);
      return StatsSection(
        key: ValueKey('stats.section.${section.key}'),
        section: section,
      );
    }

    final l = t.l10n;
    final sections = [
      part('runs', l.statsSectionRuns, data.runs, (r) => runsSection(t, r)),
      part(
        'duration',
        l.statsSectionDuration,
        data.runs,
        (r) => durationSection(t, r),
      ),
      part(
        'failures',
        l.statsSectionFailures,
        data.runs,
        (r) => failuresSection(t, r),
      ),
      part('tools', l.statsSectionTools, data.tools, (r) => toolsSection(t, r)),
      part(
        'errors',
        l.statsSectionErrors,
        data.errors,
        (r) => errorsSection(t, r),
      ),
      part(
        'costs-agent',
        l.statsSectionCostsAgent,
        data.costsByAgent,
        (c) => costSection(t, c, byAgent: true),
      ),
      part(
        'costs-space',
        l.statsSectionCostsSpace,
        data.costsBySpace,
        (c) => costSection(t, c, byAgent: false),
      ),
      part('usage', l.statsSectionUsage, data.usage, (u) => usageSection(t, u)),
    ];
    return [
      for (final s in sections)
        Padding(
          padding: const EdgeInsets.only(bottom: DsSpace.s24),
          child: s,
        ),
    ];
  }
}

/// A section whose query failed: its heading and the server's reason with "Try again".
class _FailedSection extends StatelessWidget {
  const _FailedSection({
    super.key,
    required this.title,
    required this.problem,
    required this.onRetry,
  });

  final String title;
  final api.ApiProblem? problem;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) => Column(
    crossAxisAlignment: CrossAxisAlignment.stretch,
    mainAxisSize: MainAxisSize.min,
    children: [
      _SectionTitle(title),
      const SizedBox(height: DsSpace.s8),
      ProblemBanner(problem: problem, onRetry: onRetry),
    ],
  );
}

class _SectionTitle extends StatelessWidget {
  const _SectionTitle(this.title);

  final String title;

  @override
  Widget build(BuildContext context) => Semantics(
    header: true,
    child: Text(
      title,
      style: dsTextStyle(DsType.heading)
          .copyWith(color: context.dsColors.textPrimary),
    ),
  );
}

/// One section of A7: its heading, "As table", and the chart of the basic bar mark of
/// `bothy_viz` or the table with the same values; without rows it says "No data in this range".
///
/// The chart has one row per entry: the values as text above the bar, so no value is told by
/// length or color alone (25 7.4); screen readers read the title and every row with its values.
class StatsSection extends StatefulWidget {
  /// Creates the section of [section].
  const StatsSection({super.key, required this.section});

  /// The data.
  final StatsSectionData section;

  @override
  State<StatsSection> createState() => _StatsSectionState();
}

class _StatsSectionState extends State<StatsSection> {
  bool _asTable = false;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final s = widget.section;
    final empty = s.rows.isEmpty;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Wrap(
          spacing: DsSpace.s16,
          runSpacing: DsSpace.s8,
          crossAxisAlignment: WrapCrossAlignment.center,
          children: [
            _SectionTitle(s.title),
            if (!empty)
              Semantics(
                toggled: _asTable,
                child: OutlinedButton(
                  key: ValueKey('stats.${s.key}.asTable'),
                  onPressed: () => setState(() => _asTable = !_asTable),
                  style: OutlinedButton.styleFrom(
                    backgroundColor: _asTable
                        ? colors.surfaceSunken
                        : Colors.transparent,
                    side: BorderSide(
                      color: _asTable
                          ? colors.borderControl
                          : colors.borderSubtle,
                      width: _asTable
                          ? DsBorderWidth.focus
                          : DsBorderWidth.hairline,
                    ),
                  ),
                  child: Text(
                    l.vizAsTable,
                    style: dsTextStyle(DsType.label)
                        .copyWith(color: colors.textPrimary),
                  ),
                ),
              ),
          ],
        ),
        const SizedBox(height: DsSpace.s8),
        if (empty)
          DsBanner(
            key: ValueKey('stats.${s.key}.empty'),
            tone: DsTone.info,
            text: l.statsEmpty,
          )
        else if (_asTable)
          VizTable(key: ValueKey('stats.${s.key}.table'), data: s.table)
        else
          _chart(context, l),
        if (!empty && !_asTable && s.note != null)
          Padding(
            padding: const EdgeInsets.only(top: DsSpace.s4),
            child: Text(
              s.note!,
              style: dsTextStyle(DsType.caption)
                  .copyWith(color: colors.textSecondary),
            ),
          ),
      ],
    );
  }

  Widget _chart(BuildContext context, AppLocalizations l) {
    final s = widget.section;
    final colors = context.dsColors;
    final model = statsBarModel(s.rows);
    return Semantics(
      key: ValueKey('stats.${s.key}.chart'),
      container: true,
      label: l.statsChartSummary(
        s.title,
        [for (final r in s.rows) r.text].join('; '),
      ),
      child: ExcludeSemantics(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          mainAxisSize: MainAxisSize.min,
          children: [
            for (final (i, row) in s.rows.indexed) ...[
              Text(
                [for (final p in row.parts) p.text].join(', '),
                style: dsTextStyle(DsType.caption)
                    .copyWith(color: colors.textSecondary),
              ),
              BarMark(
                model: BarModel(
                  from: model.from,
                  to: model.to,
                  rows: [model.rows[i]],
                ),
              ),
              const SizedBox(height: DsSpace.s4),
            ],
          ],
        ),
      ),
    );
  }
}

/// The bar model of [rows] on one shared scale: the bar mark of `bothy_viz` draws bars on a
/// time axis, so each value becomes a span from a fixed origin, the longest row filling the
/// track and every part stacked after the previous one.
BarModel statsBarModel(List<StatsBarRow> rows) {
  final origin = DateTime.utc(2000);
  // The track resolution in microseconds; only the ratio of the values matters.
  const span = 1000000000;
  var longest = 0.0;
  for (final r in rows) {
    final total = r.parts.fold<double>(0, (a, p) => a + p.value);
    if (total > longest) longest = total;
  }
  final unit = longest <= 0 ? 0.0 : span / longest;
  DateTime at(double v) =>
      origin.add(Duration(microseconds: (v * unit).round()));
  return BarModel(
    from: origin,
    to: origin.add(const Duration(microseconds: span)),
    rows: [
      for (final r in rows)
        BarRow(
          label: r.label,
          segments: () {
            var offset = 0.0;
            final out = <BarSegment>[];
            for (final p in r.parts) {
              if (p.value <= 0) continue;
              out.add(
                BarSegment(
                  start: at(offset),
                  end: at(offset + p.value),
                  runState: p.state,
                  colorIndex: p.colorIndex,
                ),
              );
              offset += p.value;
            }
            return out;
          }(),
        ),
    ],
  );
}
