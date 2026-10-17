// SPDX-License-Identifier: Apache-2.0

/// The flow of the onboarding screens E1 to E7 over the login flows of `auth` (docs/spec/
/// 01-produkt.md 5.6; 08 5 and 14; brief M4-18 Design note 3).
library;

import 'dart:async';

import 'package:auth/auth.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart' show AppConfigValues;
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:sync_engine/sync_engine.dart';

import '../app/account_wiper.dart';
import '../app/bootstrap.dart';
import '../app/discovery.dart';
import '../app/providers/ports.dart';
import '../app/router.dart';
import '../platform/secure_store.dart';

export '../app/router.dart' show OnboardingStep;

part 'onboarding_controller.g.dart';

/// What E1 shows below the address field.
enum ServerStatus {
  /// Nothing yet.
  idle,

  /// `getDiscovery` runs.
  connecting,

  /// The text is no address.
  invalidAddress,

  /// No instance at this address.
  noInstance,

  /// The TLS handshake failed; no way to continue (R-PR-14).
  certificateInvalid,

  /// `setup_required`.
  notSetUp,

  /// Below `min_client_versions`.
  appTooOld,
}

/// The error a login screen shows (E2, E3, E5).
enum LoginNotice {
  /// A rejected login or code; always the same text (08 3.2).
  failed,

  /// No answer from the server.
  unreachable,

  /// The OIDC login in the browser was cancelled or denied.
  browserCancelled,

  /// The OIDC attempt expired.
  browserExpired,
}

/// A method of E3 for the enrollment code.
sealed class EnrollMethod {
  const EnrollMethod();
}

/// Redeem with the device keys only (`redeemEnrollmentCode`).
final class EnrollThisDevice extends EnrollMethod {
  /// Creates the method.
  const EnrollThisDevice();
}

/// Redeem together with a new passkey.
final class EnrollWithPasskey extends EnrollMethod {
  /// Creates the method.
  const EnrollWithPasskey();
}

/// Link a login service (OIDC or proxy) with the code.
final class EnrollWithProvider extends EnrollMethod {
  /// Creates the method.
  const EnrollWithProvider(this.provider);

  /// The provider.
  final LoginProvider provider;
}

/// The state of the onboarding: which screen shows and what it needs.
final class OnboardingState {
  /// Creates a state; prefer the named constructors.
  const OnboardingState({
    required this.step,
    this.instance,
    this.services,
    this.address = '',
    this.serverStatus = ServerStatus.idle,
    this.minVersion,
    this.notice,
    this.busy = false,
    this.code,
    this.deadline,
    this.pairingExpired = false,
    this.cause,
  });

  /// The screen.
  final OnboardingStep step;

  /// The instance of E1, once discovery succeeded (branding, providers).
  final InstanceInfo? instance;

  /// The services of the instance while a login runs and after it.
  final InstanceServices? services;

  /// The address text of E1.
  final String address;

  /// The result line of E1.
  final ServerStatus serverStatus;

  /// The required version for [ServerStatus.appTooOld].
  final String? minVersion;

  /// The error of E2, E3 or E5.
  final LoginNotice? notice;

  /// Whether a request runs; the buttons are off meanwhile.
  final bool busy;

  /// The verification code (E4) or the pairing code (E5).
  final String? code;

  /// When [code] stops being valid: `created_at` plus `auth.device_pending_ttl` (E4, display
  /// only, D-W-116) or `expires_at` of the pairing (E5).
  final DateTime? deadline;

  /// Whether the pairing of E5 expired.
  final bool pairingExpired;

  /// Why the account ended (E7).
  final SignOutCause? cause;

  /// A copy with the given fields; [notice] and [code] are replaced as given (null clears).
  OnboardingState next({
    OnboardingStep? step,
    InstanceServices? services,
    bool clearServices = false,
    String? address,
    ServerStatus? serverStatus,
    String? minVersion,
    LoginNotice? notice,
    bool? busy,
    String? code,
    DateTime? deadline,
    bool? pairingExpired,
    SignOutCause? cause,
  }) => OnboardingState(
    step: step ?? this.step,
    instance: instance,
    services: clearServices ? null : (services ?? this.services),
    address: address ?? this.address,
    serverStatus: serverStatus ?? this.serverStatus,
    minVersion: minVersion ?? this.minVersion,
    notice: notice,
    busy: busy ?? false,
    code: code,
    deadline: deadline,
    pairingExpired: pairingExpired ?? false,
    cause: cause ?? this.cause,
  );

