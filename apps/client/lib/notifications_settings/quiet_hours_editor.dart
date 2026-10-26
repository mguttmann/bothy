// SPDX-License-Identifier: Apache-2.0

/// The quiet hours of D9 (01 5.1 D9; 13 3.4; brief M4-33 Scope): the summary of a rule, the
/// list with Add, Edit and Delete, and the editor of one rule with its weekdays, start, end,
/// "All day", time zone and switch.
///
/// The app never decides whether a person is inside quiet hours; it only edits the rules the
/// server checks at send time (13 3.4).
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../settings/settings_providers.dart';
import 'providers.dart';

/// The start and end of an "All day" rule: a window of 24 hours from midnight on each chosen
/// day (13 3.4 item 3; D-M3-17-4).
const String allDayTime = '00:00';

/// The bit of [weekday] (`DateTime.monday` 1 to `DateTime.sunday` 7) in the mask of 13 3.4,
/// where bit 0 is Monday.
int weekdayBit(int weekday) => 1 << (weekday - 1);

/// Whether [rule] is "All day" (`00:00` to `00:00`).
bool isAllDay(QuietHours rule) =>
    rule.startLocal == allDayTime && rule.endLocal == allDayTime;

/// Whether [text] is a time `HH:MM` of 00:00 to 23:59.
bool isLocalTime(String text) =>
    RegExp(r'^([01][0-9]|2[0-3]):[0-5][0-9]$').hasMatch(text);

/// The short names of the weekdays, Monday first.
List<String> weekdayNames(AppLocalizations l) => [
  l.quietHoursDayMon,
  l.quietHoursDayTue,
  l.quietHoursDayWed,
  l.quietHoursDayThu,
  l.quietHoursDayFri,
  l.quietHoursDaySat,
  l.quietHoursDaySun,
];

/// The days of [mask] as runs, for example `Mon-Fri` or `Mon, Wed, Sat-Sun`.
String weekdaysLabel(AppLocalizations l, int mask) {
  final names = weekdayNames(l);
  final runs = <String>[];
  var i = 0;
  while (i < 7) {
    if (mask & (1 << i) == 0) {
      i++;
      continue;
    }
    var j = i;
    while (j + 1 < 7 && mask & (1 << (j + 1)) != 0) {
      j++;
    }
    runs.add(
      j == i
          ? names[i]
          : j == i + 1
          ? '${names[i]}, ${names[j]}'
          : '${names[i]}-${names[j]}',
    );
    i = j + 1;
  }
  return runs.join(', ');
}

/// The summary of [rule] as D9 shows it, for example `Mon-Fri 19:00 to 07:00` or
/// `Sat, Sun All day`; a rule that is off says so, a rule in a zone of its own names it.
String quietHoursSummary(AppLocalizations l, QuietHours rule) {
  final range = isAllDay(rule)
      ? l.quietHoursAllDay
      : l.quietHoursRange(rule.startLocal, rule.endLocal);
  final zone = rule.timeZone;
  return [
    weekdaysLabel(l, rule.weekdays),
    range,
    if (zone != null) '($zone)',
    if (!rule.enabled) '(${l.quietHoursOff})',
  ].join(' ');
}

/// What the editor returns: the rule to store, or the wish to delete it.
sealed class QuietHoursEdit {
  const QuietHoursEdit();
}

/// Store [rule] (for a new rule its ID and version are placeholders).
final class QuietHoursSave extends QuietHoursEdit {
  /// Creates the result.
  const QuietHoursSave(this.rule);

  /// The rule.
  final QuietHours rule;
}

/// Delete the edited rule.
final class QuietHoursDelete extends QuietHoursEdit {
  /// Creates the result.
  const QuietHoursDelete();
}

/// The placeholder ID of a rule the editor creates; the writer gives it its own.
final QuietHoursId newRulePlaceholder = QuietHoursId.parse(
  '00000000-0000-7000-8000-000000000000',
);

