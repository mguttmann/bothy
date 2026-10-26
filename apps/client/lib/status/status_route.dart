// SPDX-License-Identifier: Apache-2.0

/// The status page of the apps at `/status` (docs/spec/16-operations.md 12; brief M7-21 Scope):
/// the [StatusScreen] of `admin_ui` over `getStatus`, the event `status.updated` of the sync
/// engine and the engine's connection state, on the desktop as its own screen and on the
/// phone as a page of the Me tab. The owner forms live in the administration
/// (`/admin/status`), which hands the same source to the screen.
library;

import 'dart:async';

import 'package:admin_ui/admin_ui.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart' hide ConnectionState;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:sync_engine/sync_engine.dart';

import '../admin/admin_route.dart' show adminRepositoryProvider;
import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../settings/settings_providers.dart' show personTimeZoneProvider;

part 'status_route.g.dart';

/// The path of the status page.
const String statusPath = '/status';

/// Whether the connection state [state] reports the server unreachable (true), reachable
/// (false), or says nothing about it (null: a connection attempt in progress, the app in the
/// background, a signed-out session or an app that needs an update keep the last answer).
///
/// The server is unreachable while the engine has no connection and waits (`offline`,
/// `backoff`); a live connection, its catch-up and a resync reach it (brief M7-21 requirement
/// 14; docs/decisions.d/M7-21.md D-M7-21-2).
///
/// Example: `serverUnreachableOf(const ConnectionState(ConnectionPhase.backoff))` is true.
bool? serverUnreachableOf(ConnectionState state) => switch (state.phase) {
  ConnectionPhase.offline || ConnectionPhase.backoff => true,
  ConnectionPhase.live ||
  ConnectionPhase.catchingUp ||
  ConnectionPhase.resyncing => false,
  _ => null,
};

/// The [StatusSource] of the signed-in app: `getStatus` through the session's client, the
/// engine's `status.updated` as the changes and its connection state as the reachability; null
/// while signed out.
@Riverpod(keepAlive: true)
StatusSource? appStatusSource(Ref ref) {
  final repository = ref.watch(adminRepositoryProvider);
  final engine = ref.watch(syncEngineProvider);
  if (repository == null || engine == null) return null;
  return LiveStatusSource(
    loader: repository.status,
    changes: engine.statusUpdates,
    // Each page gets the current state first, then every change.
    unreachable: Stream<ConnectionState>.multi((c) {
      c.add(engine.state);
      final sub = engine.states.listen(c.add, onDone: c.close);
      c.onCancel = sub.cancel;
    }).map(serverUnreachableOf).where((u) => u != null).cast<bool>(),
  );
}

/// The status page over the providers: "Not found or no access" ([notFound]) while signed out.
/// Active links of owner texts open in the system browser.
class AppStatusPage extends ConsumerWidget {
  /// Creates the page.
  const AppStatusPage({super.key, required this.notFound});

  /// "Not found or no access" (01 6.7).
  final Widget notFound;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final source = ref.watch(appStatusSourceProvider);
    if (source == null) return notFound;
    return SingleChildScrollView(
      padding: const EdgeInsets.all(DsSpace.s16),
      child: StatusScreen(
        key: ObjectKey(source),
        source: source,
        timeZone: ref.watch(personTimeZoneProvider),
        onOpenLink: (url) =>
            unawaited(ref.read(appPortsProvider).browser.open(url)),
      ),
    );
  }
}

/// The desktop screen of `/status`: "Back to the chat" and the [AppStatusPage].
class DesktopStatusRoute extends StatelessWidget {
  /// Creates the screen.
  const DesktopStatusRoute({super.key});

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final notFound = Center(
      child: Text(
        l.accessNotFoundOrNoAccess,
        style: dsTextStyle(DsType.body).copyWith(color: colors.textSecondary),
      ),
    );
    return Material(
      key: const ValueKey('screen.status'),
      color: colors.surfaceCanvas,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Padding(
            padding: const EdgeInsets.all(DsSpace.s8),
            child: Align(
              alignment: Alignment.centerLeft,
              child: TextButton(
                key: const ValueKey('status.back'),
                onPressed: () => context.go('/'),
                child: Text(l.adminBack),
              ),
            ),
          ),
          Expanded(
            child: Align(
              alignment: Alignment.topLeft,
              child: ConstrainedBox(
                constraints: const BoxConstraints(
                  maxWidth: DsLayout.messageMaxWidth,
                ),
                child: AppStatusPage(notFound: notFound),
              ),
            ),
          ),
        ],
      ),
    );
  }
}
