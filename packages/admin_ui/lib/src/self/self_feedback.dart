// SPDX-License-Identifier: Apache-2.0

/// What the device and session screens say after an action (docs/spec/01-product.md 9: the app
/// translates the problem code; 15 1 principle 1: controls follow the server's answers).
library;

import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:auth/auth.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/widgets.dart';
import 'package:l10n/l10n.dart';

import '../common/app_fault.dart';
import '../data/self_repository.dart';

/// A message of the screen: its [text] and [tone] (`null` for a plain success).
final class SelfNotice {
  /// Creates the notice.
  const SelfNotice(this.text, {this.tone = DsTone.info});

  /// The text in the person's language.
  final String text;

  /// The tone of the banner.
  final DsTone tone;
}

/// The failure of an action as the screen shows it.
final class SelfFailure {
  /// Creates the failure.
  const SelfFailure(this.text, {this.denied = false});

  /// The text in the person's language.
  final String text;

  /// Whether the server refused the action (`403`): the screen hides it for that item.
  final bool denied;
}

/// The failure for [error], or null when the person cancelled the step-up (nothing to say).
///
/// A `404` reads as [notFound] when given; a `403` is [SelfFailure.denied] with the translated
/// reason; no answer is the offline text; every other problem is its translated code (with its
/// reason, for example a locked password step-up). A session without any step-up method
/// ([StepUpUnavailable]) reads as `step_up_not_possible`, a refresh that got no answer
/// ([RefreshUnavailable], temporary) as the offline text, and a session that is gone
/// ([SessionEnded], [NotSignedIn]) as a revoked session. `loadProblem` and `newAgentFailureOf`
/// read the errors of the login layer the same way (ruling AUD-AA1/4).
///
/// Any other error is a fault of the app, not of the server: it goes to the reporter of the
/// package ([reportAppFault]) and reads as [AppLocalizations.selfAppFault] (ruling AUD-AM1/3).
SelfFailure? selfFailure(AppLocalizations l, Object error, {String? notFound}) {
  if (error is StepUpCancelled) return null;
  if (error is SelfActionFailed) {
    final problem = error.problem;
    if (problem == null) return SelfFailure(l.selfOffline);
    if (problem.status == 404 && notFound != null) return SelfFailure(notFound);
    return SelfFailure(
      serverProblemText(l, problem),
      denied: problem.status == 403,
    );
  }
  // A session without any step-up method is not a revoked one (SEC-STEPUP-LOCKOUT).
  if (error is StepUpUnavailable) {
    return SelfFailure(
      problemText(l, 'forbidden', reason: 'step_up_not_possible'),
      denied: true,
    );
  }
  // The refresh could not reach the server: the session is still there (brief M4-09 note 4).
  if (error is RefreshUnavailable) return SelfFailure(l.selfOffline);
  if (error is AuthException) return SelfFailure(l.problemSessionRevoked);
  reportAppFault(error);
  return SelfFailure(l.selfAppFault);
}

/// The text of the server's [problem]: its translated code with its reason, and after it the
/// correlation ID of the failed request where only the server's log can tell more, that is with
/// a server error (a status of 500 and above; 06 5: "`correlation_id` for the log search") and
/// with a code this build has no text for. A refusal or a conflict the person can read and act
/// on shows no ID.
///
/// Example: "Something went wrong on the server (correlation ID 4bf92f35-...)".
String serverProblemText(AppLocalizations l, ApiProblem problem) {
  final text = problemText(l, problem.code, reason: problem.reason);
  final id = problem.correlationId;
  if (id == null) return text;
  final unexplained =
      problem.status >= 500 || text == l.problemFallback(problem.code);
  return unexplained ? l.selfProblemCorrelation(text, id) : text;
}

/// The banner of [notice], announced to screen readers.
Widget selfNoticeBanner(SelfNotice notice) => Padding(
  padding: const EdgeInsets.only(bottom: DsSpace.s8),
  child: DsBanner(
    key: const ValueKey('self.notice'),
    tone: notice.tone,
    text: notice.text,
    liveRegion: true,
  ),
);
