// SPDX-License-Identifier: Apache-2.0

// The outgoing webhook subscriptions of an app (brief M6-08 Design note 3, Ruling 4; 11 6, 7):
// the list in every state, a disabled one with when and why, the form limited to the granted
// events and to https with the server's refusal of a host, the HMAC secret shown once, pause,
// resume and delete through one confirmation and the server's step-up, and the signing keys
// receivers verify `v1a` with.

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';
import 'hooks_support.dart';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;
  late AppRecord app;
  late int changes;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = FakeAdminRepository(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    );
    app = seedBridgeHooks(repository);
    changes = 0;
  });

  Future<void> tap(WidgetTester tester, String key) async {
    await tester.ensureVisible(find.byKey(ValueKey(key)));
    await tester.tap(find.byKey(ValueKey(key)));
    await tester.pumpAndSettle();
  }

  Future<void> type(WidgetTester tester, String key, String text) async {
    await tester.enterText(find.byKey(ValueKey(key)), text);
    await tester.pump();
  }

  Future<void> section(WidgetTester tester) async {
    await pumpInApp(
      tester,
      AppSubscriptionsSection(
        repository: repository,
        app: app,
        timeZone: 'Europe/Berlin',
        clock: testClock,
        onChanged: () => changes++,
      ),
      navigatorKey: navigator,
    );
  }

  Finder inRow(String id, Finder f) =>
      find.descendant(of: find.byKey(ValueKey('subs.row.$id')), matching: f);

  String text(WidgetTester tester, String key) =>
      tester.widget<Text>(find.byKey(ValueKey(key))).data!;

  bool enabled(WidgetTester tester, String key) =>
      tester.widget<ButtonStyleButton>(find.byKey(ValueKey(key))).onPressed !=
      null;

  testWidgets(
    'the list shows address, events, state, failures, schemes and key',
    (tester) async {
      await section(tester);

      expect(find.byKey(const ValueKey('subs.section')), findsOneWidget);
      expect(find.text('Outgoing webhooks'), findsOneWidget);
      expect(
        inRow(
          'sub-active',
          find.text('https://bridge.example.com/hooks/sub-active'),
        ),
        findsOneWidget,
      );
      expect(
        inRow(
          'sub-active',
          find.textContaining('message.created, message.updated'),
        ),
        findsOneWidget,
      );
      expect(text(tester, 'subs.state.sub-active'), 'Active');
      expect(text(tester, 'subs.state.sub-paused'), 'Paused');
      expect(text(tester, 'subs.state.sub-gap'), 'Disabled');
      expect(text(tester, 'subs.state.sub-newer'), 'Unknown state');
      expect(
        inRow('sub-paused', find.textContaining('2 failures in a row')),
        findsOneWidget,
      );
      expect(
        inRow('sub-paused', find.textContaining('v1a, v1')),
        findsOneWidget,
      );
      expect(
        inRow('sub-paused', find.textContaining('whk-2026-10')),
        findsOneWidget,
      );
      // Pause where it delivers, resume where it does not; every row can be deleted.
      expect(find.byKey(const ValueKey('subs.pause.sub-active')), findsOne);
      expect(
        find.byKey(const ValueKey('subs.resume.sub-active')),
        findsNothing,
      );
      expect(find.byKey(const ValueKey('subs.resume.sub-paused')), findsOne);
      expect(find.byKey(const ValueKey('subs.resume.sub-gap')), findsOne);
      expect(find.byKey(const ValueKey('subs.delete.sub-gap')), findsOne);
      expect(find.byKey(const ValueKey('subs.deliveries.sub-gap')), findsOne);
      expect(
        find.byKey(const ValueKey('subs.reason.sub-active')),
        findsNothing,
      );
    },
  );

  testWidgets('a disabled subscription shows when and the reason of the '
      'server: the gap, a 410 and failures in a row', (tester) async {
    await section(tester);

    expect(
      inRow('sub-gap', find.textContaining('Disabled Yesterday')),
      findsOneWidget,
    );
    expect(
      text(tester, 'subs.reason.sub-gap'),
      'A gap: events left the event log before they were delivered.',
    );
    expect(
      text(tester, 'subs.reason.sub-gone'),
      'The receiver answered 410 Gone.',
    );
    expect(
      text(tester, 'subs.reason.sub-failing'),
      'Disabled after 25 failures in a row.',
    );
    // The newest delivery of each disabled subscription was asked for, nothing else.
    expect(
      repository.deliveryRequests,
      unorderedEquals(['sub-gap', 'sub-gone', 'sub-failing']),
    );
  });

  testWidgets('the reason comes from the newest finished delivery, not from '
      'a later delivery that still waits', (tester) async {
    // The server lists by sequence, newest first (D-M2-26-20); after a gap or a 410 the
    // later events still wait behind the delivery that ended the subscription.
    for (final id in ['sub-gap', 'sub-gone']) {
      repository.deliveriesBySubscription[id]!.insert(
        0,
        hookDelivery('$id-later', seq: 5400, status: DeliveryStatus.pending),
      );
    }
    await section(tester);

    expect(
      text(tester, 'subs.reason.sub-gap'),
      'A gap: events left the event log before they were delivered.',
    );
    expect(
      text(tester, 'subs.reason.sub-gone'),
      'The receiver answered 410 Gone.',
    );
  });

  testWidgets('the form cannot be dismissed by a tap outside, so an answer '
      'with a secret is never dropped', (tester) async {
    await section(tester);
    await tap(tester, 'subs.create');
    await tester.tapAt(const Offset(4, 4));
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('subs.form')), findsOneWidget);
  });

  testWidgets('the form offers only the granted events and needs https', (
    tester,
  ) async {
    await section(tester);
    await tap(tester, 'subs.create');

    expect(find.byKey(const ValueKey('subs.form')), findsOneWidget);
    expect(find.byKey(const ValueKey('subs.event.message.created')), findsOne);
    expect(find.byKey(const ValueKey('subs.event.message.updated')), findsOne);
    // Named by the manifest, but not granted by the installation.
    expect(
      find.byKey(const ValueKey('subs.event.reaction.added')),
      findsNothing,
    );
    expect(enabled(tester, 'subs.submit'), isFalse);

    await type(tester, 'subs.url', 'http://bridge.example.com/hooks/new');
    await tap(tester, 'subs.event.message.created');
    expect(find.text('The address must start with https://.'), findsOneWidget);
    expect(enabled(tester, 'subs.submit'), isFalse);

    await type(tester, 'subs.url', 'https://bridge.example.com/hooks/new');
    expect(find.text('The address must start with https://.'), findsNothing);
    expect(enabled(tester, 'subs.submit'), isTrue);
    await tap(tester, 'subs.event.message.created');
    expect(enabled(tester, 'subs.submit'), isFalse, reason: 'no event');
    await tap(tester, 'subs.event.message.created');

    await tap(tester, 'subs.submit');
    expect(find.byKey(const ValueKey('stepUp.dialog')), findsOneWidget);
    await tap(tester, 'stepUp.continue');

    expect(repository.calls, [
      'createSubscription app-repo-bridge '
          'https://bridge.example.com/hooks/new message.created hmac=false',
    ]);
    expect(repository.stepUps, [
      'Create a webhook subscription for Repo Bridge',
    ]);
    expect(find.byKey(const ValueKey('subs.form')), findsNothing);
    // Without HMAC there is no secret to show.
    expect(find.byKey(const ValueKey('hmac.secretDialog')), findsNothing);
    expect(find.text('https://bridge.example.com/hooks/new'), findsOneWidget);
    expect(changes, 1);
  });

  testWidgets('the server refuses a host it does not allow; the form says '
      'why and stays open', (tester) async {
    repository.stepUpFresh = true;
    await section(tester);
    await tap(tester, 'subs.create');
    await type(tester, 'subs.url', 'https://other.example.org/hooks');
    await tap(tester, 'subs.event.message.updated');
    await tap(tester, 'subs.submit');

    expect(find.byKey(const ValueKey('subs.form')), findsOneWidget);
    expect(
      find.text('The webhook target is not on the allow list'),
      findsOneWidget,
    );
    expect(repository.subscriptionsByApp[app.id], hasLength(6));
    expect(changes, 0);
  });

  testWidgets('with HMAC the secret is shown once and the section keeps none', (
    tester,
  ) async {
    repository.stepUpFresh = true;
    await section(tester);
    await tap(tester, 'subs.create');
    await type(tester, 'subs.url', 'https://bridge.example.com/hooks/signed');
    await tap(tester, 'subs.event.message.created');
    await tap(tester, 'subs.hmac');
    await tap(tester, 'subs.submit');

    expect(repository.calls.single, endsWith('hmac=true'));
    final secret = repository.handedOut.single;
    expect(find.byKey(const ValueKey('hmac.secretDialog')), findsOneWidget);
    expect(find.text(secret), findsOneWidget);
    expect(find.textContaining('shown only now'), findsOneWidget);

    await tap(tester, 'hmac.secretClose');
    expect(find.byKey(const ValueKey('hmac.secretDialog')), findsNothing);
    expect(find.textContaining(secret), findsNothing);
    for (final w in tester.allWidgets) {
      expect(w.toStringDeep(), isNot(contains(secret)));
    }
    final row = inRow('sub-0', find.textContaining('v1a, v1'));
    expect(row, findsOneWidget);
  });

  testWidgets('pause and resume change the state through one confirmation', (
    tester,
  ) async {
    await section(tester);
    await tap(tester, 'subs.pause.sub-active');
    expect(find.byKey(const ValueKey('confirm.dialog')), findsOneWidget);
    await tap(tester, 'confirm.submit');
    expect(find.byKey(const ValueKey('stepUp.dialog')), findsOneWidget);
    await tap(tester, 'stepUp.continue');

    expect(
      repository.calls.single,
      startsWith('updateSubscription sub-active'),
    );
    expect(repository.calls.single, contains('paused'));
    expect(repository.stepUps, [
      'Change the webhook subscription to '
          'https://bridge.example.com/hooks/sub-active',
    ]);
    expect(text(tester, 'subs.state.sub-active'), 'Paused');
    expect(find.byKey(const ValueKey('subs.resume.sub-active')), findsOne);

    await tap(tester, 'subs.resume.sub-gap');
    await tap(tester, 'confirm.submit');
    expect(repository.calls.last, startsWith('updateSubscription sub-gap'));
    expect(repository.calls.last, contains('active'));
    expect(text(tester, 'subs.state.sub-gap'), 'Active');
    expect(find.byKey(const ValueKey('subs.reason.sub-gap')), findsNothing);
    expect(changes, 2);
  });

  testWidgets('cancelling the confirmation changes nothing', (tester) async {
    await section(tester);
    await tap(tester, 'subs.pause.sub-active');
    await tap(tester, 'confirm.cancel');
    expect(repository.calls, isEmpty);
    expect(text(tester, 'subs.state.sub-active'), 'Active');
  });

  testWidgets('delete removes the subscription after one confirmation', (
    tester,
  ) async {
    repository.stepUpFresh = true;
    await section(tester);
    await tap(tester, 'subs.delete.sub-paused');
    expect(
      find.text(
        'Delete the subscription to '
        'https://bridge.example.com/hooks/sub-paused?',
      ),
      findsOneWidget,
    );
    await tap(tester, 'confirm.submit');

    expect(repository.calls, ['deleteSubscription sub-paused']);
    expect(find.byKey(const ValueKey('subs.row.sub-paused')), findsNothing);
    expect(changes, 1);
  });

  testWidgets('a refused change shows the reason of the server', (
    tester,
  ) async {
    repository.stepUpFresh = true;
    repository.refuse['deleteSubscription'] = const ApiProblem(
      status: 403,
      code: 'forbidden',
    );
    await section(tester);
    await tap(tester, 'subs.delete.sub-active');
    await tap(tester, 'confirm.submit');

    expect(find.text('You are not allowed to do this'), findsOneWidget);
    expect(find.byKey(const ValueKey('subs.row.sub-active')), findsOneWidget);
  });

  testWidgets('a refused list shows the reason and no control', (tester) async {
    repository.refuse['subscriptions'] = const ApiProblem(
      status: 403,
      code: 'forbidden',
    );
    await section(tester);
    expect(find.byKey(const ValueKey('admin.denied')), findsOneWidget);
    expect(find.byKey(const ValueKey('subs.create')), findsNothing);
  });

  testWidgets('Deliveries opens the deliveries of the subscription', (
    tester,
  ) async {
    await section(tester);
    await tap(tester, 'subs.deliveries.sub-gone');
    expect(find.byKey(const ValueKey('deliveries.view')), findsOneWidget);
    expect(find.byKey(const ValueKey('delivery.d-gone')), findsOneWidget);
  });

  testWidgets('the keys card shows kid, state and public key with copy', (
    tester,
  ) async {
    final copied = <String>[];
    tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
      SystemChannels.platform,
      (call) async {
        if (call.method == 'Clipboard.setData') {
          copied.add((call.arguments as Map)['text'] as String);
        }
        return null;
      },
    );
    addTearDown(
      () => tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
        SystemChannels.platform,
        null,
      ),
    );
    await section(tester);

    final card = find.byKey(const ValueKey('keys.card'));
    expect(card, findsOneWidget);
    Finder inCard(Finder f) => find.descendant(of: card, matching: f);
    expect(inCard(find.textContaining('v1a')), findsWidgets);
    expect(inCard(find.text('whk-2026-10')), findsOneWidget);
    expect(inCard(find.text('Active')), findsOneWidget);
    const key = 'whpk_MCowBQYDK2VwAyEAEXAMPLEPublicKeyOfTheWebhookSignatures=';
    expect(inCard(find.text(key)), findsOneWidget);

    await tap(tester, 'keys.copy.whk-2026-10');
    expect(copied, [key]);
    expect(inCard(find.text('Copied')), findsOneWidget);
  });
}
