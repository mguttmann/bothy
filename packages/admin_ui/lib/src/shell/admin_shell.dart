// SPDX-License-Identifier: Apache-2.0

/// The frame of the administration: the navigation of A1 on the left and the open screen on the
/// right (docs/spec/01-product.md 5.2 A1; brief M4-26 Scope). The desktop app hosts it at
/// `/admin`, the console at its root (M6-04).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../apps/manifest_picker.dart';
import '../audit/audit_export.dart';
import '../data/admin_repository.dart';
import '../data/agents_repository.dart';
import '../instance/instance_common.dart';
import 'admin_body.dart';
import 'admin_routes.dart';

/// The name of the entry [section] in the navigation.
String adminSectionLabel(AppLocalizations l, AdminSection section) =>
    switch (section) {
      AdminSection.people => l.adminNavPeople,
      AdminSection.groups => l.adminNavGroups,
      AdminSection.grants => l.adminNavGrants,
      AdminSection.hints => l.adminNavHints,
      AdminSection.expiring => l.adminNavExpiring,
      AdminSection.devices => l.adminNavDevices,
      AdminSection.agents => l.adminNavAgents,
      AdminSection.audit => l.adminNavAudit,
      AdminSection.hosts => l.adminNavHosts,
      AdminSection.apps => l.adminNavApps,
      AdminSection.webhooks => l.adminNavIncomingWebhooks,
      AdminSection.instance => l.adminNavInstance,
      AdminSection.statistics => l.adminNavStatistics,
      // Entries without a screen are not listed (AdminSection.listed).
      _ => section.segment,
    };

/// The administration at [location]: the entries of [AdminSection.listed] (an entry without a
/// screen is not listed) with the count of waiting devices, and the screen of the location.
/// "Hosts" is listed when `getMe.is_owner` is true (brief M4-37 fixed requirement 11: A8 is
/// for owners; `listHosts` itself answers non-owners with their own hosts, so it is no gate).
/// "Instance and branding" (M6-09) is listed for owners in the same way; its operations are
/// owner-only, and the server still refuses everyone else.
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
    this.agentPresence,
    this.pickManifest,
    this.pickLogo,
    this.instanceChanges,
    this.openExternal,
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

  /// The live presence of an agent for its page, or null to show the presence of `getAgent`
  /// (brief M4-28 Design note 1: the desktop app maps the engine's `EphemeralStore`, the
  /// console has none).
  final Stream<AgentLiveState> Function(String agentId)? agentPresence;

  /// The host's file dialog for an app manifest (the desktop's system dialog), or null for no
  /// install and no update (brief M6-08 Design note 1).
  final ManifestPicker? pickManifest;

  /// The host's file dialog for the logo of the instance, or null for no upload (brief M6-09
  /// Design note 2).
  final LogoPicker? pickLogo;

  /// Opens the `instance.updated` events of the app, after which the instance settings and
  /// flags load again (brief M6-09 Design note 4), or null.
  final Stream<void> Function()? instanceChanges;

  /// Opens a link outside the app (the system browser), or null; [AdminBody.openExternal].
  final ValueChanged<Uri>? openExternal;

  @override
  State<AdminShell> createState() => _AdminShellState();
}

class _AdminShellState extends State<AdminShell> {
  int? _waiting;
  bool _owner = false;

  @override
  void initState() {
    super.initState();
    _count();
    _ownerFlag();
  }

  Future<void> _ownerFlag() async {
    try {
      final owner = await widget.repository.callerIsOwner();
      if (mounted) setState(() => _owner = owner);
    } on Object {
      // Without an answer the owner entries stay hidden.
    }
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
                    if ((s != AdminSection.hosts &&
                            s != AdminSection.instance) ||
                        _owner)
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
            child: AdminBody(
              repository: widget.repository,
              location: widget.location,
              onNavigate: widget.onNavigate,
              onChanged: _count,
              timeZone: widget.timeZone,
              clock: widget.clock,
              saveTarget: widget.saveTarget,
              agentPresence: widget.agentPresence,
              pickManifest: widget.pickManifest,
              pickLogo: widget.pickLogo,
              instanceChanges: widget.instanceChanges,
              openExternal: widget.openExternal,
            ),
          ),
        ],
      ),
    );
  }
}
