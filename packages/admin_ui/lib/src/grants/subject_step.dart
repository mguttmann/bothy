// SPDX-License-Identifier: Apache-2.0

/// Step 1 of the grant wizard: who (docs/spec/01-product.md 5.2 A2; 07 2.1; brief M4-27
/// Design note 1).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../data/access_repository.dart';
import '../data/admin_repository.dart';
import 'grant_labels.dart';

/// One entry of the list: the ID, the name, what tells it from an entry with the same name (the
/// handle of a person, the slug of an app; null for a group and an agent) and the name the
/// step-up of the grant shows.
typedef _Entry = ({String id, String name, String? detail, String purposeName});

/// The subject of a grant: a person, a group, an agent or an app from the directory lists,
/// searched by name.
///
/// A person shows the handle below the name and is found by it too, because two people can
/// carry the same display name and only the handle is unique (03 `users`; ruling AUD-AM1/4).
///
/// "App" is offered when the directory lists an app (`listApps`: the installed ones, also a
/// suspended one, whose grants remain, 11 4.3): a second target of an installed app, a target
/// left out at the installation and a target after an update are created here (11 4.1, 4.2;
/// ruling AUD-AM1/1). What an app may get is the server's rule (at most `write` and `upload`,
/// 05), told by the preview like for an agent.
///
/// [onKind] switches the list, [onSelect] picks an entry with its ID, its name and the name
/// for the purpose of the step-up (a person with the handle).
class SubjectStep extends StatefulWidget {
  /// Creates the step.
  const SubjectStep({
    super.key,
    required this.names,
    required this.kind,
    required this.selectedId,
    required this.onKind,
    required this.onSelect,
  });

  /// The directory lists.
  final AccessNames names;

  /// The kind whose list is shown.
  final AdminSubjectKind kind;

  /// The chosen subject, or null.
  final String? selectedId;

  /// Switches the kind.
  final ValueChanged<AdminSubjectKind> onKind;

  /// Picks the subject with its ID, its display name and the name the step-up shows.
  final void Function(String id, String name, String purposeName) onSelect;

  @override
  State<SubjectStep> createState() => _SubjectStepState();
}

class _SubjectStepState extends State<SubjectStep> {
  final TextEditingController _search = TextEditingController();

  @override
  void initState() {
    super.initState();
    _search.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _search.dispose();
    super.dispose();
  }

  List<_Entry> _entries() {
    final n = widget.names;
    _Entry plain(String id, String name) =>
        (id: id, name: name, detail: null, purposeName: name);
    final List<_Entry> all = switch (widget.kind) {
      AdminSubjectKind.user => [
        for (final p in n.people)
          if (p.status != AdminPersonStatus.deleted)
            (
              id: p.id,
              name: p.displayName,
              detail: '@${p.handle}',
              purposeName: p.nameWithHandle,
            ),
      ],
      AdminSubjectKind.group => [for (final g in n.groups) plain(g.id, g.name)],
      AdminSubjectKind.agent => [
        for (final a in n.agents) plain(a.id, a.displayName),
      ],
      AdminSubjectKind.app => [
        for (final a in n.apps)
          (
            id: a.id,
            name: a.displayName,
            detail: a.slug,
            purposeName: a.displayName,
          ),
      ],
    };
    final q = _search.text.trim().toLowerCase();
    return [
      for (final e in all)
        if (q.isEmpty ||
            e.name.toLowerCase().contains(q) ||
            (e.detail?.toLowerCase().contains(q) ?? false))
          e,
    ];
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final entries = _entries();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Wrap(
          spacing: DsSpace.s8,
          children: [
            for (final (kind, label) in [
              (AdminSubjectKind.user, l.adminWizardKindPerson),
              (AdminSubjectKind.group, l.adminWizardKindGroup),
              (AdminSubjectKind.agent, l.adminWizardKindAgent),
              if (widget.names.apps.isNotEmpty ||
                  widget.kind == AdminSubjectKind.app)
                (AdminSubjectKind.app, l.adminWizardKindApp),
            ])
              ChoiceChip(
                key: ValueKey('wizard.kind.${kind.name}'),
                label: Text(label),
                selected: widget.kind == kind,
                onSelected: (_) => widget.onKind(kind),
              ),
          ],
        ),
        TextField(
          key: const ValueKey('wizard.search'),
          controller: _search,
          decoration: InputDecoration(labelText: l.adminWizardSearch),
        ),
        const SizedBox(height: DsSpace.s8),
        if (entries.isEmpty)
          Text(
            l.adminWizardNoMatch,
            style: dsTextStyle(DsType.body)
                .copyWith(color: colors.textSecondary),
          ),
        for (final e in entries)
          ListTile(
            key: ValueKey('subject.${e.id}'),
            title: Text(e.name),
            subtitle: switch (e.detail) {
              final detail? => Text(detail),
              null => null,
            },
            selected: e.id == widget.selectedId,
            selectedColor: colors.textPrimary,
            selectedTileColor: colors.accentSubtle,
            onTap: () => widget.onSelect(e.id, e.name, e.purposeName),
          ),
      ],
    );
  }
}
