// SPDX-License-Identifier: Apache-2.0

/// The six views and the registry the hosts use for navigation and tabs (brief M6-12,
/// design note 1).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/widgets.dart';
import 'package:l10n/l10n.dart';

import '../sources/live_frame_source.dart';
import '../sources/stats_query_source.dart';
import 'backend_view.dart';
import 'delivery_view.dart';
import 'executive_view.dart';
import 'live_topology_view.dart';
import 'timeline_view.dart';
import 'viz_host_options.dart';

/// The views of docs/spec/25-visualisierung.md 6.
enum VizViewKind {
  /// Live topology (6.1).
  liveTopology,

  /// Backend view (6.2).
  backend,

  /// Delivery and sync live (6.3), instance scope only.
  delivery,

  /// Executive page (6.4).
  executive,

  /// Replayable daily timeline (6.5).
  timeline,

  /// TV mode (6.6): desktop app and console only; the host opens it as `TvMode`.
  tvMode,
}

/// What the registry says about one view.
final class VizViewSpec {
  /// Creates an entry.
  const VizViewSpec({
    required this.kind,
    required this.titleKey,
    required this.title,
    required this.glyph,
    required this.instanceOnly,
    required this.onMobile,
  });

  /// The view.
  final VizViewKind kind;

  /// Name of the `l10n` key of the title, for example `vizViewLiveTopology`.
  final String titleKey;

  /// The title in the language of [AppLocalizations].
  final String Function(AppLocalizations l10n) title;

  /// Icon of the tab or navigation entry.
  final DsGlyph glyph;

  /// The view needs the instance scope; in a space scope it shows "Only for the owner"
  /// without calling a source.
  final bool instanceOnly;

  /// The view appears on mobile (all but TV mode, 01 5.3 M10).
  final bool onMobile;
}

/// Every view in the order of the tabs.
final List<VizViewSpec> vizViewRegistry = List.unmodifiable([
  VizViewSpec(
    kind: VizViewKind.liveTopology,
    titleKey: 'vizViewLiveTopology',
    title: (l) => l.vizViewLiveTopology,
    glyph: DsIcons.agent,
    instanceOnly: false,
    onMobile: true,
  ),
  VizViewSpec(
    kind: VizViewKind.backend,
    titleKey: 'vizViewBackend',
    title: (l) => l.vizViewBackend,
    glyph: DsIcons.billingOrganization,
    instanceOnly: false,
    onMobile: true,
  ),
  VizViewSpec(
    kind: VizViewKind.delivery,
    titleKey: 'vizViewDelivery',
    title: (l) => l.vizViewDelivery,
    glyph: DsIcons.stateReconnecting,
    instanceOnly: true,
    onMobile: true,
  ),
  VizViewSpec(
    kind: VizViewKind.executive,
    titleKey: 'vizViewExecutive',
    title: (l) => l.vizViewExecutive,
    glyph: DsIcons.adminStats,
    instanceOnly: false,
    onMobile: true,
  ),
  VizViewSpec(
    kind: VizViewKind.timeline,
    titleKey: 'vizViewTimeline',
    title: (l) => l.vizViewTimeline,
    glyph: DsIcons.runQueued,
    instanceOnly: false,
    onMobile: true,
  ),
  VizViewSpec(
    kind: VizViewKind.tvMode,
    titleKey: 'vizViewTv',
    title: (l) => l.vizViewTv,
    glyph: DsIcons.adminConsole,
    instanceOnly: false,
    onMobile: false,
  ),
]);

/// The entry of [kind] in [vizViewRegistry].
VizViewSpec vizViewSpec(VizViewKind kind) =>
    vizViewRegistry.firstWhere((s) => s.kind == kind);

/// The views a host with [options] offers: never TV mode on mobile or without
/// [VizHostOptions.allowTvMode] (25 6.6).
List<VizViewSpec> vizViewsFor(VizHostOptions options) => [
  for (final spec in vizViewRegistry)
    if (spec.kind != VizViewKind.tvMode ||
        (options.allowTvMode && options.formFactor != VizFormFactor.mobile))
      if (options.formFactor != VizFormFactor.mobile || spec.onMobile) spec,
];

/// Builds the view of [kind] with the shared view contract.
///
/// Throws an [ArgumentError] for [VizViewKind.tvMode]: the host opens TV mode as an overlay
/// with its own exit callback (`TvMode`).
Widget buildVizView(
  VizViewKind kind, {
  required LiveFrameSource? live,
  required StatsQuerySource queries,
  required VizHostOptions options,
  required void Function(String threadId) onOpenThread,
  Key? key,
}) => switch (kind) {
  VizViewKind.liveTopology => LiveTopologyView(
    key: key,
    live: live,
    queries: queries,
    options: options,
    onOpenThread: onOpenThread,
  ),
  VizViewKind.backend => BackendView(
    key: key,
    live: live,
    queries: queries,
    options: options,
    onOpenThread: onOpenThread,
  ),
  VizViewKind.delivery => DeliveryView(
    key: key,
    live: live,
    queries: queries,
    options: options,
    onOpenThread: onOpenThread,
  ),
  VizViewKind.executive => ExecutiveView(
    key: key,
    live: live,
    queries: queries,
    options: options,
    onOpenThread: onOpenThread,
  ),
  VizViewKind.timeline => TimelineView(
    key: key,
    live: live,
    queries: queries,
    options: options,
    onOpenThread: onOpenThread,
  ),
  VizViewKind.tvMode => throw ArgumentError.value(
    kind,
    'kind',
    'TV mode is an overlay; build TvMode',
  ),
};
