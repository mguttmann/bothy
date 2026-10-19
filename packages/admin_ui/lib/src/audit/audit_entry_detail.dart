// SPDX-License-Identifier: Apache-2.0

/// The details of one audit entry: correlation ID, deny reason, the details the server returns
/// and the hash links (docs/spec/01-product.md 5.2 A6; 03 `audit_events`; brief M4-29 Scope).
/// Read only: an audit entry is never changed or deleted from the interface.
library;

import 'dart:convert';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import 'audit_filters.dart' show auditResultText;
import 'audit_repository.dart';

/// The translated label of the action [id] where `l10n` has one, else null (the screen then
/// shows the ID alone; brief M4-29 Design note 1).
String? auditActionLabel(AppLocalizations l, String id) => switch (id) {
  'audit.read' => l.auditActionAuditRead,
  'audit.verify' => l.auditActionAuditVerify,
  'grant.create' => l.auditActionGrantCreate,
  'grant.revoke' => l.auditActionGrantRevoke,
  'owner.add' => l.auditActionOwnerAdd,
  'owner.remove' => l.auditActionOwnerRemove,
  'user.create' => l.auditActionUserCreate,
  'user.disable' => l.auditActionUserDisable,
  'user.enable' => l.auditActionUserEnable,
  'device.approve' => l.auditActionDeviceApprove,
  'device.revoke' => l.auditActionDeviceRevoke,
  'group.create' => l.auditActionGroupCreate,
  'auth.step_up' => l.auditActionStepUp,
  _ => null,
};

/// The resource of [entry] as its kind and ID (the `AuditEvent` carries no name), or an
/// empty text without a resource.
String auditResourceText(AuditEntry entry) =>
    [?entry.resourceKind, ?entry.resourceId].join(' ');

/// The dialog with every field of [entry]; [actor] is the name of its actor (or its ID) and
/// [time] its time in the person's time zone, as the list shows them.
class AuditEntryDetail extends StatelessWidget {
  /// Creates the dialog.
  const AuditEntryDetail({
    super.key,
    required this.entry,
    required this.actor,
    required this.time,
  });

  /// The entry.
  final AuditEntry entry;

  /// The actor's name or ID, or empty for the system.
  final String actor;

  /// The time of the entry, formatted.
  final String time;

  /// Shows the dialog over [context].
  static Future<void> show(
    BuildContext context, {
    required AuditEntry entry,
    required String actor,
    required String time,
  }) => showDialog<void>(
    context: context,
    builder: (_) => AuditEntryDetail(entry: entry, actor: actor, time: time),
  );

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final e = entry;
    final label = auditActionLabel(l, e.action);
    final rows = <(String, String)>[
      (l.auditColTime, time),
      (l.auditColWho, actor),
      (l.auditFilterAction, label == null ? e.action : '$label (${e.action})'),
      (l.auditColResult, auditResultText(l, e.result)),
      (l.auditColResource, auditResourceText(e)),
      (l.auditDetailDenyReason, e.denyReason ?? ''),
      (l.auditDetailCorrelation, e.correlationId ?? ''),
      (l.auditDetailHash, e.hash),
      (l.auditDetailPrevHash, e.prevHash),
    ];
    final mono = dsTextStyle(DsType.code).copyWith(color: colors.textPrimary);
    return AlertDialog(
      key: const ValueKey('audit.detail'),
      title: Text(l.auditDetailTitle(e.seq)),
      content: SizedBox(
        width: 560,
        child: SingleChildScrollView(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            mainAxisSize: MainAxisSize.min,
            children: [
              for (final (name, value) in rows)
                if (value.isNotEmpty)
                  Padding(
                    padding: const EdgeInsets.only(bottom: DsSpace.s8),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          name,
                          style: dsTextStyle(DsType.bodySm)
                              .copyWith(color: colors.textSecondary),
                        ),
                        SelectableText(
                          value,
                          style: dsTextStyle(DsType.body)
                              .copyWith(color: colors.textPrimary),
                        ),
                      ],
                    ),
                  ),
              Text(
                l.auditDetailDetails,
                style: dsTextStyle(DsType.bodySm)
                    .copyWith(color: colors.textSecondary),
              ),
              SelectableText(
                const JsonEncoder.withIndent('  ').convert(e.details),
                key: const ValueKey('audit.detail.details'),
                style: mono,
              ),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          key: const ValueKey('audit.detail.close'),
          onPressed: () => Navigator.of(context).pop(),
          child: Text(l.auditDetailClose),
        ),
      ],
    );
  }
}