  /// The same state for [instance] (a new discovery result).
  OnboardingState withInstance(InstanceInfo? instance) => OnboardingState(
    step: step,
    instance: instance,
    services: services,
    address: address,
    serverStatus: serverStatus,
    minVersion: minVersion,
    notice: notice,
    busy: busy,
    code: code,
    deadline: deadline,
    pairingExpired: pairingExpired,
    cause: cause,
  );
}

/// Drives E1 to E7 from the outcomes of `LoginService` (Design note 3).
///
/// E1 [connect] runs discovery and composes the services of the instance; E2 and E3 run the
/// login flows; `pending` goes to E4 and polls; `signedIn` starts the engine and shows the main
/// view; every end of the account ([logout], [cancelPending], a rejected or expired device, a
/// revoked session, `4012`) runs the one [AccountWiper] and shows E7 with its cause.
@Riverpod(keepAlive: true)
class OnboardingController extends _$OnboardingController {
  late AppPorts _ports;
  late AppPreferences _prefs;
  StreamSubscription<AuthEvent>? _authEvents;
  StreamSubscription<SyncNotice>? _notices;
  int _flow = 0;
  bool _flowOwnsSignOut = false;
  bool _ending = false;
  InstanceServices? _open;

  @override
  OnboardingState build() {
    _ports = ref.watch(appPortsProvider);
    _prefs = AppPreferences(_ports.secureBackend);
    ref.onDispose(() {
      unawaited(_authEvents?.cancel());
      unawaited(_notices?.cancel());
      // The app quits (or a test ends): stop the engine and close the database, delete nothing.
      final open = _open;
      _open = null;
      if (open != null) unawaited(open.close());
    });
    scheduleMicrotask(_start);
    return const OnboardingState(step: OnboardingStep.server);
  }

  InstanceServices? get _services => state.services;

  Future<void> _start() async {
    final remembered = await _prefs.serverAddress();
    if (remembered == null) {
      state = state.next(address: _ports.branding.defaultServer);
      return;
    }
    await connect(remembered.toString(), resume: true);
  }

  /// E1 "Connect": discovery at [typed]; on success the address is remembered, the services
  /// are composed and E2 shows (or the main view, with [resume] and a stored session).
  Future<void> connect(String typed, {bool resume = false}) async {
    final address = normalizeServerAddress(typed);
    if (address == null) {
      state = state.next(
        step: OnboardingStep.server,
        address: typed,
        serverStatus: ServerStatus.invalidAddress,
      );
      return;
    }
    state = state.next(
      step: OnboardingStep.server,
      address: typed,
      serverStatus: ServerStatus.connecting,
      busy: true,
    );
    final outcome = await _ports.discovery.discover(address);
    switch (outcome) {
      case DiscoveryFound(:final instance):
        await _prefs.setServerAddress(address);
        await _closeServices();
        final InstanceServices services;
        try {
          services = await InstanceServices.open(_ports, instance);
        } on Object {
          state = state.withInstance(null).next(
            serverStatus: ServerStatus.noInstance,
            clearServices: true,
          );
          return;
        }
        _watch(services);
        state = state
            .withInstance(instance)
            .next(step: OnboardingStep.login, services: services);
        if (resume && await services.hasStoredSession()) _signedIn();
      case DiscoveryNoInstance():
        state = state.next(serverStatus: ServerStatus.noInstance);
      case DiscoveryCertificateInvalid():
        state = state.next(serverStatus: ServerStatus.certificateInvalid);
      case DiscoverySetupRequired():
        state = state.next(serverStatus: ServerStatus.notSetUp);
      case DiscoveryAppTooOld(:final minVersion):
        state = state.next(
          serverStatus: ServerStatus.appTooOld,
          minVersion: minVersion,
        );
    }
  }

  /// E2 "Change": back to E1 with the current address.
  Future<void> changeServer() async {
    _flow++;
    await _closeServices();
    state = state.withInstance(null).next(
      step: OnboardingStep.server,
      clearServices: true,
      address: state.instance?.address.toString() ?? state.address,
    );
  }

  /// E2: login with an OIDC provider in the system browser or behind a proxy.
  Future<void> loginWithProvider(LoginProvider provider) => _login(
    (login) => provider.isProxy
        ? login.loginWithTrustedHeader()
        : login.beginOidc(provider.id),
  );

  /// E2: password with one-time code (08 2.5).
  Future<void> loginWithPassword(
    String loginName,
    String password,
    String oneTimeCode,
  ) => _login(
    (login) => login.loginWithPassword(
      loginName: loginName,
      password: password,
      totpCode: oneTimeCode,
    ),
  );

