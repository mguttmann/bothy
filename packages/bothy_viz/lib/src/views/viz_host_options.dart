// SPDX-License-Identifier: Apache-2.0

/// What only the host of a view knows (docs/spec/25-visualisierung.md 6, 7.1; brief M6-12,
/// design note 1).
library;

import 'package:domain/domain.dart';

import '../model/names.dart';
import '../sources/viz_clock.dart';

/// The kind of host a view runs in.
enum VizFormFactor {
  /// The desktop app.
  desktop,

  /// The mobile app: every view but TV mode (01 5.3 M10).
  mobile,

  /// The web console.
  web,
}

/// The scope of the statistics: the whole instance or one space.
///
/// The scope is what the host asks the server for; the server decides what the person may
/// see (25 4 rule 5). The views only use it to pick the source parameters and to show the
/// instance-only views as "Only for the owner" in a space scope (design note 1).
final class VizScope {
  /// The whole instance (`stats.read` on `instance`).
  const VizScope.instance() : spaceId = null;

  /// The space [spaceId] (`stats.read` on the space).
  const VizScope.space(String this.spaceId);

  /// The space of a space scope; `null` for the instance.
  final String? spaceId;

  /// Whether this is the instance scope.
  bool get isInstance => spaceId == null;
}

/// The period of the REST queries, chosen by the host (for example the selected day).
final class VizRange {
  /// Creates the period from [from] to [to].
  const VizRange({required this.from, required this.to});

  /// Start of the period.
  final DateTime from;

  /// End of the period.
  final DateTime to;
}

/// What only the host knows, passed to every view (design note 1).
///
/// ```dart
/// final options = VizHostOptions(
///   formFactor: VizFormFactor.desktop,
///   scope: const VizScope.instance(),
///   range: VizRange(from: dayStart, to: dayEnd),
///   allowTvMode: true,
/// );
/// ```
final class VizHostOptions {
  /// Creates the options.
  ///
  /// [allowTvMode] is `false` on mobile (25 6.6). [showCostsInTv] defaults to
  /// `viz.tv_show_costs` of [config]. [onOpenGrafana] is passed only when
  /// `telemetry.dashboard_url` is set (25 9.1); the package never knows the URL. [timeZone] is
  /// the IANA zone of the person (`users.time_zone`) for clock times. [clock] drives the
  /// timeline playback and the TV rotation; tests pass a [FakeVizClock].
  VizHostOptions({
    required this.formFactor,
    required this.scope,
    required this.range,
    this.allowTvMode = false,
    bool? showCostsInTv,
    this.onOpenGrafana,
    this.names = const VizNames(),
    this.timeZone = 'UTC',
    this.clock = const SystemVizClock(),
    AppConfig? config,
  }) : config = config ?? AppConfig.defaults(),
       showCostsInTv =
           showCostsInTv ?? (config ?? AppConfig.defaults()).tvShowCosts;

  /// Desktop, mobile or web.
  final VizFormFactor formFactor;

  /// Instance or space.
  final VizScope scope;

  /// Period of the REST queries.
  final VizRange range;

  /// The host offers TV mode (desktop app and console, never mobile).
  final bool allowTvMode;

  /// TV mode shows costs (`viz.tv_show_costs`).
  final bool showCostsInTv;

  /// Opens the dashboard of `telemetry.dashboard_url` in the system browser; `null` hides the
  /// link "Open in Grafana".
  final void Function()? onOpenGrafana;

  /// Display names of agents, spaces and channels.
  final VizNames names;

  /// IANA time zone of the clock times.
  final String timeZone;

  /// Clock of the playback and the rotation.
  final VizClock clock;

  /// The app configuration (`viz.*`).
  final AppConfig config;
}
