// SPDX-License-Identifier: Apache-2.0

// The platform ports of the app (brief M4-18 Scope `lib/platform/`): secure storage, device
// key signers, the loopback redirect, the browser, lifecycle, network status and frames.

import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:math';

import 'package:client/app/deep_links.dart';
import 'package:client/platform/app_lifecycle.dart';
import 'package:client/platform/browser_launcher.dart';
import 'package:client/platform/device_key_signer.dart';
import 'package:client/platform/frame_ticker.dart';
import 'package:client/platform/loopback_redirect.dart';
import 'package:client/platform/network_status.dart';
import 'package:client/platform/secure_store.dart';
import 'package:device_key/device_key.dart' as dk;
import 'package:flutter/services.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';

import '../support/fakes.dart';

/// A device key client without a platform: keys are fixed bytes, signatures echo a marker.
class FakeDeviceKeyClient implements dk.DeviceKeyClient {
  final List<String> calls = [];
  dk.StepUpPrompt? lastPrompt;

  dk.DeviceKeyInfo _info(int seed) => dk.DeviceKeyInfo(
    x: Uint8List.fromList(List.filled(32, seed)),
    y: Uint8List.fromList(List.filled(32, seed + 1)),
    hardwareBound: true,
    requiresUserPresence: seed == 2,
  );

  @override
  Future<dk.DeviceKeyInfo> ensure(String scope, dk.KeyPurpose purpose) async {
    calls.add('ensure $scope ${purpose.name}');
    return _info(purpose == dk.KeyPurpose.dpop ? 1 : 2);
  }

  @override
  Future<Uint8List> sign(
    String scope,
    dk.KeyPurpose purpose,
    Uint8List data, {
    dk.StepUpPrompt? prompt,
  }) async {
    calls.add('sign $scope ${purpose.name}');
    lastPrompt = prompt;
    return Uint8List(64);
  }

  @override
  Future<void> delete(String scope) async => calls.add('delete $scope');

  @override
  Future<dk.DeviceKeyInfo?> keyInfo(String scope, dk.KeyPurpose purpose) =>
      throw UnimplementedError();

  @override
  Future<Uint8List?> attestation(
    String scope,
    dk.KeyPurpose purpose,
    Uint8List challenge,
  ) => throw UnimplementedError();
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('secure storage', () {
    test(
      'FlutterSecureBackend reads, writes, deletes and lists keys',
      () async {
        FlutterSecureStorage.setMockInitialValues({'a': '1'});
        final backend = FlutterSecureBackend();
        expect(await backend.read('a'), '1');
        await backend.write('b', '2');
        expect(await backend.keys(), {'a', 'b'});
        await backend.delete('a');
        expect(await backend.read('a'), isNull);
        expect(FlutterSecureBackend.macOsOptions.synchronizable, isFalse);
        expect(
          FlutterSecureBackend.macOsOptions.accessibility,
          KeychainAccessibility.first_unlock_this_device,
        );
      },
    );

    test('InstanceSecrets prefixes keys and deletes only its own', () async {
      final backend = MemorySecureBackend();
      final a = InstanceSecrets(backend, 'a');
      final b = InstanceSecrets(backend, 'b');
      await a.write('device_id', '1');
      await b.write('device_id', '2');
      await AppPreferences(backend)
          .setServerAddress(Uri.parse('https://chat.example.com'));
      expect(await a.read('device_id'), '1');
      expect(backend.values.keys, contains('instance/a/device_id'));
      await a.deleteAll();
      expect(await a.read('device_id'), isNull);
      expect(await b.read('device_id'), '2');
      expect(
        await AppPreferences(backend).serverAddress(),
        Uri.parse('https://chat.example.com'),
      );
      await b.delete('device_id');
      expect(await b.read('device_id'), isNull);
    });

    test(
      'the database key is 32 random bytes created once and then reused',
      () async {
        final secrets = InstanceSecrets(MemorySecureBackend(), 'a');
        final source = SecureDatabaseKeySource(secrets, random: Random(1));
        final first = await source.obtainKey();
        expect(first, hasLength(32));
        expect(await SecureDatabaseKeySource(secrets).obtainKey(), first);
        expect(
          base64Decode((await secrets.read(SecureDatabaseKeySource.keyName))!),
          first,
        );
      },
    );

    test('no address is remembered at first', () async {
      expect(
        await AppPreferences(MemorySecureBackend()).serverAddress(),
        isNull,
      );
    });
  });

  group('device keys', () {
    test(
      'ensure creates both keys of the scope and wraps them as ES256 signers',
      () async {
        final client = FakeDeviceKeyClient();
        final store = PlatformDeviceKeyStore(
          client,
          () =>
              const dk.StepUpPrompt(title: 'Confirm', negativeButton: 'Cancel'),
        );
        final keys = await store.ensure('instance-1');
        expect(client.calls, [
          'ensure instance-1 dpop',
          'ensure instance-1 stepUp',
        ]);
        expect(keys.dpop.alg, 'ES256');
        expect(keys.dpop.publicJwk['kty'], 'EC');
        expect(keys.dpop.thumbprint, isNot(keys.stepUp.thumbprint));
        expect(await keys.dpop.sign(Uint8List(3)), hasLength(64));
        expect(client.lastPrompt, isNull);
        await keys.stepUp.sign(Uint8List(3));
        expect(client.lastPrompt?.title, 'Confirm');
        await store.delete('instance-1');
        expect(client.calls.last, 'delete instance-1');
      },
    );
  });

