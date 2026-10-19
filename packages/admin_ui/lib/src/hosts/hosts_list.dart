// SPDX-License-Identifier: Apache-2.0

/// The hosts of screen A8 with "Add host" (docs/spec/01-product.md 5.2 A8; 24 3; brief M4-37
/// Scope and fixed requirements 11, 12).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/problem_banner.dart';
import '../data/admin_repository.dart';
import '../data/hosts_repository.dart';
import 'host_labels.dart';
import 'pairing_wizard.dart';

/// Every host the server lists (`listHosts`) with its state, connection and assignee; a row
/// opens the host ([onOpen] receives its ID). "Add host" opens the [PairingWizard]; after it the
/// list loads again and [onChanged] runs. A list the server refuses shows its reason and
/// nothing else (15 1 principle 1).
class HostsList extends StatefulWidget {
  /// Creates the list.
  const HostsList({
    super.key,
    required this.repository,
    required this.onOpen,
    this.clock = DateTime.now,
    this.onChanged,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// Opens a host.
  final ValueChanged<String> onOpen;

  /// The current time.
  final DateTime Function() clock;

  /// Runs after a change.
  final VoidCallback? onChanged;

  @override
  State<HostsList> createState() => _HostsListState();
}

class _HostsListState extends State<HostsList> {
  List<AdminHost>? _hosts;
  List<AdminPerson> _people = const [];
  Object? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final hosts = await widget.repository.hosts();
      List<AdminPerson> people;
      try {
        people = await allPeople(widget.repository);
      } on Object {
        people = const [];
      }
      if (mounted) {
        setState(() {
          _hosts = hosts;
          _people = people;
          _error = null;
        });
      }
    } on Object catch (e) {
      if (mounted) {
        setState(() {
          _hosts = null;
          _error = e;
        });
      }
    }
  }

  Future<void> _add() async {
    final host = await PairingWizard.show(
      context,
      repository: widget.repository,
      clock: widget.clock,
    );
    widget.onChanged?.call();
    if (!mounted) return;
    await _load();
    if (host != null) widget.onOpen(host.id);
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final hosts = _hosts;
    final error = _error;
    if (error != null) {
      return ProblemBanner(problem: loadProblem(error), onRetry: _load);
    }
    if (hosts == null) return const Center(child: CircularProgressIndicator());
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Row(
          children: [
            Expanded(
              child: Semantics(
                header: true,
                child: Text(
                  l.adminNavHosts,
                  style: dsTextStyle(DsType.title)
                      .copyWith(color: colors.textPrimary),
                ),
              ),
            ),
            FilledButton(
              key: const ValueKey('hosts.add'),
              onPressed: _add,
              child: Text(l.hostsAdd),
            ),
          ],
        ),
        const SizedBox(height: DsSpace.s12),
        if (hosts.isEmpty)
          Text(
            l.hostsEmpty,
            style: dsTextStyle(DsType.body)
                .copyWith(color: colors.textSecondary),
          ),
        for (final h in hosts)
          ListTile(
            key: ValueKey('hostRow.${h.id}'),
            title: Text(h.name),
            subtitle: Text(
              [
                hostStateLabel(l, h),
                h.connected ? l.hostConnected : l.hostDisconnected,
                switch (h.assignedUserId) {
                  final id? =>
                    '${l.hostFieldAssignee}: ${personName(_people, id)}',
                  null => l.hostOwnersOnly,
                },
              ].join('  |  '),
            ),
            onTap: () => widget.onOpen(h.id),
          ),
      ],
    );
  }
}
