// SPDX-License-Identifier: Apache-2.0

/// Display names of the SLOs through `l10n` (docs/spec/25-visualisierung.md 3.3).
library;

import 'package:l10n/l10n.dart';

import 'slo_catalog.g.dart';

export 'slo_catalog.g.dart';

/// The catalog entry of the `sli.*` metric [metric], or `null` for a metric without an SLO.
SloCatalogEntry? sloEntry(String metric) {
  for (final entry in sloCatalog) {
    if (entry.metric == metric) return entry;
  }
  return null;
}

/// The display name of the SLO of [metric] in [l10n], or `null` for a metric without an SLO.
///
/// The generated catalog names the key; a key without a text here is a gap between the
/// generator and `l10n`, which `slo_catalog_test.dart` catches, and also returns `null`.
String? sloDisplayName(AppLocalizations l10n, String metric) =>
    switch (sloEntry(metric)?.l10nKey) {
      'vizSliApiAvailability' => l10n.vizSliApiAvailability,
      'vizSliDeliveryLatency' => l10n.vizSliDeliveryLatency,
      'vizSliReconnect' => l10n.vizSliReconnect,
      'vizSliPushAcceptance' => l10n.vizSliPushAcceptance,
      'vizSliRunStart' => l10n.vizSliRunStart,
      'vizSliTurnDuration' => l10n.vizSliTurnDuration,
      _ => null,
    };
