// SPDX-License-Identifier: Apache-2.0

/// The page of one host with every field of A8 (first wireframe of docs/spec/01-product.md
/// 5.2 A8; 24 2.2, 2.3, 3, 4.3, 7, 8; brief M4-37 Scope and fixed requirements 13 to 18).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/admin_action.dart';
import '../common/confirm_change.dart';
import '../common/problem_banner.dart';
import '../data/admin_repository.dart';
import '../data/hosts_repository.dart';
import '../self/self_feedback.dart';
import 'doctor_results.dart';
import 'host_grants_section.dart';
import 'host_labels.dart';
import 'host_state_actions.dart';
import 'llm_login_section.dart';
import 'pairing_wizard.dart';
import 'server_host_section.dart';
import 'session_users_section.dart';

/// The host [hostId]: state, assignee, platform and version of the host service, connection,
/// fingerprint and key generation, LLM login ([LlmLoginSection]), the server host
/// ([ServerHostSection]), check results ([DoctorResults]), profiles, grants
/// ([HostGrantsSection]), session users ([SessionUsersSection]), the count of open terminal
/// sessions and the state actions ([HostStateActions]).
///
/// The check results name session users; for a name without a valid assignment the page reads
/// the audit of the host once (`listAuditEvents`, step-up of `audit.read`) to show the person
/// of its ended assignment (brief Design note 2). Every change asks a reason and goes through
/// the step-up interceptor; a section the server refuses shows its reason. [onBack] returns to
/// the list; [onChanged] runs after a change.
class HostDetail extends StatefulWidget {
  /// Creates the page.
  const HostDetail({
    super.key,
    required this.repository,
    required this.hostId,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
    this.onBack,
    this.onChanged,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The host.
  final String hostId;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// Returns to the list, or null for no button.
  final VoidCallback? onBack;

  /// Runs after a change.
  final VoidCallback? onChanged;

  @override
  State<HostDetail> createState() => _HostDetailState();
}

class _HostDetailState extends State<HostDetail>
    with AdminActionState<HostDetail> {
  AdminHost? _host;
  Object? _error;
  List<AdminPerson> _people = const [];
  int? _sessions;
  Map<String, String> _former = const {};
  bool _auditFailed = false;

  /// The names whose history was read already, so a reload asks the audit only for new ones.
  final Set<String> _historyFor = {};

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void didUpdateWidget(HostDetail old) {
    super.didUpdateWidget(old);
    if (old.hostId != widget.hostId) _load();
  }

  Future<void> _load() async {
    final r = widget.repository;
    try {
      final host = await r.host(widget.hostId);
      List<AdminPerson> people;
      try {
        people = await allPeople(r);
      } on Object {
        people = const [];
      }
      int? sessions;
      try {
        sessions = await r.openSessions(widget.hostId);
      } on Object {
        sessions = null;
      }
      if (!mounted) return;
      setState(() {
        _host = host;
        _people = people;
        _sessions = sessions;
        _error = null;
      });
      await _history(host);
    } on Object catch (e) {
      if (mounted) {
        setState(() {
          _host = null;
          _error = e;
        });
      }
    }
  }

  /// Reads the audit of the host when a check result names a session user without a valid
  /// assignment that was not looked up yet (24 10.1).
  Future<void> _history(AdminHost host) async {
    final missing = namesWithoutAssignment(host).difference(_historyFor);
    if (missing.isEmpty) return;
    _historyFor.addAll(missing);
    final l = AppLocalizations.of(context);
    try {
      final former = await widget.repository.assignmentHistory(
        host.id,
        purpose: l.stepUpPurposeHostAudit(host.name),
      );
      if (mounted) {
        setState(() {
          _former = former;
          _auditFailed = false;
        });
      }
    } on Object {
      if (mounted) setState(() => _auditFailed = true);
    }
  }

  void _changed(AdminHost host) {
    setState(() => _host = host);
    widget.onChanged?.call();
    _history(host);
  }

  Future<void> _changeAssignee(AdminHost h) async {
    final l = AppLocalizations.of(context);
    final answer = await showDialog<(String?, String)>(
      context: context,
      builder: (_) => _AssigneeDialog(host: h, people: _people),
    );
    if (answer == null || !mounted) return;
    final (assignee, reason) = answer;
    AdminHost? changed;
    await runChange('assignee', () async {
      changed = await widget.repository.updateHost(
        h,
        assigneeId: assignee,
        clearAssignee: assignee == null,
        reason: reason,
        purpose: l.stepUpPurposeUpdateHost(h.name),
      );
      return SelfNotice(l.hostAssigneeChanged(h.name));
    });
    if (changed case final host?) _changed(host);
  }

  Future<void> _act(AdminHost h, HostAction action) async {
    final l = AppLocalizations.of(context);
    if (action == HostAction.pair) {
      await PairingWizard.show(
        context,
        repository: widget.repository,
        host: h,
        clock: widget.clock,
      );
      widget.onChanged?.call();
      if (mounted) await _load();
      return;
    }
    final (title, body, label, destructive) = switch (action) {
      HostAction.repair => (
        l.hostRepairTitle(h.name),
        l.hostRepairBody,
        l.hostRepair,
        true,
      ),
      HostAction.disable => (
        l.hostDisableTitle(h.name),
        l.hostDisableBody,
        l.hostDisable,
        true,
      ),
      HostAction.enable => (
        l.hostEnableTitle(h.name),
        l.hostEnableBody,
        l.hostEnable,
        false,
      ),
      _ => (l.hostRetireTitle(h.name), l.hostRetireBody, l.hostRetire, true),
    };
    final reason = await askHostReason(
      context,
      title: title,
      body: body,
      confirmLabel: label,
      destructive: destructive,
    );
    if (reason == null || !mounted) return;
    if (action == HostAction.repair) {
      HostPairingCodeInfo? code;
      await runChange(action.name, () async {
        code = await widget.repository.newPairingCode(
          h.id,
          reason: reason,
          purpose: l.stepUpPurposePairingCode(h.name),
        );
        return null;
      });
      if (code == null || !mounted) return;
      widget.onChanged?.call();
      await _load();
      if (!mounted) return;
      await PairingWizard.show(
        context,
        repository: widget.repository,
        host: _host,
        code: code,
        reason: reason,
        clock: widget.clock,
      );
      if (mounted) await _load();
      return;
    }
    final state = switch (action) {
      HostAction.disable => HostStateRequest.disabled,
      HostAction.enable => HostStateRequest.active,
      _ => HostStateRequest.retired,
    };
    AdminHost? changed;
    await runChange(action.name, () async {
      changed = await widget.repository.setHostState(
        h,
        state,
        reason: reason,
        purpose: l.stepUpPurposeHostState(h.name),
      );
      return SelfNotice(
        l.hostStateChanged(h.name, hostStateLabel(l, changed!)),
      );
    });
    if (changed case final host?) _changed(host);
  }

  Widget _field(String label, Widget value, {String? key}) {
    final colors = context.dsColors;
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: DsSpace.s8),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 160,
            child: Text(
              label,
              style: dsTextStyle(DsType.label)
                  .copyWith(color: colors.textSecondary),
            ),
          ),
          Expanded(
            child: KeyedSubtree(
              key: key == null ? null : ValueKey(key),
              child: value,
            ),
          ),
        ],
      ),
    );
  }

  String _connection(AppLocalizations l, AdminHost h) {
    final seen = h.lastSeenAt;
    final state = h.connected ? l.hostConnected : l.hostDisconnected;
    if (seen == null) return state;
    final format = L10nFormat(Localizations.localeOf(context));
    String when;
    try {
      when =
          '${format.relativeDay(seen, now: widget.clock(), timeZone: widget.timeZone)} '
          '${format.time(seen, timeZone: widget.timeZone)}';
    } on ArgumentError {
      when =
          '${format.relativeDay(seen, now: widget.clock(), timeZone: 'UTC')} '
          '${format.time(seen, timeZone: 'UTC')}';
    }
    return '$state, ${l.hostLastSeen(when)}';
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final h = _host;
    final error = _error;
    final body = dsTextStyle(DsType.body).copyWith(color: colors.textPrimary);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        if (widget.onBack case final back?)
          Align(
            alignment: Alignment.centerLeft,
            child: TextButton(
              key: const ValueKey('host.back'),
              onPressed: back,
              child: Text(l.hostBack),
            ),
          ),
        if (error != null)
          ProblemBanner(problem: loadProblem(error), onRetry: _load)
        else if (h == null)
          const Center(child: CircularProgressIndicator())
        else ...[
          Row(
            children: [
              Expanded(
                child: Semantics(
                  header: true,
                  child: Text(
                    h.name,
                    style: dsTextStyle(DsType.title)
                        .copyWith(color: colors.textPrimary),
                  ),
                ),
              ),
              DsBadge(
                key: const ValueKey('host.stateBadge'),
                text: hostStateLabel(l, h),
                background: h.state == HostLifecycle.active
                    ? colors.accentSubtle
                    : colors.attentionSubtle,
                foreground: colors.textPrimary,
              ),
            ],
          ),
          // The plain state text, read by tests and screen readers alike.
          Text(
            hostStateLabel(l, h),
            key: const ValueKey('host.state'),
            style: dsTextStyle(DsType.bodySm)
                .copyWith(color: colors.textSecondary),
          ),
          const SizedBox(height: DsSpace.s8),
          HostStateActions(
            host: h,
            disabled: busy,
            onAction: (a) => _act(h, a),
          ),
          noticeBanner(),
          _field(
            l.hostFieldAssignee,
            Row(
              children: [
                Expanded(
                  child: Text(switch (h.assignedUserId) {
                    final id? => personName(_people, id),
                    null => l.hostOwnersOnly,
                  }, style: body),
                ),
                if (h.state != HostLifecycle.retired)
                  TextButton(
                    key: const ValueKey('host.assignee.change'),
                    onPressed: busy ? null : () => _changeAssignee(h),
                    child: Text(l.hostChange),
                  ),
              ],
            ),
            key: 'host.assignee',
          ),
          _field(
            l.hostFieldPlatform,
            Text(
              switch (h.osVersion ?? h.platform) {
                final os? => l.hostPlatformValue(os, h.hostVersion ?? '-'),
                null => l.hostNotReported,
              },
              key: const ValueKey('host.platform'),
              style: body,
            ),
          ),
          _field(
            l.hostFieldConnection,
            Text(
              _connection(l, h),
              key: const ValueKey('host.connection'),
              style: body,
            ),
          ),
          _field(
            l.hostFieldFingerprint,
            Text(
              switch (h.keyFingerprint) {
                final f? => l.hostFingerprintValue(
                  fingerprintGroups(f),
                  h.keyGeneration,
                ),
                null => l.hostNoKeys,
              },
              key: const ValueKey('host.fingerprint'),
              style: dsTextStyle(DsType.code)
                  .copyWith(color: colors.textPrimary),
            ),
          ),
          _field(
            l.hostFieldLlmLogin,
            LlmLoginSection(
              repository: widget.repository,
              host: h,
              people: _people,
              onChanged: _changed,
            ),
          ),
          _field(l.hostFieldServerHost, ServerHostSection(host: h)),
          _field(
            l.hostFieldChecks,
            DoctorResults(
              host: h,
              people: _people,
              former: _former,
              auditFailed: _auditFailed,
            ),
          ),
          _field(
            l.hostFieldProfiles,
            Text(
              h.profiles.isEmpty
                  ? l.hostNotReported
                  : [for (final p in h.profiles) p.displayName].join(', '),
              key: const ValueKey('host.profiles'),
              style: body,
            ),
          ),
          _field(
            l.hostFieldGrants,
            HostGrantsSection(
              repository: widget.repository,
              host: h,
              people: _people,
              timeZone: widget.timeZone,
              clock: widget.clock,
              onChanged: widget.onChanged,
            ),
          ),
          _field(
            l.hostFieldSessionUsers,
            SessionUsersSection(
              repository: widget.repository,
              host: h,
              people: _people,
              onChanged: _changed,
            ),
          ),
          if (_sessions case final count?)
            _field(
              l.hostFieldSessions,
              Text(
                l.hostSessionsCount(count),
                key: const ValueKey('host.sessions'),
                style: body,
              ),
            ),
        ],
      ],
    );
  }
}

