// SPDX-License-Identifier: Apache-2.0

/// The own devices and sessions of the signed-in person over REST (docs/spec/08-identity.md
/// 5.2 to 5.4, 7, 10; brief M4-25 Design note 3).
///
/// No local database: the screens of `admin_ui` serve the desktop app, the console and the
/// emergency actions on mobile, and the console has no local store (15 3.1).
library;

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:dio/dio.dart';

/// The state of an own device (`Device.state`).
enum SelfDeviceState {
  /// Waits for the confirmation with its code (08 5.2).
  pending,

  /// Signed in and active.
  active,

  /// Revoked; it deletes its data at its next contact (08 10).
  revoked,
}

/// One own device as the screens show it.
final class SelfDevice {
  /// Creates a device.
  const SelfDevice({
    required this.id,
    required this.name,
    required this.platform,
    required this.state,
    required this.createdAt,
    this.appVersion,
    this.lastSeenAt,
    this.isThisDevice = false,
  });

  /// The device ID.
  final String id;

  /// The name the device gave itself.
  final String name;

  /// The `DevicePlatform` code, for example `macos`.
  final String platform;

  /// The state.
  final SelfDeviceState state;

  /// When the device registered.
  final DateTime createdAt;

  /// The app version, if known.
  final String? appVersion;

  /// The last activity, if known.
  final DateTime? lastSeenAt;

  /// Whether this is the device the screen runs on.
  final bool isThisDevice;
}

/// How a session signed in (`Session.auth_method`).
enum SelfAuthMethod {
  /// A login service (OIDC).
  oidc,

  /// The trusted proxy header.
  trustedHeader,

  /// A passkey.
  passkey,

  /// Password and one-time code.
  passwordTotp,

  /// An enrollment code.
  enrollmentCode,

  /// A pairing with another device.
  devicePairing,

  /// A method this build does not know.
  other,
}

/// One own session as the screens show it.
final class SelfSession {
  /// Creates a session.
  const SelfSession({
    required this.id,
    required this.deviceId,
    required this.authMethod,
    required this.lastUsedAt,
    this.deviceName,
    this.current = false,
  });

  /// The session ID.
  final String id;

  /// The device of the session.
  final String deviceId;

  /// The name of that device, or null when the device list does not name it.
  final String? deviceName;

  /// How the session signed in.
  final SelfAuthMethod authMethod;

  /// The last use (`last_seen_at`, else the start).
  final DateTime lastUsedAt;

  /// Whether this is the session of the screen's own requests.
  final bool current;
}

/// A refused or failed call: [problem] is the answer of the server, null without an answer
/// (no network).
final class SelfActionFailed implements Exception {
  /// Creates the failure.
  const SelfActionFailed(this.problem);

  /// The problem of the server, or null.
  final ApiProblem? problem;

  @override
  String toString() => 'SelfActionFailed(${problem ?? 'no answer'})';
}

/// The own devices and sessions and the actions on them.
///
/// Every changing call carries the purpose text the step-up dialog shows when the server asks
/// for a step-up (`401 step_up_required`); the screen never asks on its own (05:
/// `device.approve` and `device.revoke` have step-up, `session.revoke` has none).
///
/// Errors: [SelfActionFailed] for an answer of the server or no answer, [StepUpCancelled] when
/// the person cancels the step-up.
abstract interface class SelfRepository {
  /// The own devices, this device marked.
  Future<List<SelfDevice>> devices();

  /// The own live sessions, the current one marked.
  Future<List<SelfSession>> sessions();

  /// Confirms the waiting device [deviceId] with the [code] it shows (`approveDevice`).
  ///
  /// Returns the device: `active`, or `revoked` when the code did not match (08 5.2).
  Future<SelfDevice> approveDevice(
    String deviceId,
    String code, {
    required String purpose,
  });

  /// Approves the pairing with [code] of a new device (`approvePairing`, 08 5.3); a code
  /// without an open pairing fails with `404`.
  Future<SelfDevice> approvePairing(String code, {required String purpose});

  /// Revokes the device [deviceId] with [reason] (`revokeDevice`, 08 10).
  Future<SelfDevice> revokeDevice(
    String deviceId, {
    required String reason,
    required String purpose,
  });

  /// Ends the session [sessionId] (`revokeSession`).
  Future<void> revokeSession(String sessionId);
}

