// SPDX-License-Identifier: Apache-2.0

/// A group: rename, delete, members with their expiry, add and remove members
/// (docs/spec/07-permissions.md 2.1; 05 `group.update`, `group.delete`, `group.member.add`,
/// `group.member.remove` with step-up).
library;

import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/admin_action.dart';
import '../common/confirm_change.dart';
import '../common/problem_banner.dart';
import '../data/admin_repository.dart';
import '../self/device_tile.dart' show activityText;
import '../self/self_feedback.dart';
import 'groups_list.dart' show GroupNameDialog;
import 'member_editor.dart';

/// The detail of the group [groupId]. Every change asks once and then runs through
/// [repository]; the step-up comes from the interceptor. A group whose members the identity
/// provider manages (`oidc_claim`) shows them without "Add member" and "Remove" (08 3.4).
/// [onBack] returns to the list; a deleted group returns there too.
class GroupDetail extends StatefulWidget {
  /// Creates the detail.
  const GroupDetail({
    super.key,
    required this.repository,
    required this.groupId,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
    this.onBack,
    this.onChanged,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The group.
  final String groupId;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// Returns to the list.
  final VoidCallback? onBack;

  /// Runs after a change.
  final VoidCallback? onChanged;

  @override
  State<GroupDetail> createState() => _GroupDetailState();
}

class _GroupDetailState extends State<GroupDetail>
    with AdminActionState<GroupDetail> {
  AdminGroup? _group;
  List<AdminGroupMember>? _members;
  List<AdminPerson> _people = const [];
  ApiProblem? _problem;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final group = await widget.repository.group(widget.groupId);
      final members = await widget.repository.members(widget.groupId);
      if (!mounted) return;
      setState(() {
        _group = group;
        _members = members;
        _problem = null;
      });
    } on Object catch (e) {
      if (mounted) setState(() => _problem = loadProblem(e));
      return;
    }
    try {
      final people = await widget.repository.people(
        status: AdminPersonStatus.active,
      );
      if (mounted) setState(() => _people = people.items);
    } on Object {
      // Without the people list "Add member" has no candidates and is not offered.
    }
  }

  Future<void> _change(
    String key,
    Future<SelfNotice?> Function() change,
  ) async {
    final ok = await runChange(key, change);
    if (ok) widget.onChanged?.call();
    if (mounted) await _load();
  }

  Future<void> _rename(AdminGroup group) async {
    final l = AppLocalizations.of(context);
    final named = await GroupNameDialog.show(context, current: group);
    if (named == null || !mounted) return;
    await _change('rename', () async {
      await widget.repository.renameGroup(
        group.id,
        named.name,
        purpose: l.stepUpPurposeRenameGroup(group.name),
      );
      return SelfNotice(l.adminGroupRenamed(named.name));
    });
  }

  Future<void> _delete(AdminGroup group) async {
    final l = AppLocalizations.of(context);
    final confirmed = await ConfirmChange.show(
      context,
      title: l.adminGroupDeleteTitle(group.name),
      body: l.adminGroupDeleteBody,
      confirmLabel: l.adminGroupDelete,
      destructive: true,
    );
    if (confirmed == null || !mounted) return;
    final ok = await runChange('delete', () async {
      await widget.repository.deleteGroup(
        group.id,
        purpose: l.stepUpPurposeDeleteGroup(group.name),
      );
      return null;
    });
    if (!ok) return;
    widget.onChanged?.call();
    widget.onBack?.call();
  }

  Future<void> _add(AdminGroup group) async {
    final l = AppLocalizations.of(context);
    final members = {for (final m in _members ?? const []) m.userId};
    final request = await MemberEditor.show(
      context,
      groupName: group.name,
      candidates: [
        for (final p in _people)
          if (!members.contains(p.id)) p,
      ],
    );
    if (request == null || !mounted) return;
    final name = request.person.displayName;
    await _change('add', () async {
      await widget.repository.addMember(
        group.id,
        request.person.id,
        expiresAt: request.expiresAt,
        purpose: l.stepUpPurposeAddMember(name, group.name),
      );
      return SelfNotice(l.adminMemberAdded(name, group.name));
    });
  }

