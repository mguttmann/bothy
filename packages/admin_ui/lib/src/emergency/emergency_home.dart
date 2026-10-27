// SPDX-License-Identifier: Apache-2.0

/// The first page of the emergency actions on the phone: the people with a search
/// (docs/spec/01-product.md 3.7 "Emergency actions on the phone", 4; R-PR-07; brief M5-09
/// Design notes 1 and 3).
library;

import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/problem_banner.dart';
import '../data/admin_repository.dart';
import '../people/people_list.dart' show personStatusText;

/// The people the caller may read (`listUsers` through [repository], the data source of the
/// administration, M4-26), searched by the server's `q` filter; a tap opens one person
/// ([onOpen]) with that person's devices, grants and "Disable".
///
/// The page names what the phone leaves out (creating grants, the audit log, statistics:
/// R-PR-07). A list the server refuses shows its reason and nothing else (15 1 principle 1);
/// a manager sees only the people the server returns. Pages follow the API cursor.
///
/// Example:
/// ```dart
/// EmergencyHome(repository: admin, onOpen: (id) => go('/admin/emergency/$id'));
/// ```
class EmergencyHome extends StatefulWidget {
  /// Creates the page.
  const EmergencyHome({
    super.key,
    required this.repository,
    required this.onOpen,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// Opens the emergency view of a person.
  final ValueChanged<String> onOpen;

  @override
  State<EmergencyHome> createState() => _EmergencyHomeState();
}

class _EmergencyHomeState extends State<EmergencyHome> {
  final TextEditingController _search = TextEditingController();
  List<AdminPerson>? _people;
  String? _next;
  ApiProblem? _problem;
  bool _failed = false;

  @override
  void initState() {
    super.initState();
    _load();
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
        cursor: more ? _next : null,
      );
      if (!mounted) return;
      setState(() {
        _people = [if (more) ...?_people, ...page.items];
        _next = page.nextCursor;
        _problem = null;
        _failed = false;
      });
    } on Object catch (e) {
      if (!mounted) return;
      setState(() {
        _problem = loadProblem(e);
        _failed = true;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final people = _people;
    return ListView(
      key: const ValueKey('emergency.home'),
      padding: const EdgeInsets.all(DsSpace.s16),
      // A page of headings, fields, rows, gaps and dividers: with semantic indexes every gap
      // and divider was an empty stop for TalkBack and VoiceOver and counted as an item
      // (brief M5-10).
      addSemanticIndexes: false,
      children: [
        Semantics(
          header: true,
          child: Text(
            l.emergencyTitle,
            style: dsTextStyle(DsType.title)
                .copyWith(color: colors.textPrimary),
          ),
        ),
        const SizedBox(height: DsSpace.s8),
        Text(
          l.emergencyIntro,
          key: const ValueKey('emergency.intro'),
          style: dsTextStyle(DsType.bodySm)
              .copyWith(color: colors.textSecondary),
        ),
        const SizedBox(height: DsSpace.s16),
        if (_failed)
          ProblemBanner(problem: _problem, onRetry: _load)
        else ...[
          TextField(
            key: const ValueKey('emergency.search'),
            controller: _search,
            textInputAction: TextInputAction.search,
            decoration: InputDecoration(
              labelText: l.emergencySearch,
              prefixIcon: DsIcon(DsIcons.actionSearch, decorative: true),
            ),
            onSubmitted: (_) => _load(),
          ),
          const SizedBox(height: DsSpace.s8),
          if (people != null && people.isEmpty)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: DsSpace.s16),
              child: Text(
                l.adminPeopleEmpty,
                key: const ValueKey('emergency.people.empty'),
                style: dsTextStyle(DsType.body)
                    .copyWith(color: colors.textSecondary),
              ),
            )
          else if (people != null)
            for (final person in people) ...[
              _PersonRow(
                person: person,
                onOpen: () => widget.onOpen(person.id),
              ),
              Divider(height: 1, color: colors.borderSubtle),
            ],
          if (_next != null)
            Padding(
              padding: const EdgeInsets.only(top: DsSpace.s8),
              child: DsButton(
                key: const ValueKey('emergency.people.more'),
                label: l.adminLoadMore,
                expand: true,
                onPressed: () => _load(more: true),
              ),
            ),
        ],
      ],
    );
  }
}

/// One person: avatar, name, handle, owner and state; the whole row opens the person.
class _PersonRow extends StatelessWidget {
  const _PersonRow({required this.person, required this.onOpen});

  final AdminPerson person;
  final VoidCallback onOpen;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return DsTappable(
      key: ValueKey('emergency.person.${person.id}'),
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
                    dsIsolate(person.displayName),
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
            DsIcon(
              DsIcons.actionExpand,
              size: DsIconSizeRole.sm,
              color: colors.textSecondary,
              decorative: true,
            ),
          ],
        ),
      ),
    );
  }
}
