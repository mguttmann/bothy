// SPDX-License-Identifier: Apache-2.0

// The deliveries of an outgoing subscription (brief M6-08 Design note 3, Ruling 4; 11 7;
// D-M2-26-6, D-M2-26-14): newest first with "Load more" and reload, per delivery status,
// attempts, next attempt, last status code, last error as text, the event or its absence,
// created and delivered times; "Redeliver" without step-up and the server's refusal of an
// event that left the log. A response body is never stored, so none is shown.

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';
import 'hooks_support.dart';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;
  late AppSubscription subscription;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = FakeAdminRepository(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    );
    final app = seedBridgeHooks(repository);
    subscription = repository.subscriptionsByApp[app.id]!.first;
    repository.deliveriesBySubscription[subscription.id] = [
      hookDelivery(
        'd3',
        seq: 5303,
        status: DeliveryStatus.pending,
        attempt: 3,
        responseCode: 503,
        lastError: 'throttled',
        nextAttemptAt: DateTime.utc(2026, 10, 17, 9, 5),
        createdAt: DateTime.utc(2026, 10, 17, 8, 50),
      ),
      hookDelivery(
        'd2',
        seq: 5302,
        status: DeliveryStatus.succeeded,
        responseCode: 204,
        createdAt: DateTime.utc(2026, 10, 16, 7),
        deliveredAt: DateTime.utc(2026, 10, 16, 7, 1),
      ),
      hookDelivery(
        'd1',
        seq: 5300,
        eventType: null,
        status: DeliveryStatus.dead,
        attempt: 4,
        lastError: 'brand_new_code',
        createdAt: DateTime.utc(2026, 10, 1, 7),
      ),
    ];
  });

  Future<void> tap(WidgetTester tester, String key) async {
    await tester.ensureVisible(find.byKey(ValueKey(key)));
    await tester.tap(find.byKey(ValueKey(key)));
    await tester.pumpAndSettle();
  }

  Future<void> view(WidgetTester tester) async {
    await pumpInApp(
      tester,
      DeliveriesView(
        repository: repository,
        subscription: subscription,
        timeZone: 'Europe/Berlin',
        clock: testClock,
      ),
      navigatorKey: navigator,
    );
  }

  Finder inRow(String id, Finder f) =>
      find.descendant(of: find.byKey(ValueKey('delivery.$id')), matching: f);

  String status(WidgetTester tester, String id) =>
      tester.widget<Text>(find.byKey(ValueKey('delivery.status.$id'))).data!;

  testWidgets('newest first, one page at a time with Load more', (
    tester,
  ) async {
    repository.deliveryPageSize = 2;
    await view(tester);

    expect(find.byKey(const ValueKey('deliveries.view')), findsOneWidget);
    expect(find.byKey(const ValueKey('delivery.d3')), findsOneWidget);
    expect(find.byKey(const ValueKey('delivery.d2')), findsOneWidget);
    expect(find.byKey(const ValueKey('delivery.d1')), findsNothing);
    final d3 = tester.getTopLeft(find.byKey(const ValueKey('delivery.d3')));
    final d2 = tester.getTopLeft(find.byKey(const ValueKey('delivery.d2')));
    expect(d3.dy, lessThan(d2.dy));

    await tap(tester, 'deliveries.more');
    expect(find.byKey(const ValueKey('delivery.d1')), findsOneWidget);
    expect(find.byKey(const ValueKey('deliveries.more')), findsNothing);
    expect(repository.deliveryRequests, ['sub-active', 'sub-active 2']);
  });

  testWidgets('each delivery shows status, attempts, next attempt, status '
      'code, error, event and times', (tester) async {
    await view(tester);

    expect(status(tester, 'd3'), 'Waiting');
    expect(status(tester, 'd2'), 'Delivered');
    expect(status(tester, 'd1'), 'Given up');

    expect(inRow('d3', find.textContaining('3 attempts')), findsOneWidget);
    expect(
      inRow('d3', find.textContaining('next attempt Today')),
      findsOneWidget,
    );
    expect(inRow('d3', find.textContaining('last status 503')), findsOneWidget);
    expect(
      inRow('d3', find.textContaining('last error: throttled by the receiver')),
      findsOneWidget,
    );
    expect(
      inRow('d3', find.textContaining('message.created, sequence 5303')),
      findsOneWidget,
    );
    expect(inRow('d3', find.textContaining('created Today')), findsOneWidget);

    expect(inRow('d2', find.textContaining('1 attempt')), findsOneWidget);
    expect(
      inRow('d2', find.textContaining('delivered Yesterday')),
      findsOneWidget,
    );
    expect(inRow('d2', find.textContaining('last error')), findsNothing);

    // An error code of a newer server is shown as the code.
    expect(
      inRow('d1', find.textContaining('last error: brand_new_code')),
      findsOneWidget,
    );
    expect(
      inRow('d1', find.textContaining('Event left the log, sequence 5300')),
      findsOneWidget,
    );
  });

  testWidgets('every error code of D-M2-26-6 has its own text', (tester) async {
    const codes = {
      'timeout': 'timed out',
      'connection_failed': 'connection failed',
      'tls_failed': 'TLS handshake or certificate failed',
      'address_blocked': 'the address resolves to a blocked network',
      'host_not_allowed': 'the target address is not allowed',
      'http_status': 'unexpected HTTP status',
      'throttled': 'throttled by the receiver',
      'gone': 'the receiver answered 410 Gone',
      'not_authorized': 'the app may no longer receive this event',
      'not_subscribed': 'the subscription no longer covers this event type',
      'event_expired': 'the event left the event log',
    };
    repository.deliveriesBySubscription[subscription.id] = [
      for (final (i, code) in codes.keys.indexed)
        hookDelivery('e$i', seq: 6000 + i, lastError: code),
    ];
    await view(tester);
    for (final (i, text) in codes.values.indexed) {
      expect(
        inRow('e$i', find.textContaining('last error: $text')),
        findsOneWidget,
        reason: codes.keys.elementAt(i),
      );
    }
  });

  testWidgets('reload shows what the server has now', (tester) async {
    await view(tester);
    repository.deliveriesBySubscription[subscription.id]!.insert(
      0,
      hookDelivery('d4', seq: 5304, createdAt: DateTime.utc(2026, 10, 17, 9)),
    );
    expect(find.byKey(const ValueKey('delivery.d4')), findsNothing);
    await tap(tester, 'deliveries.reload');
    expect(find.byKey(const ValueKey('delivery.d4')), findsOneWidget);
    expect(repository.deliveryRequests, ['sub-active', 'sub-active']);
  });

  testWidgets('Redeliver makes the delivery due again without step-up', (
    tester,
  ) async {
    await view(tester);
    await tap(tester, 'delivery.redeliver.d2');

    expect(repository.calls, ['redeliver d2']);
    expect(repository.stepUps, isEmpty);
    expect(find.byKey(const ValueKey('stepUp.dialog')), findsNothing);
    expect(status(tester, 'd2'), 'Waiting');
    expect(find.text('The delivery is due again.'), findsOneWidget);
  });

  testWidgets('a redelivery of an event that left the log shows the reason '
      'of the server', (tester) async {
    repository.refuse['redeliver'] = const ApiProblem(
      status: 409,
      code: 'conflict',
      reason: 'event_expired',
    );
    await view(tester);
    await tap(tester, 'delivery.redeliver.d1');

    expect(
      find.text('The event left the event log; it cannot be delivered again.'),
      findsOneWidget,
    );
    expect(status(tester, 'd1'), 'Given up');
  });

  testWidgets('a refused list shows the reason', (tester) async {
    repository.refuse['deliveries'] = const ApiProblem(
      status: 403,
      code: 'forbidden',
    );
    await view(tester);
    expect(find.byKey(const ValueKey('admin.denied')), findsOneWidget);
    expect(find.byKey(const ValueKey('deliveries.more')), findsNothing);
  });
}
