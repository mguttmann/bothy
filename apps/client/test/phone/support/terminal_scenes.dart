// SPDX-License-Identifier: Apache-2.0

// The phone screens M9 Hosts and M8 Terminal of 01 5.3 as scenes for the accessibility tests
// and the platform audits (brief M5-13 Design note 3): the host "Build host" with one own
// detached session of Alex, M9 listing it, and M8 with that session adopted as a tab (the
// banner with "Reattach", the session switcher, the screen and the extra keys bar), in the
// phone's frame over the fakes of the desktop terminal. No scene opens a stream: the session
// stays detached, so no handshake runs while a test or an audit reads the screen.

import 'dart:io';

import 'package:api_client/api_client.dart' as api show TerminalSessionState;
import 'package:client/app/providers/ports.dart';
import 'package:client/phone/terminal/hosts_screen.dart';
import 'package:client/phone/terminal/providers.dart';
import 'package:client/phone/terminal/terminal_session_screen.dart';
import 'package:client/terminal/providers.dart';
import 'package:domain/domain.dart' show AppConfig;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:terminal_ui/testing.dart';

import '../../support/fakes.dart';
import '../../terminal/support.dart';
import 'phone_scenes.dart' show phoneFrame;

FakeHost? _host;

/// Creates the host of the terminal scenes once; call it (in `setUpAll`, or before the audit
/// shows a scene) before [hostsScene] or [terminalScene]. Its key pair comes from the crypto of
/// the platform, which a widget test's fake time cannot wait for.
Future<void> prepareTerminalScenes() async =>
    _host ??= await FakeHost.create(hostId: hostIdText);

FakeHost get _prepared =>
    _host ?? (throw StateError('call prepareTerminalScenes first'));

/// The hosts API with one own session of Alex, detached on the host.
FakeHostsApi _api() => FakeHostsApi(_prepared)
  ..listed.add(
    FakeHostsApi.session(
      FakeHostsApi.sessionId(1),
      state: api.TerminalSessionState.detached,
    ),
  );

/// The providers of the terminal over [hosts]: the ports of the fakes, the host connected, no
/// coordinator (the scenes have no life cycle to follow).
Widget _scope(
  FakeHostsApi hosts, {
  required bool android,
  required Widget child,
}) {
  final services = TerminalServices(
    api: hosts,
    pins: MemoryPins(),
    events: ManualHostEvents(),
    engineEvents: const Stream.empty(),
    stepUp: ({required purposeText}) async {},
    newTransport: () => FakeTerminalTransport(_prepared),
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
        testPorts(
          dataRoot: Directory.systemTemp,
          platform: android ? 'android' : 'ios',
        ),
      ),
      phoneTerminalCoordinatorProvider.overrideWithValue(null),
    ],
    child: child,
  );
}

/// M9 Hosts: "Build host" connected with its LLM login badge, Alex's detached session and "New
/// session"; [android] in the Android frame.
Widget hostsScene({bool android = false}) => _scope(
  _api(),
  android: android,
  child: phoneFrame(
    back: 'Me',
    android: android,
    title: 'Hosts',
    body: const HostsScreen(),
  ),
);

/// M8 Terminal of "Build host" with Alex's detached session as its tab: the banner with
/// "Reattach", the session switcher with Detach and End, the screen and the extra keys bar;
/// [android] in the Android frame.
Widget terminalScene({bool android = false}) => _scope(
  _api(),
  android: android,
  child: phoneFrame(
    back: 'Hosts',
    android: android,
    title: 'Build host',
    body: const TerminalSessionScreen(hostId: hostIdText),
  ),
);
