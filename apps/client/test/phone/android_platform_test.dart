// SPDX-License-Identifier: Apache-2.0

// The Android adapters (brief M5-03 Design notes 2 and 3; 15 8, 9; 08 2.2, 2.5, 18): the OIDC
// redirect through a Custom Tab with the application ID as the private-use scheme, passkeys only
// for the App Links host and only where the Credential Manager serves them, the device name of
// the runner, and the Keystore-backed options of the secret store.

import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:client/app/discovery.dart';
import 'package:client/platform/android_platform.dart';
import 'package:client/platform/secure_store.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/fakes.dart';

/// The channel of the runner, answered by the test.
class FakeAndroidApi extends Fake implements AndroidPlatformApi {
  final List<(String, String)> sessions = [];
  final List<String> requests = [];
  Future<String> Function()? answer;
  bool supported = true;

  @override
  Future<String> authenticate(String url, String callbackScheme) {
    sessions.add((url, callbackScheme));
    return answer!();
  }

  @override
  Future<bool> passkeysSupported() async => supported;

  @override
  Future<String> deviceName() async => 'Example Phone 7';

  @override
  Future<String> passkeyAssertion(String requestOptionsJson) async {
    requests.add(requestOptionsJson);
    return jsonEncode({'id': 'abc', 'type': 'public-key'});
  }

  @override
  Future<String> passkeyRegistration(String creationOptionsJson) async {
    requests.add(creationOptionsJson);
    return jsonEncode({'id': 'def', 'type': 'public-key'});
  }
}

