// SPDX-License-Identifier: Apache-2.0

// "Trigger again" beside the red X of a message that failed because the project or the
// thread's agent was not ready (09 3.8; F-PROJEKT-BEREIT): the cause for everyone, the button
// for the author, a refusal with its reason, and nothing for another failure.

import 'package:client/sync/message_ops_app.dart';
import 'package:client/thread/retrigger_notice.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/widgets.dart';

Message _failed(String reason) => msg(
  1,
  author: alex,
).copyWith(triggerState: TriggerState.failed, triggerStateReason: reason);

Future<AppLocalizations> pumpNotice(
  WidgetTester tester,
  Message message, {
  required bool mine,
  Retrigger? retrigger,
}) async {
  await tester.pumpWidget(
    ProviderScope(
      child: hostScreen(
        Scaffold(
          body: RetriggerNotice(
            message: message,
            mine: mine,
            retrigger: retrigger,
          ),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
  return AppLocalizations.delegate.load(const Locale('en'));
}

void main() {
  testWidgets(
    'the author triggers a message that failed for the project again',
    (tester) async {
      final sent = <MessageId>[];
      final message = _failed('operator_cannot_serve_project');
      final l = await pumpNotice(
        tester,
        message,
        mine: true,
        retrigger: (id) async {
          sent.add(id);
          return null;
        },
      );
      expect(
        find.text(l.triggerReasonOperatorCannotServeProject),
        findsOneWidget,
      );
      await tester.tap(find.byKey(const ValueKey('retrigger.button')));
      await tester.pumpAndSettle();
      expect(sent, [message.id]);
      // Accepted: the button waits for the new trigger state of the message.
      final button = tester.widget<DsButton>(
        find.byKey(const ValueKey('retrigger.button')),
      );
      expect(button.onPressed, isNull);
    },
  );

  testWidgets('failed again shows its reason and offers the button again', (
    tester,
  ) async {
    // The server answers 200 with the message failed again; the executor of
    // retriggerMessage turns that into this refusal (retrigger_op_test.dart).
    final l = await pumpNotice(
      tester,
      _failed('project_not_ready'),
      mine: true,
      retrigger: (_) async => (
        code: AppMessageOps.retriggerStillFailed,
        reason: 'project_not_ready',
      ),
    );
    await tester.tap(find.byKey(const ValueKey('retrigger.button')));
    await tester.pumpAndSettle();
    expect(
      find.text(l.retriggerRefused(l.liveBoxReasonProjectNotReady)),
      findsOneWidget,
    );
    final button = tester.widget<DsButton>(
      find.byKey(const ValueKey('retrigger.button')),
    );
    expect(button.onPressed, isNotNull);
  });

  testWidgets(
    'a new revision of the message ends the wait of an accepted send',
    (tester) async {
      final message = _failed('operator_cannot_serve_project');
      Future<void> host(Message m) => tester.pumpWidget(
        ProviderScope(
          child: hostScreen(
            Scaffold(
              body: RetriggerNotice(
                message: m,
                mine: true,
                retrigger: (_) async => null,
              ),
            ),
          ),
        ),
      );
      await host(message);
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('retrigger.button')));
      await tester.pump();
      DsButton button() => tester.widget<DsButton>(
        find.byKey(const ValueKey('retrigger.button')),
      );
      expect(button().onPressed, isNull);
      await host(
        message.copyWith(triggerStateAt: DateTime.utc(2026, 11, 4, 12)),
      );
      await tester.pump();
      expect(button().onPressed, isNotNull);
    },
  );

  testWidgets('another person sees the cause without the button', (
    tester,
  ) async {
    final l = await pumpNotice(
      tester,
      _failed('operator_not_in_channel'),
      mine: false,
    );
    expect(find.text(l.triggerReasonOperatorNotInChannel), findsOneWidget);
    expect(find.byKey(const ValueKey('retrigger.button')), findsNothing);
  });

  test('only the reasons of 09 3.8 offer "Trigger again"', () {
    expect(offersRetrigger(_failed('project_not_ready')), isTrue);
    expect(offersRetrigger(_failed('operator_cannot_serve_project')), isTrue);
    expect(offersRetrigger(_failed('operator_not_in_channel')), isTrue);
    expect(offersRetrigger(_failed('no_trigger_right')), isFalse);
  });
}
