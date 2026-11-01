// SPDX-License-Identifier: Apache-2.0

// What the screens say after a failed step-up (SEC-STEPUP-LOCKOUT): a session without any
// step-up method is not a revoked session, and a locked password step-up shows its reason.

import 'package:admin_ui/src/common/problem_banner.dart';
import 'package:admin_ui/src/data/self_repository.dart';
import 'package:admin_ui/src/self/self_feedback.dart';
import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
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
}
