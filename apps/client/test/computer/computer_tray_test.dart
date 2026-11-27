// SPDX-License-Identifier: Apache-2.0

// The tray entry of computer use (brief CU-07 requirement 15; 27 11 item 4): while a computer
// session of the owner is live the tray menu has "Stop computer control (<desktops>)", which
// stops every live session; the entry goes when none is live. On Windows the icon stays with
// Open and Quit; on macOS the icon shows only while a session is live.

import 'dart:async';

import 'package:client/computer/computer_tray.dart';
import 'package:client/computer/providers.dart';
import 'package:client/desktop/providers.dart' show computerTrayProvider;
import 'package:client/desktop/tray.dart';
import 'package:client/notifications_settings/providers.dart'
    show isOwnerProvider;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart' show HostEvent;

import '../support/desktop_fakes.dart';
import 'support.dart';

/// A window behavior that only counts.
final class _NoWindow implements TrayWindow {
  int opened = 0;

  @override
  Future<void> open() async => opened++;

  @override
  Future<void> quit() async {}
}

void main() {
  group('DesktopTray', () {
    test('Windows: the entry comes and goes next to Open and Quit', () async {
      final port = FakeTray();
      var stops = 0;
      final tray = DesktopTray(port: port, behavior: _NoWindow());
      await tray.start(
        tooltip: 'ExampleChat',
        openLabel: 'Open',
        quitLabel: 'Quit',
      );
      await tray.setComputerEntry(
        'Stop computer control (Desk)',
        onStop: () => stops++,
      );
      port.click(TrayAction.stopComputer);
      await pumpEventQueue();
      expect(stops, 1);
      await tray.setComputerEntry(null);
      // Without the entry a stale click stops nothing.
      port.click(TrayAction.stopComputer);
      await pumpEventQueue();
      expect(stops, 1);
      expect(port.shown, ['ExampleChat|Open|Quit']);
      expect(port.computer, ['Stop computer control (Desk)', null]);
      await tray.dispose();
    });

    test('macOS: the icon shows only while a session is live', () async {
      final port = FakeTray();
      final tray = DesktopTray(
        port: port,
        behavior: _NoWindow(),
        persistent: false,
      );
      await tray.start(
        tooltip: 'ExampleChat',
        openLabel: 'Open',
        quitLabel: 'Quit',
      );
      expect(port.shown, isEmpty);
      await tray.setComputerEntry(
        'Stop computer control (Desk)',
        onStop: () {},
      );
      expect(port.shown, ['ExampleChat|Open|Quit']);
      await tray.setComputerEntry(null);
      expect(port.shown, ['ExampleChat|Open|Quit', 'hide']);
      await tray.dispose();
    });
  });

  test('the provider follows the live sessions and stops them all', () async {
    final api = FakeComputerApi()
      ..sessions = [computerSession(1), computerSession(2, runId: deskHost)];
    final events = StreamController<HostEvent>.broadcast(sync: true);
    addTearDown(events.close);
    final port = FakeTray();
    final tray = DesktopTray(port: port, behavior: _NoWindow());
    await tray.start(
      tooltip: 'ExampleChat',
      openLabel: 'Open',
      quitLabel: 'Quit',
    );
    final c = ProviderContainer(
      overrides: [
        computerPortsProvider.overrideWithValue(
          ComputerPorts(api: api, events: events.stream),
        ),
        isOwnerProvider.overrideWith((ref) async => true),
        computerTrayProvider.overrideWith(
          (ref) => ComputerTray(
            tray: tray,
            ref: ref,
            label: (d) => 'Stop computer control ($d)',
          ),
        ),
      ],
    );
    addTearDown(c.dispose);
    c.listen(computerTrayProvider, (_, _) {});
    await c.read(isOwnerProvider.future);
    await pumpEventQueue();
    expect(port.computer.last, 'Stop computer control (Desk)');
    port.click(TrayAction.stopComputer);
    await pumpEventQueue();
    expect(api.calls.first, 'list');
    expect(
      api.calls.skip(1),
      unorderedEquals([
        'stop ${computerSessionId(1)}',
        'stop ${computerSessionId(2)}',
      ]),
    );
    expect(port.computer.last, isNull);
  });
}
