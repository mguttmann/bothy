// SPDX-License-Identifier: Apache-2.0

/// The groups: list and "Create group" (docs/spec/07-permissions.md 2.1; 05 `group.create`
/// with step-up).
library;

import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/admin_action.dart';
import '../common/confirm_change.dart';
import '../common/problem_banner.dart';
import '../data/admin_repository.dart';
import '../self/self_feedback.dart';

/// The groups of the instance with their member counts; pages follow the API cursor. "Create
/// group" asks once with the name and description ([GroupNameDialog]). A `403` of the list
/// shows the server's reason instead. [onOpen] opens a group.
class GroupsList extends StatefulWidget {
  /// Creates the list.
  const GroupsList({
    super.key,
    required this.repository,
    required this.onOpen,
    this.onChanged,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// Opens a group.
  final ValueChanged<String> onOpen;

  /// Runs after a change.
  final VoidCallback? onChanged;

  @override
  State<GroupsList> createState() => _GroupsListState();
}

class _GroupsListState extends State<GroupsList>
    with AdminActionState<GroupsList> {
  List<AdminGroup>? _groups;
  String? _next;
  ApiProblem? _problem;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load({bool more = false}) async {
    try {
      final page = await widget.repository.groups(cursor: more ? _next : null);
      if (!mounted) return;
      setState(() {
        _groups = [if (more) ...?_groups, ...page.items];
        _next = page.nextCursor;
        _problem = null;
      });
    } on Object catch (e) {
      if (mounted) setState(() => _problem = loadProblem(e));
    }
  }

  Future<void> _create() async {
    final l = AppLocalizations.of(context);
    final named = await GroupNameDialog.show(context);
    if (named == null || !mounted) return;
    AdminGroup? created;
    final ok = await runChange('create', () async {
      created = await widget.repository.createGroup(
        named.name,
        description: named.description,
        purpose: l.stepUpPurposeCreateGroup(named.name),
      );
      return SelfNotice(l.adminGroupCreated(named.name));
    });
    if (ok) widget.onChanged?.call();
    if (!mounted) return;
    await _load();
    if (created case final group?) widget.onOpen(group.id);
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final groups = _groups;
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
                  l.adminGroupsTitle,
                  style: dsTextStyle(DsType.title)
                      .copyWith(color: colors.textPrimary),
                ),
              ),
            ),
            if (groups != null && !denied.contains('create'))
              FilledButton(
                key: const ValueKey('groups.create'),
                onPressed: busy ? null : _create,
                child: Text(l.adminGroupsCreate),
              ),
          ],
        ),
        const SizedBox(height: DsSpace.s12),
        noticeBanner(),
        if (_problem case final problem?)
          ProblemBanner(problem: problem, onRetry: _load)
        else if (groups != null && groups.isEmpty)
          Text(
            l.adminGroupsEmpty,
            style: dsTextStyle(DsType.body)
                .copyWith(color: colors.textSecondary),
          )
        else if (groups != null)
          for (final group in groups) ...[
            InkWell(
              key: ValueKey('group.${group.id}'),
              onTap: () => widget.onOpen(group.id),
              child: Padding(
                padding: const EdgeInsets.symmetric(vertical: DsSpace.s8),
                child: Row(
                  children: [
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            group.name,
                            style: dsTextStyle(DsType.bodyStrong)
                                .copyWith(color: colors.textPrimary),
                          ),
                          if (group.description.isNotEmpty)
                            Text(
                              group.description,
                              style: dsTextStyle(DsType.bodySm)
                                  .copyWith(color: colors.textSecondary),
                            ),
                        ],
                      ),
                    ),
                    Text(
                      l.adminGroupMembers(group.memberCount),
                      style: dsTextStyle(DsType.bodySm)
                          .copyWith(color: colors.textSecondary),
                    ),
                  ],
                ),
              ),
            ),
            Divider(height: 1, color: colors.borderSubtle),
          ],
        if (_next != null)
          Align(
            alignment: Alignment.centerLeft,
            child: TextButton(
              key: const ValueKey('groups.more'),
              onPressed: () => _load(more: true),
              child: Text(l.adminLoadMore),
            ),
          ),
      ],
    );
  }
}

/// What [GroupNameDialog] asks for.
final class GroupName {
  /// Creates the answer.
  const GroupName(this.name, this.description);

  /// The name.
  final String name;

  /// The description (empty for none).
  final String description;
}

/// The confirmation of "Create group" (without [current]) or "Rename" (with [current]) with
/// its fields ([ConfirmChange]). Pops a [GroupName] or null.
class GroupNameDialog extends StatefulWidget {
  /// Creates the dialog; [current] is the group to rename, or null to create one.
  const GroupNameDialog({super.key, this.current});

  /// The group to rename, or null.
  final AdminGroup? current;

  /// Shows the dialog over [context].
  static Future<GroupName?> show(BuildContext context, {AdminGroup? current}) =>
      showDialog<GroupName>(
        context: context,
        builder: (_) => GroupNameDialog(current: current),
      );

  @override
  State<GroupNameDialog> createState() => _GroupNameDialogState();
}

class _GroupNameDialogState extends State<GroupNameDialog> {
  late final TextEditingController _name = TextEditingController(
    text: widget.current?.name ?? '',
  );
  final TextEditingController _description = TextEditingController();

  @override
  void initState() {
    super.initState();
    _name.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _name.dispose();
    _description.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final current = widget.current;
    final name = _name.text.trim();
    return ConfirmChange(
      title: current == null
          ? l.adminGroupCreateTitle
          : l.adminGroupRenameTitle(current.name),
      body: current == null ? l.adminGroupCreateBody : l.adminGroupRenameBody,
      confirmLabel: current == null ? l.adminGroupsCreate : l.adminGroupRename,
      canConfirm: name.isNotEmpty && name != current?.name,
      onConfirm: (_) =>
          Navigator.of(context).pop(GroupName(name, _description.text.trim())),
      fields: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          TextField(
            key: const ValueKey('groupName.name'),
            controller: _name,
            autofocus: true,
            decoration: InputDecoration(labelText: l.adminGroupName),
          ),
          if (current == null)
            TextField(
              key: const ValueKey('groupName.description'),
              controller: _description,
              decoration: InputDecoration(labelText: l.adminGroupDescription),
            ),
        ],
      ),
    );
  }
}
