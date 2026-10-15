// SPDX-License-Identifier: Apache-2.0

/// Color roles of the marks: only `color.viz.*`, the run state roles and the state roles
/// (docs/spec/25-visualisierung.md 10; R-VZ-10).
library;

import 'package:api_client/api_client.dart' as api;
import 'package:design_system/design_system.dart';
import 'package:flutter/painting.dart';

import '../model/run_card_model.dart';
import '../model/series_model.dart';
import '../model/traffic_light_model.dart';

/// The series role `color.viz.series.<index % 8 + 1>`: the fixed order with the wrap that the
/// color vision check of M6-10 holds (1 with 2 up to 8 with 1).
Color vizSeriesColor(DsColors colors, int index) =>
    switch (index % vizSeriesCount) {
      0 => colors.vizSeries1,
      1 => colors.vizSeries2,
      2 => colors.vizSeries3,
      3 => colors.vizSeries4,
      4 => colors.vizSeries5,
      5 => colors.vizSeries6,
      6 => colors.vizSeries7,
      _ => colors.vizSeries8,
    };

/// The run state role of an active run (components.md 2).
Color runCardStateColor(DsColors colors, RunCardState state) => switch (state) {
  RunCardState.queued => colors.runQueued,
  RunCardState.running => colors.runRunning,
  RunCardState.waitingInput => colors.runWaiting,
  RunCardState.unknown => colors.borderDefault,
};

/// The icon of an active run state; the state is never color alone (25 7.4).
DsGlyph runCardStateGlyph(RunCardState state) => switch (state) {
  RunCardState.queued => DsIcons.runQueued,
  RunCardState.running => DsIcons.runRunning,
  RunCardState.waitingInput => DsIcons.runWaiting,
  RunCardState.unknown => DsIcons.stateInfo,
};

/// The run state role of a run of the timeline.
Color runStateColor(DsColors colors, api.RunState state) =>
    <api.RunState, Color>{
      api.RunState.queued: colors.runQueued,
      api.RunState.running: colors.runRunning,
      api.RunState.waitingInput: colors.runWaiting,
      api.RunState.completed: colors.runCompleted,
      api.RunState.failed: colors.runFailed,
      api.RunState.cancelled: colors.runCancelled,
    }[state] ??
    colors.borderDefault;

/// Foreground and subtle background of a state role.
({Color fg, Color subtle, Color border}) stateRoleColors(
  DsColors colors,
  VizStateRole role,
) => switch (role) {
  VizStateRole.success => (
    fg: colors.successFg,
    subtle: colors.successSubtle,
    border: colors.successBorder,
  ),
  VizStateRole.warning => (
    fg: colors.warningFg,
    subtle: colors.warningSubtle,
    border: colors.warningBorder,
  ),
  VizStateRole.danger => (
    fg: colors.dangerFg,
    subtle: colors.dangerSubtle,
    border: colors.dangerBorder,
  ),
  VizStateRole.neutral => (
    fg: colors.textSecondary,
    subtle: colors.surfaceSunken,
    border: colors.borderDefault,
  ),
};
