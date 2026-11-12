// SPDX-License-Identifier: Apache-2.0

// What the screens say after a failed step-up (SEC-STEPUP-LOCKOUT): a session without any
// step-up method is not a revoked session, and a locked password step-up shows its reason.
// And the one reading of every error of the login layer by the three mappings of the package
// (`selfFailure`, `loadProblem`, `newAgentFailureOf`; ruling AUD-AA1/4): cancelled, no step-up
// method, offline, session ended.

import 'package:admin_ui/src/agents/new_agent/new_agent_port.dart';
import 'package:admin_ui/src/common/problem_banner.dart';
import 'package:admin_ui/src/data/self_repository.dart';
import 'package:admin_ui/src/self/self_feedback.dart';
import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:dio/dio.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

void main() {
  final l = lookupAppLocalizations(const Locale('en'));

  test('a session without a step-up method says so, not "revoked"', () {
    expect(
      selfFailure(l, const StepUpUnavailable())?.text,
      l.problemForbiddenStepUpNotPossible,
    );
    expect(selfFailure(l, const StepUpUnavailable())?.denied, isTrue);
    final problem = loadProblem(const StepUpUnavailable())!;
    expect(
      (problem.status, problem.code, problem.reason),
      (403, 'forbidden', 'step_up_not_possible'),
    );
    expect(
      selfFailure(l, const SessionEnded(SignOutReason.sessionRevoked))?.text,
      l.problemSessionRevoked,
    );
  });

  test('a locked password step-up shows its reason', () {
    const locked = ApiProblem(
      status: 401,
      code: 'step_up_required',
      reason: 'password_locked',
    );
    expect(
      selfFailure(l, const SelfActionFailed(locked))?.text,
      l.problemStepUpRequiredPasswordLocked,
    );
  });

  group(
    'every error of the login layer reads the same in the three mappings',
    () {
      const revoked = ApiProblem(status: 401, code: 'session_revoked');
      // What `auth` throws, and what the screens say: the text of an action (null: nothing to
      // say), the problem of a load (null: no answer, the offline text) and the failure of "New
      // agent". A load that asks for a step-up handles its cancel itself, so `loadProblem` has no
      // reading of it.
      final cases =
          <
            ({
              AuthException error,
              String? text,
              ApiProblem? load,
              NewAgentFailure failure,
            })
          >[
            (
              error: const StepUpUnavailable(),
              text: l.problemForbiddenStepUpNotPossible,
              load: const ApiProblem(
                status: 403,
                code: 'forbidden',
                reason: 'step_up_not_possible',
              ),
              failure: const NewAgentFailure.problem(
                'forbidden',
                reason: 'step_up_not_possible',
              ),
            ),
            // A refresh that got no answer is temporary: the session is still there.
            (
              error: const RefreshUnavailable(),
              text: l.selfOffline,
              load: null,
              failure: const NewAgentFailure.offline(),
            ),
            (
              error: const RefreshUnavailable(status: 503),
              text: l.selfOffline,
              load: null,
              failure: const NewAgentFailure.offline(),
            ),
            (
              error: const SessionEnded(SignOutReason.sessionRevoked),
              text: l.problemSessionRevoked,
              load: revoked,
              failure: const NewAgentFailure.problem('session_revoked'),
            ),
            (
              error: const NotSignedIn(),
              text: l.problemSessionRevoked,
              load: revoked,
              failure: const NewAgentFailure.problem('session_revoked'),
            ),
          ];

      void expectFailure(Object thrown, NewAgentFailure failure) {
        final mapped = newAgentFailureOf(thrown)!;
        expect(
          (mapped.kind, mapped.code, mapped.reason),
          (failure.kind, failure.code, failure.reason),
        );
      }

      for (final c in cases) {
        test('${c.error}', () {
          expect(selfFailure(l, c.error)?.text, c.text);
          final loaded = loadProblem(c.error);
          expect(
            (loaded?.status, loaded?.code, loaded?.reason),
            (c.load?.status, c.load?.code, c.load?.reason),
          );
          expectFailure(c.error, c.failure);
          // The interceptors of `auth` hand the error over inside a `DioException`.
          expectFailure(
            DioException(requestOptions: RequestOptions(), error: c.error),
            c.failure,
          );
        });
      }

      test(
        'a cancelled step-up says nothing after an action and is a cancel in '
        '"New agent"',
        () {
          expect(selfFailure(l, const StepUpCancelled()), isNull);
          expectFailure(
            const StepUpCancelled(),
            const NewAgentFailure.stepUpCancelled(),
          );
        },
      );
    },
  );
}
