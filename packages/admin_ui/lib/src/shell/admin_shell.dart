// SPDX-License-Identifier: Apache-2.0

/// The frame of the administration: the navigation of A1 on the left and the open screen on the
/// right (docs/spec/01-product.md 5.2 A1; brief M4-26 Scope). The desktop app hosts it at
/// `/admin`, the console at its root (M6-04).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../audit/audit_export.dart';
import '../audit/audit_list.dart';
import '../data/admin_repository.dart';
import '../devices/waiting_devices.dart';
import '../grants/grants_list.dart';
import '../groups/group_detail.dart';
import '../groups/groups_list.dart';
import '../people/people_list.dart';
import '../people/person_detail.dart';
import 'admin_routes.dart';

/// The name of the entry [section] in the navigation.
String adminSectionLabel(AppLocalizations l, AdminSection section) =>
    switch (section) {
      AdminSection.people => l.adminNavPeople,
      AdminSection.groups => l.adminNavGroups,
      AdminSection.grants => l.adminNavGrants,
      AdminSection.devices => l.adminNavDevices,
      AdminSection.audit => l.adminNavAudit,
      // Entries without a screen are not listed (AdminSection.listed).
      _ => section.segment,
    };

/// The administration at [location]: the entries of [AdminSection.listed] (an entry without a
/// screen is not listed) with the count of waiting devices, and the screen of the location.
///
/// The host maps locations to its paths: [onNavigate] receives the location to open (the app
/// goes to `/admin` plus [AdminLocation.path]); [onExit] leaves the administration. The count
/// comes from the same data as the device queue (`listUsers`, `listUserDevices`) and loads
/// again after every change. The shell holds no permission logic: a screen whose list the
/// server refuses shows the server's reason (fixed requirement 9).
///
/// Example:
/// ```dart
/// AdminShell(
///   repository: admin,
///   location: AdminLocation.parse(segments) ?? AdminLocation.start,
///   onNavigate: (to) => context.go('/admin${to.path}'),
/// );
/// ```
class AdminShell extends StatefulWidget {
  /// Creates the shell.
  const AdminShell({
    super.key,
    required this.repository,
    required this.location,
    required this.onNavigate,
    this.onExit,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
    this.saveTarget,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The open place.
  final AdminLocation location;

  /// Opens another place.
  final ValueChanged<AdminLocation> onNavigate;

  /// Leaves the administration, or null for no "Back" entry.
  final VoidCallback? onExit;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// The host's save dialog for the audit export, or null for no export (brief M4-29 Design
  /// note 3).
  final AuditSaveTarget? saveTarget;

  @override
  State<AdminShell> createState() => _AdminShellState();
}

class _AdminShellState extends State<AdminShell> {
  int? _waiting;

  @override
  void initState() {
    super.initState();
    _count();
  }

  Future<void> _count() async {
    try {
      final waiting = await widget.repository.waitingDevices();
      if (mounted) setState(() => _waiting = waiting.length);
    } on Object {
      // Without an answer the entry shows no count.
      if (mounted) setState(() => _waiting = null);
    }
  }

  Widget _body() {
    final r = widget.repository;
    final at = widget.location;
    final id = at.id;
    void changed() => _count();
    return switch (at.section) {
      AdminSection.people when id != null => PersonDetail(
        key: ValueKey('admin.person.$id'),
        repository: r,
        userId: id,
        timeZone: widget.timeZone,
        clock: widget.clock,
        onBack: () => widget.onNavigate(AdminLocation.start),
        onChanged: changed,
      ),
      AdminSection.groups when id != null => GroupDetail(
        key: ValueKey('admin.group.$id'),
        repository: r,
        groupId: id,
        timeZone: widget.timeZone,
        clock: widget.clock,
        onBack: () =>
            widget.onNavigate(const AdminLocation(AdminSection.groups)),
        onChanged: changed,
      ),
      AdminSection.groups => GroupsList(
        repository: r,
        onOpen: (g) =>
            widget.onNavigate(AdminLocation(AdminSection.groups, id: g)),
        onChanged: changed,
      ),
      AdminSection.grants => GrantsList(
        key: ValueKey('admin.grants.$id'),
        repository: r,
        targetKey: id,
        timeZone: widget.timeZone,
        clock: widget.clock,
        onTarget: (key) =>
            widget.onNavigate(AdminLocation(AdminSection.grants, id: key)),
        onChanged: changed,
      ),
      AdminSection.audit => AuditLogView(
        repository: r,
        timeZone: widget.timeZone,
        clock: widget.clock,
        saveTarget: widget.saveTarget,
      ),
      AdminSection.devices => WaitingDevicesView(
        repository: r,
        timeZone: widget.timeZone,
        clock: widget.clock,
        onChanged: changed,
      ),
      _ => PeopleList(
        repository: r,
        timeZone: widget.timeZone,
        onOpen: (p) =>
            widget.onNavigate(AdminLocation(AdminSection.people, id: p)),
        onChanged: changed,
      ),
    };
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final waiting = _waiting;
    return Material(
      key: const ValueKey('screen.admin'),
      color: colors.surfaceCanvas,
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          SizedBox(
            width: DsLayout.sidebarWidth,
            child: Material(
              color: colors.surfaceRaised,
              child: ListView(
                padding: const EdgeInsets.all(DsSpace.s12),
                children: [
                  if (widget.onExit case final exit?)
                    Align(
                      alignment: Alignment.centerLeft,
                      child: TextButton(
                        key: const ValueKey('admin.back'),
                        onPressed: exit,
                        child: Text(l.adminBack),
                      ),
                    ),
                  Padding(
                    padding: const EdgeInsets.all(DsSpace.s8),
                    child: Semantics(
                      header: true,
                      child: Text(
                        l.adminTitle,
                        style: dsTextStyle(DsType.title)
                            .copyWith(color: colors.textPrimary),
                      ),
                    ),
                  ),
                  for (final s in AdminSection.listed)
                    ListTile(
                      key: ValueKey('admin.nav.${s.segment}'),
                      selected: s == widget.location.section,
                      selectedColor: colors.textPrimary,
                      selectedTileColor: colors.accentSubtle,
                      textColor: colors.textSecondary,
                      title: Text(
                        s == AdminSection.devices && waiting != null
                            ? l.adminNavDevicesWaiting(waiting)
                            : adminSectionLabel(l, s),
                      ),
                      onTap: () => widget.onNavigate(AdminLocation(s)),
                    ),
                ],
              ),
            ),
          ),
          Expanded(
            child: widget.location.section == AdminSection.audit
                // The audit list scrolls itself (virtualized), so it gets the body's height.
                ? Padding(
                    padding: const EdgeInsets.all(DsSpace.s24),
                    child: KeyedSubtree(
                      key: ValueKey('admin.body.${widget.location.path}'),
                      child: _body(),
                    ),
                  )
                : SingleChildScrollView(
                    padding: const EdgeInsets.all(DsSpace.s24),
                    child: Align(
                      alignment: Alignment.topLeft,
                      child: ConstrainedBox(
                        constraints: const BoxConstraints(maxWidth: 880),
                        child: KeyedSubtree(
                          key: ValueKey('admin.body.${widget.location.path}'),
                          child: _body(),
                        ),
                      ),
                    ),
                  ),
          ),
        ],
      ),
    );
  }
}
