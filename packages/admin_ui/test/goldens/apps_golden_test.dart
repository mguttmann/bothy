// SPDX-License-Identifier: Apache-2.0

// Goldens of the apps screens of M6-08 (brief M6-08 Tests): the apps list, the consent of the
// install dialog, its grants step with the server's preview, the diff of an update and the
// uninstall dialog with the effects of 11 4.3; light and dark, text scaling 1 and 2, German
// and English. One file per view keeps each file below repo.max_file_kb. The dialogs reach
// their step the way a person does: the steps run in every cell before the capture.

import 'dart:convert';
import 'dart:typed_data';

import 'package:admin_ui/admin_ui.dart';
import 'package:alchemist/alchemist.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';

Widget _frame(Widget child) => Material(
  type: MaterialType.transparency,
  child: SizedBox(
    width: 760,
    child: Padding(padding: const EdgeInsets.all(16), child: child),
  ),
);

FakeAdminRepository _repository() =>
    FakeAdminRepository(prompt: StepUpDialogPrompt(() => null))
      ..stepUpFresh = true;

PickedFile _file(Map<String, Object?> manifest) => PickedFile(
  'repo-bridge.json',
  Uint8List.fromList(utf8.encode(jsonEncode(manifest))),
);

/// Version 2.0.0 of Repo Bridge: a new required scope, a dropped scope and command, a new
/// event, another destination and a space grant instead of the channel grant.
Map<String, Object?> _v2() => Map.of(
  repoBridgeManifest(
    version: '2.0.0',
    required: const ['messages:write', 'files:write'],
    events: const ['message.created', 'reaction.added'],
    eventsUrl: 'https://relay.example.com/hooks/events',
    grants: const [
      {
        'target_kind': 'space',
        'level': 'read',
        'capabilities': <String>[],
        'purpose': 'Read the space',
      },
    ],
  ),
)..['commands'] = const <Object?>[];

/// Presses the button with [key] in every cell. The golden surface is laid out after these
/// steps, so a tap at a position could miss; the button's own callback runs instead, as a tap
/// would run it. The press moves the dialog on, so the button leaves its cell.
Future<void> _pressEach(WidgetTester tester, String key) async {
  final f = find.byKey(ValueKey(key));
  final count = f.evaluate().length;
  for (var i = 0; i < count && f.evaluate().isNotEmpty; i++) {
    tester.widget<ButtonStyleButton>(f.first).onPressed!();
    await tester.pumpAndSettle();
  }
}

Future<void> _typeEach(WidgetTester tester, String key, String text) async {
  final count = find.byKey(ValueKey(key)).evaluate().length;
  for (var i = 0; i < count; i++) {
    await tester.enterText(find.byKey(ValueKey(key)).at(i), text);
    await tester.pump();
  }
}

/// The install dialog of every cell at its consent: the manifest picked and checked.
Future<void> _toConsent(WidgetTester tester) async {
  await dsPumpIcons(tester);
  await _pressEach(tester, 'install.pick');
}

/// The install dialog of every cell at its grants step: the channel picked, the token and
/// the reason typed, "Install" sent.
Future<void> _toGrants(WidgetTester tester) async {
  await _toConsent(tester);
  final pickers = find.byKey(const ValueKey('consent.grant.0'));
  for (var i = 0; i < pickers.evaluate().length; i++) {
    tester.widget<DropdownButton<String>>(pickers.at(i)).onChanged!(
      'channel.campaigns',
    );
    await tester.pump();
  }
  await _typeEach(tester, 'consent.tokenName', 'Bridge');
  await _typeEach(tester, 'consent.tokenLastDay', '2026-12-31');
  await _typeEach(tester, 'consent.reason', 'Ticket bridge');
  await _pressEach(tester, 'install.submit');
}

/// The update dialog of every cell at its diff.
Future<void> _toDiff(WidgetTester tester) async {
  await dsPumpIcons(tester);
  await _pressEach(tester, 'update.pick');
}

final Map<String, (Widget Function(), Future<void> Function(WidgetTester))>
_views = {
  'list': (
    () {
      final r = _repository();
      r.seedApp(repoBridgeManifest());
      final report = Map.of(repoBridgeManifest())
        ..['slug'] = 'cron-report'
        ..['name'] = 'Cron Report'
        ..['version'] = '0.3.1';
      r.seedApp(report, state: AppInstallState.suspended);
      return _frame(
        AppsList(
          repository: r,
          onOpen: (_) {},
          pickManifest: () async => null,
          clock: testClock,
        ),
      );
    },
    dsPumpIcons,
  ),
  'consent': (
    () => InstallAppDialog(
      repository: _repository(),
      pickManifest: () async => _file(repoBridgeManifest()),
      timeZone: 'Europe/Berlin',
    ),
    _toConsent,
  ),
  'grants': (
    () => InstallAppDialog(
      repository: _repository(),
      pickManifest: () async => _file(repoBridgeManifest()),
      timeZone: 'Europe/Berlin',
    ),
    _toGrants,
  ),
  'update_diff': (
    () {
      final r = _repository();
      final app = r.seedApp(
        repoBridgeManifest(),
        grantedScopes: const [
          'messages:write',
          'commands:write',
          'messages:read',
        ],
      );
      return UpdateAppDialog(
        repository: r,
        app: app,
        pickManifest: () async => _file(_v2()),
      );
    },
    _toDiff,
  ),
  'uninstall': (
    () => const AppLifecycleDialog(
      appName: 'Repo Bridge',
      action: AppLifecycleAction.uninstall,
    ),
    dsPumpIcons,
  ),
};

void main() {
  for (final MapEntry(key: name, value: (view, steps)) in _views.entries) {
    goldenTest(
      '$name in light and dark, text scale 1 and 2, de and en',
      fileName: 'apps_$name',
      pumpBeforeTest: steps,
      builder: () => dsGoldenMatrix(
        name: name,
        builder: view,
        textScales: const [1.0, 2.0],
        locales: const [Locale('de'), Locale('en')],
      ),
    );
  }
}
