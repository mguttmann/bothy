// SPDX-License-Identifier: Apache-2.0

/// The check results of `bothy-host doctor` with the people of the named session users
/// (docs/spec/01-product.md 5.2 A8; 24 2.2 rule for names, 7.4, 10.1; brief M4-37 Design
/// note 2 and fixed requirement 16).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../data/admin_repository.dart';
import '../data/hosts_repository.dart';
import 'host_labels.dart';

/// The names of failed entries of [host] that have no valid assignment: the page reads the
/// audit of the host only for these (24 10.1), since `getHost` returns valid assignments only.
Set<String> namesWithoutAssignment(AdminHost host) {
  final valid = {for (final s in host.sessionUsers) s.osUser};
  return {
    for (final d in host.doctor)
      if (!d.ok && d.sessionUser != null && !valid.contains(d.sessionUser))
        d.sessionUser!,
  };
}

/// Every entry of [host]'s `doctor` in the server's order, also for a disconnected host after
/// a refused start (24 2.2): the check ID (HD1 to HD11), "ok" or the code, and where the
/// server names a session user that name with the person of its assignment. A valid
/// assignment comes from `session_users`; a name without one reads "formerly assigned to"
/// the person [former] gives (the last assignment in the audit of the host); without either
/// the name stands alone with "no assignment known". The screen never guesses a name.
/// [auditFailed] adds a note when the audit could not be read.
///
/// Example: `DoctorResults(host: host, people: people, former: {'term-robin': robinId})`.
class DoctorResults extends StatelessWidget {
  /// Creates the results.
  const DoctorResults({
    super.key,
    required this.host,
    required this.people,
    this.former = const {},
    this.auditFailed = false,
  });

  /// The host.
  final AdminHost host;

  /// The people (for the names of the assignments).
  final List<AdminPerson> people;

  /// The person of the last assignment per name, read from the audit.
  final Map<String, String> former;

  /// Whether the audit of the host could not be read.
  final bool auditFailed;

  String _name(AppLocalizations l, String osUser) {
    final valid = host.sessionUsers
        .where((s) => s.osUser == osUser)
        .firstOrNull;
    if (valid != null) {
      return l.hostDoctorAssigned(osUser, personName(people, valid.userId));
    }
    final was = former[osUser];
    if (was != null) return l.hostDoctorFormer(osUser, personName(people, was));
    return l.hostDoctorUnknown(osUser);
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    if (host.doctor.isEmpty) {
      return Text(
        l.hostDoctorNone,
        style: dsTextStyle(DsType.bodySm).copyWith(color: colors.textSecondary),
      );
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        for (final d in host.doctor)
          Row(
            key: ValueKey('doctor.${d.check}.${d.sessionUser ?? '-'}'),
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Padding(
                padding: const EdgeInsets.only(right: DsSpace.s8),
                child: Icon(
                  d.ok ? Icons.check_circle_outline : Icons.error_outline,
                  size: 18,
                  color: d.ok ? colors.textSecondary : colors.dangerFg,
                ),
              ),
              Expanded(
                child: Text.rich(
                  TextSpan(
                    children: [
                      TextSpan(
                        text: d.ok
                            ? l.hostDoctorOk(d.label)
                            : l.hostDoctorFailed(d.label, d.code ?? '-'),
                      ),
                      if (d.sessionUser case final name?)
                        TextSpan(text: '  ${_name(l, name)}'),
                    ],
                  ),
                  style: dsTextStyle(DsType.body).copyWith(
                    color: d.ok ? colors.textSecondary : colors.textPrimary,
                  ),
                ),
              ),
            ],
          ),
        if (auditFailed)
          Text(
            l.hostDoctorAuditFailed,
            key: const ValueKey('doctor.auditFailed'),
            style: dsTextStyle(DsType.bodySm)
                .copyWith(color: colors.textSecondary),
          ),
      ],
    );
  }
}
