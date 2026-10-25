// SPDX-License-Identifier: Apache-2.0

// The people of the E2E suite who act through the public API, not through the app (brief M4-34
// Design note 2, docs/decisions.d/M4-34.md): Alex's other device (it issues enrollment codes,
// approves and revokes devices, creates forum posts) and Sam (sends and reads messages). Each is
// an `AuthSession` of the auth package with its own software keys, signed in with an enrollment
// code against the real server; a step-up uses the device key.

import 'dart:math';

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:domain/domain.dart' hide Message;

import 'e2e_keys.dart';
import 'e2e_world.dart';

/// One signed-in device of a person, acting through the public API.
final class ApiActor {
  ApiActor._(this.auth, this._ids);

  /// Redeems [code] as a new device named [name] of the instance of [world] and returns its
  /// session. Throws a [StateError] when the server does not answer with an active device.
  static Future<ApiActor> enroll(
    E2eWorld world,
    String code, {
    required String name,
  }) async {
    final random = Random.secure();
    final ids = Uuid7Generator(
      nowMillis: () => DateTime.now().millisecondsSinceEpoch,
      random: (bits) => random.nextInt(1 << (bits > 31 ? 31 : bits)),
    );
    final auth = AuthSession(
      serverUrl: world.server,
      keys: keysFor('actor/${randomNonce()}'),
      secrets: _Secrets(),
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
    final outcome = await auth.login.redeemEnrollmentCode(code);
    if (outcome is! LoginSignedIn) {
      await auth.dispose();
      throw StateError('$name: the enrollment ended with $outcome');
    }
    return ApiActor._(auth, ids);
  }

  /// The session of the device.
  final AuthSession auth;

  final Uuid7Generator _ids;

  /// The generated client of the public API.
  ApiClient get api => auth.api;

  /// This device's ID.
  String get deviceId => auth.session!.deviceId.value;

  /// Issues an enrollment code for the person [userId] (`createEnrollmentCode`, step-up); it
  /// ends every other unused code of that person (08 3.2).
  Future<String> issueCode(String userId) async {
    final r = await api.getUsersApi().createEnrollmentCode(
      userId: userId,
      idempotencyKey: _ids.next(),
    );
    return r.data!.code;
  }

  /// Approves the waiting device [deviceId] with the verification [code] it shows.
  Future<void> approveDevice(String deviceId, String code) =>
      api.getDevicesApi().approveDevice(
        deviceId: deviceId,
        deviceApprove: DeviceApprove((b) => b..verificationCode = code),
        idempotencyKey: _ids.next(),
      );

  /// Revokes the device [deviceId] (`revokeDevice`, step-up).
  Future<void> revokeDevice(String deviceId) =>
      api.getDevicesApi().revokeDevice(
        deviceId: deviceId,
        reasonBody: ReasonBody((b) => b..reason = 'Desktop E2E revocation'),
        idempotencyKey: _ids.next(),
      );

  /// Creates a post with [title] and the first message [text] in the forum [forumId]; returns
  /// the thread ID.
  Future<String> createPost(String forumId, String title, String text) async {
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
  }

  /// Sends [text] into [threadId]; returns the message.
  Future<Message> post(String threadId, String text) async {
    final r = await api.getMessagesApi().createMessage(
      threadId: threadId,
      messageCreate: _message(text),
    );
    return r.data!;
  }

  /// The newest messages of [threadId], oldest first.
  Future<List<Message>> messages(String threadId) async {
    final r = await api.getMessagesApi().listThreadMessages(
      threadId: threadId,
      limit: 100,
    );
    return r.data!.items.toList();
  }

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
