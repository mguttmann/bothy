// SPDX-License-Identifier: Apache-2.0

/// The flow of the onboarding screens E1 to E7 over the login flows of `auth` (docs/spec/
/// 01-product.md 5.6; 08 5 and 14; brief M4-18 Design note 3).
library;

import 'dart:async';
import 'dart:convert';
import 'dart:developer' as developer;

import 'package:api_client/api_client.dart' show Device, DeviceStateEnum;
import 'package:auth/auth.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart'
    show
        AppConfigValues,
        ClientVersionAllowed,
        ClientVersionBelowMinimum,
        ClientVersionDecision,
        DeviceId;
import 'package:local_store/local_store.dart' show LocalStoreKeyError;
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
    this.starting = false,
    this.wipeIncomplete = false,
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

  /// Whether the app has not decided its first screen yet: the start screen shows instead of
  /// E1 while the stored address, the session and the local data are read.
  final bool starting;

  /// Whether the wipe behind E7 left something on the device; E7 says so instead of "deleted"
  /// and the app deletes the rest at its next start (R-SY-14).
  final bool wipeIncomplete;

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
    bool starting = false,
    bool? wipeIncomplete,
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
    starting: starting,
    wipeIncomplete: wipeIncomplete ?? this.wipeIncomplete,
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
    starting: starting,
    wipeIncomplete: wipeIncomplete,
  );
}

