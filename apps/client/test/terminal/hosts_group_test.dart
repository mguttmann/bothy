// SPDX-License-Identifier: Apache-2.0

// The group "Hosts" of the sidebar (01 D11; brief M4-36 fixed requirement 12): every visible
// host with its connection and the count of own sessions, "disabled" for a disabled host, no
// group without hosts, a notice when the list failed, and a click that opens the host.

import 'package:client/terminal/hosts_group.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/widgets.dart';

const List<HostRowData> rows = [
  HostRowData(
    id: 'h1',
    name: 'Build host',
    connected: true,
    disabled: false,
    ownSessions: 2,
  ),
  HostRowData(
    id: 'h2',
    name: 'Linux VM',
    connected: false,
    disabled: false,
    ownSessions: 0,
  ),
  HostRowData(
    id: 'h3',
    name: 'Test Windows',
    connected: false,
    disabled: true,
    ownSessions: 0,
  ),
];

void main() {
  testWidgets('rows show connection, own sessions and disabled', (
    tester,
  ) async {
    final opened = <String>[];
    await tester.pumpWidget(
      hostScreen(
        Material(
          child: HostsGroupView(
            rows: rows,
            selectedHost: 'h1',
            onOpen: opened.add,
          ),
        ),
      ),
    );
    expect(find.text(en.hostsGroupTitle), findsOneWidget);
    expect(find.textContaining('Build host'), findsOneWidget);
    expect(find.byKey(const ValueKey('hosts.count.h1')), findsOneWidget);
    expect(find.text('2'), findsOneWidget);
    expect(find.byKey(const ValueKey('hosts.count.h2')), findsNothing);
    expect(find.byKey(const ValueKey('hosts.disabled.h3')), findsOneWidget);
    expect(find.byKey(const ValueKey('hosts.disabled.h1')), findsNothing);
    expect(
      find.bySemanticsLabel(en.hostRowLabel('Build host', en.hostConnected, 2)),
      findsOneWidget,
    );
    expect(
      find.bySemanticsLabel(
        en.hostRowLabel('Linux VM', en.hostDisconnected, 0),
      ),
      findsOneWidget,
    );
    expect(
      find.bySemanticsLabel(
        en.hostRowLabel('Test Windows', en.hostStateDisabled, 0),
      ),
      findsOneWidget,
    );
    await tester.tap(find.byKey(const ValueKey('hosts.row.h2')));
    expect(opened, ['h2']);
  });

  testWidgets('without hosts there is no group; a failed list shows a notice', (
    tester,
  ) async {
    await tester.pumpWidget(
      hostScreen(
        Material(
          child: HostsGroupView(rows: const [], onOpen: (_) {}),
        ),
      ),
    );
    expect(find.text(en.hostsGroupTitle), findsNothing);
    await tester.pumpWidget(
      hostScreen(
        Material(
          child: HostsGroupView(rows: const [], onOpen: (_) {}, failed: true),
        ),
      ),
    );
    expect(find.text(en.hostsLoadFailed), findsOneWidget);
  });
}
