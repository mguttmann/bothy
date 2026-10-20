// SPDX-License-Identifier: Apache-2.0

/// A1 People: the list with search and filters, "Add person" and the owners
/// (docs/spec/01-product.md 5.2 A1; 08 3.2; brief M4-26 Scope).
library;

import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/admin_action.dart';
import '../common/problem_banner.dart';
import '../data/admin_repository.dart';
import '../self/self_feedback.dart';
import 'create_person_dialog.dart';
import 'enrollment_code_view.dart';
import 'owners_section.dart';

/// The name of the state [status].
String personStatusText(AppLocalizations l, AdminPersonStatus status) =>
    switch (status) {
      AdminPersonStatus.pending => l.adminStatusPending,
      AdminPersonStatus.active => l.adminStatusActive,
      AdminPersonStatus.disabled => l.adminStatusDisabled,
      AdminPersonStatus.deleted => l.adminStatusDeleted,
    };

/// The people of the instance: search and the state filter are request parameters of
/// `listUsers`; the group filter has no request parameter and filters the loaded page by the
/// group's members, with a visible note (brief M4-26 Design note 1). Pages follow the API
/// cursor ("Load more").
///
/// "Add person" asks with [CreatePersonDialog] and shows the enrollment code once
/// ([EnrollmentCodeView]). A `403` of the list shows the server's reason instead of the list
/// and no other control (fixed requirement 9). [onOpen] opens a person's detail.
///
/// Example:
/// ```dart
/// PeopleList(repository: admin, onOpen: (id) => go('/admin/people/$id'));
/// ```
class PeopleList extends StatefulWidget {
  /// Creates the list.
  const PeopleList({
    super.key,
    required this.repository,
    required this.onOpen,
    this.timeZone = 'UTC',
    this.onChanged,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// Opens the detail of a person.
  final ValueChanged<String> onOpen;

  /// The time zone of the signed-in person (`users.time_zone`).
  final String timeZone;

  /// Runs after a change (the shell reloads its count).
  final VoidCallback? onChanged;

  @override
  State<PeopleList> createState() => _PeopleListState();
}

class _PeopleListState extends State<PeopleList>
    with AdminActionState<PeopleList> {
  final TextEditingController _search = TextEditingController();
  List<AdminPerson>? _people;
  String? _next;
  bool _loaded = false;
  ApiProblem? _problem;
  AdminPersonStatus? _status;
  List<AdminGroup> _groups = const [];
  String? _group;
  Set<String>? _groupMembers;

  @override
  void initState() {
    super.initState();
    _load();
    _loadGroups();
  }

  @override
  void dispose() {
    _search.dispose();
    super.dispose();
  }

  Future<void> _load({bool more = false}) async {
    try {
      final page = await widget.repository.people(
        query: _search.text.trim(),
        status: _status,
        cursor: more ? _next : null,
      );
      if (!mounted) return;
      setState(() {
        _people = [if (more) ...?_people, ...page.items];
        _next = page.nextCursor;
        _problem = null;
        _loaded = true;
      });
    } on Object catch (e) {
      if (!mounted) return;
      setState(() {
        _problem = loadProblem(e);
        _loaded = true;
      });
    }
  }

  Future<void> _loadGroups() async {
    try {
      final page = await widget.repository.groups();
      if (mounted) setState(() => _groups = page.items);
    } on Object {
      // Without groups the group filter is not offered.
    }
  }

  Future<void> _filterGroup(String? groupId) async {
    setState(() {
      _group = groupId;
      _groupMembers = null;
    });
    if (groupId == null) return;
    try {
      final members = await widget.repository.members(groupId);
      if (!mounted || _group != groupId) return;
      setState(() => _groupMembers = {for (final m in members) m.userId});
    } on Object catch (e) {
      if (!mounted) return;
      setState(() {
        _group = null;
        notice = SelfNotice(
          adminFailure(AppLocalizations.of(context), e)?.text ?? '',
          tone: DsTone.danger,
        );
      });
    }
  }

  Future<void> _create() async {
    final l = AppLocalizations.of(context);
    final request = await CreatePersonDialog.show(context);
    if (request == null || !mounted) return;
    PersonCreated? created;
    final ok = await runChange('create', () async {
      created = await widget.repository.createPerson(
        handle: request.handle,
        displayName: request.displayName,
        localAccount: request.localAccount,
        purpose: l.stepUpPurposeCreatePerson(request.displayName),
      );
      return SelfNotice(l.adminPersonCreated(request.displayName));
    });
    if (!ok || !mounted) return;
    widget.onChanged?.call();
    final code = created?.enrollment;
    // 08 3.2: the code is shown once; the dialog holds the only copy.
    created = null;
    if (code != null) {
      await EnrollmentCodeView.show(
        context,
        name: request.displayName,
        code: code,
        timeZone: widget.timeZone,
      );
    }
    if (mounted) await _load();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final people = _people;
    final members = _groupMembers;
    final shown = people == null
        ? null
        : [
            for (final p in people)
              if (members == null || members.contains(p.id)) p,
          ];
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
                  l.adminPeopleTitle,
                  style: dsTextStyle(DsType.title)
                      .copyWith(color: colors.textPrimary),
                ),
              ),
            ),
            if (_problem == null && !denied.contains('create'))
              FilledButton(
                key: const ValueKey('people.add'),
                onPressed: busy || people == null ? null : _create,
                child: Text(l.adminPeopleAdd),
              ),
          ],
        ),
        const SizedBox(height: DsSpace.s12),
        noticeBanner(),
        if (_problem case final problem?)
          ProblemBanner(problem: problem, onRetry: _load)
        else ...[
          Wrap(
            spacing: DsSpace.s12,
            runSpacing: DsSpace.s8,
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              SizedBox(
                width: 280,
                child: TextField(
                  key: const ValueKey('people.search'),
                  controller: _search,
                  decoration: InputDecoration(
                    labelText: l.adminPeopleSearch,
                    prefixIcon: DsIcon(DsIcons.actionSearch, decorative: true),
                  ),
                  onSubmitted: (_) => _load(),
                ),
              ),
              DropdownButton<AdminPersonStatus?>(
                key: const ValueKey('people.status'),
                value: _status,
                hint: Text(l.adminPeopleStatusAll),
                items: [
                  DropdownMenuItem(child: Text(l.adminPeopleStatusAll)),
                  for (final s in const [
                    AdminPersonStatus.active,
                    AdminPersonStatus.pending,
                    AdminPersonStatus.disabled,
                  ])
                    DropdownMenuItem(
                      key: ValueKey('people.status.${s.name}'),
                      value: s,
                      child: Text(personStatusText(l, s)),
                    ),
                ],
                onChanged: (s) {
                  setState(() => _status = s);
                  _load();
                },
              ),
              if (_groups.isNotEmpty)
                DropdownButton<String?>(
                  key: const ValueKey('people.group'),
                  value: _group,
                  hint: Text(l.adminPeopleGroupAll),
                  items: [
                    DropdownMenuItem(child: Text(l.adminPeopleGroupAll)),
                    for (final g in _groups)
                      DropdownMenuItem(
                        key: ValueKey('people.group.${g.id}'),
                        value: g.id,
                        child: Text(g.name),
                      ),
                  ],
                  onChanged: _filterGroup,
                ),
            ],
          ),
          if (_group != null)
            Padding(
              padding: const EdgeInsets.only(top: DsSpace.s8),
              child: Text(
                l.adminPeopleGroupNote,
                key: const ValueKey('people.groupNote'),
                style: dsTextStyle(DsType.bodySm)
                    .copyWith(color: colors.textSecondary),
              ),
            ),
          const SizedBox(height: DsSpace.s8),
          if (!_loaded)
            const SizedBox.shrink()
          else if (shown != null && shown.isEmpty)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: DsSpace.s16),
              child: Text(
                l.adminPeopleEmpty,
                style: dsTextStyle(DsType.body)
                    .copyWith(color: colors.textSecondary),
              ),
            )
          else if (shown != null)
            for (final person in shown) ...[
              _PersonRow(
                person: person,
                onOpen: () => widget.onOpen(person.id),
              ),
              Divider(height: 1, color: colors.borderSubtle),
            ],
          if (_next != null)
            Align(
              alignment: Alignment.centerLeft,
              child: TextButton(
                key: const ValueKey('people.more'),
                onPressed: () => _load(more: true),
                child: Text(l.adminLoadMore),
              ),
            ),
          const SizedBox(height: DsSpace.s24),
          OwnersSection(
            repository: widget.repository,
            timeZone: widget.timeZone,
            onChanged: () {
              widget.onChanged?.call();
              _load();
            },
          ),
        ],
      ],
    );
  }
}

