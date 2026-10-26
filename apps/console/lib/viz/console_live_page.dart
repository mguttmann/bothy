// SPDX-License-Identifier: Apache-2.0

/// The page "Live" of the console (docs/spec/01-product.md 5.5; docs/spec/25-visualization.md
/// 5.1, 6, 9.1; brief M6-16 Design note 4): the views of D12 with TV mode D13 over the console's
/// REST client.
library;

import 'dart:async';

import 'package:admin_ui/admin_ui.dart';
import 'package:bothy_viz/bothy_viz.dart';
import 'package:domain/domain.dart' show AppConfig, AppConfigValues;
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../admin/console_admin.dart';
import '../platform/browser.dart';
import '../session/console_controller.dart';
import 'polling_live_source.dart';

/// The [LiveScreen] of `admin_ui` with the console's transport: the live frame polled from
/// `getStatsLive` ([PollingLiveFrameSource], stopped while the tab is hidden), the queries
/// over the cookie client ([ApiStatsQuerySource]), the names and the telemetry link of the
/// administration's repository, and "Open in Grafana" in a new tab ([ExternalLinks], never an
/// iframe). The console has no chat (ADR 0028): a run card shows "Open in the app" with the
/// thread ID instead of opening it.
class ConsoleLivePage extends StatefulWidget {
  /// Creates the page.
  const ConsoleLivePage({
    super.key,
    required this.controller,
    required this.admin,
    this.links,
    this.visibility,
    this.config,
    this.clock = const SystemVizClock(),
  });

  /// The console's session; its REST client carries the cookie and the CSRF header.
  final ConsoleController controller;

  /// The administration state: the repository (names, telemetry link) and the time zone.
  final ConsoleAdmin admin;

  /// Opens a link in a new tab, or null (no "Open in Grafana").
  final ExternalLinks? links;

  /// The visibility of the tab, or null (always visible).
  final PageVisibility? visibility;

  /// The `viz.*` values; null takes the defaults.
  final AppConfig? config;

  /// The clock of polling, staleness and the TV rotation.
  final VizClock clock;

  @override
  State<ConsoleLivePage> createState() => _ConsoleLivePageState();
}

class _ConsoleLivePageState extends State<ConsoleLivePage> {
  late final AppConfig _config = widget.config ?? AppConfig.defaults();
  late final ApiStatsQuerySource _queries = ApiStatsQuerySource(
    widget.controller.api,
  );
  late final PollingLiveFrameSource _live = PollingLiveFrameSource(
    queries: _queries,
    clock: widget.clock,
    pollInterval: _config.livePollInterval,
    staleAfter: _config.staleAfter,
    visibility: widget.visibility,
  );

  @override
  void dispose() {
    unawaited(_live.stop());
    super.dispose();
  }

  void _openThread(String threadId) {
    final l = AppLocalizations.of(context);
    ScaffoldMessenger.maybeOf(context)?.showSnackBar(
      SnackBar(
        key: const ValueKey('console.live.threadInApp'),
        content: Text(l.consoleThreadInApp(threadId)),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final links = widget.links;
    return LiveScreen(
      key: const ValueKey('console.live'),
      formFactor: VizFormFactor.web,
      live: _live,
      queries: _queries,
      directory: widget.admin.repository,
      onOpenThread: _openThread,
      openExternal: links?.open,
      config: _config,
      timeZone: widget.admin.timeZone,
      clock: widget.clock,
    );
  }
}
