// SPDX-License-Identifier: Apache-2.0

/// The state of the console and the flows that change it: start with the version check, the
/// login forms of E2, the step-up directly after the login, the session and E7
/// (docs/spec/08-identitaet.md 7.5, brief M6-02 Design note 2).
library;

import 'dart:math';

import 'package:admin_ui/admin_ui.dart' show AdminLocation;
import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/json_object.dart';
import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';

import '../admin/console_route.dart';
import '../admin/step_up.dart';
import '../http/console_client.dart';
import '../login/oidc_flow.dart';
import '../platform/browser.dart';
import '../routes.dart';
import '../version/version_check.dart';

/// What the console shows: one state at a time.
sealed class ConsoleState {
  const ConsoleState();
}

/// Discovery and the version check are running.
final class ConsoleStarting extends ConsoleState {
  /// Creates the state.
  const ConsoleStarting();
}

/// The server could not be reached at the start.
final class ConsoleUnavailable extends ConsoleState {
  /// Creates the state.
  const ConsoleUnavailable();
}

/// The console build and the server have different API versions; every action is blocked.
final class ConsoleOutdated extends ConsoleState {
  /// Creates the state.
  const ConsoleOutdated();
}

/// No session: E2.
final class ConsoleSignedOut extends ConsoleState {
  /// Creates the state.
  const ConsoleSignedOut();
}

/// A session without a valid step-up: the step-up screen, nothing else (08 7.5).
final class ConsoleNeedsStepUp extends ConsoleState {
  /// Creates the state for [session].
  const ConsoleNeedsStepUp(this.session);

  /// The session from `getConsoleSession` or the login.
  final ConsoleSession session;
}

/// A session with a step-up: the console with its navigation.
final class ConsoleSignedIn extends ConsoleState {
  /// Creates the state for [session].
  const ConsoleSignedIn(this.session);

  /// The session from `getConsoleSession`.
  final ConsoleSession session;
}

/// E7: the session ended.
final class ConsoleLoggedOut extends ConsoleState {
  /// Creates the state; [reasonCode] is the problem code of the `401`, null after the person's
  /// own logout.
  const ConsoleLoggedOut({this.reasonCode});

  /// Problem code that ended the session (`session_revoked`, `unauthenticated`), or null.
  final String? reasonCode;
}

/// A message on the current screen; each names the cause and the next step (01 5.6).
enum ConsoleNotice {
  /// A refused password login.
  loginFailed,

  /// A cancelled or refused passkey.
  passkeyFailed,

  /// An OIDC login the server or the login service refused.
  oidcFailed,

  /// An OIDC login cancelled at the login service.
  oidcCancelled,

  /// An OIDC callback without its stored flow, or an expired flow.
  oidcExpired,

  /// No answer from the server, or a server error.
  unavailable,

  /// A refused step-up.
  stepUpFailed,
}

/// A login provider of `getDiscovery` as the views need it.
class ConsoleProvider {
  /// Creates the provider.
  const ConsoleProvider({
    required this.id,
    required this.kind,
    required this.name,
  });

  /// `auth_providers[].id`.
  final String id;

  /// The kind of the provider.
  final ConsoleProviderKind kind;

  /// The name the button shows.
  final String name;
}

/// The kinds of login providers the console offers a button for.
enum ConsoleProviderKind {
  /// OIDC: the tab goes to the login service.
  oidc,

  /// A trusted reverse proxy: login without input.
  trustedHeader,
}

/// The step-up methods of `step_up_methods_available`.
enum StepUpMethod {
  /// WebAuthn passkey with user verification.
  passkey,

  /// New OIDC login.
  oidcReauth,

  /// Password with TOTP.
  passwordTotp,
}

/// The state machine of the console. One instance per page load; the browser keeps only the
/// cookie, which the console never reads (08 7.5).
class ConsoleController extends ChangeNotifier {
  /// Creates the controller for [environment]; [random] draws the PKCE verifiers (a secure
  /// source unless a test passes one).
  ConsoleController(this.environment, {Random? random})
    : _random = random ?? Random.secure() {
    actionStepUp = ConsoleActionStepUp(
      api: () => api,
      passkeys: environment.passkeys,
      methods: () => _methodsOf(session),
      oidcProviders: () => [
        for (final p in providers)
          if (p.kind == ConsoleProviderKind.oidc) p,
      ],
      prompt: () => stepUpPrompt,
      onOidc: stepUpWithOidc,
      onConfirmed: () => _loadSession(own: true),
    );
    api = createConsoleApiClient(
      origin: environment.origin,
      onUnauthorized: _onUnauthorized,
      interceptors: [
        ConsoleStepUpInterceptor(
          dio: () => api.dio,
          stepUp: actionStepUp.confirm,
          active: () => _state is ConsoleSignedIn,
        ),
      ],
      adapter: environment.adapter,
    );
  }

