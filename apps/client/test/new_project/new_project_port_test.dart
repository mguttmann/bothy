// SPDX-License-Identifier: Apache-2.0

// The errors of the new project port as the dialog shows them: a session without any step-up
// method is not a cancel, and a locked password step-up shows its reason (SEC-STEPUP-LOCKOUT).

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:client/new_project/new_project_dialog.dart';
import 'package:client/new_project/new_project_flow.dart';
import 'package:client/new_project/new_project_port.dart';
import 'package:dio/dio.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

DioException _wrapped(Object error) => DioException(
  requestOptions: RequestOptions(path: '/channels'),
  error: error,
);

DioException _answered(ApiProblem problem) {
  final options = RequestOptions(path: '/channels');
  return DioException(
    requestOptions: options,
    response: Response<Object?>(
      requestOptions: options,
      statusCode: problem.status,
    ),
    error: problem,
  );
}

void main() {
  final l = lookupAppLocalizations(const Locale('en'));

  test('a cancelled step-up and a lost session read as a cancel', () {
    for (final e in [
      const StepUpCancelled(),
      _wrapped(const StepUpCancelled()),
      _wrapped(const NotSignedIn()),
    ]) {
      expect(
        newProjectFailureOf(e)?.kind,
        NewProjectFailureKind.stepUpCancelled,
      );
    }
  });

  test('a session without a step-up method is not a cancel', () {
    for (final e in [
      const StepUpUnavailable(),
      _wrapped(const StepUpUnavailable()),
    ]) {
      final failure = newProjectFailureOf(e)!;
      expect(
        (failure.kind, failure.code, failure.reason),
        (NewProjectFailureKind.problem, 'forbidden', 'step_up_not_possible'),
      );
      expect(
        newProjectFailureText(l, NewProjectStep.createForum, failure),
        l.problemForbiddenStepUpNotPossible,
      );
    }
  });

  test('a locked password step-up shows its reason', () {
    final failure = newProjectFailureOf(
      _answered(
        const ApiProblem(
          status: 401,
          code: 'step_up_required',
          reason: 'password_locked',
        ),
      ),
    )!;
    expect(
      newProjectFailureText(l, NewProjectStep.bindProject, failure),
      l.problemStepUpRequiredPasswordLocked,
    );
  });

  test('no answer is offline and other errors pass', () {
    expect(
      newProjectFailureOf(_wrapped(Exception('socket')))?.kind,
      NewProjectFailureKind.offline,
    );
    expect(newProjectFailureOf(StateError('bug')), isNull);
  });
}
