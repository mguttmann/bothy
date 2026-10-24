// SPDX-License-Identifier: Apache-2.0

/// The text of a slash command in the composer and its options (01 D5; components.md 9;
/// 09 10.5; brief M4-21 Design note 4).
///
/// Syntax: a slash with the command's name, optionally followed by `@` and the owner's handle, then the option values in the command's order,
/// separated by spaces; a value may also name its option (`ticket:47`). The last option takes
/// the rest of the line, so a free text such as a title may contain spaces.
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';

/// The slash command at the start of [text]: the token after the slash (name with an
/// optional `@handle`) and the rest after the first space; null when [text] does not start
/// with "/" or the token is empty.
({String token, String rest})? slashCommandOf(String text) {
  if (!text.startsWith('/')) return null;
  final firstLine = text.split('\n').first;
  final space = firstLine.indexOf(' ');
  final token = space < 0
      ? firstLine.substring(1)
      : firstLine.substring(1, space);
  if (token.isEmpty) return null;
  final rest = space < 0 ? '' : text.substring(space + 1);
  return (token: token, rest: rest);
}

/// The query of the suggestion list: the text after "/" while the cursor ([cursor]) is still
/// in the first word of a text that starts with "/"; null otherwise (the list closes).
String? slashQuery(String text, int cursor) {
  if (!text.startsWith('/') || cursor < 1 || cursor > text.length) return null;
  final head = text.substring(1, cursor);
  if (head.contains(' ') || head.contains('\n')) return null;
  return head;
}

/// The values of [command]'s options in [rest], in option order; an option without a value
/// is null.
List<String?> optionValues(Command command, String rest) =>
    _parse(command, rest).values;

/// The option being typed at the end of [rest] and its partial value, or null when every
/// option has a value and the text does not end in the last one.
({CommandOption option, String value})? activeOption(
  Command command,
  String rest,
) {
  final parsed = _parse(command, rest);
  final index = parsed.active;
  if (index == null || index >= command.options.length) return null;
  return (option: command.options[index], value: parsed.values[index] ?? '');
}

/// [command] as the composer shows it after selection: its name and every option as a
/// placeholder with its value from [rest]; a required option without a value disables send
/// with "Missing option: `name`".
DsComposerCommand composerCommand(Command command, String rest) {
  final values = optionValues(command, rest);
  return DsComposerCommand(
    name: '/${command.name}',
    options: [
      for (var i = 0; i < command.options.length; i++)
        DsCommandOption(
          name: command.options[i].name,
          required: command.options[i].required,
          value: values[i],
        ),
    ],
  );
}

/// The `options` of `CommandInvoke` for [command] with the values in [rest]: name and value
/// of every option that has one.
List<({String name, String value})> invokeOptions(
  Command command,
  String rest,
) {
  final values = optionValues(command, rest);
  return [
    for (var i = 0; i < command.options.length; i++)
      if (values[i] case final v? when v.isNotEmpty)
        (name: command.options[i].name, value: v),
  ];
}

/// [rest] with the value of [option] replaced by [value] (an accepted suggestion of the
/// agent), followed by a space so the next option starts.
String withOptionValue(
  Command command,
  String rest,
  CommandOption option,
  String value,
) {
  final values = optionValues(command, rest);
  final index = command.options.indexOf(option);
  if (index < 0) return rest;
  values[index] = value;
  final parts = <String>[];
  for (var i = 0; i < values.length; i++) {
    final v = values[i];
    if (v == null || v.isEmpty) break;
    parts.add(v);
  }
  final last = index == command.options.length - 1;
  return last ? parts.join(' ') : '${parts.join(' ')} ';
}

({List<String?> values, int? active}) _parse(Command command, String rest) {
  final options = command.options;
  final values = List<String?>.filled(options.length, null);
  if (options.isEmpty) return (values: values, active: null);
  var positional = 0;
  int? active = 0;
  var i = 0;
  final text = rest.replaceAll('\n', ' ');
  while (i < text.length) {
    while (i < text.length && text[i] == ' ') {
      i++;
    }
    if (i >= text.length) break;
    // A named value: `name:value`.
    final end = text.indexOf(' ', i);
    final word = end < 0 ? text.substring(i) : text.substring(i, end);
    final colon = word.indexOf(':');
    int? target;
    var value = word;
    if (colon > 0) {
      final name = word.substring(0, colon);
      final at = options.indexWhere((o) => o.name == name);
      if (at >= 0) {
        target = at;
        value = word.substring(colon + 1);
      }
    }
    if (target == null) {
      while (positional < options.length && values[positional] != null) {
        positional++;
      }
      if (positional >= options.length) break;
      target = positional;
      if (target == options.length - 1) {
        // The last option takes the rest of the line.
        value = text.substring(i).trimRight();
        values[target] = value;
        active = target;
        return (values: values, active: active);
      }
    }
    values[target] = value;
    active = end < 0 ? target : null;
    i = end < 0 ? text.length : end;
  }
  if (active == null || text.endsWith(' ')) {
    final next = values.indexWhere((v) => v == null);
    active = next < 0 ? null : next;
  }
  return (values: values, active: active);
}
