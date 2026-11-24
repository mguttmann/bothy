// SPDX-License-Identifier: Apache-2.0

// The state of a status view (brief M7-21 Design note 1): a read that ends with an error of the
// login layer or of the app ends the loading state, tells the listeners and never leaves an old
// page as if it were current.

import 'dart:async';

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart' as api;
import 'package:auth/auth.dart'
    show RefreshUnavailable, SessionEnded, SignOutReason;
import 'package:flutter_test/flutter_test.dart';

void main() {
  Future<StatusController> started(Object error) async {
    final controller = StatusController(
      LiveStatusSource(
        loader: () async => throw error,
        changes: const Stream.empty(),
        unreachable: const Stream.empty(),
      ),
    );
    addTearDown(controller.dispose);
    var notified = 0;
    controller
      ..addListener(() => notified++)
      ..start();
    await controller.refresh();
    expect(notified, greaterThan(0), reason: 'the view hears of the end');
    expect(controller.loading, isFalse);
    return controller;
  }

  test(
    'a refresh without an answer (RefreshUnavailable) reads as unreachable',
    () async {
      final controller = await started(const RefreshUnavailable());
      expect(controller.unreachable, isTrue);
      expect(controller.page, isNull);
    },
  );

  test('an ended session is a problem of the read, not a hang', () async {
    final controller = await started(
      const SessionEnded(SignOutReason.sessionRevoked),
    );
    expect(controller.problem?.code, 'session_revoked');
    expect(controller.unreachable, isFalse);
  });

  test('a fault of the app is reported and shown as one', () async {
    final reported = <Object>[];
    final before = adminFaultReporter;
    adminFaultReporter = (error, _) => reported.add(error);
    addTearDown(() => adminFaultReporter = before);
    final controller = await started(TypeError());
    expect(controller.problem?.code, appFaultCode);
    expect(reported.single, isA<TypeError>());
  });

  test('a refused read keeps its problem', () async {
    final controller = await started(
      const SelfActionFailed(api.ApiProblem(status: 403, code: 'forbidden')),
    );
    expect(controller.problem?.code, 'forbidden');
  });
}