/// [SelfRepository] over the generated client of `api_client`.
///
/// The `api` argument is the client of the signed-in session (`AuthSession.api`), so DPoP,
/// refresh and the step-up interceptor apply; [newIdempotencyKey] returns a fresh UUIDv7 per
/// action.
///
/// Example:
/// ```dart
/// final repository = ApiSelfRepository(session.api, newIdempotencyKey: ids.next);
/// final devices = await repository.devices();
/// ```
final class ApiSelfRepository implements SelfRepository {
  /// Creates the repository.
  ApiSelfRepository(this._api, {required String Function() newIdempotencyKey})
    : _newKey = newIdempotencyKey;

  final ApiClient _api;
  final String Function() _newKey;

  DevicesApi get _devices => _api.getDevicesApi();

  @override
  Future<List<SelfDevice>> devices() => _call(() async {
    final me = (await _api.getMeApi().getMe()).data!;
    final items = <Device>[];
    String? cursor;
    do {
      final page = (await _devices.listMyDevices(cursor: cursor)).data!;
      items.addAll(page.items);
      cursor = page.nextCursor;
    } while (cursor != null);
    return [for (final d in items) _device(d, thisDevice: me.deviceId)];
  });

  @override
  Future<List<SelfSession>> sessions() => _call(() async {
    final names = {for (final d in await devices()) d.id: d.name};
    final items = <Session>[];
    String? cursor;
    do {
      final page = (await _devices.listMySessions(cursor: cursor)).data!;
      items.addAll(page.items);
      cursor = page.nextCursor;
    } while (cursor != null);
    return [
      for (final s in items)
        SelfSession(
          id: s.id,
          deviceId: s.deviceId,
          deviceName: names[s.deviceId],
          authMethod: switch (s.authMethod) {
            SessionAuthMethodEnum.oidc => SelfAuthMethod.oidc,
            SessionAuthMethodEnum.trustedHeader => SelfAuthMethod.trustedHeader,
            SessionAuthMethodEnum.passkey => SelfAuthMethod.passkey,
            SessionAuthMethodEnum.passwordTotp => SelfAuthMethod.passwordTotp,
            SessionAuthMethodEnum.enrollmentCode =>
              SelfAuthMethod.enrollmentCode,
            SessionAuthMethodEnum.devicePairing => SelfAuthMethod.devicePairing,
            _ => SelfAuthMethod.other,
          },
          lastUsedAt: s.lastSeenAt ?? s.createdAt,
          current: s.current ?? false,
        ),
    ];
  });

  @override
  Future<SelfDevice> approveDevice(
    String deviceId,
    String code, {
    required String purpose,
  }) => _call(() async {
    final response = await _devices.approveDevice(
      deviceId: deviceId,
      deviceApprove: DeviceApprove((b) => b..verificationCode = code),
      idempotencyKey: _newKey(),
      extra: {stepUpPurposeExtra: purpose},
    );
    return _device(response.data!);
  });

  @override
  Future<SelfDevice> approvePairing(String code, {required String purpose}) =>
      _call(() async {
        final response = await _devices.approvePairing(
          pairingApprove: PairingApprove((b) => b..pairingCode = code),
          idempotencyKey: _newKey(),
          extra: {stepUpPurposeExtra: purpose},
        );
        return _device(response.data!);
      });

  @override
  Future<SelfDevice> revokeDevice(
    String deviceId, {
    required String reason,
    required String purpose,
  }) => _call(() async {
    final response = await _devices.revokeDevice(
      deviceId: deviceId,
      reasonBody: ReasonBody((b) => b..reason = reason),
      idempotencyKey: _newKey(),
      extra: {stepUpPurposeExtra: purpose},
    );
    return _device(response.data!);
  });

  @override
  Future<void> revokeSession(String sessionId) => _call(
    () =>
        _devices.revokeSession(sessionId: sessionId, idempotencyKey: _newKey()),
  );

  static SelfDevice _device(Device d, {String? thisDevice}) => SelfDevice(
    id: d.id,
    name: d.name,
    platform: d.platform.name,
    state: switch (d.state) {
      DeviceStateEnum.pending => SelfDeviceState.pending,
      DeviceStateEnum.active => SelfDeviceState.active,
      _ => SelfDeviceState.revoked,
    },
    createdAt: d.createdAt,
    appVersion: d.appVersion,
    lastSeenAt: d.lastSeenAt,
    isThisDevice: d.id == thisDevice,
  );

  /// Runs [call] and turns its errors into [SelfActionFailed]; an error of the login layer
  /// ([StepUpCancelled], a lost session) passes as it is.
  static Future<T> _call<T>(Future<T> Function() call) async {
    try {
      return await call();
    } on DioException catch (e) {
      final cause = e.error;
      if (cause is AuthException) throw cause;
      final response = e.response;
      throw SelfActionFailed(
        ApiProblem.of(e) ??
            (response == null ? null : ApiProblem.fromResponse(response)),
      );
    }
  }
}
