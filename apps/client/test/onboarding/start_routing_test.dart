// SPDX-License-Identifier: Apache-2.0

// The class test of the start and onboarding routing (docs/DECISIONS.md D-M4-18-13 and
// D-M4-18-15; brief M4-18 Design note 3; 01 5.6, 3.6): every stored state of the app times
// every network result of the start times what the network does afterwards, run through the
// real controller, auth session, local database and engine against the fake server. The
// network result is produced by the real discovery client over a scripted HTTP adapter, so
// the classification of `getDiscovery` is part of the table; `getDevice` gets the same result
// from the fake's fault plan.

import 'dart:io';
import 'dart:typed_data';

import 'package:client/app/discovery.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import '../support/fake_instance.dart';
import '../support/fakes.dart' show testBranding;
import '../support/other_device.dart';

/// What the app holds for the instance when it starts.
enum Stored {
  /// Nothing: no remembered address; the person types it on E1.
  none,

  /// The remembered address after a logout, no session.
  signedOut,

  /// A session of an active device.
  signedIn,

  /// A session of a device that waits for its confirmation (E4).
  pending,
}

/// What the network answers at the start.
enum Network {
  noAnswer,
  timeout,
  ok,
  notFound404,
  badGateway502,
  unavailable503,
  gatewayTimeout504,
  setupRequired,
  certificateInvalid;

  int? get status => switch (this) {
    notFound404 => 404,
    badGateway502 => 502,
    unavailable503 => 503,
    gatewayTimeout504 => 504,
    _ => null,
  };
}

/// What the network does after the start.
enum Later { comesOnline, staysOffline }

/// The screen: the onboarding step and, on E1, its result line.
typedef Screen = (OnboardingStep, ServerStatus?);

/// The screen the brief and the spec prescribe (01 5.6, 3.6; D-M4-18-3, D-M4-18-13,
/// D-M4-18-15): a certificate error, `setup_required` and E2 for a found instance without a
/// session are E1 and E2 as always; a stored session opens its local data whatever else the
/// network answers (the main view, or E4 for a waiting device); without a session every
/// other answer is "No instance at this address".
Screen expected(Stored stored, Network network) {
  if (network == Network.certificateInvalid) {
    return (OnboardingStep.server, ServerStatus.certificateInvalid);
  }
  if (network == Network.setupRequired) {
    return (OnboardingStep.server, ServerStatus.notSetUp);
  }
  return switch (stored) {
    Stored.none || Stored.signedOut =>
      network == Network.ok
          ? (OnboardingStep.login, null)
          : (OnboardingStep.server, ServerStatus.noInstance),
    Stored.signedIn => (OnboardingStep.signedIn, null),
    Stored.pending => (OnboardingStep.pending, null),
  };
}

/// A discovery client whose network the test switches: [Network.ok] asks the fake, every other
/// value runs the real [ApiDiscoveryClient] over an adapter that produces that result.
final class SwitchedDiscovery implements DiscoveryClient {
  late DiscoveryClient real;
  Network network = Network.ok;

  @override
  Future<DiscoveryOutcome> discover(Uri address) => network == Network.ok
      ? real.discover(address)
      : ApiDiscoveryClient(
          platform: 'macos',
          appVersion: testBranding.appVersion,
          allowPlainHttp: true,
          adapter: _NetworkAdapter(network),
        ).discover(address);
}

final class _NetworkAdapter implements HttpClientAdapter {
  _NetworkAdapter(this.network);