class _PersonRow extends StatelessWidget {
  const _PersonRow({required this.person, required this.onOpen});

  final AdminPerson person;
  final VoidCallback onOpen;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return InkWell(
      key: ValueKey('person.${person.id}'),
      onTap: onOpen,
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: DsSpace.s8),
        child: Row(
          children: [
            DsAvatar(name: person.displayName, origin: DsMessageOrigin.human),
            const SizedBox(width: DsSpace.s12),
            Expanded(
              child: Wrap(
                spacing: DsSpace.s8,
                runSpacing: DsSpace.s4,
                crossAxisAlignment: WrapCrossAlignment.center,
                children: [
                  Text(
                    person.displayName,
                    style: dsTextStyle(DsType.bodyStrong)
                        .copyWith(color: colors.textPrimary),
                  ),
                  Text(
                    '@${person.handle}',
                    style: dsTextStyle(DsType.bodySm)
                        .copyWith(color: colors.textSecondary),
                  ),
                  if (person.isOwner)
                    DsBadge(
                      text: l.adminOwnerBadge,
                      background: colors.accentSubtle,
                      foreground: colors.textPrimary,
                    ),
                  if (person.status != AdminPersonStatus.active)
                    DsBadge(
                      text: personStatusText(l, person.status),
                      background: person.status == AdminPersonStatus.pending
                          ? colors.attentionSubtle
                          : colors.dangerSubtle,
                      foreground: colors.textPrimary,
                    ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
