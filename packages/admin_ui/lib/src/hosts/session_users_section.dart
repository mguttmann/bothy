// SPDX-License-Identifier: Apache-2.0

/// Session users of a host: assign from the reported names, end, and the taken names
/// (docs/spec/01-product.md 5.2 A8; 24 2.2, 3.2, ruling T12; brief M4-37 Design notes 1 and 3,
/// fixed requirement 15).
library;

import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/admin_action.dart';
import '../common/confirm_change.dart';
import '../data/admin_repository.dart';
import '../data/hosts_repository.dart';
import '../data/self_repository.dart';
import '../self/self_feedback.dart';
import 'host_labels.dart';

/// The names of [host] that a picker offers: the reported names without a valid assignment,
/// minus [chosen] (names other rows of the same form hold). Names are never typed: the server
/// accepts only reported names (24 3.2), and a name once given stays bound to its person, which
/// the server answers with `session_user_taken` because `getHost` lists valid assignments only.
List<String> offeredNames(AdminHost host, {Set<String> chosen = const {}}) {
  final given = {for (final s in host.sessionUsers) s.osUser};
  return [
    for (final r in host.reportedSessionUsers ?? const <HostReportedName>[])
      if (!given.contains(r.osUser) && !chosen.contains(r.osUser)) r.osUser,
  ];
}

/// Whether [host] reports [osUser] as its owner account (ADR 0073): the account with
/// administrator rights the host service runs as, assigned only with the explicit opt-in.
bool isOwnerAccount(AdminHost host, String? osUser) =>
    osUser != null &&
    (host.reportedSessionUsers ?? const <HostReportedName>[]).any(
      (r) => r.osUser == osUser && r.ownerAccount,
    );

/// The explanation of a refused assignment: `409 conflict` with `session_user_taken` (a name
/// stays bound to its person on the host, T12), `400 validation_failed` with
/// `session_user_not_on_host` (the host must report the name first) or with an owner account
/// code, and `403 forbidden` with `owner_account_owner_only` (ADR 0073); null for any other
/// problem.
String? sessionUserProblemText(AppLocalizations l, ApiProblem? problem) {
  if (problem == null) return null;
  if (problem.code == 'conflict' && problem.reason == 'session_user_taken') {
    return l.hostSessionUserTaken;
  }
  if (problem.code == 'forbidden' &&
      problem.reason == 'owner_account_owner_only') {
    return l.hostOwnerAccountOwnerOnly;
  }
  if (problem.code == 'validation_failed' &&
      problem.errors.any((e) => e.code == 'session_user_not_on_host')) {
    return l.hostSessionUserNotOnHost;
  }
  if (problem.code == 'validation_failed' &&
      problem.errors.any(
        (e) =>
            e.code == 'owner_account_opt_in_required' ||
            e.code == 'owner_account_not_on_host',
      )) {
    return l.hostOwnerAccountMismatch;
  }
  return null;
}

/// The warning of the owner account [osUser] (ADR 0073, D-OA-11).
Widget ownerAccountWarning(BuildContext context, String osUser, {Key? key}) =>
    DsBanner(
      key: key,
      tone: DsTone.warning,
      text: AppLocalizations.of(context).hostOwnerAccountWarning(osUser),
    );

/// One row of [SessionUserRows]: a person and a reported name, either still unset. The
/// wizard owns the rows, so they survive leaving step 3 and are exactly what it sends.
final class SessionUserRow {
  /// The person, or null.
  String? userId;

  /// The reported name, or null.
  String? osUser;

  /// The name is the owner account of the host (ADR 0073); the row then needs [ownerAccount].
  bool ownerAccountName = false;

  /// The explicit opt-in for the owner account.
  bool ownerAccount = false;

  /// The assignment of a complete row, or null; a row with the owner account needs the opt-in.
  HostAssignment? get assignment => switch ((userId, osUser)) {
    (final u?, final n?) when !ownerAccountName || ownerAccount =>
      HostAssignment(userId: u, osUser: n, ownerAccount: ownerAccountName),
    _ => null,
  };
}

