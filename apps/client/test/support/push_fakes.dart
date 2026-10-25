// SPDX-License-Identifier: Apache-2.0

// Fakes of push (briefs M5-04 and M5-05): the Pigeon hosts of the iOS, macOS and Android
// runners and the REST operations of a push registration.

// The fake implements the generated host API, whose fields carry Pigeon's names.
// ignore_for_file: non_constant_identifier_names

import 'package:client/platform/push_android.dart';
import 'package:client/platform/push_apple.dart';
import 'package:flutter/services.dart';

/// The instance of the push tests.
const String instance = '0192a000-0000-7000-8000-00000000c0de';

/// The runner's push host in memory: the shared keychain, the App Group values and the
/// permission the test sets.
class FakePushHost implements PushAppleHostApi {
  bool available = true;
  bool authorized = false;
  bool grant = true;
  String token = 'aa11';
  int permissionRequests = 0;
  final Map<String, PushRegistrationRecord> keychain = {};
  final List<String> settings = [];
  PushDisplayCounters counters = PushDisplayCounters(
    bucketBounds: [0.5, 1, 10],
    bucketCounts: [0, 0, 0],
    placeholders: 0,
  );
  List<PushTap> pending = [];

  @override
  BinaryMessenger? get pigeonVar_binaryMessenger => null;

  @override
  String get pigeonVar_messageChannelSuffix => '';

  @override
  Future<bool> isAvailable() async => available;

  @override
  Future<String> environment() async => 'sandbox';

  @override
  Future<String> appId() async => 'com.example.chat';

  @override
  Future<bool> isAuthorized() async => authorized;

  @override
  Future<bool> requestAuthorization() async {
    permissionRequests++;
    authorized = grant;
    return grant;
  }

  @override
  Future<String> deviceToken() async => token;

  @override
  Future<void> saveRegistration(PushRegistrationRecord record) async =>
      keychain[record.instanceId] = record;

  @override
  Future<String?> registrationId(String instanceId) async =>
      keychain[instanceId]?.registrationId;

  @override
  Future<void> deleteRegistration(String instanceId) async {
    keychain.remove(instanceId);
    settings.add('deleted $instanceId');
  }

  @override
  Future<void> writeSettings(
    String instanceId,
    String previewMode,
    int fetchTimeoutMillis,
  ) async => settings.add('$instanceId $previewMode $fetchTimeoutMillis');

  @override
  Future<PushDisplayCounters> takeDisplayCounters() async {
    final taken = counters;
    counters = PushDisplayCounters(
      bucketBounds: taken.bucketBounds,
      bucketCounts: [0, 0, 0],
      placeholders: 0,
    );
    return taken;
  }

  @override
  Future<List<PushTap>> takePendingTaps() async {
    final taken = pending;
    pending = [];
    return taken;
  }
}

/// `registerPush` and `unregisterPush` that record their calls.
class FakeRegistrations implements PushRegistrations {
  final List<String> calls = [];
  int next = 0;

  @override
  Future<RegisteredPush> register({
    required String provider,
    required String token,
    required String appId,
    String? environment,
  }) async {
    next++;
    calls.add(
      provider == 'apns'
          ? 'register $token $environment $appId'
          : 'register $provider $token $appId',
    );
    return RegisteredPush(
      id: 'reg-$next',
      previewToken: 'bothy_pvw_EXAMPLE$next',
    );
  }

  @override
  Future<void> unregister(String registrationId) async =>
      calls.add('unregister $registrationId');
}

/// The Android runner's push host in memory: the encrypted store of the push service, the
/// settings it reads and the permission the test sets.
class FakeAndroidPushHost implements PushAndroidHostApi {
  bool available = true;
  bool authorized = false;
  bool grant = true;
  String fcmToken = 'fcm-token-1';
  int permissionRequests = 0;
  final Map<String, AndroidPushRegistration> store = {};
  final List<String> settings = [];
  AndroidPushCounters counters = AndroidPushCounters(
    bucketBounds: [0.5, 1, 10],
    bucketCounts: [0, 0, 0],
    placeholders: 0,
  );
  List<AndroidPushTap> pending = [];

  @override
  BinaryMessenger? get pigeonVar_binaryMessenger => null;

  @override
  String get pigeonVar_messageChannelSuffix => '';

  @override
  Future<bool> isAvailable() async => available;

  @override
  Future<String> appId() async => 'com.example.chat';

  @override
  Future<bool> isAuthorized() async => authorized;

  @override
  Future<bool> requestAuthorization() async {
    permissionRequests++;
    authorized = grant;
    return grant;
  }

  @override
  Future<String> token() async => fcmToken;

  @override
  Future<void> saveRegistration(AndroidPushRegistration registration) async =>
      store[registration.instanceId] = registration;

  @override
  Future<String?> registrationId(String instanceId) async =>
      store[instanceId]?.registrationId;

  @override
  Future<void> deleteRegistration(String instanceId) async {
    store.remove(instanceId);
    settings.add('deleted $instanceId');
  }

  @override
  Future<void> writeSettings(
    String instanceId,
    String previewMode,
    int fetchTimeoutMillis,
  ) async => settings.add('$instanceId $previewMode $fetchTimeoutMillis');

  @override
  Future<AndroidPushCounters> takeDisplayCounters() async {
    final taken = counters;
    counters = AndroidPushCounters(
      bucketBounds: taken.bucketBounds,
      bucketCounts: [0, 0, 0],
      placeholders: 0,
    );
    return taken;
  }

  @override
  Future<List<AndroidPushTap>> takePendingTaps() async {
    final taken = pending;
    pending = [];
    return taken;
  }
}
