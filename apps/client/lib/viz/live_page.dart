// SPDX-License-Identifier: Apache-2.0

/// The page "Live" of the apps: D12 with TV mode D13 on the desktop, M10 under Me on the phone
/// (docs/spec/01-product.md 5.1, 5.3; docs/spec/25-visualization.md 6, 7.1, 9.1; brief M6-16
/// Design notes 2 and 3).
library;

import 'dart:async';

import 'package:admin_ui/admin_ui.dart';
import 'package:bothy_viz/bothy_viz.dart';
import 'package:domain/domain.dart' show AppConfigValues;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:sync_engine/sync_engine.dart' show SyncEngine;

import '../app/bootstrap.dart' show InstanceServices;
import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../settings/settings_providers.dart' show personTimeZoneProvider;
import 'engine_live_source.dart';

/// The path of the page.
const String livePath = '/live';

/// The [LiveScreen] of `admin_ui` with the app's transport: the live stream of the sync
/// engine ([EngineLiveFrameSource]), the REST queries over the client of the signed-in
/// session ([ApiStatsQuerySource], DPoP and refresh apply), the spaces, agent names and
/// telemetry link of the same client, the system browser for "Open in Grafana" (never a web
/// view, 25 9.1) and the deep link `/t/{thread_id}` for a run card.
///
/// On the phone ([phone]) the views use the form factor `mobile`: no TV mode (25 6.6); the
/// density `touch` comes from the app's theme on a touch platform. Signed out the page is
/// empty.
class LivePage extends ConsumerStatefulWidget {
  /// Creates the page.
  const LivePage({super.key, this.phone = false});

  /// Whether the page is M10 of the phone.
  final bool phone;

  @override
  ConsumerState<LivePage> createState() => _LivePageState();
}

class _LivePageState extends ConsumerState<LivePage> {
  EngineLiveFrameSource? _live;
  SyncEngine? _engine;

  /// One source per engine: a new sign-in builds a new engine and so a new source.
  EngineLiveFrameSource? _sourceFor(SyncEngine? engine) {
    if (identical(engine, _engine)) return _live;
    final old = _live;
    if (old != null) unawaited(old.stop());
    _engine = engine;
    _live = engine != null
        ? EngineLiveFrameSource(
            engine.stats,
            clock: const SystemVizClock(),
            staleAfter: ref.read(appConfigProvider).staleAfter,
          )
        : null;
    return _live;
  }

  Object? _services;
  late ApiStatsQuerySource _queries;
  late ApiAdminStatsSource _directory;

  /// The REST sources of the session; rebuilt only with a new session.
  void _restFor(InstanceServices services) {
    if (identical(services, _services)) return;
    _services = services;
    _queries = ApiStatsQuerySource(services.auth.api);
    _directory = ApiAdminStatsSource(services.auth.api);
  }

  @override
  void dispose() {
    final live = _live;
    if (live != null) unawaited(live.stop());
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final services = ref.watch(signedInServicesProvider);
    final engine = ref.watch(syncEngineProvider);
    if (services == null) return const SizedBox.expand();
    _restFor(services);
    return LiveScreen(
      key: ValueKey(services),
      formFactor: widget.phone ? VizFormFactor.mobile : VizFormFactor.desktop,
      live: _sourceFor(engine),
      queries: _queries,
      directory: _directory,
      onOpenThread: (threadId) => context.go('/t/$threadId'),
      openExternal: (url) =>
          unawaited(ref.read(appPortsProvider).browser.open(url)),
      config: ref.watch(appConfigProvider),
      timeZone: ref.watch(personTimeZoneProvider),
    );
  }
}
