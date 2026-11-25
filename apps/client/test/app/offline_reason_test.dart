// SPDX-License-Identifier: Apache-2.0

// The offline banner after a close that waits for the person (15 7.2): `4015`, `4017` and
// `4018` name their reason and offer "Reconnect"; any other offline state stays "Offline".

import 'package:client/phone/phone_shell.dart' show PhoneBanner;
import 'package:flutter/material.dart' hide ConnectionState;
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';

import '../support/widgets.dart';

void main() {
  final cases = <(String, ConnectionReason, String)>[
    ('4017', ConnectionReason.deviceReplaced, en.connectionDeviceReplaced),
    ('4018', ConnectionReason.forbidden, en.connectionForbidden),
    (
      '4015',
      const ConnectionReason.closedByServer(4015),
      en.connectionEndedByServer,
    ),
  ];
  for (final (code, reason, text) in cases) {
    testWidgets('after $code the banner names the reason and reconnects', (
      tester,
    ) async {
      var reconnects = 0;
      await tester.pumpWidget(
        hostScreen(
          Scaffold(
            body: PhoneBanner(
              phase: ConnectionPhase.offline,
              waiting: 2,
              reason: reason,
              onReconnect: () => reconnects++,
            ),
          ),
        ),
      );
      expect(find.text(text), findsOneWidget);
      await tester.tap(find.text(en.connectionReconnect));
      expect(reconnects, 1);
    });
  }

  for (final reason in [
    ConnectionReason.none,
    ConnectionReason.networkUnavailable,
    const ConnectionReason.closedByServer(4013),
  ]) {
    testWidgets('$reason keeps "Offline" without an action', (tester) async {
      await tester.pumpWidget(
        hostScreen(
          Scaffold(
            body: PhoneBanner(
              phase: ConnectionPhase.backoff,
              waiting: 2,
              reason: reason,
              onReconnect: () {},
            ),
          ),
        ),
      );
      expect(find.text(en.phoneOfflineWaiting(2)), findsOneWidget);
      expect(find.text(en.connectionReconnect), findsNothing);
    });
  }
}