  /// E2: passkey login (only where the build has a passkey port, D-M4-18-9).
  Future<void> loginWithPasskey(String loginName) => _login(
    (login) =>
        login.loginWithPasskey(loginName: loginName.isEmpty ? null : loginName),
  );

  /// E2 "I have an enrollment code": shows E3.
  void openEnroll() => state = state.next(step: OnboardingStep.enroll);

  /// E3 and E5 "Back": shows E2.
  void backToLogin() {
    _flow++;
    state = state.next(step: OnboardingStep.login);
  }

  /// E3 "Continue": redeems [code] with [method]; a redeemed code makes the device active at
  /// once (08 3.2).
  Future<void> redeem(String code, EnrollMethod method) => _login(
    (login) => switch (method) {
      EnrollThisDevice() => login.redeemEnrollmentCode(code),
      EnrollWithPasskey() => login.enrollWithPasskey(
        code,
        passkeyName: _ports.deviceName,
      ),
      EnrollWithProvider(:final provider) =>
        provider.isProxy
            ? login.loginWithTrustedHeader(enrollmentCode: code)
            : login.beginOidc(provider.id, enrollmentCode: code),
    },
    step: OnboardingStep.enroll,
  );

  /// E2 "Pair with another device" and E5 "New code": starts a pairing, shows its code and
  /// polls until it is approved (main view) or expired (E5 with the expiry text), 08 5.3.
  Future<void> openPairing() async {
    final services = _services;
    if (services == null) return;
    final flow = ++_flow;
    state = state.next(step: OnboardingStep.pair, busy: true);
    final PairingTicket ticket;
    try {
      ticket = await services.auth.login.beginPairing();
    } on DioException catch (e) {
      if (flow != _flow) return;
      state = state.next(
        step: OnboardingStep.pair,
        notice: e.response == null
            ? LoginNotice.unreachable
            : LoginNotice.failed,
      );
      return;
    }
    if (flow != _flow) return;
    state = state.next(
      step: OnboardingStep.pair,
      code: ticket.code,
      deadline: ticket.expiresAt,
    );
    final outcome = await services.auth.login.pollPairing(
      ticket.pairingId,
      isCancelled: () => flow != _flow,
    );
    if (flow != _flow) return;
    if (outcome is LoginSignedIn) {
      _signedIn();
    } else {
      state = state.next(step: OnboardingStep.pair, pairingExpired: true);
    }
  }

  /// E4 "Cancel": logs out and deletes the keys of the device (01 5.6, 08 5.2).
  Future<void> cancelPending() => logout();

  /// Logs out (E6 "Log out", the shell): the server session ends, every local datum of the
  /// account is deleted and E7 shows.
  Future<void> logout() async {
    _flow++;
    final services = _services;
    if (services == null) return;
    await _endAccount(SignOutCause.loggedOut, before: services.auth.login.logout);
  }

  /// E7 "Log in again": composes the services again (new device keys) and shows E2.
  Future<void> loginAgain() async {
    final instance = state.instance;
    if (instance == null) {
      state = state.next(step: OnboardingStep.server);
      return;
    }
    state = state.next(step: OnboardingStep.signedOut, busy: true);
    try {
      final services = await InstanceServices.open(_ports, instance);
      _watch(services);
      state = state.next(step: OnboardingStep.login, services: services);
    } on Object {
      state = state.withInstance(null).next(
        step: OnboardingStep.server,
        serverStatus: ServerStatus.noInstance,
      );
    }
  }

  Future<void> _login(
    Future<LoginOutcome> Function(LoginService login) run, {
    OnboardingStep step = OnboardingStep.login,
  }) async {
    final services = _services;
    if (services == null) return;
    final flow = ++_flow;
    state = state.next(step: step, busy: true);
    LoginOutcome outcome;
    try {
      outcome = await run(services.auth.login);
    } on Object {
      // A refused passkey or device key, a missing port: the same text as a rejected login.
      outcome = const LoginFailed(reason: 'login_failed');
    }
    if (flow != _flow) return;
    switch (outcome) {
      case LoginSignedIn():
        _signedIn();
      case LoginPending(:final device):
        await _pending(services, device, flow);
      case LoginNeedsSecondFactor():
        state = state.next(step: step, notice: LoginNotice.failed);
      case LoginFailed(:final reason):
        state = state.next(step: step, notice: _noticeOf(reason));
    }
  }

