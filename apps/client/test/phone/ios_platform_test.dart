// SPDX-License-Identifier: Apache-2.0

// The iOS adapters (brief M5-02 Design notes 3 and 6; 15 6.3, 8, 9; 08 2.2, 2.5): the OIDC
// redirect through ASWebAuthenticationSession with a reverse-domain scheme, passkeys only for
// the associated domain, the keychain classes of the secrets, and the protection class set on
// the database files before the first open.

import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:client/app/bootstrap.dart';
import 'package:client/app/discovery.dart';
import 'package:client/app/form_factor.dart';
import 'package:client/platform/ios_platform.dart';
import 'package:client/platform/secure_store.dart';
import 'package:flutter/services.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/fakes.dart';

/// The channel of the runner, answered by the test.
class FakeIosApi extends Fake implements IosPlatformApi {
  final List<(String, String)> sessions = [];
  final List<String> assertions = [];
  Future<String> Function()? answer;

  @override
  Future<String> authenticate(String url, String callbackScheme) {
    sessions.add((url, callbackScheme));
    return answer!();
  }

  @override
  Future<String> passkeyAssertion(String requestOptionsJson) async {
    assertions.add(requestOptionsJson);
    return jsonEncode({'id': 'abc', 'type': 'public-key'});
  }

  @override
  Future<String> passkeyRegistration(String creationOptionsJson) async {
    assertions.add(creationOptionsJson);
    return jsonEncode({'id': 'def', 'type': 'public-key'});
  }
}

void main() {
  group('AppleWebAuthentication', () {
    final redirect = Uri.parse('com.example.chat:/oidc/callback');

    test('the redirect URI is the bundle ID as a private-use scheme', () async {
      final login = AppleWebAuthentication(
        api: FakeIosApi(),
        bundleId: 'com.example.chat',
      );
      expect(await login.redirectUri(), redirect);
    });

    test('one session per login; its callback is the redirect', () async {
      final api = FakeIosApi()
        ..answer = () async => '$redirect?login_code=c&login_attempt_id=a';
      final login = AppleWebAuthentication(
        api: api,
        bundleId: 'com.example.chat',
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
      'a closed sheet and a failure come back as errors of the redirect',
      () async {
        for (final (code, error) in [
          ('cancelled', 'cancelled'),
          ('failed', 'login_failed'),
          ('unavailable', 'login_failed'),
        ]) {
          final api = FakeIosApi()
            ..answer = () => Future.error(PlatformException(code: code));
          final login = AppleWebAuthentication(
            api: api,
            bundleId: 'com.example.chat',
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
      'waiting without a session is an error; the flow deadline holds',
      () async {
        final api = FakeIosApi()..answer = () => Completer<String>().future;
        final login = AppleWebAuthentication(
          api: api,
          bundleId: 'com.example.chat',
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
      'only the relying party of the associated domain reaches the system',
      () async {
        final api = FakeIosApi();
        final passkeys = ApplePasskeys(
          api: api,
          associatedDomain: 'example.com',
        );
        expect(
          () => passkeys.getAssertion({
            'rpId': 'other.example.org',
            'challenge': 'x',
          }),
          throwsStateError,
        );
        expect(
          () => passkeys.createCredential({
            'rp': {'id': 'other.example.org'},
          }),
          throwsStateError,
        );
        expect(api.assertions, isEmpty);
        expect(
          await passkeys.getAssertion({
            'rpId': 'example.com',
            'challenge': 'x',
          }),
          {'id': 'abc', 'type': 'public-key'},
        );
        expect(
          await passkeys.createCredential({
            'rp': {'id': 'example.com'},
          }),
          {'id': 'def', 'type': 'public-key'},
        );
      },
    );

    test('the associated domain serves its host and the hosts below it', () {
      expect(passkeyDomainServes('example.com', 'example.com'), isTrue);
      expect(passkeyDomainServes('example.com', 'chat.example.com'), isTrue);
      expect(passkeyDomainServes('chat.example.com', 'example.com'), isFalse);
      expect(passkeyDomainServes('example.com', 'badexample.com'), isFalse);
      expect(passkeyDomainServes('', 'example.com'), isFalse);
    });

    test(
      'E2 offers passkeys only where the build can serve the instance',
      () async {
        final root = await Directory.systemTemp.createTemp('passkeys');
        addTearDown(() => root.delete(recursive: true));
        final passkeys = ApplePasskeys(
          api: FakeIosApi(),
          associatedDomain: 'example.com',
        );
        InstanceInfo at(String host) => InstanceInfo(
          address: Uri.parse('https://$host'),
          instanceId: '0193f2a7-0000-7000-8000-0000000000ab',
          productName: 'Team Chat',
        );
        final phone = testPorts(
          dataRoot: root,
          platform: 'ios',
          passkeys: passkeys,
          passkeyDomain: 'example.com',
        );
        expect(phone.passkeysFor(at('chat.example.com')), same(passkeys));
        expect(phone.passkeysFor(at('chat.example.org')), isNull);
        // Without an associated domain an iOS build has no passkeys at all (productionPorts).
        final desktop = testPorts(dataRoot: root, passkeys: passkeys);
        expect(desktop.passkeysFor(at('chat.example.org')), same(passkeys));
      },
    );
  });

  test('the database key is kept only while unlocked, the rest after the first unlock', () {
    expect(
      FlutterSecureBackend.iosOptionsFor('instance/abc/database_key'),
      FlutterSecureBackend.iosDatabaseKeyOptions,
    );
    expect(
      FlutterSecureBackend.iosOptionsFor(
        'instance/abc/auth/https://x/refresh_token',
      ),
      FlutterSecureBackend.iosOptions,
    );
    expect(
      FlutterSecureBackend.iosDatabaseKeyOptions.toMap()['accessibility'],
      KeychainAccessibility.unlocked_this_device.name,
    );
    expect(
      FlutterSecureBackend.iosOptions.toMap()['accessibility'],
      KeychainAccessibility.first_unlock_this_device.name,
    );
    expect(FlutterSecureBackend.iosOptions.toMap()['synchronizable'], 'false');
  });

  test('the platform decides the form factor', () {
    expect(FormFactor.ofPlatform('ios'), FormFactor.phone);
    expect(FormFactor.ofPlatform('android'), FormFactor.phone);
    expect(FormFactor.ofPlatform('macos'), FormFactor.desktop);
    expect(FormFactor.ofPlatform('windows'), FormFactor.desktop);
  });

  test(
    'the database files get the protection class before the first open',
    () async {
      final root = await Directory.systemTemp.createTemp('protect');
      addTearDown(() => root.delete(recursive: true));
      final events = <String>[];
      final ports = testPorts(
        dataRoot: root,
        platform: 'ios',
        protectFiles: (paths) async => events.add('protect ${paths.join(' ')}'),
        openDatabase: (file, keys) async {
          events.add('open ${file.path}');
          return openLocalStore(file: file, keys: keys, inBackground: false);
        },
      );
      final services = await InstanceServices.open(
        ports,
        InstanceInfo(
          address: Uri.parse('https://chat.example.com'),
          instanceId: '0193f2a7-0000-7000-8000-0000000000ab',
          productName: 'Team Chat',
        ),
      );
      addTearDown(services.close);
      final folder =
          '${root.path}/instances/0193f2a7-0000-7000-8000-0000000000ab';
      final db = '$folder/local.db';
      final protect = 'protect $folder $db $db-wal $db-shm';
      expect(events, [protect, 'open $db', protect]);
    },
  );
}
