// SPDX-License-Identifier: Apache-2.0

/// Login, tokens, DPoP proofs, step-up and refresh of the apps, without Flutter (15 3.1,
/// 15 8).
///
/// Build one [AuthSession] per server address; it installs [RefreshInterceptor],
/// [DpopInterceptor] and [StepUpInterceptor] into the client of `createApiClient`, exposes the
/// login flows ([LoginService]), the single-flight refresh ([RefreshCoordinator]), the step-up
/// ([StepUpService]) and the DPoP proofs ([DpopProofFactory]) and reports [AuthEvent]s. The
/// outside world comes in through the ports of `ports.dart`.
library;

export 'src/auth_events.dart';
export 'src/auth_session.dart';
export 'src/dpop.dart';
export 'src/errors.dart';
export 'src/interceptors.dart'
    show
        DpopInterceptor,
        RefreshInterceptor,
        StepUpInterceptor,
        stepUpPurposeExtra;
export 'src/jose.dart' show accessTokenHash, jwkThumbprint;
export 'src/login_flows.dart';
export 'src/nonce_store.dart';
export 'src/pkce.dart';
export 'src/ports.dart';
export 'src/redact.dart';
export 'src/refresh.dart' show RefreshCoordinator;
export 'src/step_up.dart';
export 'src/tokens.dart' show SessionInfo, TokenState;
