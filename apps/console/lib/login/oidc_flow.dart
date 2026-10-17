// SPDX-License-Identifier: Apache-2.0

/// Keys of the OIDC flows in the tab's sessionStorage (brief M6-02 Design note 2).
library;

/// The two OIDC flows of the console.
enum OidcFlowKind {
  /// The login (E2), back on `/console/login/callback`.
  login('login'),

  /// The re-login of the step-up, back on `/console/step-up/callback`.
  stepUp('step_up');

  const OidcFlowKind(this.tag);

  /// The tag of the kind in the storage key.
  final String tag;
}

/// The storage key of the `client_verifier` of one flow: bound to its kind and its
/// `login_attempt_id`, so a callback can only redeem the verifier of its own flow.
///
/// Example: `oidcFlowKey(OidcFlowKind.login, attemptId)`.
String oidcFlowKey(OidcFlowKind kind, String loginAttemptId) =>
    'bothy.console.oidc.${kind.tag}.$loginAttemptId';
