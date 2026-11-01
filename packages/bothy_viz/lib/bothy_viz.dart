// SPDX-License-Identifier: Apache-2.0

/// Visualizations of the statistics: view models, ports, basic forms, fixtures and harness
/// support (docs/spec/25-visualization.md 7).
///
/// * View models ([LiveTopologyModel], [BackendModel], [DeliveryModel], [ExecutiveModel],
///   [TimelineModel], [RunCardModel], [LaneModel], [SeriesModel], [TrafficLightModel]) are
///   pure classes derived from the generated `api_client` types; they never call the network.
/// * Ports: [LiveFrameSource] (live frames, stale and error events) and [StatsQuerySource]
///   (the REST queries), with the fixture implementations [FixtureLiveFrameSource] and
///   [FixtureStatsQuerySource] on a [VizClock].
/// * Basic forms: [LaneMark], [RunCardMark], [AreaMark], [BarMark], [TrafficLightMark] and
///   [MetricTileMark], each a `CustomPainter` with semantics, colored only by the tokens.
/// * [VizFixtures] decodes the files of `fixtures/`; [sloCatalog] lists the SLOs.
/// * The six views ([LiveTopologyView], [BackendView], [DeliveryView], [ExecutiveView],
///   [TimelineView], [TvMode]) with their table views, the registry [vizViewRegistry] of
///   [VizViewKind] and the [VizHostOptions] the hosts pass; [TimelinePlayback] plays the
///   timeline on a [VizClock]. The views know no transport: the hosts plug in the sources.
///
/// The package depends on `design_system`, `l10n`, `domain` and `api_client` only, never on
/// `sync_engine`, `local_store`, a WebView or an iframe (15 3.1 rule 6).
library;

export 'src/fixtures/viz_fixtures.dart';
export 'src/marks/area_mark.dart';
export 'src/marks/bar_mark.dart';
export 'src/marks/lane_mark.dart';
export 'src/marks/metric_tile_mark.dart';
export 'src/marks/run_card_mark.dart';
export 'src/marks/task_tree_mark.dart';
export 'src/marks/traffic_light_mark.dart';
export 'src/marks/viz_colors.dart';
export 'src/marks/viz_texts.dart';
export 'src/model/backend_model.dart';
export 'src/model/bar_model.dart';
export 'src/model/costs.dart';
export 'src/model/delivery_model.dart';
export 'src/model/executive_model.dart';
export 'src/model/live_topology_model.dart';
export 'src/model/metric_tile_model.dart';
export 'src/model/names.dart';
export 'src/model/run_card_model.dart';
export 'src/model/series_model.dart';
export 'src/model/task_tree_model.dart';
export 'src/model/timeline_model.dart';
export 'src/model/traffic_light_model.dart';
export 'src/run_graph/run_card.dart';
export 'src/run_graph/run_graph_announcer.dart';
export 'src/run_graph/run_graph_model.dart';
export 'src/run_graph/run_graph_texts.dart';
export 'src/run_graph/run_graph_view.dart';
export 'src/slo/slo_catalog.dart';
export 'src/sources/fixture_sources.dart';
export 'src/sources/live_frame_source.dart';
export 'src/sources/stats_query_source.dart';
export 'src/sources/viz_clock.dart';
export 'src/views/backend_view.dart';
export 'src/views/delivery_view.dart';
export 'src/views/executive_view.dart';
export 'src/views/live_topology_view.dart';
export 'src/views/table/viz_table.dart';
export 'src/views/timeline/timeline_playback.dart';
export 'src/views/timeline_view.dart';
export 'src/views/tv_mode.dart';
export 'src/views/view_scaffold.dart' show VizTvScope;
export 'src/views/viz_host_options.dart';
export 'src/views/viz_view_kind.dart';