/// Opens the editor for [rule], or for a new rule (Monday to Friday, 19:00 to 07:00, the
/// person's zone) when null; [personZone] is the zone a rule without its own follows.
Future<QuietHoursEdit?> showQuietHoursEditor(
  BuildContext context, {
  QuietHours? rule,
  required String personZone,
  List<String> Function() timeZones = ianaTimeZones,
}) => showDialog<QuietHoursEdit>(
  context: context,
  builder: (_) => QuietHoursEditor(
    rule: rule,
    personZone: personZone,
    timeZones: timeZones,
  ),
);

/// The editor dialog of one rule.
class QuietHoursEditor extends StatefulWidget {
  /// Creates the editor of [rule] (null: a new rule).
  const QuietHoursEditor({
    super.key,
    required this.rule,
    required this.personZone,
    this.timeZones = ianaTimeZones,
  });

  /// The rule, or null for a new one.
  final QuietHours? rule;

  /// The person's zone, the default of a rule (13 3.4: `time_zone` NULL).
  final String personZone;

  /// The zones offered.
  final List<String> Function() timeZones;

  @override
  State<QuietHoursEditor> createState() => _QuietHoursEditorState();
}

class _QuietHoursEditorState extends State<QuietHoursEditor> {
  late int _mask = widget.rule?.weekdays ?? 0x1f;
  late bool _allDay = widget.rule != null && isAllDay(widget.rule!);
  late final TextEditingController _start = TextEditingController(
    text: widget.rule?.startLocal ?? '19:00',
  );
  late final TextEditingController _end = TextEditingController(
    text: widget.rule?.endLocal ?? '07:00',
  );
  late String? _zone = widget.rule?.timeZone;
  late bool _enabled = widget.rule?.enabled ?? true;
  late final List<String> _zones = widget.timeZones();
  bool _tried = false;

  @override
  void dispose() {
    _start.dispose();
    _end.dispose();
    super.dispose();
  }

  bool get _valid =>
      _mask != 0 &&
      (_allDay || (isLocalTime(_start.text) && isLocalTime(_end.text)));

  void _save() {
    setState(() => _tried = true);
    if (!_valid) return;
    final old = widget.rule;
    Navigator.of(context).pop(
      QuietHoursSave(
        QuietHours(
          id: old?.id ?? newRulePlaceholder,
          weekdays: _mask,
          startLocal: _allDay ? allDayTime : _start.text,
          endLocal: _allDay ? allDayTime : _end.text,
          timeZone: _zone,
          enabled: _enabled,
          version: old?.version ?? const ResourceVersion(0),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final names = weekdayNames(l);
    String? timeError(TextEditingController c) =>
        _tried && !_allDay && !isLocalTime(c.text)
        ? l.quietHoursTimeInvalid
        : null;
    final label = dsTextStyle(DsType.bodyStrong)
        .copyWith(color: colors.textPrimary);
    return AlertDialog(
      key: const ValueKey('quiet.editor'),
      title: Text(l.quietHoursEditorTitle),
      content: SizedBox(
        width: 420,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(l.quietHoursDays, style: label),
            const SizedBox(height: DsSpace.s4),
            Wrap(
              spacing: DsSpace.s4,
              runSpacing: DsSpace.s4,
              children: [
                for (var i = 0; i < 7; i++)
                  FilterChip(
                    key: ValueKey('quiet.day.$i'),
                    label: Text(names[i]),
                    showCheckmark: false,
                    selected: _mask & (1 << i) != 0,
                    onSelected: (on) => setState(
                      () => _mask = on ? _mask | (1 << i) : _mask & ~(1 << i),
                    ),
                  ),
              ],
            ),
            if (_tried && _mask == 0)
              Text(
                l.quietHoursNoDays,
                key: const ValueKey('quiet.noDays'),
                style: dsTextStyle(DsType.bodySm)
                    .copyWith(color: colors.dangerFg),
              ),
            CheckboxListTile(
              key: const ValueKey('quiet.allDay'),
              contentPadding: EdgeInsets.zero,
              value: _allDay,
              onChanged: (on) => setState(() => _allDay = on ?? false),
              title: Text(l.quietHoursAllDay),
            ),
            if (!_allDay)
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Expanded(
                    child: TextField(
                      key: const ValueKey('quiet.start'),
                      controller: _start,
                      decoration: InputDecoration(
                        labelText: l.quietHoursStart,
                        errorText: timeError(_start),
                      ),
                      onChanged: (_) => setState(() {}),
                    ),
                  ),
                  const SizedBox(width: DsSpace.s12),
                  Expanded(
                    child: TextField(
                      key: const ValueKey('quiet.end'),
                      controller: _end,
                      decoration: InputDecoration(
                        labelText: l.quietHoursEnd,
                        errorText: timeError(_end),
                      ),
                      onChanged: (_) => setState(() {}),
                    ),
                  ),
                ],
              ),
            const SizedBox(height: DsSpace.s12),
            // The field carries its name as its label (brief M4-32).
            DsDropdownField<String>(
              key: const ValueKey('quiet.zone'),
              label: l.quietHoursTimeZone,
              width: 300,
              enableFilter: true,
              initialSelection: _zone ?? '',
              menuHeight: 320,
              entries: [
                DropdownMenuEntry(
                  value: '',
                  label: l.quietHoursTimeZoneMine(widget.personZone),
                ),
                for (final z in _zones) DropdownMenuEntry(value: z, label: z),
              ],
              onSelected: (value) => setState(
                () => _zone = value == null || value.isEmpty ? null : value,
              ),
            ),
            SwitchListTile(
              key: const ValueKey('quiet.enabled'),
              contentPadding: EdgeInsets.zero,
              value: _enabled,
              onChanged: (on) => setState(() => _enabled = on),
              title: Text(l.quietHoursEnabled),
            ),
          ],
        ),
      ),
      actions: [
        if (widget.rule != null)
          TextButton(
            key: const ValueKey('quiet.delete'),
            onPressed: () =>
                Navigator.of(context).pop(const QuietHoursDelete()),
            child: Text(l.quietHoursDelete),
          ),
        TextButton(
          key: const ValueKey('quiet.cancel'),
          onPressed: () => Navigator.of(context).pop(),
          child: Text(l.quietHoursCancel),
        ),
        FilledButton(
          key: const ValueKey('quiet.save'),
          onPressed: _save,
          child: Text(l.quietHoursSave),
        ),
      ],
    );
  }
}

