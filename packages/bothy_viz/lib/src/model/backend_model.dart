// SPDX-License-Identifier: Apache-2.0

/// The backend view (docs/spec/25-visualisierung.md 6.2).
library;

import 'package:api_client/api_client.dart' as api;

import 'costs.dart';
import 'names.dart';

/// One agent of the backend view: configured and reported mode, supported modes, switches,
/// denials, costs per mode and rate-limit state; product neutral (25 6.2, I5).
final class BackendRowModel {
  /// Creates a row.
  const BackendRowModel({
    required this.agentId,
    required this.agentName,
    required this.billingMode,
    required this.supportedModes,
    required this.switches,
    required this.i2Denied,
    required this.billingModeMismatch,
    this.reportedBillingMode,
    this.costs,
    this.rateLimit,
    this.rateLimitResetsAt,
    this.presence,
  });

  /// Agent ID.
  final String agentId;

  /// Display name of the agent.
  final String agentName;

  /// Configured mode (`agents.billing_mode`).
  final api.BillingMode billingMode;

  /// Reported mode; `null` when the adapter follows the configured mode per run.
  final api.BillingMode? reportedBillingMode;

  /// Modes the agent supports.
  final List<api.BillingMode> supportedModes;

  /// Billing mode switches in the range.
  final int switches;

  /// Denials before delivery (I2).
  final int i2Denied;

  /// Runs failed with `billing_mode_mismatch` (I3).
  final int billingModeMismatch;

  /// Costs per mode; `null` when not shown (no `cost.read.aggregate`), never zero.
  final List<VizCost>? costs;

  /// Rate-limit state reported by the adapter.
  final api.AgentRateLimitStateStateEnum? rateLimit;

  /// When the rate limit resets.
  final DateTime? rateLimitResetsAt;

  /// Presence from the live frame; `null` without a live frame.
  final api.AgentRuntimeState? presence;

  /// The reported mode differs from the configured one: the warning badge "Reported: ..."
  /// (components.md 10).
  bool get modeMismatch =>
      reportedBillingMode != null && reportedBillingMode != billingMode;
}

/// Every agent of `getStatsBackends`, supplemented live by the `agents` of a frame.
final class BackendModel {
  /// Creates the model.
  const BackendModel({required this.rows});

  /// Derives the rows from [report]; with [live], presence, reported mode and rate limit come
  /// from the frame, which is newer than the report.
  factory BackendModel.fromReport(
    api.StatsBackendReport report, {
    VizNames names = const VizNames(),
    api.StatsLiveFrame? live,
  }) {
    final agents = {for (final a in live?.agents ?? <api.StatsLiveAgent>[]) a.agentId: a};
    return BackendModel(
      rows: List.unmodifiable([
        for (final item in report.items)
          _row(item, names, agents[item.agentId]),
      ]),
    );
  }

  static BackendRowModel _row(
    api.StatsBackendAgent item,
    VizNames names,
    api.StatsLiveAgent? live,
  ) {
    final costs = item.costs;
    final rateLimit = live?.rateLimit ?? item.rateLimit;
    return BackendRowModel(
      agentId: item.agentId,
      agentName: names.agent(item.agentId),
      billingMode: item.billingMode,
      reportedBillingMode: live == null
          ? item.reportedBillingMode
          : live.reportedBillingMode,
      supportedModes: List.unmodifiable(item.billingModesSupported),
      switches: item.switches,
      i2Denied: item.invocationsDenied.i2Denied,
      billingModeMismatch: item.invocationsDenied.billingModeMismatch,
      costs: costs == null
          ? null
          : List.unmodifiable([
              for (final c in costs)
                VizCost.of(
                  micros: c.amountMicros,
                  currency: c.currency,
                  mode: c.billingMode,
                  isEstimate: c.isEstimate,
                ),
            ]),
      rateLimit: rateLimit?.state,
      rateLimitResetsAt: rateLimit?.resetsAt,
      presence: live?.presence,
    );
  }

  /// One row per agent in the order of the report.
  final List<BackendRowModel> rows;
}