  /// The browser ports and the origin.
  final ConsoleEnvironment environment;

  /// The generated API over the console's HTTP factory.
  late final ApiClient api;

  /// The step-up of an administration action (brief M6-04 Design note 2).
  late final ConsoleActionStepUp actionStepUp;

  /// The dialog of the action step-up; the app sets it once its navigator exists, and without
  /// it an action that needs a step-up is cancelled.
  ConsoleStepUpPrompt? stepUpPrompt;

  final Random _random;
  bool _started = false;
  ConsoleState _state = const ConsoleStarting();
  ConsoleRoute _route = ConsoleRoute.ofPage(ConsolePage.live);
  ConsolePage? _initial;

  /// The public instance metadata, null until the start read it.
  Discovery? discovery;

  /// The message of the current screen, or null.
  ConsoleNotice? notice;

  /// Whether a request of the current screen is running.
  bool busy = false;

  /// The current state.
  ConsoleState get state => _state;

  /// The page the browser shows for the state: the login, the step-up, E7 or the administration
  /// page the person chose; null while starting on a path outside the table.
  ConsolePage? get page => switch (_state) {
    ConsoleStarting() || ConsoleUnavailable() || ConsoleOutdated() => _initial,
    ConsoleSignedOut() => ConsolePage.login,
    ConsoleNeedsStepUp() => ConsolePage.stepUp,
    ConsoleLoggedOut() => ConsolePage.loggedOut,
    ConsoleSignedIn() => _route.page,
  };

  /// The place of the administration the signed-in console shows (brief M6-04 Design note 1).
  ConsoleRoute get route => _route;

  /// The session of the current state, or null without one.
  ConsoleSession? get session => switch (_state) {
    ConsoleSignedIn(:final session) ||
    ConsoleNeedsStepUp(:final session) => session,
    _ => null,
  };

  /// The path the browser shows for the current page and place, relative to the console base.
  String? get path => switch (_state) {
    ConsoleSignedIn() => _route.path,
    _ => page?.path,
  };

  /// The login providers of discovery the console offers a button for.
  List<ConsoleProvider> get providers => [
    for (final p in discovery?.authProviders ?? BuiltList<AuthProviderInfo>())
      if (p.kind == IdentityProviderKind.oidc)
        ConsoleProvider(id: p.id, kind: ConsoleProviderKind.oidc, name: p.name)
      else if (p.kind == IdentityProviderKind.trustedHeader)
        ConsoleProvider(
          id: p.id,
          kind: ConsoleProviderKind.trustedHeader,
          name: p.name,
        ),
  ];

  /// Whether a `local` driver is active (passkey login).
  bool get localEnabled =>
      discovery?.authProviders.any(
        (p) => p.kind == IdentityProviderKind.local,
      ) ??
      false;

  /// Whether the password login is on.
  bool get passwordEnabled =>
      localEnabled && (discovery?.localPasswordEnabled ?? false);

  /// The step-up methods of the current session, in the order of 08 7.5.
  List<StepUpMethod> get stepUpMethods {
    final s = _state;
    if (s is! ConsoleNeedsStepUp) return const [];
    return _methodsOf(s.session);
  }

  static List<StepUpMethod> _methodsOf(ConsoleSession? session) {
    if (session == null) return const [];
    final available = session.stepUpMethodsAvailable;
    return [
      if (available.contains(ConsoleSessionStepUpMethodsAvailableEnum.passkey))
        StepUpMethod.passkey,
      if (available.contains(
        ConsoleSessionStepUpMethodsAvailableEnum.oidcReauth,
      ))
        StepUpMethod.oidcReauth,
      if (available.contains(
        ConsoleSessionStepUpMethodsAvailableEnum.passwordTotp,
      ))
        StepUpMethod.passwordTotp,
    ];
  }

