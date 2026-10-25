// SPDX-License-Identifier: Apache-2.0

// The app's terminal against the real server and a real host service in a container (brief
// M4-36 Design note 6; make terminal-stack): the Go helper of services/server/test/terminal/
// clientstack starts the stack of M2-48 and writes its connection data to the file that
// BOTHY_CLIENTSTACK_OUT names; this test runs the app's adapters and the TerminalController
// headless against it: open with the step-up, run a command and read its output, detach,
// reattach with the scrollback replayed, an attach from a second device (4103 here) and a
// revoked grant (4100 with access_revoked).
//
// It lives outside integration_test/, so no app build runs it, and this entry point alone
// installs the trust of the test CA and maps the instance origin to the published port of the
// TLS front (an HttpOverrides for the REST client and the WebSockets); production code has no
// such hook. Without BOTHY_CLIENTSTACK_OUT the test is skipped.

import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:math';

import 'package:api_client/api_client.dart' hide File;
import 'package:auth/auth.dart';
import 'package:client/terminal/pin_store_adapter.dart';
import 'package:client/terminal/terminal_api_adapter.dart';
import 'package:client/terminal/terminal_transport_io.dart';
import 'package:dio/dio.dart' show Interceptor;
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:terminal_ui/terminal_ui.dart';

import '../test/support/software_keys.dart';

/// How long one step of the stack may take on a loaded machine (the only bound of a wait).
const Duration patience = Duration(minutes: 2);

/// The connection data the Go helper wrote.
final class _Conn {
  _Conn(Map<String, Object?> j)
    : origin = Uri.parse(j['origin']! as String),
      instanceHost = j['instance_host']! as String,
      port = j['published_port']! as int,
      ca = utf8.encode(j['ca_certificate_pem']! as String),
      hostId = j['host_id']! as String,
      grantId = j['grant_id']! as String,
      alexId = j['alex_id']! as String,
      alexCode = j['alex_enrollment_code']! as String,
      ownerCode = j['owner_enrollment_code']! as String;

  final Uri origin;
  final String instanceHost;
  final int port;
  final List<int> ca;
  final String hostId;
  final String grantId;
  final String alexId;
  final String alexCode;
  final String ownerCode;
}

/// Trusts the test CA and dials the published port for the instance origin, keeping the TLS
/// name of the origin (the same mapping as the Go dialer of M2-48).
final class _StackOverrides extends HttpOverrides {
  _StackOverrides(this.conn);

  final _Conn conn;

  @override
  HttpClient createHttpClient(SecurityContext? context) {
    final trust = SecurityContext(withTrustedRoots: false)
      ..setTrustedCertificatesBytes(conn.ca);
    return super.createHttpClient(trust)
      ..connectionFactory = (uri, proxyHost, proxyPort) async {
        final mapped =
            uri.host == conn.instanceHost && uri.port == conn.origin.port;
        final task = await Socket.startConnect(
          mapped ? InternetAddress.loopbackIPv4 : uri.host,
          mapped ? conn.port : uri.port,
        );
        final secure = task.socket.then(
          (s) => SecureSocket.secure(s, host: uri.host, context: trust),
        );
        return ConnectionTask.fromSocket(secure, task.cancel);
      };
  }
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

final class _Yes implements StepUpPrompt {
  const _Yes();

  @override
  Future<bool> ask(String purposeText) async => true;
}

/// Answers every question of the controller with yes and the preselected choice.
final class _Prompts implements TerminalPrompts {
  @override
  Future<bool> confirmKeyChange(Host host, PinChanged change) async => true;

  @override
  Future<bool> confirmForeignHost(Host host) async => true;

  @override
  Future<OpenChoice?> confirmOpen(Host host, OpenChoice initial) async =>
      initial;

  @override
  String stepUpPurpose(Host host, StepUpPurpose purpose) => host.name;
}

final class _Links implements LinkOpener {
  @override
  Future<void> open(Uri target) async {}
}

/// One enrolled device: its auth session and the terminal's REST client without the
/// interceptor that answers step-ups itself (as the app builds it).
final class _Device {
  _Device(_Conn conn, String name) {
    final random = Random(name.hashCode);
    ApiClient client(List<Interceptor> interceptors) => createApiClient(
      baseUrl: conn.origin,
      traceContext: () => TraceContext(traceId: '3' * 32, sampled: false),
      readRetry: ReadRetryPolicy(
        base: Duration.zero,
        cap: Duration.zero,
        maxAttempts: 1,
      ),
      authInterceptors: interceptors,
    );
    auth = AuthSession(
      serverUrl: conn.origin,
      keys: DeviceKeys(
        dpop: SoftwareP256Signer.seeded('$name/dpop'),
        stepUp: SoftwareP256Signer.seeded('$name/step_up'),
      ),
      secrets: _Secrets(),
      clock: const SystemClock(),
      ids: Uuid7Generator(
        nowMillis: () => DateTime.now().millisecondsSinceEpoch,
        random: (bits) => random.nextInt(1 << (bits > 31 ? 31 : bits)),
      ),
      config: AppConfig.defaults(),
      device: DeviceDescription(name: name, platform: 'macos'),
      stepUpPrompt: const _Yes(),
      buildClient: client,
    );
    terminalApi = ApiTerminalServerApi(
      client([
        for (final i in auth.interceptors)
          if (i is! StepUpInterceptor) i,
      ]),
      closeReason: 'Stack test',
    );
  }

