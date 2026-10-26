// SPDX-License-Identifier: Apache-2.0

// Accessibility of the terminal view on the desktop (01 8 row "Terminal"; brief M4-36,
// docs/quality/accessibility-checklist.md "Terminal view"): the controls of the view and of
// the group "Hosts" carry names, text keeps its contrast, and an open tab shows the header,
// the tabs and the footer with their names.

import 'package:client/terminal/hosts_group.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/widgets.dart' show hostScreen;
import 'support.dart';

void main() {
  testWidgets('the view of a host: labelled controls and contrast', (
    tester,
  ) async {
    final handle = tester.ensureSemantics();
    final w = await TerminalWorld.create(tester);
    final c = await w.pump(tester);
    await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
    await expectLater(tester, meetsGuideline(textContrastGuideline));
    final tabs = await openShell(tester, c);
    await tester.pump();
    expect(find.text(en.terminalDetach), findsOneWidget);
    expect(find.text(en.terminalEnd), findsOneWidget);
    expect(find.text(en.terminalFooterEncrypted), findsOneWidget);
    await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
    await tabs.current!.detach();
    await tester.pump(const Duration(seconds: 1));
    handle.dispose();
  });

  testWidgets('the group Hosts: every row has its name, state and count', (
    tester,
  ) async {
    final handle = tester.ensureSemantics();
    await tester.pumpWidget(
      hostScreen(
        Material(
          child: HostsGroupView(
            rows: const [
              HostRowData(
                id: 'h1',
                name: 'Build host',
                connected: true,
                disabled: false,
                ownSessions: 1,
              ),
            ],
            onOpen: (_) {},
          ),
        ),
      ),
    );
    await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
    expect(
      find.bySemanticsLabel(en.hostRowLabel('Build host', en.hostConnected, 1)),
      findsOneWidget,
    );
    handle.dispose();
  });
}
