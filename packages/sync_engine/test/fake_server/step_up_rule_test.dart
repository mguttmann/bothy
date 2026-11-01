// SPDX-License-Identifier: Apache-2.0

// The step-up rule of the fake server follows the server (07 section 4.2 step 8, 08 6.3): a
// step-up counts for auth.step_up_max_age, but the terminal's fresh step-up only for
// terminal.step_up_max_age and only with the device key.

import 'package:sync_engine/src/testing/rest/self_handlers.dart';
import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

void main() {
  final start = DateTime.utc(2026, 11, 1, 10);
  var now = start;
  final world = FakeWorld(now: () => now);

  FakeRequest request({required Duration age, String method = 'device_key'}) {
    final session =
        FakeSession(
            id: 's1',
            userId: 'u1',
            deviceId: 'd1',
            authMethod: 'oidc',
            createdAt: world.stamp(start),
            expiresAt: world.stamp(start.add(const Duration(days: 1))),
            jkt: 'jkt',
          )
          ..stepUpAt = start
          ..stepUpMethod = method;
    now = start.add(age);
    return FakeRequest(
      operationId: 'openTerminalSession',
      method: 'POST',
      uri: Uri.parse('http://127.0.0.1/api/v1/terminal/sessions'),
      pathParams: const {},
      headers: const {},
      body: null,
      world: world,
      baseUri: Uri.parse('http://127.0.0.1'),
      session: session,
    );
  }

  Matcher refused(String? reason) => throwsA(
    isA<FakeProblem>()
        .having((p) => p.status, 'status', 401)
        .having((p) => p.code, 'code', 'step_up_required')
        .having((p) => p.reason, 'reason', reason),
  );

  test('a step-up counts for auth.step_up_max_age', () {
    expect(() => requireStepUp(request(age: stepUpMaxAge)), returnsNormally);
    expect(
      () => requireStepUp(
        request(age: stepUpMaxAge + const Duration(seconds: 1)),
      ),
      refused(null),
    );
  });

  test('the fresh step-up counts only for terminal.step_up_max_age', () {
    expect(
      () => requireStepUp(request(age: terminalStepUpMaxAge), fresh: true),
      returnsNormally,
    );
    expect(
      () => requireStepUp(
        request(age: terminalStepUpMaxAge + const Duration(seconds: 1)),
        fresh: true,
      ),
      refused('terminal_step_up_not_fresh'),
    );
    expect(
      () => requireStepUp(
        request(age: stepUpMaxAge + const Duration(seconds: 1)),
        fresh: true,
      ),
      refused('terminal_step_up_not_fresh'),
    );
  });

  test('the fresh step-up needs the device key', () {
    expect(
      () => requireStepUp(
        request(age: Duration.zero, method: 'password_totp'),
        fresh: true,
      ),
      refused('terminal_step_up_not_fresh'),
    );
  });
}
