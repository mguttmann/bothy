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

/// The subject of a grant: a person, a group or an agent from the directory lists, searched by
/// name. Apps are not offered: the shell has no Apps entry yet (M6-08).
///
/// [onKind] switches the list, [onSelect] picks an entry with its ID and name.
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

  /// Picks the subject with its ID and display name.
  final void Function(String id, String name) onSelect;

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

  List<(String, String)> _entries() {
    final n = widget.names;
    final all = switch (widget.kind) {
      AdminSubjectKind.user => [
        for (final p in n.people)
          if (p.status != AdminPersonStatus.deleted) (p.id, p.displayName),
      ],
      AdminSubjectKind.group => [for (final g in n.groups) (g.id, g.name)],
      AdminSubjectKind.agent => [
        for (final a in n.agents) (a.id, a.displayName),
      ],
      AdminSubjectKind.app => <(String, String)>[],
    };
    final q = _search.text.trim().toLowerCase();
    return [
      for (final e in all)
        if (q.isEmpty || e.$2.toLowerCase().contains(q)) e,
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
        for (final (id, name) in entries)
          ListTile(
            key: ValueKey('subject.$id'),
            title: Text(name),
            selected: id == widget.selectedId,
            selectedColor: colors.textPrimary,
            selectedTileColor: colors.accentSubtle,
            onTap: () => widget.onSelect(id, name),
          ),
      ],
    );
  }
}