/// The dialog of "Change" next to the assignee: a person or "Owners only", and a reason; pops
/// (assignee or null, reason).
class _AssigneeDialog extends StatefulWidget {
  const _AssigneeDialog({required this.host, required this.people});

  final AdminHost host;
  final List<AdminPerson> people;

  @override
  State<_AssigneeDialog> createState() => _AssigneeDialogState();
}

class _AssigneeDialogState extends State<_AssigneeDialog> {
  final TextEditingController _reason = TextEditingController();
  late String? _assignee = widget.host.assignedUserId;

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
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return ConfirmChange(
      title: l.hostAssigneeTitle(widget.host.name),
      body: l.hostAssigneeBody,
      confirmLabel: l.hostChange,
      fields: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          DropdownButtonFormField<String?>(
            key: const ValueKey('assignee.person'),
            initialValue: _assignee,
            decoration: InputDecoration(labelText: l.hostFieldAssignee),
            items: [
              DropdownMenuItem(child: Text(l.hostOwnersOnly)),
              for (final p in widget.people)
                DropdownMenuItem(value: p.id, child: Text(p.displayName)),
            ],
            onChanged: (v) => setState(() => _assignee = v),
          ),
          TextField(
            key: const ValueKey('confirm.reason'),
            controller: _reason,
            decoration: InputDecoration(labelText: l.adminReasonRequired),
          ),
        ],
      ),
      canConfirm:
          _reason.text.trim().isNotEmpty &&
          _assignee != widget.host.assignedUserId,
      onConfirm: (_) =>
          Navigator.of(context).pop((_assignee, _reason.text.trim())),
    );
  }
}