/// Drives E1 to E7 from the outcomes of `LoginService` (Design note 3).
///
/// E1 [connect] runs discovery and composes the services of the instance; E2 and E3 run the
/// login flows; `pending` goes to E4 and polls; `signedIn` starts the engine and shows the main
/// view; every end of the account ([logout], [cancelPending], a rejected or expired device, a
/// revoked session, `4012`) runs the one [AccountWiper] and shows E7 with its cause. A start
/// with the session of an active device opens its local data before the network is asked.
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

  /// The instance of a stored session whose database the stored key did not open; E1 offers
  /// [resetLocalData] for it.
  InstanceInfo? _unreadable;

  /// What an incomplete wipe left to delete, once [_unfinishedRead] from the preferences.
  UnfinishedWipe? _unfinished;
  bool _unfinishedRead = false;

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
    return const OnboardingState(step: OnboardingStep.server, starting: true);
  }

  InstanceServices? get _services => state.services;

  /// The start of the app: finishes an incomplete wipe, then shows E1 without a remembered
  /// address, the local data of a stored session ([_startFromLocalData]) or what "Connect" at
  /// the remembered address leads to.
  Future<void> _start() async {
    try {
      await _finishWipe();
      if (!ref.mounted) return;
      final remembered = await _prefs.serverAddress();
      if (remembered == null) {
        state = state.next(address: _ports.branding.defaultServer);
        return;
      }
      if (await _startFromLocalData(remembered)) return;
      await connect(remembered.toString());
    } finally {
      // Whatever the start ran into, E1 takes over from the start screen.
      if (ref.mounted && state.starting) state = state.next();
    }
  }

  /// A cold start with the stored session of an active device: the local data of the
  /// remembered instance at [address] open and the main view shows before the network is
  /// asked, as the start without network does (D-M4-18-13); [_catchUpStart] applies what the
  /// network answers. Returns false when nothing is stored that the app could open, for a
  /// waiting device, whose E4 is decided by the server, and for a build an instance refused as
  /// too old, which asks before it opens anything (18 9 item 2); those go through [connect].
  Future<bool> _startFromLocalData(Uri address) async {
    final known = InstanceInfo.fromJson(_decode(await _prefs.instanceJson()));
    if (known == null || known.address != address) return false;
    if (await _storedAccount(known) != StoredAccount.signedIn) return false;
    if (await _prefs.refusedMinVersion() != null) return false;
    if (!ref.mounted) return true;
    // E1 shows the address when the local data do not open.
    state = state.next(address: address.toString(), starting: true);
    final services = await _openServices(known);
    if (services == null) return true;
    state = state
        .withInstance(known)
        .next(services: services, serverStatus: ServerStatus.idle);
    _signedIn(restored: true);
    unawaited(_catchUpStart(services, known));
    return true;
  }

  /// What the network says after a start from the local data, applied when it answers
  /// ([routeStart] as for every "Connect", D-M4-18-15): a certificate error, `setup_required`
  /// and a too old app close the services and show E1 with their text; another instance at
  /// the address and a device the server still holds for confirmation go the way of
  /// "Connect" (E2 or E4); a found instance refreshes the remembered description and the
  /// branding. Without a usable answer the main view stays as it is.
  Future<void> _catchUpStart(
    InstanceServices services,
    InstanceInfo known,
  ) async {
    bool shown() =>
        ref.mounted &&
        !_ending &&
        _services == services &&
        state.step == OnboardingStep.signedIn;
    final outcome = await _ports.discovery.discover(known.address);
    if (!shown()) return;
    final route = routeStart(
      stored: StoredAccount.signedIn,
      discovery: outcome,
    );
    if (route.step == OnboardingStep.server) {
      _flow++;
      await _closeServices();
      if (outcome is DiscoveryAppTooOld) {
        await _rememberRefusal(outcome.minVersion);
      }
      if (!ref.mounted) return;
      state = state
          .withInstance(null)
          .next(
            step: OnboardingStep.server,
            clearServices: true,
            address: known.address.toString(),
            serverStatus: route.serverStatus,
            minVersion: route.minVersion,
          );
      return;
    }
    var instance = known;
    if (outcome is DiscoveryFound) {
      instance = outcome.instance;
      _version = outcome.version;
      await _prefs.setInstanceJson(jsonEncode(instance.toJson()));
      if (!shown()) return;
      if (instance.instanceId != known.instanceId) {
        // Another instance answers at the address: what "Connect" finds there.
        await _openInstance(instance, await _storedAccount(instance), outcome);
        return;
      }
      state = state.withInstance(instance);
    }
    final device = await _deviceOf(services);
    if (!shown()) return;
    if (device?.state == DeviceStateEnum.pending) {
      // The server's answer about the device wins over what was stored: E4.
      await _openInstance(instance, StoredAccount.signedIn, outcome);
    }
  }

  /// Remembers that an instance refused this build as too old ([minVersion]), so the next
  /// start asks before it opens a stored session ([_startFromLocalData]); null forgets it,
  /// once discovery finds an instance again. A failure of the store goes to the log: it
  /// costs or keeps only that shortcut and never holds E1 back.
  Future<void> _rememberRefusal(String? minVersion) async {
    try {
      await _prefs.setRefusedMinVersion(minVersion);
    } on Object catch (error, stack) {
      developer.log(
        'the refused version could not be stored: $error',
        name: 'onboarding',
        error: error,
        stackTrace: stack,
      );
    }
  }

  /// What `getDevice` says about this device, or null without a stored device ID or a usable
  /// answer.
  Future<Device?> _deviceOf(InstanceServices services) async {
    final id = await services.storedDeviceId();
    if (id == null) return null;
    try {
      return (await services.auth.api.getDevicesApi().getDevice(deviceId: id))
          .data;
    } on DioException {
      return null;
    }
  }

  /// Deletes what an incomplete wipe left on the device (R-SY-14): at the start and before
  /// the services of an instance are composed again. A failure goes to the log and the entry
  /// stays for the next start; it never holds the app back.
  Future<void> _finishWipe() async {
    try {
      if (!_unfinishedRead) {
        _unfinished = UnfinishedWipe.fromJson(
          _decode(await _prefs.unfinishedWipe()),
        );
        _unfinishedRead = true;
      }
      final left = _unfinished;
      if (left == null) return;
      final wiper = await InstanceServices.leftoverWiper(
        _ports,
        instanceId: left.instanceId,
        keyScope: left.keyScope,
      );
      await wiper.wipe();
      _unfinished = null;
      await _prefs.setUnfinishedWipe(null);
    } on Object catch (error, stack) {
      _logWipeFailure(error, stack);
    }
  }

  /// A session of the instance of [services] exists again: what an incomplete wipe left
  /// there is in use, so no later start deletes it.
  void _sessionBegan(InstanceServices services) {
    if (_unfinished?.instanceId != services.instance.instanceId) return;
    _unfinished = null;
    unawaited(_prefs.setUnfinishedWipe(null).catchError(_logWipeFailure));
  }

  /// Logs why a wipe is incomplete; E7 shows only that it is.
  static void _logWipeFailure(Object error, StackTrace stack) => developer.log(
    'the wipe of the local data is incomplete: $error',
    name: 'onboarding',
    error: error,
    stackTrace: stack,
  );

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
    if (outcome is DiscoveryAppTooOld) {
      await _rememberRefusal(outcome.minVersion);
    } else if (outcome is DiscoveryFound) {
      await _rememberRefusal(null);
    }
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
    final services = await _openServices(instance);
    if (services == null) return;
    state = state
        .withInstance(instance)
        .next(
          step: OnboardingStep.login,
          services: services,
          serverStatus: ServerStatus.idle,
        );
    if (stored != StoredAccount.none) await _resume(services, stored, outcome);
  }

  /// Closes the open services, finishes an incomplete wipe and composes the services of
  /// [instance]. Returns null after a failure, which E1 shows with its text
  /// ([openFailureStatus]); a database of a stored session that the stored key does not open
  /// can be reset from there ([resetLocalData]).
  Future<InstanceServices?> _openServices(InstanceInfo instance) async {
    await _closeServices();
    await _finishWipe();
    try {
      final services = await InstanceServices.open(
        _ports,
        instance,
        version: _version,
      );
      if (!ref.mounted) {
        // The app quit while the services were composed: nothing shows them any more.
        unawaited(services.close());
        return null;
      }
      _unreadable = null;
      _watch(services);
      return services;
    } on Object catch (error, stack) {
      _logOpenFailure(error, stack);
      if (!ref.mounted) return null;
      _unreadable = error is LocalStoreKeyError ? instance : null;
      state = state
          .withInstance(null)
          .next(
            step: OnboardingStep.server,
            serverStatus: openFailureStatus(error),
            clearServices: true,
          );
      return null;
    }
  }

  /// E1 "Reset local data" ([ServerStatus.localDataUnreadable]): deletes the database files
  /// and the file cache of the instance whose database did not open, keeps its session, device
  /// keys and secrets, and connects again, so the engine loads everything anew. What only that
  /// database held (messages and drafts that were not sent yet, the view states of the device,
  /// the pinned host keys of the terminal) goes with it.
  Future<void> resetLocalData() async {
    final instance = _unreadable;
    if (instance == null) return;
    _unreadable = null;
    final address = state.address;
    state = state.next(serverStatus: ServerStatus.connecting, busy: true);
    try {
      await InstanceServices.discardLocalData(_ports, instance);
    } on Object catch (error, stack) {
      _logOpenFailure(error, stack);
      _unreadable = instance;
      state = state.next(serverStatus: ServerStatus.localDataUnreadable);
      return;
    }
    await connect(address);
  }

  /// What is stored for [instance], without opening anything: a refresh token, and whether the
  /// device waited for its confirmation ([pendingKey]).
  Future<StoredAccount> _storedAccount(InstanceInfo instance) async {
    final secrets = InstanceSecrets(_ports.secureBackend, instance.instanceId);
    if (await secrets.read('auth/${instance.address.origin}/refresh_token') ==
        null) {
      return StoredAccount.none;
    }
    return await secrets.read(pendingKey) == null
        ? StoredAccount.signedIn
        : StoredAccount.devicePending;
  }

  /// The instance secret that marks a waiting device: its verification code and `created_at`
  /// as JSON, written when E4 shows and deleted at the login (D-M4-18-15).
  static const pendingKey = 'onboarding/pending_device';

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
  /// account is deleted and E7 shows. The instance has [AppPorts.serverLogoutLimit] to answer;
  /// after it the local data go anyway.
  Future<void> logout() async {
    _flow++;
    final services = _services;
    if (services == null) return;
    await _endAccount(
      SignOutCause.loggedOut,
      before: () => _serverLogout(services),
    );
  }

  /// The server part of [logout], bounded by [AppPorts.serverLogoutLimit]: an instance that
  /// does not answer must not keep the local data on the device. At the limit the connections
  /// of the session's REST client are cut, so both calls end at once and nothing of them runs
  /// after the wipe (the logout clears the stored session when its call ends).
  Future<void> _serverLogout(InstanceServices services) async {
    Future<void> calls() async {
      // M5-04, M5-05: `unregisterPush` needs the session, so it runs before the server
      // logout; it is best effort, the logout disables the session's registrations anyway
      // (13 6.4).
      await _ports.push?.logout();
      await services.auth.login.logout();
    }

    final running = calls();
    try {
      await running.timeout(_ports.serverLogoutLimit);
    } on TimeoutException {
      services.auth.api.dio.close(force: true);
      await running;
    }
  }

  /// E7 "Log in again": composes the services again (new device keys) and shows E2.
  Future<void> loginAgain() async {
    final instance = state.instance;
    if (instance == null) {
      state = state.next(step: OnboardingStep.server);
      return;
    }
    state = state.next(step: OnboardingStep.signedOut, busy: true);
    final services = await _openServices(instance);
    if (services == null) return;
    state = state.next(step: OnboardingStep.login, services: services);
  }

  /// Logs why `InstanceServices.open` failed; E1 shows only the kind ([openFailureStatus]).
  static void _logOpenFailure(Object error, StackTrace stack) => developer.log(
    'opening the services of the instance failed: $error',
    name: 'onboarding',
    error: error,
    stackTrace: stack,
  );

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
    _sessionBegan(services);
    await services.secrets.write(
      pendingKey,
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
    final saved = _decode(await services.secrets.read(pendingKey));
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
    _sessionBegan(services);
    unawaited(services.secrets.delete(pendingKey));
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
    await _rememberRefusal(minVersion);
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
  /// the wiper; E7 shows [cause] even when a step of the wipe failed. What a failed step left
  /// is logged and remembered as an [UnfinishedWipe], E7 says that not everything was deleted,
  /// and [_finishWipe] deletes the rest at the next start (R-SY-14).
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
      var complete = true;
      try {
        await services.wiper.wipe();
      } on Object catch (error, stack) {
        // The wiper ran every step regardless of failures; its error names the failed ones.
        complete = false;
        _logWipeFailure(error, stack);
        final left = UnfinishedWipe(
          instanceId: services.wiper.instanceId,
          keyScope: services.wiper.keyScope,
        );
        _unfinished = left;
        _unfinishedRead = true;
        try {
          await _prefs.setUnfinishedWipe(jsonEncode(left.toJson()));
        } on Object catch (error, stack) {
          _logWipeFailure(error, stack);
        }
      }
      await services.auth.dispose();
      // The app quit or a test ended while the wipe ran: the data is gone, no state is left.
      if (!ref.mounted) return;
      state = state.next(
        step: OnboardingStep.signedOut,
        clearServices: true,
        cause: cause,
        wipeIncomplete: !complete,
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
