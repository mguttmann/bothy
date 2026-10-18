// SPDX-License-Identifier: Apache-2.0

/// The administration of `admin_ui` at `/admin` of the desktop app (docs/spec/01-product.md
/// 5.2; 15 11; brief M4-26 Scope).
library;

import 'package:admin_ui/admin_ui.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../app/providers/session.dart';
import '../settings/settings_providers.dart' show personTimeZoneProvider;

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
/// router, so links and the back button work; "Back to the chat" goes to `/`.
class AdminRoute extends ConsumerWidget {
  /// Creates the screen for [uri].
  const AdminRoute({super.key, required this.uri});

  /// The path the router shows.
  final Uri uri;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final repository = ref.watch(adminRepositoryProvider);
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
    );
  }
}