  Future<void> _remove(AdminGroup group, AdminGroupMember member) async {
    final l = AppLocalizations.of(context);
    final confirmed = await ConfirmChange.show(
      context,
      title: l.adminMemberRemoveTitle(member.displayName, group.name),
      body: l.adminMemberRemoveBody(member.displayName),
      confirmLabel: l.adminMemberRemove,
      destructive: true,
    );
    if (confirmed == null || !mounted) return;
    await _change('${member.userId}.remove', () async {
      await widget.repository.removeMember(
        group.id,
        member.userId,
        purpose: l.stepUpPurposeRemoveMember(member.displayName, group.name),
      );
      return SelfNotice(l.adminMemberRemoved(member.displayName, group.name));
    });
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final group = _group;
    final members = _members;
    final now = widget.clock();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        if (widget.onBack case final back?)
          Align(
            alignment: Alignment.centerLeft,
            child: TextButton(
              key: const ValueKey('group.back'),
              onPressed: back,
              child: Text(l.adminGroupBack),
            ),
          ),
        if (_problem case final problem?)
          ProblemBanner(problem: problem, onRetry: _load)
        else if (group != null) ...[
          Semantics(
            header: true,
            child: Text(
              group.name,
              key: const ValueKey('group.name'),
              style: dsTextStyle(DsType.title)
                  .copyWith(color: colors.textPrimary),
            ),
          ),
          if (group.description.isNotEmpty)
            Text(
              group.description,
              style: dsTextStyle(DsType.body)
                  .copyWith(color: colors.textSecondary),
            ),
          if (group.fromProvider)
            Text(
              l.adminGroupFromProvider,
              style: dsTextStyle(DsType.bodySm)
                  .copyWith(color: colors.textSecondary),
            ),
          const SizedBox(height: DsSpace.s12),
          Wrap(
            spacing: DsSpace.s8,
            runSpacing: DsSpace.s8,
            children: [
              if (!denied.contains('rename'))
                OutlinedButton(
                  key: const ValueKey('group.rename'),
                  onPressed: busy ? null : () => _rename(group),
                  child: Text(l.adminGroupRename),
                ),
              if (!denied.contains('delete'))
                OutlinedButton(
                  key: const ValueKey('group.delete'),
                  style: OutlinedButton.styleFrom(
                    foregroundColor: colors.dangerFg,
                  ),
                  onPressed: busy ? null : () => _delete(group),
                  child: Text(l.adminGroupDelete),
                ),
            ],
          ),
          const SizedBox(height: DsSpace.s8),
          noticeBanner(),
          Padding(
            padding: const EdgeInsets.only(top: DsSpace.s16),
            child: Row(
              children: [
                Expanded(
                  child: Semantics(
                    header: true,
                    child: Text(
                      l.adminMembersTitle,
                      style: dsTextStyle(DsType.heading)
                          .copyWith(color: colors.textPrimary),
                    ),
                  ),
                ),
                if (!group.fromProvider &&
                    _people.isNotEmpty &&
                    !denied.contains('add'))
                  FilledButton(
                    key: const ValueKey('group.addMember'),
                    onPressed: busy ? null : () => _add(group),
                    child: Text(l.adminMemberAdd),
                  ),
              ],
            ),
          ),
          if (members != null && members.isEmpty)
            Text(
              l.adminMembersEmpty,
              style: dsTextStyle(DsType.body)
                  .copyWith(color: colors.textSecondary),
            ),
          for (final member in members ?? const <AdminGroupMember>[]) ...[
            Padding(
              key: ValueKey('member.${member.userId}'),
              padding: const EdgeInsets.symmetric(vertical: DsSpace.s8),
              child: Row(
                children: [
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          member.displayName,
                          style: dsTextStyle(DsType.bodyStrong)
                              .copyWith(color: colors.textPrimary),
                        ),
                        if (member.expiresAt case final until?)
                          Text(
                            l.adminMemberExpires(
                              activityText(
                                context,
                                until,
                                now: now,
                                timeZone: widget.timeZone,
                              ),
                            ),
                            key: ValueKey('member.${member.userId}.expires'),
                            style: dsTextStyle(DsType.bodySm)
                                .copyWith(color: colors.textSecondary),
                          ),
                      ],
                    ),
                  ),
                  if (!group.fromProvider &&
                      !denied.contains('${member.userId}.remove'))
                    OutlinedButton(
                      key: ValueKey('member.${member.userId}.remove'),
                      onPressed: busy ? null : () => _remove(group, member),
                      child: Text(l.adminMemberRemove),
                    ),
                ],
              ),
            ),
            Divider(height: 1, color: colors.borderSubtle),
          ],
        ],
      ],
    );
  }
}