/// The session user rows of the confirmation (second wireframe of A8): per row a person, then
/// a name of the host's report that no other row holds; "Add a session user" adds a row.
///
/// [rows] belong to the caller, which sends exactly the complete rows shown here (brief Design
/// note 1); [onChanged] runs after every change of them. A name the host no longer reports is
/// shown as unset.
class SessionUserRows extends StatelessWidget {
  /// Creates the rows.
  const SessionUserRows({
    super.key,
    required this.host,
    required this.people,
    required this.rows,
    required this.onChanged,
  });

  /// The host in `confirming` with its reported names.
  final AdminHost host;

  /// The people to choose from.
  final List<AdminPerson> people;

  /// The rows, owned and kept by the caller.
  final List<SessionUserRow> rows;

  /// Runs after a change of [rows].
  final VoidCallback onChanged;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          l.hostSessionRowsTitle,
          style: dsTextStyle(DsType.bodyStrong)
              .copyWith(color: colors.textPrimary),
        ),
        for (var i = 0; i < rows.length; i++)
          // The row object keys the subtree, so the pickers follow their row when another
          // row is removed.
          KeyedSubtree(key: ObjectKey(rows[i]), child: _row(l, i)),
        Align(
          alignment: Alignment.centerLeft,
          child: TextButton(
            key: const ValueKey('sessionRows.add'),
            onPressed: () {
              rows.add(SessionUserRow());
              onChanged();
            },
            child: Text(l.hostSessionRowsAdd),
          ),
        ),
        Text(
          l.hostSessionUsersOnlyReported,
          style: dsTextStyle(DsType.bodySm)
              .copyWith(color: colors.textSecondary),
        ),
      ],
    );
  }

  Widget _row(AppLocalizations l, int i) {
    final row = rows[i];
    final names = offeredNames(
      host,
      chosen: {
        for (var j = 0; j < rows.length; j++)
          if (j != i) ?rows[j].osUser,
      },
    );
    final picker = _picker(l, i, row, names);
    if (!row.ownerAccountName) return picker;
    return Builder(
      builder: (context) => Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          picker,
          ownerAccountWarning(context, row.osUser!),
          CheckboxListTile(
            key: ValueKey('sessionRow.$i.ownerAccount'),
            value: row.ownerAccount,
            title: Text(l.hostOwnerAccountOptIn(row.osUser!)),
            onChanged: (v) {
              row.ownerAccount = v ?? false;
              onChanged();
            },
          ),
        ],
      ),
    );
  }

  Widget _picker(
    AppLocalizations l,
    int i,
    SessionUserRow row,
    List<String> names,
  ) {
    return Row(
      key: ValueKey('sessionRow.$i'),
      children: [
        Expanded(
          child: DropdownButtonFormField<String>(
            key: ValueKey('sessionRow.$i.person'),
            initialValue: row.userId,
            decoration: InputDecoration(labelText: l.hostSessionUserPerson),
            items: [
              for (final p in people)
                DropdownMenuItem(value: p.id, child: Text(p.displayName)),
            ],
            onChanged: (v) {
              row.userId = v;
              onChanged();
            },
          ),
        ),
        const SizedBox(width: DsSpace.s8),
        Expanded(
          child: DropdownButtonFormField<String>(
            key: ValueKey('sessionRow.$i.name'),
            initialValue: names.contains(row.osUser) ? row.osUser : null,
            decoration: InputDecoration(labelText: l.hostSessionUserName),
            items: [
              for (final n in names) DropdownMenuItem(value: n, child: Text(n)),
            ],
            onChanged: (v) {
              row.osUser = v;
              row.ownerAccountName = isOwnerAccount(host, v);
              row.ownerAccount = false;
              onChanged();
            },
          ),
        ),
        IconButton(
          key: ValueKey('sessionRow.$i.remove'),
          tooltip: l.hostSessionRowRemove,
          icon: const Icon(Icons.close),
          onPressed: () {
            rows.removeAt(i);
            onChanged();
          },
        ),
      ],
    );
  }
}

