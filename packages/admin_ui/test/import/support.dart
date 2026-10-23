// SPDX-License-Identifier: Apache-2.0

// Shared setup of the import tests (brief M8-08 Tests): the fake repository with the step-up
// dialog as prompt (the fake answers like the server behind the interceptor), the pages pumped
// in the app frame, and the mapping file the picker hands over.

import 'dart:convert';
import 'dart:typed_data';

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../instance/support.dart';

export '../instance/support.dart';

/// A white-label mapping file: two channels, one a forum, the first with the planned operator
/// [operator].
String mappingYaml({String operator = 'agent:texter'}) =>
    '''
version: 1
guild_id: "100"
defaults:
  unmapped_human: placeholder
  unmapped_bot: placeholder
  uncategorized_space: "200"
spaces:
  - discord_category: "200"
    target: {create: {name: Imported}}
channels:
  - discord_channel: "301"
    target: {create: {name: tickets}}
    planned_operator: "$operator"
  - discord_channel: "302"
    target: {create: {name: questions}}
users:
  - discord_user: "401"
    target: "user:alex"
''';

/// A picker that hands over the files of [files] in turn and records each call.
class QueuedPicker {
  /// Creates the picker.
  QueuedPicker(this.files);

  /// The files still to hand over, as text.
  final List<String> files;

  /// How often the picker was opened.
  int opened = 0;

  /// The [MappingPicker].
  Future<PickedFile?> call() async {
    opened++;
    if (files.isEmpty) return null;
    return PickedFile(
      'mapping.yaml',
      Uint8List.fromList(utf8.encode(files.removeAt(0))),
    );
  }
}

/// Pumps the page of the source [id] over [repository].
Future<void> pumpSource(
  WidgetTester tester,
  FakeAdminRepository repository,
  String id,
  GlobalKey<NavigatorState> navigator, {
  Locale locale = const Locale('en'),
}) async {
  await pumpInstance(
    tester,
    ImportSourcePage(repository: repository, sourceId: id, onBack: () {}),
    navigator,
    locale: locale,
  );
  await tester.pumpAndSettle();
}

/// Taps the widget with [key] (scrolled into view) and settles.
Future<void> tapKey(WidgetTester tester, String key) async {
  final f = find.byKey(ValueKey(key));
  await tester.ensureVisible(f);
  await tester.pumpAndSettle();
  await tester.tap(f);
  await tester.pumpAndSettle();
}

/// Moves the app through [states] (each a legal step of the lifecycle, as the browser reports
/// `visibilitychange` and focus) and settles.
Future<void> setLifecycle(
  WidgetTester tester,
  List<AppLifecycleState> states,
) async {
  for (final s in states) {
    tester.binding.handleAppLifecycleStateChanged(s);
  }
  await tester.pumpAndSettle();
}