  /// Starts the console at [location], the path below `/console` with its query. Reads
  /// discovery and `version.json` and stops on a version mismatch; completes an OIDC callback;
  /// otherwise reads the session (`401` leads to the login). Runs once; later calls do nothing.
  Future<void> start(Uri location) async {
    if (_started) return;
    _started = true;
    final at = ConsoleRoute.parse(location.path);
    final requested = at?.page;
    _initial = requested;
    if (at != null && at.page.isAdministration) _route = at;
    try {
      final d = (await api.getDiscoveryApi().getDiscovery()).data!;
      final built = await readBuiltSpecVersion(
        environment.origin,
        adapter: environment.adapter,
      );
      discovery = d;
      if (!versionsMatch(built: built, server: d.specVersion)) {
        _set(const ConsoleOutdated());
        return;
      }
    } on DioException {
      _set(const ConsoleUnavailable());
      return;
    }
    final query = location.queryParameters;
    if (requested == ConsolePage.loginCallback) {
      await _completeOidcLogin(query);
      return;
    }
    // A stored view whose end passed while the tab was closed answers the first request with
    // 401 step_up_required and ends it (07 9.2); the session itself is valid, so read again.
    await _loadSession(again: true);
    if (requested == ConsolePage.stepUpCallback) {
      await _completeStepUpOidc(query);
    }
  }

  /// Shows [page] at its first place when the person may see it: an administration page only
  /// with a step-up.
  void open(ConsolePage page) => openRoute(ConsoleRoute.ofPage(page));

  /// Shows the place [to] when the person may see it (an administration page only with a
  /// step-up); the same place again changes nothing.
  void openRoute(ConsoleRoute to) {
    if (_state is ConsoleSignedIn && to.page.isAdministration && to != _route) {
      _route = to;
      notifyListeners();
    }
  }

  /// Shows the place [location] of the administration, on the page of its section.
  void openAdmin(AdminLocation location) =>
      openRoute(ConsoleRoute.forAdmin(location));

  /// Reads the session again; a `401` moves to E7 through the interceptor.
  ///
  /// [afterViewEnd] is for the end of a stored view by its time: the server answers the first
  /// request after the end with `401 step_up_required` (07 9.2), so this read leaves that answer
  /// to itself and reads once more instead of starting a step-up.
  Future<void> refreshSession({bool afterViewEnd = false}) =>
      _loadSession(own: afterViewEnd, again: afterViewEnd);

  /// Logs in with [provider]: OIDC sends the tab to the login service, a proxy provider logs in
  /// without input.
  Future<void> loginWithProvider(ConsoleProvider provider) => _run(
    () async {
      if (provider.kind == ConsoleProviderKind.trustedHeader) {
        final r = await api.getAuthApi().loginWithTrustedHeader(
          trustedHeaderLoginRequest: TrustedHeaderLoginRequest(
            (b) => b.client = LoginClient.console,
          ),
        );
        _applySession(r.data!);
        return;
      }
      final pkce = PkcePair.generate(_random);
      final r = await api.getAuthApi().beginOidcLogin(
        oidcBeginRequest: OidcBeginRequest(
          (b) => b
            ..client = LoginClient.console
            ..providerId = provider.id
            ..appRedirectUri = _redirect(ConsolePage.loginCallback)
            ..clientChallenge = pkce.challenge,
        ),
      );
      _redirectTo(OidcFlowKind.login, r.data!, pkce);
    },
    refused: provider.kind == ConsoleProviderKind.oidc
        ? ConsoleNotice.oidcFailed
        : ConsoleNotice.loginFailed,
  );

  /// Logs in with a passkey; an empty [loginName] lets the browser offer discoverable passkeys.
  Future<void> loginWithPasskey(String loginName) => _run(() async {
    final name = loginName.trim();
    final auth = api.getAuthApi();
    final begin = (await auth.beginPasskeyLogin(
      passkeyLoginBeginRequest: PasskeyLoginBeginRequest((b) {
        if (name.isNotEmpty) b.loginName = name;
      }),
    )).data!;
    final credential = await environment.passkeys.get(_plain(begin.publicKey));
    final r = await auth.completePasskeyLogin(
      passkeyLoginCompleteRequest: PasskeyLoginCompleteRequest(
        (b) => b
          ..client = LoginClient.console
          ..challengeId = begin.challengeId
          ..credential.replace(_json(credential)),
      ),
    );
    _applySession(r.data!);
  }, refused: ConsoleNotice.passkeyFailed);

