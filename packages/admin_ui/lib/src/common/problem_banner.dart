// SPDX-License-Identifier: Apache-2.0

/// What the administration says after an action or a refused list: the server's problem code
/// through `problemText` (docs/spec/01-product.md 9; 15 1 principle 1: a section follows the
/// server's answer, never an own reading of grants; brief M4-26 Design note 1).
library;

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../data/self_repository.dart';
import '../self/self_feedback.dart';

/// The text of a refused or failed administration call [error], or null when the person
/// cancelled the step-up (nothing to say).
///
/// Three problems get an explanation beyond the code: a replayed enrollment code
/// (`409 conflict`, `one_time_secret`, 06 7), `409 i1_violation` on enabling a person,
/// which the agent administration resolves (10 5.4, M4-28), and `409 i6_violation` of a grant
/// or a change of a host, with what I6 means and the server's `hint` (24 7.3). Everything else is
/// `problemText` of the code and its reason; a `403` is [SelfFailure.denied].
SelfFailure? adminFailure(AppLocalizations l, Object error) {
  if (error is SelfActionFailed) {
    final problem = error.problem;
    if (problem?.code == 'conflict' && problem?.reason == 'one_time_secret') {
      return SelfFailure(l.adminOneTimeSecret);
    }
    if (problem?.code == 'i6_violation') {
      // 24 7.3: explained with what I6 means and the server's way out, not just rejected.
      return SelfFailure(
        [
          '${problemText(l, 'i6_violation')}.',
          l.hostLlmI6Explained,
          ?problem?.hint,
        ].join(' '),
      );
    }
    if (problem?.code == 'i1_violation') {
      return SelfFailure(
        '${problemText(l, 'i1_violation')} ${l.adminI1Explained}',
      );
    }
  }
  return selfFailure(l, error);
}

/// The banner of a section whose list the server refused (`403`) or could not answer: the
/// translated reason of [problem], or the offline text without an answer. The section shows
/// nothing else, so no control appears that the server did not offer (fixed requirement 9).
class ProblemBanner extends StatelessWidget {
  /// Creates the banner for [problem] (null: no answer); [onRetry] offers "Try again".
  const ProblemBanner({super.key, required this.problem, this.onRetry});

  /// The problem of the server, or null.
  final ApiProblem? problem;

  /// Loads again, or null for no button.
  final VoidCallback? onRetry;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final p = problem;
    final denied = p != null && (p.status == 403 || p.status == 404);
    return DsBanner(
      key: ValueKey(denied ? 'admin.denied' : 'admin.loadFailed'),
      tone: denied ? DsTone.info : DsTone.warning,
      text: p == null
          ? l.selfOffline
          : problemText(l, p.code, reason: p.reason),
      actionLabel: denied || onRetry == null ? null : l.selfRetry,
      onAction: denied ? null : onRetry,
      liveRegion: true,
    );
  }
}

/// The problem of a failed load [error]: the server's answer, or null without one (a session
/// without any step-up method reads as `403 forbidden` with reason `step_up_not_possible`,
/// another error of the login layer as a revoked session).
ApiProblem? loadProblem(Object error) => switch (error) {
  SelfActionFailed(:final problem) => problem,
  StepUpUnavailable() => const ApiProblem(
    status: 403,
    code: 'forbidden',
    reason: 'step_up_not_possible',
  ),
  AuthException() => const ApiProblem(status: 401, code: 'session_revoked'),
  _ => const ApiProblem(status: 500, code: 'internal_error'),
};