  final Network network;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    switch (network) {
      case Network.noAnswer:
        throw const SocketException('Connection refused');
      case Network.timeout:
        throw DioException.connectionTimeout(
          timeout: const Duration(seconds: 10),
          requestOptions: options,
        );
      case Network.certificateInvalid:
        throw const HandshakeException('CERTIFICATE_VERIFY_FAILED');
      case Network.setupRequired:
        return ResponseBody.fromString(
          '{"type":"about:blank","title":"Setup required","status":503,'
          '"code":"setup_required"}',
          503,
          headers: {
            Headers.contentTypeHeader: ['application/problem+json'],
          },
        );
      case Network.ok:
        throw StateError('ok goes to the fake');
      case Network.notFound404 ||
          Network.badGateway502 ||
          Network.unavailable503 ||
          Network.gatewayTimeout504:
        // The error page of a reverse proxy, not a problem of the instance.
        return ResponseBody.fromString(
          '<html><body>${network.status}</body></html>',
          network.status!,
          headers: {
            Headers.contentTypeHeader: ['text/html'],
          },
        );
    }
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  HttpOverrides? saved;
  late FakeInstance f;
  late ProviderContainer c;
  late SwitchedDiscovery net;
  OtherDevice? other;

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    net = SwitchedDiscovery();
    f = await FakeInstance.start(discovery: (real) => net..real = real);
    c = appContainer(f);
  });

  tearDown(() async {
    c.dispose();
    await other?.dispose();
    other = null;
    await f.stop();
    HttpOverrides.global = saved;
  });

  OnboardingState state() => c.read(onboardingControllerProvider);
  Screen screen() => (
    state().step,
    state().step == OnboardingStep.server ? state().serverStatus : null,
  );
  bool settled() =>
      !state().busy && state().serverStatus != ServerStatus.connecting;

  Future<void> login(OnboardingController control) => control.loginWithPassword(
    WorldSeed.alexLogin,
    WorldSeed.password,
    WorldSeed.totp,
  );

  /// Brings the app into [stored]; returns the E4 code and device ID of a waiting device.
  Future<(String?, String?)> prepare(Stored stored) async {
    if (stored == Stored.none) return (null, null);
    if (stored == Stored.pending) {
      other = OtherDevice(f.server);
      await other!.signIn();
      f.seed.alex.newDevicesPending = true;
    }
    final control = await connected(c, f);
    await login(control);
    switch (stored) {
      case Stored.none:
        break;
      case Stored.signedOut:
        await control.logout();
        expect(state().step, OnboardingStep.signedOut);
      case Stored.signedIn:
        await eventually(
          'live',
          () => state().services?.engine.state.phase == ConnectionPhase.live,
        );
      case Stored.pending:
        expect(state().step, OnboardingStep.pending);
        return (state().code, state().services!.auth.session!.deviceId.value);
    }
    return (null, null);
  }

  /// How many `getDevice` requests fail while the network is down; far more than a test
  /// sends, so only coming online ends it.
  const offlineAnswers = 100000;

  /// Answers `getDevice` like [network] for [times] requests.
  void restAnswers(Network network, int times) {
    final status = network.status;
    if (status != null && status >= 500) {
      f.server.faults.onRequest(
        'getDevice',
        status: status,
        code: 'unavailable',
        times: times,
      );
    } else {
      f.server.faults.dropResponseAfterCommit('getDevice', times: times);
    }
  }

  Future<void> run(Stored stored, Network network, Later later) async {
    final (code, deviceId) = await prepare(stored);
    // The app quits and starts again with the network of the case.
    c.dispose();
    net.network = network;
    if (network != Network.ok) {
      restAnswers(network, offlineAnswers);
    }
    c = appContainer(f);
    final want = expected(stored, network);
    if (stored == Stored.none) {
      final control = await connected(c, f);
      expect(control, isNotNull);
    }
    await eventually(
      'screen $want',
      () => settled() && screen() == want,
      timeout: const Duration(seconds: 5),
    ).catchError((Object _) {
      fail('start: expected $want, got ${screen()}');
    });
    if (want.$1 == OnboardingStep.pending) {
      expect(state().code, code, reason: 'E4 shows the code of the device');
      expect(state().deadline, isNotNull);
    }

    // Afterwards.
    if (later == Later.comesOnline) {
      net.network = Network.ok;
      while (f.server.faults.pending.isNotEmpty) {
        f.server.faults.take('getDevice');
      }
    }
    if (later == Later.comesOnline && want.$1 == OnboardingStep.pending) {
      await other!.confirm(deviceId!, code!);
      await eventually(
        'main view after the confirmation',
        () => state().step == OnboardingStep.signedIn,
      ).catchError((Object _) {
        fail('after coming online: expected the main view, got ${screen()}');
      });
      return;
    }
    await Future<void>.delayed(const Duration(milliseconds: 300));
    expect(screen(), want, reason: 'the screen holds ($later)');
    if (want.$1 == OnboardingStep.pending) expect(state().code, code);
  }

  group('start routing: stored x network x later', () {
    for (final stored in Stored.values) {
      for (final network in Network.values) {
        for (final later in Later.values) {
          test(
            '${stored.name} x ${network.name} x ${later.name} -> '
            '${expected(stored, network)}',
            () => run(stored, network, later),
          );
        }
      }
    }
  });

  group('regressions of the delta red team (rt-M4-18)', () {
    for (final network in [
      Network.badGateway502,
      Network.unavailable503,
      Network.gatewayTimeout504,
    ]) {
      test(
        'a signed-in person behind a proxy answering ${network.status} '
        'starts in the main view',
        () => run(Stored.signedIn, network, Later.staysOffline),
      );
    }
    test(
      'a waiting device started offline shows E4 with its code and signs in '
      'once confirmed after the network returns',
      () => run(Stored.pending, Network.noAnswer, Later.comesOnline),
    );
  });
}