  /// Logs in with login name, password and the TOTP one-time code.
  Future<void> loginWithPassword(
    String loginName,
    String password,
    String code,
  ) => _run(() async {
    final r = await api.getAuthApi().loginWithPassword(
      passwordLoginRequest: PasswordLoginRequest(
        (b) => b
          ..client = LoginClient.console
          ..loginName = loginName.trim()
          ..password = password
          ..totpCode = code.trim(),
      ),
    );
    _applySession(r.data!);
  }, refused: ConsoleNotice.loginFailed);

  /// Step-up with a passkey over a fresh challenge (`createStepUpChallenge`,
  /// `completeStepUpWithPasskey`).
  Future<void> stepUpWithPasskey() => _run(() async {
    final auth = api.getAuthApi();
    final challenge = (await auth.createStepUpChallenge()).data!;
    final options = challenge.passkeyOptions;
    if (options == null) throw const PasskeyCancelled();
    final credential = await environment.passkeys.get(_plain(options));
    await auth.completeStepUpWithPasskey(
      stepUpPasskeyRequest: StepUpPasskeyRequest(
        (b) => b
          ..challengeId = challenge.challengeId
          ..credential.replace(_json(credential)),
      ),
    );
    await _loadSession();
  }, refused: ConsoleNotice.stepUpFailed);

  /// Step-up with a new OIDC login at [provider]; the tab goes to the login service and comes
  /// back on `/console/step-up/callback`.
  Future<void> stepUpWithOidc(ConsoleProvider provider) => _run(() async {
    final auth = api.getAuthApi();
    final challenge = (await auth.createStepUpChallenge()).data!;
    final pkce = PkcePair.generate(_random);
    final r = await auth.beginStepUpOidcReauth(
      stepUpOidcBeginRequest: StepUpOidcBeginRequest(
        (b) => b
          ..challengeId = challenge.challengeId
          ..providerId = provider.id
          ..appRedirectUri = _redirect(ConsolePage.stepUpCallback)
          ..clientChallenge = pkce.challenge,
      ),
    );
    _redirectTo(OidcFlowKind.stepUp, r.data!, pkce);
  }, refused: ConsoleNotice.stepUpFailed);

  /// Step-up with password and TOTP code.
  Future<void> stepUpWithPassword(String password, String code) =>
      _run(() async {
        final auth = api.getAuthApi();
        final challenge = (await auth.createStepUpChallenge()).data!;
        await auth.stepUpWithPassword(
          stepUpPasswordRequest: StepUpPasswordRequest(
            (b) => b
              ..challengeId = challenge.challengeId
              ..password = password
              ..totpCode = code.trim(),
          ),
        );
        await _loadSession();
      }, refused: ConsoleNotice.stepUpFailed);

  /// Ends the session (`logout`) and shows E7; the server clears the cookie.
  Future<void> logout() => _run(() async {
    await api.getAuthApi().logout();
    _set(const ConsoleLoggedOut());
  }, refused: ConsoleNotice.unavailable);

  /// From E7 back to E2.
  void logInAgain() => _set(const ConsoleSignedOut());

  /// Reloads the page (after a version mismatch or a failed start).
  void reload() => environment.navigation.reload();

  Future<void> _completeOidcLogin(Map<String, String> query) async {
    final verifier = _takeVerifier(OidcFlowKind.login, query);
    _set(const ConsoleSignedOut());
    final error = query['error'];
    final code = query['login_code'];
    if (error != null) {
      _notify(switch (error) {
        'access_denied' => ConsoleNotice.oidcCancelled,
        'flow_expired' => ConsoleNotice.oidcExpired,
        _ => ConsoleNotice.oidcFailed,
      });
      return;
    }
    if (verifier == null || code == null) {
      _notify(ConsoleNotice.oidcExpired);
      return;
    }
    await _run(() async {
      final r = await api.getAuthApi().completeOidcLogin(
        oidcCompleteRequest: OidcCompleteRequest(
          (b) => b
            ..client = LoginClient.console
            ..loginCode = code
            ..clientVerifier = verifier,
        ),
      );
      _applySession(r.data!);
    }, refused: ConsoleNotice.oidcFailed);
  }