void main() {
  group('AndroidCustomTabsLogin', () {
    final redirect = Uri.parse('com.example.chat:/oidc/callback');

    test(
      'the redirect URI is the application ID as a private-use scheme',
      () async {
        final login = AndroidCustomTabsLogin(
          api: FakeAndroidApi(),
          applicationId: 'com.example.chat',
        );
        expect(await login.redirectUri(), redirect);
      },
    );

    test('one tab per login; the redirect it receives comes back', () async {
      final api = FakeAndroidApi()
        ..answer = () async => '$redirect?login_code=c&login_attempt_id=a';
      final login = AndroidCustomTabsLogin(
        api: api,
        applicationId: 'com.example.chat',
      );
      final uri = await login.redirectUri();
      await login.open(Uri.parse('https://id.example.com/authorize?x=1'));
      final back = await login.waitForRedirect(uri, const Duration(minutes: 5));
      expect(api.sessions, [
        ('https://id.example.com/authorize?x=1', 'com.example.chat'),
      ]);
      expect(back.queryParameters, {
        'login_code': 'c',
        'login_attempt_id': 'a',
      });
    });

    test(
      'a closed tab and a failure come back as errors of the redirect',
      () async {
        for (final (code, error) in [
          ('cancelled', 'cancelled'),
          ('failed', 'login_failed'),
          ('unavailable', 'login_failed'),
        ]) {
          final api = FakeAndroidApi()
            ..answer = () => Future.error(PlatformException(code: code));
          final login = AndroidCustomTabsLogin(
            api: api,
            applicationId: 'com.example.chat',
          );
          final uri = await login.redirectUri();
          await login.open(Uri.parse('https://id.example.com/authorize'));
          final back = await login.waitForRedirect(
            uri,
            const Duration(minutes: 5),
          );
          expect(back.queryParameters['error'], error, reason: code);
        }
      },
    );

    test(
      'waiting without a tab is an error; the flow deadline holds',
      () async {
        final api = FakeAndroidApi()..answer = () => Completer<String>().future;
        final login = AndroidCustomTabsLogin(
          api: api,
          applicationId: 'com.example.chat',
        );
        final uri = await login.redirectUri();
        expect(
          () => login.waitForRedirect(uri, Duration.zero),
          throwsStateError,
        );
        await login.open(Uri.parse('https://id.example.com/authorize'));
        await expectLater(
          login.waitForRedirect(uri, Duration.zero),
          throwsA(isA<TimeoutException>()),
        );
      },
    );
  });

  group('passkeys', () {
    test(
      'only the relying party of the App Links host reaches the system',
      () async {
        final api = FakeAndroidApi();
        final passkeys = AndroidPasskeys(
          api: api,
          appLinksHost: 'chat.example.com',
        );
        expect(
          () =>
              passkeys.getAssertion({'rpId': 'example.com', 'challenge': 'x'}),
          throwsStateError,
        );
        expect(
          () => passkeys.createCredential({
            'rp': {'id': 'other.example.org'},
          }),
          throwsStateError,
        );
        expect(api.requests, isEmpty);
        expect(
          await passkeys.getAssertion({
            'rpId': 'chat.example.com',
            'challenge': 'x',
          }),
          {'id': 'abc', 'type': 'public-key'},
        );
        expect(
          await passkeys.createCredential({
            'rp': {'id': 'chat.example.com'},
          }),
          {'id': 'def', 'type': 'public-key'},
        );
      },
    );

    test('the App Links host serves only the same host', () {
      expect(
        appLinksHostServes('chat.example.com', 'chat.example.com'),
        isTrue,
      );
      expect(appLinksHostServes('example.com', 'chat.example.com'), isFalse);
      expect(appLinksHostServes('chat.example.com', 'example.com'), isFalse);
      expect(appLinksHostServes('', 'chat.example.com'), isFalse);
    });

    test('E2 offers passkeys only for the App Links host', () async {
      final root = await Directory.systemTemp.createTemp('passkeys');
      addTearDown(() => root.delete(recursive: true));
      final passkeys = AndroidPasskeys(
        api: FakeAndroidApi(),
        appLinksHost: 'chat.example.com',
      );
      InstanceInfo at(String host) => InstanceInfo(
        address: Uri.parse('https://$host'),
        instanceId: '0193f2a7-0000-7000-8000-0000000000ab',
        productName: 'Team Chat',
      );
      final phone = testPorts(
        dataRoot: root,
        platform: 'android',
        passkeys: passkeys,
        passkeyDomain: 'chat.example.com',
      );
      expect(phone.passkeysFor(at('chat.example.com')), same(passkeys));
      // iOS accepts a host below its associated domain; Android only the host itself.
      expect(phone.passkeysFor(at('eu.chat.example.com')), isNull);
      expect(phone.passkeysFor(at('chat.example.org')), isNull);
    });

    test(
      'the runner tells whether the Credential Manager serves passkeys',
      () async {
        final api = FakeAndroidApi();
        final info = await AndroidRunnerInfo.load(api);
        expect(info.passkeysSupported, isTrue);
        expect(info.deviceName, 'Example Phone 7');
        api.supported = false;
        expect((await AndroidRunnerInfo.load(api)).passkeysSupported, isFalse);
      },
    );

    test('passkeys need a supported device and an App Links host', () {
      final api = FakeAndroidApi();
      expect(
        androidPasskeys(api, supported: true, appLinksHost: 'chat.example.com'),
        isA<AndroidPasskeys>(),
      );
      expect(
        androidPasskeys(
          api,
          supported: false,
          appLinksHost: 'chat.example.com',
        ),
        isNull,
      );
      expect(androidPasskeys(api, supported: true, appLinksHost: ''), isNull);
    });
  });

  test(
    'the secrets use the Keystore-wrapped AES-GCM store without biometrics',
    () {
      final options = FlutterSecureBackend.androidOptions.toMap();
      expect(
        options['keyCipherAlgorithm'],
        'RSA_ECB_OAEPwithSHA_256andMGF1Padding',
      );
      expect(options['storageCipherAlgorithm'], 'AES_GCM_NoPadding');
      expect(options['enforceBiometrics'], 'false');
      expect(options['migrateOnAlgorithmChange'], 'true');
    },
  );
}
