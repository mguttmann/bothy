// SPDX-License-Identifier: Apache-2.0

// The hosts of the group and the tabs of a host (brief M4-36, D-M4-36-4, D-M4-36-6): the
// overview keeps only the person's own sessions that are not closed, follows
// `host.presence_changed` in place and reloads after the other host events; the tabs switch,
// wrap around and size the font within its bounds.

import 'dart:async';

import 'package:api_client/api_client.dart' show Host;
import 'package:api_client/api_client.dart' as api show TerminalSessionState;
import 'package:client/terminal/providers.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart' as sync;
import 'package:terminal_ui/testing.dart';

import 'support.dart';

/// Counts the lists of hosts the overview loads.
final class _CountingApi extends FakeHostsApi {
  _CountingApi(super.host);

  int lists = 0;

  @override
  Future<List<Host>> listHosts() async {
    lists++;
    return super.listHosts();
  }
}

void main() {
  setUpAll(TerminalWorld.prepareIdleHost);

  test('the overview keeps the own sessions that are not closed', () async {
    final fake = FakeHostsApi(await FakeHost.create(hostId: hostIdText));
    fake.listed.addAll([
      FakeHostsApi.session(FakeHostsApi.sessionId(1)),
      FakeHostsApi.session(
        FakeHostsApi.sessionId(2),
        state: api.TerminalSessionState.closed,
      ),
      FakeHostsApi.session(FakeHostsApi.sessionId(3), userId: samId),
    ]);
    final o = await loadHostsOverview(fake, alexId);
    expect(o.sessionCount(hostIdText), 1);
    expect(o.isConnected(hostIdText), isTrue);
    expect(o.host(hostIdText)?.name, 'Build host');
    expect(o.host('other'), isNull);
    expect(
      o.withConnection(hostIdText, false).isConnected(hostIdText),
      isFalse,
    );
  });

  test(
    'presence changes in place; other host events reload the hosts',
    () async {
      final fake = _CountingApi(await FakeHost.create(hostId: hostIdText));
      final events = StreamController<sync.HostEvent>.broadcast(sync: true);
      addTearDown(events.close);
      final base = TerminalWorld.servicesFor(fake, events.stream);
      final c = ProviderContainer(
        overrides: [
          terminalServicesProvider.overrideWithValue(base),
          terminalSelfUserIdProvider.overrideWithValue(alexId),
        ],
      );
      addTearDown(c.dispose);
      final sub = c.listen(hostsListProvider, (_, _) {});
      addTearDown(sub.close);
      final first = await c.read(hostsListProvider.future);
      expect(first!.isConnected(hostIdText), isTrue);
      expect(fake.lists, 1);
      events.add(
        const sync.HostPresenceChanged(hostId: hostIdText, connected: false),
      );
      expect(c.read(hostsListProvider).value!.isConnected(hostIdText), isFalse);
      expect(fake.lists, 1);
      events
        ..add(const sync.HostRemoved(hostId: hostIdText))
        ..add(const sync.HostAccessGranted(hostId: hostIdText));
      for (var i = 0; i < 100 && fake.lists < 3; i++) {
        await Future<void>.delayed(Duration.zero);
      }
      expect(fake.lists, greaterThanOrEqualTo(2));
      expect(c.read(hostsListProvider).value!.isConnected(hostIdText), isTrue);
    },
  );

  test(
    'tabs: select, next and previous wrap, the font stays in its bounds',
    () async {
      final fake = FakeHostsApi(await FakeHost.create(hostId: hostIdText));
      fake.listed.addAll([
        FakeHostsApi.session(FakeHostsApi.sessionId(1)),
        FakeHostsApi.session(FakeHostsApi.sessionId(2)),
      ]);
      final tabs = TerminalTabs(
        hostId: hostIdText,
        services: TerminalWorld.servicesFor(fake, const Stream.empty()),
        selfUserId: alexId,
      );
      addTearDown(tabs.dispose);
      await tabs.refresh();
      await tabs.refresh();
      expect(tabs.tabs, hasLength(2), reason: 'a second refresh adds nothing');
      expect(tabs.labels, [('shell', 1), ('shell', 2)]);
      tabs.next();
      expect(tabs.selected, 1);
      tabs.next();
      expect(tabs.selected, 0);
      tabs.previous();
      expect(tabs.selected, 1);
      tabs.select(5);
      expect(tabs.selected, 1);
      final size = tabs.fontSize;
      for (var i = 0; i < 100; i++) {
        tabs.changeFont(1);
      }
      expect(tabs.fontSize, TerminalTabs.maxFontSize);
      for (var i = 0; i < 100; i++) {
        tabs.changeFont(-1);
      }
      expect(tabs.fontSize, TerminalTabs.minFontSize);
      expect(
        size,
        inInclusiveRange(TerminalTabs.minFontSize, TerminalTabs.maxFontSize),
      );
    },
  );
}
