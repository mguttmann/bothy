// SPDX-License-Identifier: Apache-2.0

/// The filters of the audit log: action, person, result, resource and time range, each one
/// query parameter of `listAuditEvents` (docs/spec/01-product.md 5.2 A6; brief M4-29 Design
/// note 1).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../data/admin_repository.dart';
import '../groups/member_editor.dart' show parseDay;
import 'audit_repository.dart';

/// The text of the result [result].
String auditResultText(AppLocalizations l, AuditResult result) =>
    switch (result) {
      AuditResult.allow => l.auditResultAllow,
      AuditResult.deny => l.auditResultDeny,
      AuditResult.error => l.auditResultError,
    };

/// The filter bar. [actions] are the action IDs of the loaded entries, offered as suggestions
/// of the action field, which also takes any typed ID (the catalogue is not shipped to the
/// client as a list of its own); [people] fill the person picker. Days are typed as
/// `YYYY-MM-DD` and read in [timeZone]: "from" is the start of its day, "to" the start of the
/// day after (the last day counts).
///
/// The dropdowns apply at once; the text fields apply on Enter or with "Apply filters".
/// [onChanged] receives the new filter.
class AuditFilters extends StatefulWidget {
  /// Creates the bar.
  const AuditFilters({
    super.key,
    required this.actions,
    required this.people,
    required this.onChanged,
    this.timeZone = 'UTC',
  });

  /// The action IDs offered as suggestions.
  final List<String> actions;

  /// The people of the person picker.
  final List<AdminPerson> people;

  /// Receives the filter to load.
  final ValueChanged<AuditFilter> onChanged;

  /// The time zone of the signed-in person.
  final String timeZone;

  @override
  State<AuditFilters> createState() => _AuditFiltersState();
}

class _AuditFiltersState extends State<AuditFilters> {
  final TextEditingController _kind = TextEditingController();
  final TextEditingController _id = TextEditingController();
  final TextEditingController _from = TextEditingController();
  final TextEditingController _to = TextEditingController();
  TextEditingController? _action;
  String? _actor;
  AuditResult? _result;
  bool _badFrom = false;
  bool _badTo = false;

  @override
  void dispose() {
    _kind.dispose();
    _id.dispose();
    _from.dispose();
    _to.dispose();
    super.dispose();
  }

  /// The start of the typed day [text] in the person's time zone, the day after it when
  /// [after]; null for an empty or invalid text.
  DateTime? _day(String text, {bool after = false}) {
    final day = parseDay(text);
    if (day == null) return null;
    final d = after ? DateTime.utc(day.year, day.month, day.day + 1) : day;
    final format = L10nFormat(Localizations.localeOf(context));
    try {
      return format.dayStart(d.year, d.month, d.day, timeZone: widget.timeZone);
    } on ArgumentError {
      return d;
    }
  }

  String? _text(TextEditingController? c) {
    final t = c?.text.trim() ?? '';
    return t.isEmpty ? null : t;
  }

  void _apply() {
    final fromText = _from.text.trim();
    final toText = _to.text.trim();
    final from = _day(fromText);
    final to = _day(toText, after: true);
    setState(() {
      _badFrom = fromText.isNotEmpty && from == null;
      _badTo = toText.isNotEmpty && to == null;
    });
    if (_badFrom || _badTo) return;
    widget.onChanged(
      AuditFilter(
        action: _text(_action),
        actorId: _actor,
        result: _result,
        resourceKind: _text(_kind),
        resourceId: _text(_id),
        from: from,
        to: to,
      ),
    );
  }

  Widget _field(
    String key,
    TextEditingController controller,
    String label, {
    double width = 180,
    String? error,
  }) => SizedBox(
    width: width,
    child: TextField(
      key: ValueKey(key),
      controller: controller,
      decoration: InputDecoration(labelText: label, errorText: error),
      onSubmitted: (_) => _apply(),
    ),
  );

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return Wrap(
      spacing: DsSpace.s12,
      runSpacing: DsSpace.s8,
      crossAxisAlignment: WrapCrossAlignment.center,
      children: [
        SizedBox(
          width: 220,
          child: Autocomplete<String>(
            optionsBuilder: (value) {
              final typed = value.text.trim().toLowerCase();
              return [
                for (final a in widget.actions)
                  if (a.toLowerCase().contains(typed)) a,
              ];
            },
            onSelected: (_) => _apply(),
            fieldViewBuilder: (context, controller, focus, _) {
              _action = controller;
              // Enter applies the typed text as it is (a free action ID or none); a
              // suggestion applies when the person picks it.
              return TextField(
                key: const ValueKey('audit.filter.action'),
                controller: controller,
                focusNode: focus,
                decoration: InputDecoration(labelText: l.auditFilterAction),
                onSubmitted: (_) => _apply(),
              );
            },
          ),
        ),
        DropdownButton<String?>(
          key: const ValueKey('audit.filter.actor'),
          value: _actor,
          hint: Text(l.auditFilterActorAll),
          items: [
            DropdownMenuItem(child: Text(l.auditFilterActorAll)),
            for (final p in widget.people)
              DropdownMenuItem(
                key: ValueKey('audit.filter.actor.${p.id}'),
                value: p.id,
                child: Text(p.displayName),
              ),
          ],
          onChanged: (id) {
            setState(() => _actor = id);
            _apply();
          },
        ),
        DropdownButton<AuditResult?>(
          key: const ValueKey('audit.filter.result'),
          value: _result,
          hint: Text(l.auditResultAll),
          items: [
            DropdownMenuItem(child: Text(l.auditResultAll)),
            for (final r in AuditResult.values)
              DropdownMenuItem(
                key: ValueKey('audit.filter.result.${r.name}'),
                value: r,
                child: Text(auditResultText(l, r)),
              ),
          ],
          onChanged: (r) {
            setState(() => _result = r);
            _apply();
          },
        ),
        _field('audit.filter.kind', _kind, l.auditFilterResourceKind),
        _field('audit.filter.id', _id, l.auditFilterResourceId, width: 300),
        _field(
          'audit.filter.from',
          _from,
          l.auditFilterFrom,
          error: _badFrom ? l.auditFilterInvalidDate : null,
        ),
        _field(
          'audit.filter.to',
          _to,
          l.auditFilterTo,
          error: _badTo ? l.auditFilterInvalidDate : null,
        ),
        FilledButton(
          key: const ValueKey('audit.filter.apply'),
          onPressed: _apply,
          child: Text(l.auditFilterApply),
        ),
      ],
    );
  }
}