/// The row "Quiet hours" of D9: the summary of each rule with Edit, and Add.
class QuietHoursList extends ConsumerWidget {
  /// Creates the list; [timeZones] are the zones the editor offers.
  const QuietHoursList({super.key, this.timeZones = ianaTimeZones});

  /// The zones offered in the editor.
  final List<String> Function() timeZones;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final rules = ref.watch(quietHoursRulesProvider).value ?? const [];
    final writer = ref.watch(notifyWriterProvider);
    final zone = ref.watch(personTimeZoneProvider);

    Future<void> edit(QuietHours? rule) async {
      final result = await showQuietHoursEditor(
        context,
        rule: rule,
        personZone: zone,
        timeZones: timeZones,
      );
      if (writer == null) return;
      switch (result) {
        case QuietHoursSave(:final rule) when rule.id == newRulePlaceholder:
          await writer.addQuietHours(rule);
        case QuietHoursSave(:final rule):
          await writer.putQuietHours(rule);
        case QuietHoursDelete() when rule != null:
          await writer.deleteQuietHours(rule.id);
        case _:
          break;
      }
    }

    return Column(
      key: const ValueKey('notify.quietHours'),
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        if (rules.isEmpty)
          Text(
            l.quietHoursNone,
            style: dsTextStyle(DsType.body)
                .copyWith(color: colors.textSecondary),
          ),
        for (final r in rules)
          Wrap(
            spacing: DsSpace.s8,
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              Text(
                quietHoursSummary(l, r),
                key: ValueKey('quiet.rule.${r.id.value}'),
                style: dsTextStyle(DsType.body)
                    .copyWith(color: colors.textPrimary),
              ),
              TextButton(
                key: ValueKey('quiet.edit.${r.id.value}'),
                onPressed: writer == null ? null : () => unawaited(edit(r)),
                child: Text(l.quietHoursEdit),
              ),
            ],
          ),
        TextButton(
          key: const ValueKey('quiet.add'),
          onPressed: writer == null ? null : () => unawaited(edit(null)),
          child: Text(l.quietHoursAdd),
        ),
      ],
    );
  }
}
