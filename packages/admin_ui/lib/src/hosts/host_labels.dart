// SPDX-License-Identifier: Apache-2.0

/// The labels of screen A8 and the one reason dialog of its changes (docs/spec/01-product.md
/// 5.2 A8; 24 3.1, 4.3, 7.2; brief M4-37 fixed requirement 17).
///
/// Labels only: whether a person gets a terminal is the server's answer (15 1 principle 1).
library;

import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/confirm_change.dart';
import '../data/admin_repository.dart';
import '../data/hosts_repository.dart';

/// The label of the state of [host], for example "active".
String hostStateLabel(AppLocalizations l, AdminHost host) =>
    switch (host.state) {
      HostLifecycle.pairing => l.hostStatePairing,
      HostLifecycle.confirming => l.hostStateConfirming,
      HostLifecycle.active => l.hostStateActive,
      HostLifecycle.disabled =>
        host.ownerDisabled ? l.hostStateDisabled : l.hostStateDuplicate,
      HostLifecycle.retired => l.hostStateRetired,
      HostLifecycle.unknown => l.hostStateUnknown,
    };

/// The label of [login] without its holder, for example "Organization".
String llmLoginLabel(AppLocalizations l, HostLlmLogin login) => switch (login) {
  HostLlmLogin.none => l.hostLlmNone,
  HostLlmLogin.personal => l.hostLlmPersonal,
  HostLlmLogin.organization => l.hostLlmOrganization,
};

/// [fingerprint] in groups of four separated by spaces (24 4.3: the form `bothy-host pair`
/// shows), for example `7KQ2 9XTA M4PB`.
String fingerprintGroups(String fingerprint) =>
    fingerprint.replaceAll('-', ' ');

/// The display name of the person [id] among [people], or [id] when the list does not hold it.
String personName(List<AdminPerson> people, String id) =>
    people.where((p) => p.id == id).firstOrNull?.displayName ?? id;

/// Asks for the reason of one change of a host: a [ConfirmChange] whose "Confirm" waits for a
/// reason, since every change of A8 carries one (01 5.2 A8; `host.manage` stores it in the
/// audit). Returns the reason, or null for "Cancel".
///
/// Example:
/// ```dart
/// final reason = await askHostReason(context, title: t, body: b, confirmLabel: l.hostRetire,
///     destructive: true);
/// ```
Future<String?> askHostReason(
  BuildContext context, {
  required String title,
  required String body,
  required String confirmLabel,
  bool destructive = false,
}) => showDialog<String>(
  context: context,
  builder: (_) => _ReasonDialog(
    title: title,
    body: body,
    confirmLabel: confirmLabel,
    destructive: destructive,
  ),
);

class _ReasonDialog extends StatefulWidget {
  const _ReasonDialog({
    required this.title,
    required this.body,
    required this.confirmLabel,
    required this.destructive,
  });

  final String title;
  final String body;
  final String confirmLabel;
  final bool destructive;

  @override
  State<_ReasonDialog> createState() => _ReasonDialogState();
}

class _ReasonDialogState extends State<_ReasonDialog> {
  final TextEditingController _reason = TextEditingController();

  @override
  void initState() {
    super.initState();
    _reason.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _reason.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => ConfirmChange(
    title: widget.title,
    body: widget.body,
    confirmLabel: widget.confirmLabel,
    destructive: widget.destructive,
    fields: TextField(
      key: const ValueKey('confirm.reason'),
      controller: _reason,
      decoration: InputDecoration(
        labelText: AppLocalizations.of(context).adminReasonRequired,
      ),
    ),
    canConfirm: _reason.text.trim().isNotEmpty,
    onConfirm: (_) => Navigator.of(context).pop(_reason.text.trim()),
  );
}
