// SPDX-License-Identifier: Apache-2.0

// The people of the E2E suite who act through the public API, not through the app (brief M4-34
// Design note 2, docs/decisions.d/M4-34.md): Alex's other device (it issues enrollment codes,
// approves and revokes devices, creates forum posts) and Sam (sends and reads messages). Each is
// an `AuthSession` of the auth package with its own software keys, signed in with an enrollment
// code against the real server (the mobile suites of M5-11 sign Alex's other device in again
// with password and TOTP, `ApiActor.signIn`); a step-up uses the device key.

import 'dart:math';

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:domain/domain.dart' hide Message, NotificationLevel;

import 'e2e_keys.dart';
import 'e2e_network.dart';
import 'e2e_world.dart';

/// One signed-in device of a person, acting through the public API.
final class ApiActor {
  ApiActor._(this.auth, this._ids, this._network);

  /// Redeems [code] as a new device named [name] of the instance of [world] and returns its
  /// session. Every request of the actor goes directly to the server ([E2eNetwork.direct]), so
  /// cutting the app's network leaves the actor online. Throws a [StateError] when the server
  /// does not answer with an active device.
  static Future<ApiActor> enroll(
    E2eWorld world,
    E2eNetwork network,
    String code, {
    required String name,
  }) => signIn(
    world,
    network,
    name: name,
    keys: keysFor('actor/${randomNonce()}'),
    login: (auth) => auth.login.redeemEnrollmentCode(code),
  );

  /// Signs the device named [name] with [keys] in to the instance of [world] through [login]
  /// and returns its session, with the requests going directly to the server as for [enroll].
  ///
  /// [deviceId] fixes the device ID the registration carries (by default a new one): the
  /// mobile suites of brief M5-11 run one app process per suite against one stack, so they
  /// sign the same device of Alex in again in every process, which the server accepts for a
  /// known active device with the same key (08 5.2), while an enrollment code works only once.
  /// Throws a [StateError] when [login] does not end with an active device.
  static Future<ApiActor> signIn(
    E2eWorld world,
    E2eNetwork network, {
    required String name,
    required DeviceKeys keys,
    required Future<LoginOutcome> Function(AuthSession auth) login,
    String? deviceId,
  }) => network.direct(() async {
    final random = Random.secure();
    final ids = Uuid7Generator(
      nowMillis: () => DateTime.now().millisecondsSinceEpoch,
      random: (bits) => random.nextInt(1 << (bits > 31 ? 31 : bits)),
    );
    final secrets = _Secrets();
    if (deviceId != null) {
      // The key under which the auth package keeps the device ID of a server (session_core).
      await secrets.write('auth/${world.server.origin}/device_id', deviceId);
    }
    final auth = AuthSession(
      serverUrl: world.server,
      keys: keys,
      secrets: secrets,
      clock: const SystemClock(),
      ids: ids,
      config: AppConfig.defaults(),
      device: DeviceDescription(name: name, platform: 'macos'),
      stepUpPrompt: const _Confirm(),
      buildClient: (interceptors) => createApiClient(
        baseUrl: world.server,
        traceContext: () => TraceContext(traceId: '3' * 32, sampled: false),
        readRetry: ReadRetryPolicy.fromBackoff(
          base: const Duration(milliseconds: 50),
          cap: const Duration(seconds: 1),
        ),
        authInterceptors: interceptors,
      ),
    );
    final outcome = await login(auth);
    if (outcome is! LoginSignedIn) {
      await auth.dispose();
      throw StateError('$name: the login ended with $outcome');
    }
    return ApiActor._(auth, ids, network);
  });

  /// The session of the device.
  final AuthSession auth;

  final Uuid7Generator _ids;
  final E2eNetwork _network;

  /// The generated client of the public API.
  ApiClient get api => auth.api;

  /// This device's ID.
  String get deviceId => auth.session!.deviceId.value;

