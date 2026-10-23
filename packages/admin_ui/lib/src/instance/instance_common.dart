// SPDX-License-Identifier: Apache-2.0

/// Pieces the instance screens share: the text of a value, the line "version, who, when", the
/// field errors of the server at their input and the picker of the logo (brief M6-09 Design
/// notes 1 to 4).
library;

import 'dart:convert';

import 'package:api_client/api_client.dart' show ApiProblem, ProblemFieldError;
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../apps/manifest_picker.dart' show PickedFile;
import '../data/admin_repository.dart';
import '../data/self_repository.dart';
import '../self/device_tile.dart' show activityText;

/// Opens the host's file dialog for an image (`.png`, `.jpg`, `.jpeg`, `.webp`) and returns the
/// chosen file, or null when the person cancels. The desktop app opens the system's file
/// dialog, the console a file input of the browser.
typedef LogoPicker = Future<PickedFile?> Function();

/// The text of a setting value: a string as it is, anything else as compact JSON, and the dash
/// free word for "no value" of [l] for null.
String settingValueText(AppLocalizations l, Object? value) => switch (value) {
  null => l.instanceValueNone,
  final String s when s.isEmpty => l.instanceValueEmpty,
  final String s => s,
  _ => jsonEncode(value),
};

/// "Version 3, changed by Alex, Today 9:30" or "Default, never changed" for a value at
/// [version] that [byName] wrote at [at].
String changedText(
  BuildContext context, {
  required int version,
  required String? byName,
  required DateTime? at,
  required DateTime now,
  required String timeZone,
}) {
  final l = AppLocalizations.of(context);
  if (version == 0 || at == null) return l.instanceNeverChanged;
  final when = activityText(context, at, now: now, timeZone: timeZone);
  return byName == null
      ? l.instanceChangedAt(version, when)
      : l.instanceChangedBy(version, byName, when);
}

/// The display names of [ids] (people who changed a value), best effort: an ID the server does
/// not answer keeps the ID.
Future<Map<String, String>> peopleNames(
  AdminRepository repository,
  Iterable<String?> ids,
) async {
  final out = <String, String>{};
  for (final id in {...ids.nonNulls}) {
    try {
      out[id] = (await repository.person(id)).displayName;
    } on Object {
      out[id] = id;
    }
  }
  return out;
}

/// The field errors of a refused write [error] (`400 validation_failed`), by pointer; empty for
/// any other error. The pointers are those of the server (`/value`, `/product_name`,
/// `/config/issuer`).
Map<String, ProblemFieldError> fieldErrorsOf(Object error) {
  final problem = error is SelfActionFailed ? error.problem : null;
  if (problem == null || problem.code != 'validation_failed') return const {};
  return {for (final e in problem.errors) e.pointer: e};
}

/// The translated text of the field error [error] of the server; a code this build does not
/// know shows the code, so support can still read it.
String fieldErrorText(AppLocalizations l, ProblemFieldError error) =>
    switch (error.code) {
      'required' => l.instanceFieldRequired,
      'too_long' => l.instanceFieldTooLong,
      'invalid_text' => l.instanceFieldInvalidText,
      'unknown_color_key' => l.instanceFieldUnknownColor,
      'invalid_enum' => l.instanceFieldInvalidEnum,
      'invalid_format' || 'invalid' => l.instanceFieldInvalidFormat,
      'not_allowed' => l.instanceFieldNotAllowed,
      'duplicate_key' => l.instanceFieldDuplicate,
      'null_not_allowed' => l.instanceFieldRequired,
      'invalid_type' => l.instanceFieldInvalidType,
      'unknown_field' || 'unknown_member' => l.instanceFieldUnknown,
      final code => l.instanceFieldOther(code),
    };

/// The first field error of [errors] at [pointer] or inside it (`/config/allowed_tenant_ids/1`
/// belongs to `/config/allowed_tenant_ids`), as text, or null.
String? fieldErrorAt(
  AppLocalizations l,
  Map<String, ProblemFieldError> errors,
  String pointer,
) {
  for (final MapEntry(:key, :value) in errors.entries) {
    if (key == pointer || key.startsWith('$pointer/')) {
      return fieldErrorText(l, value);
    }
  }
  return null;
}

/// The problem of [error] when the server refused with a `409` for the loopback rule of the
/// proxy header driver (08 2.4 rule 1), else null.
ApiProblem? loopbackRefusal(Object error) {
  final problem = error is SelfActionFailed ? error.problem : null;
  return problem != null &&
          problem.status == 409 &&
          problem.reason == 'trusted_header_requires_loopback'
      ? problem
      : null;
}
