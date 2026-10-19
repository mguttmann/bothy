// SPDX-License-Identifier: Apache-2.0

/// Texts of one row of the backend view, shared by the panel and the table.
library;

import 'package:api_client/api_client.dart' as api;
import 'package:design_system/design_system.dart';

import '../marks/viz_texts.dart';
import '../model/backend_model.dart';
import 'view_texts.dart';

/// The presence of [row] from the live frame, or "Unknown" without a frame.
String backendPresence(VizTexts texts, BackendRowModel row) {
  final l10n = texts.l10n;
  return switch (row.presence) {
    api.AgentRuntimeState.online => l10n.vizPresenceOnline,
    api.AgentRuntimeState.degraded => l10n.vizPresenceDegraded,
    api.AgentRuntimeState.offline => l10n.vizPresenceOffline,
    _ => l10n.vizPresenceUnknown,
  };
}

/// The icon of the presence of [row]; never color alone.
DsGlyph backendPresenceGlyph(BackendRowModel row) => switch (row.presence) {
  api.AgentRuntimeState.online => DsIcons.triggerDone,
  api.AgentRuntimeState.degraded => DsIcons.stateWarning,
  api.AgentRuntimeState.offline => DsIcons.stateOffline,
  _ => DsIcons.stateInfo,
};

/// The reported mode of [row]: "Reported: Personal" when it differs from the configured
/// mode (components.md 10), the mode when it matches, "follows the run" when the adapter
/// reports none.
String backendReported(VizTexts texts, BackendRowModel row) {
  final reported = row.reportedBillingMode;
  if (reported == null) return texts.l10n.vizFollowsRun;
  final badge = texts.badge(reported);
  return row.modeMismatch ? texts.l10n.vizReportedMismatch(badge) : badge;
}

/// The supported modes of [row], comma separated.
String backendSupported(VizTexts texts, BackendRowModel row) =>
    row.supportedModes.map(texts.badge).join(', ');

/// The costs per mode of [row] with their labels, or `null` when not shown.
String? backendCosts(VizTexts texts, BackendRowModel row) {
  final costs = row.costs;
  if (costs == null || costs.isEmpty) return null;
  return [for (final c in costs) '${texts.badge(c.mode)}: ${texts.cost(c)}']
      .join('; ');
}

/// The rate-limit state of [row] with its reset time; "Unknown" without a state.
String backendRateLimit(VizTexts texts, BackendRowModel row, String timeZone) {
  final l10n = texts.l10n;
  final state = switch (row.rateLimit) {
    api.AgentRateLimitStateStateEnum.ok => l10n.vizRateLimitOk,
    api.AgentRateLimitStateStateEnum.nearLimit => l10n.vizRateLimitNearLimit,
    api.AgentRateLimitStateStateEnum.limited => l10n.vizRateLimitLimited,
    _ => l10n.vizRateLimitUnknown,
  };
  final resets = row.rateLimitResetsAt;
  if (resets == null) return state;
  return '$state, ${l10n.vizRateLimitResets(vizClockTime(texts, resets, timeZone))}';
}

/// The icon of the rate-limit state of [row].
DsGlyph backendRateLimitGlyph(BackendRowModel row) => switch (row.rateLimit) {
  api.AgentRateLimitStateStateEnum.ok => DsIcons.triggerDone,
  api.AgentRateLimitStateStateEnum.nearLimit => DsIcons.stateWarning,
  api.AgentRateLimitStateStateEnum.limited => DsIcons.stateError,
  _ => DsIcons.stateInfo,
};