  late final AuthSession auth;
  late final ApiTerminalServerApi terminalApi;

  Future<void> enroll(String code) async {
    final outcome = await auth.login.redeemEnrollmentCode(code);
    if (outcome is! LoginSignedIn) throw StateError('enrollment: $outcome');
  }
}

Future<void> _until(
  String what,
  bool Function() condition, [
  Object? Function()? state,
]) async {
  final end = DateTime.now().add(patience);
  while (!condition()) {
    if (DateTime.now().isAfter(end)) {
      throw StateError('timed out: $what (${state?.call()})');
    }
    await Future<void>.delayed(const Duration(milliseconds: 20));
  }
}

void main() {
  final path = Platform.environment['BOTHY_CLIENTSTACK_OUT'];
  test(
    'open, run, detach, reattach, another device and a revocation against the stack',
    () async {
      final conn = _Conn(
        (jsonDecode(File(path!).readAsStringSync()) as Map)
            .cast<String, Object?>(),
      );
      HttpOverrides.global = _StackOverrides(conn);
      addTearDown(() => HttpOverrides.global = null);
      final alex = _Device(conn, 'alex-laptop');
      final phone = _Device(conn, 'alex-phone');
      final owner = _Device(conn, 'owner-laptop');
      await alex.enroll(conn.alexCode);
      await owner.enroll(conn.ownerCode);
      // Only the newest code of a person is valid (08 3.2): the Owner creates the code of
      // Alex's second device after the first one was redeemed.
      final second = await owner.auth.api.getUsersApi().createEnrollmentCode(
        userId: conn.alexId,
      );
      await phone.enroll(second.data!.code);
      final db = openInMemoryForTests();
      addTearDown(db.close);
      final controller = TerminalController(
        api: alex.terminalApi,
        transport: IoTerminalTransport(origin: conn.origin, auth: alex.auth),
        pins: LocalHostKeyPinStore(db.hostKeyPinsDao),
        stepUp: alex.auth.stepUp.confirm,
        links: _Links(),
        clock: const SystemClock(),
        prompts: _Prompts(),
        selfUserId: conn.alexId,
        config: AppConfig.defaults(),
        scheduleTick: scheduleMicrotask,
      );
      addTearDown(controller.dispose);
      bool phase(TerminalPhase p) => controller.state.phase == p;
      Object state() => controller.state;

      await controller.open(
        host: HostId.parse(conn.hostId),
        profile: 'shell',
        cols: 80,
        rows: 24,
      );
      await _until('connected', () => phase(TerminalPhase.connected), state);
      expect((await db.hostKeyPinsDao.all()).single.hostId.value, conn.hostId);
      controller.input(utf8.encode("printf 'stack-%s\\n' ok\r"));
      await _until(
        'the output of the command',
        () => controller.screen.visibleText().contains('stack-ok'),
      );

      await controller.detach();
      expect(controller.state.phase, TerminalPhase.detached);
      await controller.attach();
      await _until('reattached', () => phase(TerminalPhase.connected), state);
      await _until(
        'the replayed scrollback',
        () => controller.screen.visibleText().contains('stack-ok'),
      );

      // A second device of Alex attaches: this stream ends with 4103.
      await phone.auth.api.getTerminalApi().attachTerminalSession(
        sessionId: controller.state.sessionId!,
        terminalSessionAttach: TerminalSessionAttach(
          (b) => b
            ..cols = 80
            ..rows = 24
            ..clientEphemeralKeyHash = 'A' * 43,
        ),
      );
      await _until(
        'replaced',
        () => controller.state.detachCause == DetachCause.replaced,
      );

      // Back on this device, then the grant is revoked: 4100 with access_revoked.
      await controller.attach();
      await _until('attached again', () => phase(TerminalPhase.connected), state);
      await owner.auth.api.getAccessApi().revokeGrant(
        grantId: conn.grantId,
        reasonBody: ReasonBody((b) => b.reason = 'Stack test'),
      );
      await _until('revoked', () => phase(TerminalPhase.blocked), state);
      expect(controller.state.blockCause, BlockCause.revoked);
      expect(controller.state.closeReason, 'access_revoked');
      for (final d in [alex, phone, owner]) {
        await d.auth.dispose();
      }
    },
    skip: path == null
        ? 'BOTHY_CLIENTSTACK_OUT is not set: run make terminal-stack'
        : false,
    timeout: Timeout.none,
  );
}
