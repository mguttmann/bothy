// SPDX-License-Identifier: Apache-2.0

// The goldens of screen M8 on the phone (brief M5-12 Tests; 01 5.3 M8; components.md 12):
// the terminal of a host with a detached session (the banner with "Reattach", the session
// switcher with Detach and End, and the extra keys bar), the key change warning and the dialog
// "Open terminal" in the phone window. The density `touch`, light and dark, text scaling 1 and
// 2, German and English. One file per scene and theme keeps each file below repo.max_file_kb.

import 'dart:io';

import 'package:alchemist/alchemist.dart';
import 'package:api_client/api_client.dart' as api show TerminalSessionState;
import 'package:client/app/providers/ports.dart';
import 'package:client/phone/terminal/providers.dart';
import 'package:client/phone/terminal/terminal_session_screen.dart';
import 'package:client/terminal/providers.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:domain/domain.dart' show AppConfig;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';
import 'package:terminal_ui/testing.dart';

import '../../phone/support/phone_scenes.dart' show phoneFrame;
import '../../support/fakes.dart';
import '../../terminal/support.dart';

const String _stored =
    '7KQ2-9XTA-M4PB-1C8D-0000-0000-0000-0000-0000-0000-0000-0000-0000';
const String _current =
    'Q0ZJ-55RW-2HNE-K7TS-0000-0000-0000-0000-0000-0000-0000-0000-0000';

late FakeHost _host;

/// One own session of Alex, detached on the host: M8 adopts it as a tab with "Reattach".
FakeHostsApi _api() => FakeHostsApi(_host)
  ..listed.add(
    FakeHostsApi.session(
      FakeHostsApi.sessionId(1),
      state: api.TerminalSessionState.detached,
    ),
  );

Widget _detached() {
  final hosts = _api();
  final services = TerminalServices(
    api: hosts,
    pins: MemoryPins(),
    events: ManualHostEvents(),
    engineEvents: const Stream.empty(),
    stepUp: ({required purposeText}) async {},
    newTransport: () => FakeTerminalTransport(_host),
    links: SystemLinkOpener(launch: (_) async => true),
    // The header counts the remaining time from the opening (09:30) to this clock.
    clock: TestClock(DateTime.utc(2026, 10, 20, 10, 18)),
    config: AppConfig.defaults(),
    context: () => null,
  );
  return ProviderScope(
    overrides: [
      terminalServicesProvider.overrideWithValue(services),
      terminalSelfUserIdProvider.overrideWithValue(alexId),
      hostsListProvider.overrideWith(
        () => FixedHosts(
          HostsOverview(
            hosts: [hosts.current],
            ownSessions: {hostIdText: List.of(hosts.listed)},
            connected: const {hostIdText: true},
          ),
        ),
      ),
      appPortsProvider.overrideWithValue(
        testPorts(dataRoot: Directory.systemTemp, platform: 'ios'),
      ),
      phoneTerminalCoordinatorProvider.overrideWithValue(null),
    ],
    child: phoneFrame(
      back: 'Hosts',
      body: const TerminalSessionScreen(hostId: hostIdText),
    ),
  );
}

Widget _dialog(Widget dialog) => phoneFrame(
  back: 'Hosts',
  body: Center(child: SingleChildScrollView(child: dialog)),
);

final Map<String, Widget Function()> _scenes = {
  'detached': _detached,
  'key_changed': () =>
      _dialog(const KeyChangedDialog(stored: _stored, current: _current)),
  'open_dialog': () => _dialog(
    OpenTerminalDialog(
      host: buildHost(_host),
      initial: const OpenChoice(profile: 'shell'),
      selfUserId: alexId,
    ),
  ),
};

Future<void> _settle(WidgetTester tester) async {
  await dsPumpIcons(tester);
  // M8 loads the host's sessions once (FakeHostsApi answers at once).
  for (var i = 0; i < 5; i++) {
    await tester.pump();
  }
}

void main() {
  setUpAll(() async {
    _host = await FakeHost.create(hostId: hostIdText);
  });

  for (final MapEntry(key: name, value: scene) in _scenes.entries) {
    for (final theme in DsThemeMode.values) {
      goldenTest(
        'M8 $name in ${theme.name}, text scale 1 and 2, de and en',
        fileName: 'phone_m8_${name}_${theme.name}',
        pumpBeforeTest: _settle,
        builder: () => dsGoldenMatrix(
          name: 'm8_$name',
          themes: [theme],
          densities: const [DsDensityMode.touch],
          textScales: const [1.0, 2.0],
          locales: const [Locale('de'), Locale('en')],
          builder: scene,
        ),
      );
    }
  }
}
