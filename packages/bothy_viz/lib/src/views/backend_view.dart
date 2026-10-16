// SPDX-License-Identifier: Apache-2.0

/// The backend view (docs/spec/25-visualisierung.md 6.2; ruling T8).
library;

import 'dart:async';

import 'package:api_client/api_client.dart' as api;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';

import '../marks/viz_texts.dart';
import '../model/backend_model.dart';
import '../sources/live_frame_source.dart';
import '../sources/stats_query_source.dart';
import 'backend_texts.dart';
import 'table/backend_table.dart';
import 'view_scaffold.dart';
import 'viz_host_options.dart';

/// Per agent what it runs on: configured and reported billing mode with the mismatch
/// marker, supported modes, switches, denials by I2 and I3, costs per mode and the
/// rate-limit state reported by the adapter.
///
/// Reads `getStatsBackends` once for the host's range and updates presence, reported mode
/// and rate limit from the `agents` of every live frame. Product neutral: no runtime, no
/// subscription and no credential appear, because the schema carries none (I5, T8).
class BackendView extends StatefulWidget {
  /// Creates the view.
  const BackendView({
    super.key,
    required this.live,
    required this.queries,
    required this.options,
    required this.onOpenThread,
  });

  /// The live stream for the `agents` of the frame; `null` shows the report alone.
  final LiveFrameSource? live;

  /// The REST queries (`getStatsBackends`).
  final StatsQuerySource queries;

  /// What the host knows.
  final VizHostOptions options;

  /// Not used by this view; the view contract is the same for all six views.
  final void Function(String threadId) onOpenThread;

  @override
  State<BackendView> createState() => _BackendViewState();
}

class _BackendViewState extends State<BackendView> with VizLiveSubscription {
  api.StatsBackendReport? _report;
  String? _queryError;

  @override
  void initState() {
    super.initState();
    unawaited(_load());
    startLive(widget.live, widget.options.scope);
  }

  Future<void> _load() async {
    final range = widget.options.range;
    try {
      final report = await widget.queries.backends(
        from: range.from,
        to: range.to,
        spaceId: widget.options.scope.spaceId,
      );
      if (mounted) setState(() => _report = report);
    } on Object catch (error) {
      if (!mounted) return;
      setState(() => _queryError = vizProblemCode(error));
      unawaited(stopLive());
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = VizTexts.of(context).l10n;
    final report = _report;
    final error = _queryError ?? liveError;
    final model = report == null
        ? null
        : BackendModel.fromReport(
            report,
            names: widget.options.names,
            live: liveFrame?.frame,
          );
    final costs = VizTvScope.costsVisible(context);
    return VizViewScaffold(
      title: l10n.vizViewBackend,
      options: widget.options,
      status: error != null
          ? VizFailed(error)
          : model == null
          ? const VizLoading()
          : const VizReady(),
      notes: vizLiveNotes(l10n, stale: liveStale),
      tables: (texts) => backendTables(
        texts,
        model!,
        costsVisible: costs,
        timeZone: widget.options.timeZone,
      ),
      chart: (context) => Wrap(
        spacing: DsSpace.s8,
        runSpacing: DsSpace.s8,
        children: [
          for (final row in model!.rows)
            _AgentPanel(
              row: row,
              costsVisible: costs,
              timeZone: widget.options.timeZone,
            ),
        ],
      ),
    );
  }
}

/// One agent of the backend view.
class _AgentPanel extends StatelessWidget {
  const _AgentPanel({
    required this.row,
    required this.costsVisible,
    required this.timeZone,
  });

  final BackendRowModel row;
  final bool costsVisible;
  final String timeZone;

  @override
  Widget build(BuildContext context) {
    final texts = VizTexts.of(context);
    final l10n = texts.l10n;
    final colors = context.dsColors;
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    final costs = costsVisible ? backendCosts(texts, row) : null;
    final lines = <String>[
      '${l10n.vizColConfiguredMode}: ${texts.badge(row.billingMode)}',
      '${l10n.vizColReportedMode}: ${backendReported(texts, row)}',
      '${l10n.vizColSupportedModes}: ${backendSupported(texts, row)}',
      '${l10n.vizColSwitches}: ${texts.format.count(row.switches)}',
      '${l10n.vizColI2Denied}: ${texts.format.count(row.i2Denied)}',
      '${l10n.vizColI3Denied}: ${texts.format.count(row.billingModeMismatch)}',
      if (costs != null) '${l10n.vizColCosts}: $costs',
    ];
    final presence = backendPresence(texts, row);
    final rateLimit = backendRateLimit(texts, row, timeZone);
    return Semantics(
      container: true,
      label: [
        row.agentName,
        presence,
        ...lines,
        '${l10n.vizColRateLimit}: $rateLimit',
      ].join(', '),
      child: ExcludeSemantics(
        child: Container(
          constraints: const BoxConstraints(
            minWidth: DsLayout.columnMin,
            maxWidth: DsLayout.overlayWidth,
          ),
          padding: const EdgeInsets.all(DsSpace.s12),
          decoration: BoxDecoration(
            color: colors.surfaceRaised,
            borderRadius: BorderRadius.circular(DsRadius.lg),
            border: Border.all(
              color: row.modeMismatch
                  ? colors.warningBorder
                  : colors.borderSubtle,
              width: DsBorderWidth.hairline,
            ),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                row.agentName,
                style: dsTextStyle(DsType.heading)
                    .copyWith(color: colors.textPrimary),
              ),
              _IconText(
                glyph: backendPresenceGlyph(row),
                text: presence,
                color: colors.textSecondary,
              ),
              if (row.modeMismatch)
                _IconText(
                  glyph: DsIcons.stateWarning,
                  text: backendReported(texts, row),
                  color: colors.warningFg,
                ),
              for (final line in lines) Text(line, style: secondary),
              _IconText(
                glyph: backendRateLimitGlyph(row),
                text: '${l10n.vizColRateLimit}: $rateLimit',
                color: colors.textSecondary,
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// A state as icon and text, never color alone.
class _IconText extends StatelessWidget {
  const _IconText({
    required this.glyph,
    required this.text,
    required this.color,
  });

  final DsGlyph glyph;
  final String text;
  final Color color;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.symmetric(vertical: DsSpace.s2),
    child: Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        DsIcon(glyph, size: DsIconSizeRole.sm, color: color, decorative: true),
        const SizedBox(width: DsSpace.s4),
        Flexible(
          child: Text(
            text,
            style: dsTextStyle(DsType.bodySm).copyWith(color: color),
          ),
        ),
      ],
    ),
  );
}
