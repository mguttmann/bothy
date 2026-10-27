// SPDX-License-Identifier: Apache-2.0

/// The flow of the onboarding screens E1 to E7 over the login flows of `auth` (docs/spec/
/// 01-product.md 5.6; 08 5 and 14; brief M4-18 Design note 3).
library;

import 'dart:async';
import 'dart:convert';

import 'package:api_client/api_client.dart' show Device;
import 'package:auth/auth.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart'
    show
        AppConfigValues,
        ClientVersionAllowed,
        ClientVersionBelowMinimum,
        ClientVersionDecision,
        DeviceId;
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:sync_engine/sync_engine.dart';

import '../app/account_wiper.dart';
import '../app/bootstrap.dart';
import '../app/discovery.dart';
import '../app/providers/ports.dart';
import '../app/router.dart';
import '../platform/secure_store.dart';
import 'start_route.dart';

export '../app/router.dart' show OnboardingStep;
export 'start_route.dart' show ServerStatus;

part 'onboarding_controller.g.dart';

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
    this.restored = false,
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

  /// Whether the main view shows a session this start resumed from storage (a cold start)
  /// rather than one a login just created; the app lock locks at once only then (brief M5-08
  /// Design note 2).
  final bool restored;

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
    bool restored = false,
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
    restored: restored,
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
    restored: restored,
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
  StreamSubscription<ClientVersionDecision>? _versions;
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
      unawaited(_versions?.cancel());
      // The app quits (or a test ends): the running polls stop, the engine stops and the
      // database closes; nothing is deleted.
      _flow++;
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
    await connect(remembered.toString());
  }

  /// E1 "Connect": discovery at [typed]; on success the address and the instance are
  /// remembered. [routeStart] decides the screen from what is stored for the instance and
  /// what discovery answered: E1 with its text, E2, or the stored session from its local data
  /// (the main view or E4), also when discovery got no usable answer (D-M4-18-13,
  /// D-M4-18-15).
  Future<void> connect(String typed) async {
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
    final InstanceInfo? instance;
    if (outcome is DiscoveryFound) {
      instance = outcome.instance;
      await _prefs.setServerAddress(address);
      await _prefs.setInstanceJson(jsonEncode(instance.toJson()));
    } else {
      final known = InstanceInfo.fromJson(_decode(await _prefs.instanceJson()));
      instance = known?.address == address ? known : null;
    }
    final stored = instance == null
        ? StoredAccount.none
        : await _storedAccount(instance);
    final route = routeStart(stored: stored, discovery: outcome);
    if (instance == null || route.step == OnboardingStep.server) {
      state = state.next(
        serverStatus: route.serverStatus,
        minVersion: route.minVersion,
      );
      return;
    }
    if (outcome is DiscoveryFound) _version = outcome.version;
    await _openInstance(instance, stored, outcome);
  }

  /// The decision of the version gate of the last discovery that found the instance; the
  /// services start from it (brief M7-13).
  ClientVersionDecision _version = const ClientVersionAllowed();

  /// Composes the services of [instance] and shows E2, or resumes the [stored] session.
  Future<void> _openInstance(
    InstanceInfo instance,
    StoredAccount stored,
    DiscoveryOutcome outcome,
  ) async {
    await _closeServices();
    final InstanceServices services;
    try {
      services = await InstanceServices.open(
        _ports,
        instance,
        version: _version,
      );
    } on Object {
      state = state
          .withInstance(null)
          .next(serverStatus: ServerStatus.noInstance, clearServices: true);
      return;
    }
    _watch(services);
    state = state
        .withInstance(instance)
        .next(
          step: OnboardingStep.login,
          services: services,
          serverStatus: ServerStatus.idle,
        );
    if (stored != StoredAccount.none) await _resume(services, stored, outcome);
  }

  /// What is stored for [instance], without opening anything: a refresh token, and whether the
  /// device waited for its confirmation ([_pendingKey]).
  Future<StoredAccount> _storedAccount(InstanceInfo instance) async {
    final secrets = InstanceSecrets(_ports.secureBackend, instance.instanceId);
    if (await secrets.read('auth/${instance.address.origin}/refresh_token') ==
        null) {
      return StoredAccount.none;
    }
    return await secrets.read(_pendingKey) == null
        ? StoredAccount.signedIn
        : StoredAccount.devicePending;
  }

  /// The instance secret that marks a waiting device: its verification code and `created_at`
  /// as JSON, written when E4 shows and deleted at the login (D-M4-18-15).
  static const _pendingKey = 'onboarding/pending_device';

  static Object? _decode(String? json) {
    if (json == null) return null;
    try {
      return jsonDecode(json);
    } on FormatException {
      return null;
    }
  }

  /// E2 "Change": back to E1 with the current address.
  Future<void> changeServer() async {
    _flow++;
    await _closeServices();
    state = state
        .withInstance(null)
        .next(
          step: OnboardingStep.server,
          clearServices: true,
          serverStatus: ServerStatus.idle,
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
    } on Object {
      // A device key the system refused: the same text as a rejected request.
      if (flow == _flow) {
        state = state.next(
          step: OnboardingStep.pair,
          notice: LoginNotice.failed,
        );
      }
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

  /// Ends the account on this device after the server already ended it from the settings
  /// (this device revoked, the current session ended; 08 10, R-SY-14): no server logout,
  /// the wiper deletes every local datum and E7 shows [cause].
  Future<void> endLocally(SignOutCause cause) async {
    _flow++;
    await _endAccount(cause);
  }

  /// Logs out (E6 "Log out", the shell): the server session ends, every local datum of the
  /// account is deleted and E7 shows.
  Future<void> logout() async {
    _flow++;
    final services = _services;
    if (services == null) return;
    await _endAccount(
      SignOutCause.loggedOut,
      before: () async {
        // M5-04, M5-05: `unregisterPush` needs the session, so it runs before the server
        // logout; it is best effort, the logout disables the session's registrations anyway
        // (13 6.4).
        await _ports.push?.logout();
        await services.auth.login.logout();
      },
    );
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
      final services = await InstanceServices.open(
        _ports,
        instance,
        version: _version,
      );
      _watch(services);
      state = state.next(step: OnboardingStep.login, services: services);
    } on Object {
      state = state
          .withInstance(null)
          .next(
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
    int flow, {
    DateTime? createdAt,
  }) async {
    var since = createdAt;
    if (since == null) {
      try {
        since = (await services.auth.api.getDevicesApi().getDevice(
          deviceId: device.deviceId.value,
        )).data?.createdAt;
      } on DioException {
        // The remaining time is display only (D-W-116); the poll reports the expiry.
      }
    }
    since ??= _ports.clock.now();
    if (flow != _flow) return;
    await services.secrets.write(
      _pendingKey,
      jsonEncode({
        'code': device.verificationCode,
        'created_at': since.toUtc().toIso8601String(),
      }),
    );
    if (flow != _flow) return;
    state = state.next(
      step: OnboardingStep.pending,
      code: device.verificationCode,
      deadline: since.add(_ports.config.devicePendingTtl),
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
    } on Object {
      // The session was closed under the poll (the app quits); nothing to show.
      return;
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

  /// A start with a [stored] session: [routeStart] with what `getDevice` says about this device
  /// decides between E4 (the waiting device with its code; 08 5.2) and the main view. Without
  /// an answer the stored state decides: a device that waited shows E4 with its stored code and
  /// polls until the network returns, any other starts signed in and the engine reconnects
  /// later (D-M4-18-15).
  Future<void> _resume(
    InstanceServices services,
    StoredAccount stored,
    DiscoveryOutcome outcome,
  ) async {
    final flow = ++_flow;
    final id = await services.storedDeviceId();
    Device? device;
    if (id != null) {
      try {
        device = (await services.auth.api.getDevicesApi().getDevice(
          deviceId: id,
        )).data;
      } on DioException {
        // No usable answer: the stored state decides; a revoked session ends through the
        // session's events.
      }
    }
    if (flow != _flow) return;
    final route = routeStart(
      stored: stored,
      discovery: outcome,
      device: device?.state,
    );
    if (route.step != OnboardingStep.pending || id == null) {
      _signedIn(restored: true);
      return;
    }
    final saved = _decode(await services.secrets.read(_pendingKey));
    final savedMap = saved is Map ? saved : const <Object?, Object?>{};
    final code = device?.verificationCode ?? savedMap['code'];
    if (flow != _flow) return;
    await _pending(
      services,
      PendingDevice(
        deviceId: DeviceId.parse(id),
        verificationCode: code is String ? code : null,
      ),
      flow,
      createdAt:
          device?.createdAt ?? DateTime.tryParse('${savedMap['created_at']}'),
    );
  }

  void _signedIn({bool restored = false}) {
    final services = _services;
    if (services == null) return;
    state = state.next(step: OnboardingStep.signedIn, restored: restored);
    unawaited(services.secrets.delete(_pendingKey));
    services.engine.start();
  }

  void _watch(InstanceServices services) {
    _open = services;
    unawaited(_authEvents?.cancel());
    unawaited(_notices?.cancel());
    unawaited(_versions?.cancel());
    _versions = services.versions.changes.listen((decision) {
      _version = decision;
      if (decision is ClientVersionBelowMinimum) {
        unawaited(_belowMinimum(services, decision.minVersion));
      }
    });
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

  /// The instance raised `client.min_version.<platform>` above this build while it ran (the
  /// gate before a reconnect refused, 18 9 item 2): the engine stopped before a socket opened;
  /// the services close without deleting anything (the outbox and the local data stay, brief
  /// M7-13 requirement 13), so no authenticated request leaves the app, and E1 shows "App too
  /// old" with the update path.
  Future<void> _belowMinimum(
    InstanceServices services,
    String minVersion,
  ) async {
    if (_services != services || _ending) return;
    _flow++;
    final address = state.instance?.address.toString() ?? state.address;
    await _closeServices();
    if (!ref.mounted) return;
    state = state
        .withInstance(null)
        .next(
          step: OnboardingStep.server,
          clearServices: true,
          address: address,
          serverStatus: ServerStatus.appTooOld,
          minVersion: minVersion,
        );
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
      await _versions?.cancel();
      _authEvents = null;
      _notices = null;
      _versions = null;
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
      // The app quit or a test ended while the wipe ran: the data is gone, no state is left.
      if (!ref.mounted) return;
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
    await _versions?.cancel();
    _authEvents = null;
    _notices = null;
    _versions = null;
    await services.close();
  }
}