  group('loopback redirect', () {
    // flutter_test answers every HttpClient request with 400; these tests need real sockets.
    HttpOverrides? saved;
    setUp(() {
      saved = HttpOverrides.current;
      HttpOverrides.global = null;
    });
    tearDown(() => HttpOverrides.global = saved);

    test(
      'binds 127.0.0.1 with a free port and returns the first callback',
      () async {
        final receiver = LoopbackRedirectReceiver(doneText: () => 'done');
        final uri = await receiver.redirectUri();
        expect(uri.host, '127.0.0.1');
        expect(uri.port, greaterThan(0));
        expect(uri.path, LoopbackRedirectReceiver.callbackPath);
        final waiting = receiver.waitForRedirect(
          uri,
          const Duration(seconds: 5),
        );
        final client = HttpClient();
        final other = await (await client.getUrl(uri.replace(path: '/other')))
            .close();
        expect(other.statusCode, 404);
        await other.drain<void>();
        final response = await (await client.getUrl(
          uri.replace(query: 'login_code=abc&login_attempt_id=1'),
        )).close();
        expect(await response.transform(utf8.decoder).join(), 'done');
        final redirect = await waiting;
        expect(redirect.queryParameters['login_code'], 'abc');
        client.close(force: true);
        await expectLater(
          HttpClient().getUrl(uri).then((r) => r.close()),
          throwsA(isA<SocketException>()),
        );
      },
    );

    test('times out without a redirect and needs redirectUri first', () async {
      final receiver = LoopbackRedirectReceiver(doneText: () => 'done');
      await expectLater(
        receiver.waitForRedirect(
          Uri.parse('http://127.0.0.1:1/callback'),
          Duration.zero,
        ),
        throwsStateError,
      );
      final uri = await receiver.redirectUri();
      await expectLater(
        receiver.waitForRedirect(uri, const Duration(milliseconds: 20)),
        throwsA(isA<TimeoutException>()),
      );
    });
  });

  group('browser', () {
    test(
      'opens the URL externally and fails when the system refuses',
      () async {
        final opened = <Uri>[];
        final url = Uri.parse('https://chat.example.com/login');
        await SystemBrowserLauncher(
          opener: (u) async {
            opened.add(u);
            return true;
          },
        ).open(url);
        expect(opened, [url]);
        await expectLater(
          SystemBrowserLauncher(opener: (_) async => false).open(url),
          throwsStateError,
        );
      },
    );
  });

  group('lifecycle and network', () {
    test(
      'resumed and inactive are foreground; a return to it is a network hint',
      () async {
        final lifecycle = FlutterAppLifecycle(
          initial: AppLifecycleState.resumed,
        );
        final network = LifecycleNetworkStatus(lifecycle);
        final activities = <AppActivity>[];
        final hints = <bool>[];
        lifecycle.changes.listen(activities.add);
        network.changes.listen(hints.add);
        expect(lifecycle.current, AppActivity.foreground);
        lifecycle
          ..onState(AppLifecycleState.inactive)
          ..onState(AppLifecycleState.hidden)
          ..onState(AppLifecycleState.paused)
          ..onState(AppLifecycleState.resumed);
        await Future<void>.delayed(Duration.zero);
        expect(activities, [AppActivity.background, AppActivity.foreground]);
        expect(hints, [true]);
        expect(network.online, isTrue);
        await network.dispose();
        await lifecycle.dispose();
      },
    );
  });

  testWidgets('the frame ticker runs the callback in the next frame', (
    tester,
  ) async {
    var ran = 0;
    SchedulerFrameTicker().requestFrame(() => ran++);
    expect(ran, 0);
    await tester.pump();
    expect(ran, 1);
  });

  group('deep link channel', () {
    const channel = MethodChannel('bothy/deep_links');

    test('takes the pending links and streams the later ones', () async {
      final messenger =
          TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
      messenger.setMockMethodCallHandler(channel, (call) async {
        expect(call.method, 'takePending');
        return ['chat-example://t/a', '::bad'];
      });
      final links = MethodChannelDeepLinks();
      expect(
        (await links.takePending()).first,
        Uri.parse('chat-example://t/a'),
      );
      final got = <Uri>[];
      links.links.listen(got.add);
      await messenger.handlePlatformMessage(
        channel.name,
        const StandardMethodCodec().encodeMethodCall(
          const MethodCall('open', 'chat-example://c/b'),
        ),
        (_) {},
      );
      expect(got, [Uri.parse('chat-example://c/b')]);
      messenger.setMockMethodCallHandler(channel, null);
    });

    test('a host without the runner channel has no pending links', () async {
      expect(await MethodChannelDeepLinks().takePending(), isEmpty);
    });
  });
}
