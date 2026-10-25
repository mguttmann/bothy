// SPDX-License-Identifier: Apache-2.0

/// The administration of `admin_ui` at `/admin` of the desktop app (docs/spec/01-product.md
/// 5.2; 15 11; brief M4-26 Scope).
library;

import 'dart:async';

import 'package:admin_ui/admin_ui.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart'
    show AgentPresence, AgentRuntimeState, BillingMode, PrincipalId;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:sync_engine/sync_engine.dart' show EphemeralStore;

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../settings/settings_providers.dart' show personTimeZoneProvider;
import 'manifest_file.dart';

part 'admin_route.g.dart';

/// The location of the path [segments] (`['admin', ...]`), or null for a path that names no
/// screen of this build.
///
/// Example: `adminLocationOf(['admin', 'people', id])` is that person's detail.
AdminLocation? adminLocationOf(List<String> segments) => AdminLocation.parse(
  segments.isNotEmpty && segments.first == 'admin'
      ? segments.sublist(1)
      : segments,
);

/// The app path of [location], for example `/admin/groups/<id>`.
String adminPathOf(AdminLocation location) => '/admin${location.path}';

/// The live presence of [agentId] for the agent page of A5, from the engine's [store]
/// (`agent.presence_changed`, 04 12 item 2; brief M4-28 Design note 1). A value the store does
/// not have (an agent this device sees nowhere) is skipped, so the page keeps the presence of
/// `getAgent`; an ID that is no UUID gives an empty stream.
///
/// Example: `agentPresenceOf(engine.ephemeral, agentId).listen(show)`.
Stream<AgentLiveState> agentPresenceOf(EphemeralStore store, String agentId) {
  final PrincipalId id;
  try {
    id = PrincipalId.parse(agentId);
  } on FormatException {
    return const Stream.empty();
  }
  return store
      .watchPresence(id)
      .where((p) => p != null)
      .map((p) => agentLiveStateOf(p!));
}

/// The [AgentLiveState] of the engine's [presence].
AgentLiveState agentLiveStateOf(AgentPresence presence) => AgentLiveState(
  state: switch (presence.state) {
    AgentRuntimeState.online => AgentLiveStateKind.online,
    AgentRuntimeState.degraded => AgentLiveStateKind.degraded,
    AgentRuntimeState.offline => AgentLiveStateKind.offline,
    AgentRuntimeState.unknown => AgentLiveStateKind.unknown,
  },
  reportedMode: switch (presence.reportedBillingMode) {
    BillingMode.personal => AgentBillingMode.personal,
    BillingMode.organization => AgentBillingMode.organization,
    BillingMode.unknown => AgentBillingMode.unknown,
    null => null,
  },
);

/// The REST source of the administration over the client of the signed-in session (DPoP,
/// refresh and the step-up interceptor apply), or null before the login.
@Riverpod(keepAlive: true)
AdminRepository? adminRepository(Ref ref) {
  final services = ref.watch(signedInServicesProvider);
  if (services == null) return null;
  return ApiAdminRepository(
    services.auth.api,
    newIdempotencyKey: services.ids.next,
  );
}

/// The screen of `/admin/...`: the [AdminShell] at the location of [uri]; a path that names no
/// screen shows "Not found or no access" (01 6.7). Navigation inside the shell goes through the
/// router, so links and the back button work; "Back to the chat" goes to `/`. The manifest of an
/// app comes from the system file dialog ([manifestPicker], M6-08), the logo of the instance as
/// well ([logoPicker], M6-09), and `instance.updated` reloads the instance screen. The telemetry
/// link of the statistics A7 opens in the system browser (M6-06).
class AdminRoute extends ConsumerWidget {
  /// Creates the screen for [uri].
  const AdminRoute({super.key, required this.uri});

  /// The path the router shows.
  final Uri uri;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final repository = ref.watch(adminRepositoryProvider);
    final engine = ref.watch(syncEngineProvider);
    final db = ref.watch(localDatabaseProvider);
    final location = adminLocationOf(uri.pathSegments);
    final colors = context.dsColors;
    if (repository == null || location == null) {
      return Material(
        color: colors.surfaceCanvas,
        child: Center(
          child: Text(
            AppLocalizations.of(context).accessNotFoundOrNoAccess,
            key: const ValueKey('admin.notFound'),
            style: dsTextStyle(DsType.body)
                .copyWith(color: colors.textSecondary),
          ),
        ),
      );
    }
    return AdminShell(
      repository: repository,
      location: location,
      timeZone: ref.watch(personTimeZoneProvider),
      onNavigate: (to) => context.go(adminPathOf(to)),
      onExit: () => context.go('/'),
      agentPresence: engine == null
          ? null
          : (agentId) => agentPresenceOf(engine.ephemeral, agentId),
      pickManifest: manifestPicker(
        label: AppLocalizations.of(context).adminManifestFileType,
      ),
      pickLogo: logoPicker(
        label: AppLocalizations.of(context).adminLogoFileType,
      ),
      // `instance.updated` stores the instance again (sync engine); each change after the first
      // value reloads the instance settings and flags (brief M6-09 Design note 4).
      // The telemetry link of the statistics (M6-06) opens the system browser, never a web
      // view (25 9.1).
      openExternal: (url) =>
          unawaited(ref.read(appPortsProvider).browser.open(url)),
      instanceChanges: db == null
          ? null
          : () => db.instanceDao.watch().skip(1).map((_) {}),
    );
  }
}