  Future<void> _completeStepUpOidc(Map<String, String> query) async {
    final verifier = _takeVerifier(OidcFlowKind.stepUp, query);
    // Also a signed-in console: the OIDC re-login of an action step-up (M6-04) returns to a
    // session that had its first step-up already.
    if (_state is! ConsoleNeedsStepUp && _state is! ConsoleSignedIn) return;
    final code = query['login_code'];
    if (query['error'] != null || verifier == null || code == null) {
      _notify(ConsoleNotice.stepUpFailed);
      return;
    }
    await _run(() async {
      await api.getAuthApi().completeStepUpOidcReauth(
        identityLinkComplete: IdentityLinkComplete(
          (b) => b
            ..loginCode = code
            ..clientVerifier = verifier,
        ),
      );
      await _loadSession();
    }, refused: ConsoleNotice.stepUpFailed);
  }

  /// Reads and deletes the stored verifier of the flow of the callback [query]; the entry is
  /// gone after every callback, successful or not.
  String? _takeVerifier(OidcFlowKind kind, Map<String, String> query) {
    final attempt = query['login_attempt_id'];
    if (attempt == null || attempt.isEmpty) return null;
    final key = oidcFlowKey(kind, attempt);
    final verifier = environment.flowStore.read(key);
    environment.flowStore.delete(key);
    return verifier;
  }

  /// Stores the verifier right before the tab leaves for the login service.
  void _redirectTo(OidcFlowKind kind, OidcBeginResponse begin, PkcePair pkce) {
    environment.flowStore.write(
      oidcFlowKey(kind, begin.loginAttemptId),
      pkce.verifier,
    );
    environment.navigation.assign(Uri.parse(begin.authorizationUrl));
  }

  String _redirect(ConsolePage page) =>
      environment.origin.replace(path: page.fullPath).toString();

  Future<void> _loadSession({bool own = false, bool again = false}) async {
    try {
      _applySession(
        (await api.getAuthApi().getConsoleSession(
          extra: own ? const {consoleNoStepUpExtra: true} : null,
        )).data!,
      );
    } on DioException catch (e) {
      final code = e.response == null
          ? null
          : ApiProblem.fromResponse(e.response!).code;
      if (again && code == 'step_up_required') {
        return _loadSession(own: true);
      }
      if (_state is ConsoleStarting) {
        _set(
          e.response?.statusCode == 401
              ? const ConsoleSignedOut()
              : const ConsoleUnavailable(),
        );
      }
    }
  }

  void _applySession(ConsoleSession session) => _set(
    session.stepUpAt == null
        ? ConsoleNeedsStepUp(session)
        : ConsoleSignedIn(session),
  );

  /// A `401` of any call: during a session every code but `step_up_required` goes to E7 with
  /// the reason; before a session (login forms, the start) it changes nothing, the caller shows
  /// its own message. `step_up_required` of a signed-in console is the action's step-up (the
  /// adapter of [actionStepUp] runs it and repeats the request, brief M6-04 Design note 2); on
  /// the step-up screen it changes nothing either.
  void _onUnauthorized(ApiProblem problem) {
    final current = session;
    if (current == null || problem.code == 'step_up_required') return;
    _set(ConsoleLoggedOut(reasonCode: problem.code));
  }

  /// Runs [action] as the request of the current screen: busy while it runs, then the notice of
  /// its failure ([refused] for a refusal by the server or a passkey, unavailable without an
  /// answer or on a server error).
  Future<void> _run(
    Future<void> Function() action, {
    required ConsoleNotice refused,
  }) async {
    busy = true;
    notice = null;
    notifyListeners();
    try {
      await action();
    } on DioException catch (e) {
      final status = e.response?.statusCode;
      notice = status == null || status >= 500
          ? ConsoleNotice.unavailable
          : refused;
    } on PasskeyCancelled {
      notice = refused;
    } on FormatException {
      notice = refused;
    } finally {
      busy = false;
      notifyListeners();
    }
  }

  void _set(ConsoleState state) {
    // E7 ends the session: the next login in this tab starts at the first page, not at a place
    // of the person who logged out (brief M6-04, D-M6-04-3).
    if (state is ConsoleLoggedOut) {
      _route = ConsoleRoute.ofPage(ConsolePage.live);
    }
    _state = state;
    notice = null;
    notifyListeners();
  }

  void _notify(ConsoleNotice value) {
    notice = value;
    notifyListeners();
  }

  static BuiltMap<String, JsonObject?> _json(Map<String, Object?> map) =>
      BuiltMap<String, JsonObject?>({
        for (final e in map.entries)
          e.key: e.value == null ? null : JsonObject(e.value),
      });

  static Map<String, Object?> _plain(BuiltMap<String, JsonObject?> map) => {
    for (final e in map.entries) e.key: e.value?.value,
  };
}
