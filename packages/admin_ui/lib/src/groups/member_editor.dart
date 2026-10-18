// SPDX-License-Identifier: Apache-2.0

/// Adding a member to a group, with an optional end of the membership
/// (docs/spec/07-permissions.md 2.1: members with `expires_at`; 05 `group.member.add` with
/// step-up).
library;

import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/confirm_change.dart';
import '../data/admin_repository.dart';

/// What [MemberEditor] asks for: the person and, optionally, the day the membership ends.
final class MemberRequest {
  /// Creates the request.
  const MemberRequest(this.person, {this.expiresAt});

  /// The new member.
  final AdminPerson person;

  /// The end of the membership (the start of that day in UTC), or null.
  final DateTime? expiresAt;
}

/// Parses `YYYY-MM-DD` into the start of that day in UTC; null for any other text.
///
/// Example: `parseDay('2026-12-31')` is `DateTime.utc(2026, 12, 31)`.
DateTime? parseDay(String text) {
  final m = RegExp(r'^(\d{4})-(\d{2})-(\d{2})$').firstMatch(text.trim());
  if (m == null) return null;
  final day = DateTime.utc(
    int.parse(m[1]!),
    int.parse(m[2]!),
    int.parse(m[3]!),
  );
  // DateTime rolls over invalid days (2026-02-30); such a text is no date.
  if (day.month != int.parse(m[2]!) || day.day != int.parse(m[3]!)) return null;
  return day;
}

/// The confirmation of "Add member" for [groupName] with the person (one of [candidates]) and
/// an optional expiry as `YYYY-MM-DD` ([ConfirmChange]). Pops a [MemberRequest] or null.
class MemberEditor extends StatefulWidget {
  /// Creates the editor.
  const MemberEditor({
    super.key,
    required this.groupName,
    required this.candidates,
  });

  /// The name of the group.
  final String groupName;

  /// The people who can be added.
  final List<AdminPerson> candidates;

  /// Shows the editor over [context].
  static Future<MemberRequest?> show(
    BuildContext context, {
    required String groupName,
    required List<AdminPerson> candidates,
  }) => showDialog<MemberRequest>(
    context: context,
    builder: (_) => MemberEditor(groupName: groupName, candidates: candidates),
  );

  @override
  State<MemberEditor> createState() => _MemberEditorState();
}

class _MemberEditorState extends State<MemberEditor> {
  final TextEditingController _until = TextEditingController();
  AdminPerson? _person;

  @override
  void initState() {
    super.initState();
    _until.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _until.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final text = _until.text.trim();
    final until = parseDay(text);
    final validDate = text.isEmpty || until != null;
    return ConfirmChange(
      title: l.adminMemberAddTitle(widget.groupName),
      body: l.adminMemberAddBody,
      confirmLabel: l.adminMemberAdd,
      canConfirm: _person != null && validDate,
      onConfirm: (_) =>
          Navigator.of(context).pop(MemberRequest(_person!, expiresAt: until)),
      fields: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          DropdownButton<AdminPerson>(
            key: const ValueKey('member.person'),
            isExpanded: true,
            value: _person,
            hint: Text(l.adminPickPerson),
            items: [
              for (final p in widget.candidates)
                DropdownMenuItem(
                  key: ValueKey('member.person.${p.id}'),
                  value: p,
                  child: Text(p.displayName),
                ),
            ],
            onChanged: (p) => setState(() => _person = p),
          ),
          TextField(
            key: const ValueKey('member.until'),
            controller: _until,
            keyboardType: TextInputType.datetime,
            decoration: InputDecoration(
              labelText: l.adminMemberUntil,
              errorText: validDate ? null : l.adminMemberUntilInvalid,
            ),
          ),
        ],
      ),
    );
  }
}