/// The session users of a confirmed [host]: each valid assignment with its person, name and
/// work folders and "End"; each reported name without an assignment with "Assign"; the note
/// of SR12. Assigning and ending ask a reason and run `updateHost` (step-up from the
/// interceptor); a refusal is explained ([sessionUserProblemText]). [onChanged] receives the
/// changed host.
class SessionUsersSection extends StatefulWidget {
  /// Creates the section.
  const SessionUsersSection({
    super.key,
    required this.repository,
    required this.host,
    required this.people,
    required this.onChanged,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The host.
  final AdminHost host;

  /// The people.
  final List<AdminPerson> people;

  /// Receives the changed host.
  final ValueChanged<AdminHost> onChanged;

  @override
  State<SessionUsersSection> createState() => _SessionUsersSectionState();
}

class _SessionUsersSectionState extends State<SessionUsersSection>
    with AdminActionState<SessionUsersSection> {
  String? _problem;

  Future<void> _run(
    Future<AdminHost> Function() call,
    SelfNotice notice,
  ) async {
    final l = AppLocalizations.of(context);
    setState(() => _problem = null);
    AdminHost? changed;
    await runChange('sessionUsers', () async {
      try {
        changed = await call();
      } on SelfActionFailed catch (e) {
        final text = sessionUserProblemText(l, e.problem);
        if (text == null) rethrow;
        if (mounted) setState(() => _problem = text);
        return null;
      }
      return notice;
    });
    if (changed case final host?) widget.onChanged(host);
  }

  Future<void> _assign({String? name}) async {
    final l = AppLocalizations.of(context);
    final h = widget.host;
    final answer = await showDialog<(HostAssignment, String)>(
      context: context,
      builder: (_) => _AssignDialog(host: h, people: widget.people, name: name),
    );
    if (answer == null || !mounted) return;
    final (assignment, reason) = answer;
    await _run(
      () => widget.repository.updateHost(
        h,
        add: [assignment],
        reason: reason,
        purpose: l.stepUpPurposeUpdateHost(h.name),
      ),
      SelfNotice(
        l.hostSessionUserAssigned(
          personName(widget.people, assignment.userId),
          assignment.osUser,
        ),
      ),
    );
  }

  Future<void> _end(HostSessionUserInfo s) async {
    final l = AppLocalizations.of(context);
    final h = widget.host;
    final name = personName(widget.people, s.userId);
    final reason = await askHostReason(
      context,
      title: l.hostSessionUserEndTitle(name),
      body: l.hostSessionUserEndBody(name, s.osUser),
      confirmLabel: l.hostSessionUserEnd,
      destructive: true,
    );
    if (reason == null || !mounted) return;
    await _run(
      () => widget.repository.updateHost(
        h,
        revoke: [s.userId],
        reason: reason,
        purpose: l.stepUpPurposeUpdateHost(h.name),
      ),
      SelfNotice(l.hostSessionUserEnded(name, s.osUser)),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final h = widget.host;
    final body = dsTextStyle(DsType.body).copyWith(color: colors.textPrimary);
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    final workdirs = {
      for (final r in h.reportedSessionUsers ?? const <HostReportedName>[])
        r.osUser: r.workdirs,
    };
    final editable = h.state != HostLifecycle.retired && !busy;
    final unassigned = offeredNames(h);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        for (final s in h.sessionUsers)
          Row(
            key: ValueKey('sessionUser.${s.userId}'),
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      '${personName(widget.people, s.userId)}: ${s.osUser}',
                      style: body,
                    ),
                    Text(
                      (workdirs[s.osUser] ?? const []).isEmpty
                          ? l.hostWorkdirsNone
                          : l.hostWorkdirs(workdirs[s.osUser]!.join(', ')),
                      style: secondary,
                    ),
                    if (s.ownerAccount) ownerAccountWarning(context, s.osUser),
                  ],
                ),
              ),
              if (editable)
                TextButton(
                  key: ValueKey('sessionUser.${s.userId}.end'),
                  onPressed: () => _end(s),
                  child: Text(l.hostSessionUserEnd),
                ),
            ],
          ),
        for (final n in unassigned)
          Row(
            key: ValueKey('reported.$n'),
            children: [
              Expanded(
                child: isOwnerAccount(h, n)
                    ? Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            l.hostSessionUsersUnassigned(n),
                            style: secondary,
                          ),
                          Text(l.hostOwnerAccountBadge, style: secondary),
                        ],
                      )
                    : Text(l.hostSessionUsersUnassigned(n), style: secondary),
              ),
              if (editable)
                TextButton(
                  key: ValueKey('reported.$n.assign'),
                  onPressed: () => _assign(name: n),
                  child: Text(l.hostSessionUserAssign),
                ),
            ],
          ),
        Text(l.hostSessionUsersSr12, style: secondary),
        if (editable && unassigned.isNotEmpty)
          Align(
            alignment: Alignment.centerLeft,
            child: TextButton(
              key: const ValueKey('sessionUsers.assign'),
              onPressed: _assign,
              child: Text(l.hostSessionUserAssignTitle),
            ),
          ),
        if (_problem case final text?)
          DsBanner(
            key: const ValueKey('sessionUsers.problem'),
            tone: DsTone.danger,
            liveRegion: true,
            text: text,
          ),
        noticeBanner(),
      ],
    );
  }
}