  static LoginNotice _noticeOf(String reason) => switch (reason) {
    'network' => LoginNotice.unreachable,
    'access_denied' || 'cancelled' => LoginNotice.browserCancelled,
    'flow_expired' => LoginNotice.browserExpired,
    _ => LoginNotice.failed,
  };

  Future<void> _pending(
    InstanceServices services,
    PendingDevice device,
    int flow,
  ) async {
    var createdAt = _ports.clock.now();
    try {
      final info = (await services.auth.api.getDevicesApi().getDevice(
        deviceId: device.deviceId.value,
      )).data;
      if (info != null) createdAt = info.createdAt;
    } on DioException {
      // The remaining time is display only (D-W-116); the poll reports the expiry.
    }
    if (flow != _flow) return;
    state = state.next(
      step: OnboardingStep.pending,
      code: device.verificationCode,
      deadline: createdAt.add(_ports.config.devicePendingTtl),
    );
    // The login call returns once E4 shows; the poll runs on (08 5.2).
    unawaited(_pollPending(services, device, flow));
  }

  Future<void> _pollPending(
    InstanceServices services,
    PendingDevice device,
    int flow,
  ) async {
    _flowOwnsSignOut = true;
    final LoginOutcome outcome;
    try {
      outcome = await services.auth.login.pollPendingDevice(
        device.deviceId,
        isCancelled: () => flow != _flow,
      );
    } finally {
      _flowOwnsSignOut = false;
    }
    if (flow != _flow) return;
    switch (outcome) {
      case LoginSignedIn():
        _signedIn();
      case LoginFailed(:final reason):
        await _endAccount(_pendingEndCause(reason));
      case LoginPending() || LoginNeedsSecondFactor():
        break;
    }
  }

  /// The E7 cause when the poll of a waiting device ends without a login (D-M4-18-10): the
  /// revoke reason when `getDevice` returned it; when the server ended the session with the
  /// device first (`401 session_revoked`), the app only knows its own deadline, so before it
  /// the confirmation was rejected and after it the device expired.
  SignOutCause _pendingEndCause(String reason) {
    final known = SignOutCause.ofRevokeReason(reason);
    if (known != SignOutCause.sessionRevoked) return known;
    final deadline = state.deadline;
    return deadline != null && !_ports.clock.now().isBefore(deadline)
        ? SignOutCause.pendingExpired
        : SignOutCause.rejected;
  }

  void _signedIn() {
    final services = _services;
    if (services == null) return;
    state = state.next(step: OnboardingStep.signedIn);
    services.engine.start();
  }

  void _watch(InstanceServices services) {
    _open = services;
    unawaited(_authEvents?.cancel());
    unawaited(_notices?.cancel());
    _authEvents = services.auth.events.listen((event) {
      if (event is! SignedOut || _flowOwnsSignOut) return;
      unawaited(
        _endAccount(switch (event.reason) {
          SignOutReason.loggedOut => SignOutCause.loggedOut,
          SignOutReason.refreshRejected => SignOutCause.sessionExpired,
          SignOutReason.sessionRevoked ||
          SignOutReason.deviceRevoked => SignOutCause.sessionRevoked,
        }),
      );
    });
    _notices = services.engine.notices.listen((notice) {
      if (notice is SignedOutNotice) {
        unawaited(_endAccount(SignOutCause.sessionRevoked));
      }
    });
  }

  /// The one end of an account (Design note 2): [before] (the server logout) runs first, then
  /// the wiper; E7 shows [cause] even when a step of the wipe failed.
  Future<void> _endAccount(
    SignOutCause cause, {
    Future<void> Function()? before,
  }) async {
    final services = _services;
    if (_ending || services == null) return;
    _ending = true;
    _open = null;
    _flow++;
    try {
      await _authEvents?.cancel();
      await _notices?.cancel();
      _authEvents = null;
      _notices = null;
      if (before != null) {
        try {
          await before();
        } on Object {
          // The logout call already tolerates being offline; nothing may stop the wipe.
        }
      }
      try {
        await services.wiper.wipe();
      } on Object {
        // E7 still says the data was deleted only as far as the wiper got; the wiper runs
        // every step regardless of failures, so what is left is reported by its error.
      }
      await services.auth.dispose();
      state = state.next(
        step: OnboardingStep.signedOut,
        clearServices: true,
        cause: cause,
      );
    } finally {
      _ending = false;
    }
  }

  Future<void> _closeServices() async {
    final services = _services;
    if (services == null) return;
    _open = null;
    await _authEvents?.cancel();
    await _notices?.cancel();
    _authEvents = null;
    _notices = null;
    await services.close();
  }
}
