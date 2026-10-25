// SPDX-License-Identifier: Apache-2.0

/// The emergency actions of the administration on the phone (docs/spec/01-product.md 3.7, 4,
/// 5.3 M13; R-PR-07; brief M5-09 Design notes 1 and 3): the entry under Me and the two pages
/// of the stack below it, `/admin/emergency` (the people) and `/admin/emergency/{user_id}`
/// (one person). The views are those of `admin_ui` over the administration's data source.
library;

import 'package:admin_ui/admin_ui.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';
import 'package:local_store/local_store.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../admin/admin_route.dart' show adminRepositoryProvider;
import '../../app/providers/session.dart';
import '../../notifications_settings/providers.dart' show isOwnerProvider;
import '../../settings/settings_providers.dart' show personTimeZoneProvider;

part 'emergency_entry.g.dart';

/// The path of the emergency area.
const String emergencyPath = '/admin/emergency';

/// The path of the emergency view of the person [userId].
String emergencyPersonPath(String userId) =>
    '$emergencyPath/${Uri.encodeComponent(userId)}';

/// Who the emergency area is for, from the server's data the app already holds.
enum EmergencyRole {
  /// Neither an owner nor a manager of a space: no entry.
  none,

  /// A manager of at least one space: the grants of people within the scope the server
  /// returns, and stopping runs where `run.cancel` allows it.
  manager,

  /// An owner: all four actions.
  owner,
}

/// The [EmergencyRole] of [owner] (`getMe`, `is_owner`) and the caller's reported access to
/// the stored spaces [spaceAccess] (the `SpaceView` of the snapshot and of `access.changed`).
///
/// No derivation from grants (15 1 principle 1): the client reads only the owner flag and the
/// capability `manage` the server reported for a space; what the caller may do in the area is
/// the server's answer to each list and action.
///
/// Example: `emergencyRoleOf(owner: false, spaceAccess: [view])` is
/// [EmergencyRole.manager] when `view` holds `manage`.
EmergencyRole emergencyRoleOf({
  required bool owner,
  required Iterable<AccessView?> spaceAccess,
}) {
  if (owner) return EmergencyRole.owner;
  final manages = spaceAccess.any(
    (a) => a != null && a.capabilities.contains(Capability.manage),
  );
  return manages ? EmergencyRole.manager : EmergencyRole.none;
}

/// The [EmergencyRole] of the signed-in person; [EmergencyRole.none] while signed out. Follows
/// the stored spaces, so a `manage` given or taken through `access.changed` shows or hides the
/// entry.
@riverpod
Stream<EmergencyRole> emergencyRole(Ref ref) async* {
  final owner = await ref.watch(isOwnerProvider.future);
  if (owner) {
    yield EmergencyRole.owner;
    return;
  }
  final db = ref.watch(localDatabaseProvider);
  if (db == null) {
    yield EmergencyRole.none;
    return;
  }
  final structure = db.structureDao;
  yield* structure.watchSpaces().asyncMap(
    (spaces) async => emergencyRoleOf(
      owner: false,
      spaceAccess: [
        for (final s in spaces) await structure.watchSpaceAccess(s.id).first,
      ],
    ),
  );
}

/// The entry "Emergency" under Me, shown only for an owner or a manager of a space
/// ([emergencyRoleProvider]); it opens [emergencyPath].
class EmergencyEntry extends ConsumerWidget {
  /// Creates the entry.
  const EmergencyEntry({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final role = ref.watch(emergencyRoleProvider).value ?? EmergencyRole.none;
    if (role == EmergencyRole.none) return const SizedBox.shrink();
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return ListTile(
      key: const ValueKey('phone.me.emergency'),
      leading: DsIcon(
        DsIcons.stateError,
        size: DsIconSizeRole.md,
        color: colors.dangerFg,
        decorative: true,
      ),
      title: Text(l.emergencyEntry),
      subtitle: Text(l.emergencyEntryHint),
      trailing: DsIcon(
        DsIcons.actionExpand,
        size: DsIconSizeRole.sm,
        color: colors.textSecondary,
        decorative: true,
      ),
      onTap: () => context.go(emergencyPath),
    );
  }
}

/// The page `/admin/emergency`: the [EmergencyHome] of `admin_ui`; "Not found or no access"
/// ([notFound]) without a login or for a person the entry is not for.
class PhoneEmergencyPage extends ConsumerWidget {
  /// Creates the page; [notFound] is the page of a target that is not there.
  const PhoneEmergencyPage({super.key, required this.notFound});

  /// "Not found or no access" (01 6.7).
  final Widget notFound;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final repository = ref.watch(adminRepositoryProvider);
    final role = ref.watch(emergencyRoleProvider);
    if (role.value == null && role.isLoading) return const SizedBox.expand();
    if (repository == null || role.value == EmergencyRole.none) {
      return notFound;
    }
    return EmergencyHome(
      repository: repository,
      onOpen: (id) => context.go(emergencyPersonPath(id)),
    );
  }
}

/// The page `/admin/emergency/{user_id}`: the [PersonEmergencyView] of [userId], with the
/// devices and "Disable person" for an owner only (brief M5-09 Design note 3).
class PhoneEmergencyPersonPage extends ConsumerWidget {
  /// Creates the page.
  const PhoneEmergencyPersonPage({
    super.key,
    required this.userId,
    required this.notFound,
  });

  /// The person.
  final String userId;

  /// "Not found or no access" (01 6.7).
  final Widget notFound;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final repository = ref.watch(adminRepositoryProvider);
    final role = ref.watch(emergencyRoleProvider);
    if (role.value == null && role.isLoading) return const SizedBox.expand();
    if (repository == null || role.value == EmergencyRole.none) {
      return notFound;
    }
    return PersonEmergencyView(
      key: ValueKey('emergency.view.$userId'),
      repository: repository,
      userId: userId,
      owner: role.value == EmergencyRole.owner,
      timeZone: ref.watch(personTimeZoneProvider),
    );
  }
}