/// The dialog of "Assign": a person, a reported name (preselected with [name]) and a reason;
/// pops the assignment and the reason.
class _AssignDialog extends StatefulWidget {
  const _AssignDialog({required this.host, required this.people, this.name});

  final AdminHost host;
  final List<AdminPerson> people;
  final String? name;

  @override
  State<_AssignDialog> createState() => _AssignDialogState();
}

class _AssignDialogState extends State<_AssignDialog> {
  final TextEditingController _reason = TextEditingController();
  String? _person;
  late String? _name = widget.name;
  bool _optIn = false;

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
    final names = offeredNames(widget.host);
    final owner = names.contains(_name) && isOwnerAccount(widget.host, _name);
    return ConfirmChange(
      title: l.hostSessionUserAssignTitle,
      body: l.hostSessionUserAssignBody,
      confirmLabel: l.hostSessionUserAssign,
      fields: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          DropdownButtonFormField<String>(
            key: const ValueKey('assign.person'),
            initialValue: _person,
            decoration: InputDecoration(labelText: l.hostSessionUserPerson),
            items: [
              for (final p in widget.people)
                DropdownMenuItem(value: p.id, child: Text(p.displayName)),
            ],
            onChanged: (v) => setState(() => _person = v),
          ),
          DropdownButtonFormField<String>(
            key: const ValueKey('assign.name'),
            initialValue: names.contains(_name) ? _name : null,
            decoration: InputDecoration(labelText: l.hostSessionUserName),
            items: [
              for (final n in names) DropdownMenuItem(value: n, child: Text(n)),
            ],
            onChanged: (v) => setState(() {
              _name = v;
              _optIn = false;
            }),
          ),
          if (owner) ...[
            ownerAccountWarning(
              context,
              _name!,
              key: const ValueKey('assign.ownerAccountWarning'),
            ),
            CheckboxListTile(
              key: const ValueKey('assign.ownerAccount'),
              value: _optIn,
              title: Text(l.hostOwnerAccountOptIn(_name!)),
              onChanged: (v) => setState(() => _optIn = v ?? false),
            ),
          ],
          TextField(
            key: const ValueKey('confirm.reason'),
            controller: _reason,
            decoration: InputDecoration(labelText: l.adminReasonRequired),
          ),
        ],
      ),
      canConfirm:
          _person != null &&
          _name != null &&
          (!owner || _optIn) &&
          _reason.text.trim().isNotEmpty,
      onConfirm: (_) => Navigator.of(context).pop((
        HostAssignment(userId: _person!, osUser: _name!, ownerAccount: owner),
        _reason.text.trim(),
      )),
    );
  }
}
