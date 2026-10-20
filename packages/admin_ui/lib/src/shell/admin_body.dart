// SPDX-License-Identifier: Apache-2.0

/// The screen of one place of the administration without the navigation (docs/spec/01-product.md
/// 5.2, 5.5; brief M6-04 Design note 1). `AdminShell` puts it next to its navigation on the
/// desktop; the console hosts it inside its own shell at the full browser width.
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';

import '../access/who_has_access.dart' show AdminViewAs;
import '../agents/agent_page.dart';
import '../agents/agents_list.dart';
import '../audit/audit_export.dart';
import '../audit/audit_list.dart';
import '../data/admin_repository.dart';
import '../data/agents_repository.dart';
import '../devices/waiting_devices.dart';
import '../grants/grants_list.dart';
import '../groups/group_detail.dart';
import '../groups/groups_list.dart';
import '../hosts/host_page.dart';
import '../hosts/hosts_list.dart';
import '../people/people_list.dart';
import '../people/person_detail.dart';
import 'admin_routes.dart';

/// The screen of [location] with the padding of the administration: the audit log gets the
/// height of the body, because its list is virtualized and scrolls itself; every other screen
/// scrolls in the body, at most [maxWidth] wide.
///
/// Parameters: [repository] serves every screen; [onNavigate] receives the place a screen opens
/// (a detail, its list, another target); [onChanged] runs after every change a screen makes,
/// the shell counts the waiting devices again with it; [timeZone] and [clock] format times;
/// [saveTarget] is the host's save dialog of the audit export (null: no export buttons);
/// [agentPresence] streams the live presence of an agent (null: the presence of `getAgent`);
/// [maxWidth] limits the content (880 on the desktop; null fills the width, for the browser);
/// [onViewAs] starts "View as person" in the host from [PersonDetail] and from "Who has
/// access?" of [GrantsList] (null: the dialog of the desktop).
///
/// The body holds no permission logic: a screen the server refuses shows the server's reason
/// (15 1 principle 1). A section without a screen shows the people list, as the shell does.
///
/// Example:
/// ```dart
/// AdminBody(
///   repository: admin,
///   location: AdminLocation.parse(segments) ?? AdminLocation.start,
///   onNavigate: (to) => context.go(to.path),
///   maxWidth: null,
///   onViewAs: (person) => viewAs.start(person),
/// );
/// ```
class AdminBody extends StatelessWidget {
  /// Creates the body.
  const AdminBody({
    super.key,
    required this.repository,
    required this.location,
    required this.onNavigate,
    this.onChanged,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
    this.saveTarget,
    this.agentPresence,
    this.maxWidth = 880,
    this.onViewAs,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The open place.
  final AdminLocation location;

  /// Opens another place.
  final ValueChanged<AdminLocation> onNavigate;

  /// Runs after every change of a screen, or null.
  final VoidCallback? onChanged;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// The host's save dialog for the audit export, or null for no export (brief M4-29 Design
  /// note 3).
  final AuditSaveTarget? saveTarget;

  /// The live presence of an agent for its page, or null to show the presence of `getAgent`
  /// (brief M4-28 Design note 1).
  final Stream<AgentLiveState> Function(String agentId)? agentPresence;

  /// The widest the content of a scrolling screen gets, or null for the full width.
  final double? maxWidth;

  /// Starts "View as person" in the host, or null for the dialog of the desktop.
  final AdminViewAs? onViewAs;

  Widget _screen() {
    final r = repository;
    final at = location;
    final id = at.id;
    void changed() => onChanged?.call();
    return switch (at.section) {
      AdminSection.people when id != null => PersonDetail(
        key: ValueKey('admin.person.$id'),
        repository: r,
        userId: id,
        timeZone: timeZone,
        clock: clock,
        onBack: () => onNavigate(AdminLocation.start),
        onChanged: changed,
        onViewAs: onViewAs,
      ),
      AdminSection.groups when id != null => GroupDetail(
        key: ValueKey('admin.group.$id'),
        repository: r,
        groupId: id,
        timeZone: timeZone,
        clock: clock,
        onBack: () => onNavigate(const AdminLocation(AdminSection.groups)),
        onChanged: changed,
      ),
      AdminSection.groups => GroupsList(
        repository: r,
        onOpen: (g) => onNavigate(AdminLocation(AdminSection.groups, id: g)),
        onChanged: changed,
      ),
      AdminSection.grants => GrantsList(
        key: ValueKey('admin.grants.$id'),
        repository: r,
        targetKey: id,
        timeZone: timeZone,
        clock: clock,
        onTarget: (key) =>
            onNavigate(AdminLocation(AdminSection.grants, id: key)),
        onChanged: changed,
        onViewAs: onViewAs,
      ),
      AdminSection.agents when id != null => AgentPage(
        key: ValueKey('admin.agent.$id'),
        repository: r,
        agentId: id,
        presence: agentPresence?.call(id),
        timeZone: timeZone,
        clock: clock,
        onBack: () => onNavigate(const AdminLocation(AdminSection.agents)),
        onChanged: changed,
      ),
      AdminSection.agents => AgentsList(
        repository: r,
        onOpen: (a) => onNavigate(AdminLocation(AdminSection.agents, id: a)),
        onChanged: changed,
      ),
      AdminSection.audit => AuditLogView(
        repository: r,
        timeZone: timeZone,
        clock: clock,
        saveTarget: saveTarget,
      ),
      AdminSection.hosts when id != null => HostDetail(
        key: ValueKey('admin.host.$id'),
        repository: r,
        hostId: id,
        timeZone: timeZone,
        clock: clock,
        onBack: () => onNavigate(const AdminLocation(AdminSection.hosts)),
        onChanged: changed,
      ),
      AdminSection.hosts => HostsList(
        repository: r,
        clock: clock,
        onOpen: (h) => onNavigate(AdminLocation(AdminSection.hosts, id: h)),
        onChanged: changed,
      ),
      AdminSection.devices => WaitingDevicesView(
        repository: r,
        timeZone: timeZone,
        clock: clock,
        onChanged: changed,
      ),
      _ => PeopleList(
        repository: r,
        timeZone: timeZone,
        onOpen: (p) => onNavigate(AdminLocation(AdminSection.people, id: p)),
        onChanged: changed,
      ),
    };
  }

  @override
  Widget build(BuildContext context) {
    final screen = KeyedSubtree(
      key: ValueKey('admin.body.${location.path}'),
      child: _screen(),
    );
    if (location.section == AdminSection.audit) {
      // The audit list scrolls itself (virtualized), so it gets the body's height.
      return Padding(padding: const EdgeInsets.all(DsSpace.s24), child: screen);
    }
    final width = maxWidth;
    return SingleChildScrollView(
      padding: const EdgeInsets.all(DsSpace.s24),
      // Without a maximum the scroll view's tight width reaches the screen, so it fills the
      // browser; with one the screen keeps to the left at most that wide.
      child: width == null
          ? screen
          : Align(
              alignment: Alignment.topLeft,
              child: ConstrainedBox(
                constraints: BoxConstraints(maxWidth: width),
                child: screen,
              ),
            ),
    );
  }
}