  /// Issues an enrollment code for the person [userId] (`createEnrollmentCode`, step-up); it
  /// ends every other unused code of that person (08 3.2).
  Future<String> issueCode(String userId) => _network.direct(() async {
    final r = await api.getUsersApi().createEnrollmentCode(
      userId: userId,
      idempotencyKey: _ids.next(),
    );
    return r.data!.code;
  });

  /// Approves the waiting device [deviceId] with the verification [code] it shows; returns
  /// the device as the server answered (a wrong code answers it `revoked`, 08 5.2).
  Future<Device> approveDevice(String deviceId, String code) async {
    final r = await api.getDevicesApi().approveDevice(
      deviceId: deviceId,
      deviceApprove: DeviceApprove((b) => b..verificationCode = code),
      idempotencyKey: _ids.next(),
    );
    return r.data!;
  }

  /// Revokes the device [deviceId] (`revokeDevice`, step-up).
  Future<void> revokeDevice(String deviceId) => _network.direct(
    () => api.getDevicesApi().revokeDevice(
      deviceId: deviceId,
      reasonBody: ReasonBody((b) => b..reason = 'Desktop E2E revocation'),
      idempotencyKey: _ids.next(),
    ),
  );

  /// Creates a post with [title] and the first message [text] in the forum [forumId]; returns
  /// the thread ID.
  Future<String> createPost(String forumId, String title, String text) =>
      _network.direct(() async {
        final r = await api.getThreadsApi().createThread(
          channelId: forumId,
          threadCreate: ThreadCreate(
            (b) => b
              ..title = title
              ..firstMessage.replace(_message(text)),
          ),
          idempotencyKey: _ids.next(),
        );
        return r.data!.thread.id;
      });

  /// Sends [text] into [threadId]; returns the message.
  Future<Message> post(String threadId, String text) =>
      _network.direct(() async {
        final r = await api.getMessagesApi().createMessage(
          threadId: threadId,
          messageCreate: _message(text),
        );
        return r.data!;
      });

  /// The newest messages of [threadId] in thread order (`thread_seq`).
  Future<List<Message>> messages(String threadId) => _network.direct(() async {
    final r = await api.getMessagesApi().listThreadMessages(
      threadId: threadId,
      limit: 100,
    );
    return r.data!.items.toList()
      ..sort((a, b) => a.threadSeq.compareTo(b.threadSeq));
  });

  /// Sets this person's notification level of the [kind] (`channel`, `thread`) [id] to
  /// [level] (`all`, `mentions`, `mute`; 13 3.1).
  Future<void> setLevel(String kind, String id, String level) =>
      api.getPreferencesApi().putNotificationPreference(
        targetKind: kind,
        targetId: id,
        notificationPreferencePut: NotificationPreferencePut(
          (b) => b..level = NotificationLevel.valueOf(level),
        ),
      );

  /// This person's stored notification level of the [kind] [id], or null.
  Future<String?> levelOf(String kind, String id) => _network.direct(() async {
    final r = await api.getPreferencesApi().listMyNotificationPreferences();
    for (final p in r.data!.items) {
      if (p.targetKind.name == kind && p.targetId == id) return p.level.name;
    }
    return null;
  });

  MessageCreate _message(String text) => MessageCreate(
    (b) => b
      ..clientMsgId = _ids.next()
      ..contentMd = text,
  );

  /// Ends the session's local state; the device stays registered on the server.
  Future<void> dispose() => auth.dispose();
}

final class _Secrets implements SecretStore {
  final Map<String, String> _values = {};

  @override
  Future<String?> read(String key) async => _values[key];

  @override
  Future<void> write(String key, String value) async => _values[key] = value;

  @override
  Future<void> delete(String key) async => _values.remove(key);
}

/// Confirms every step-up at once: the actor is the test itself.
final class _Confirm implements StepUpPrompt {
  const _Confirm();

  @override
  Future<bool> ask(String purposeText) async => true;
}
