// SPDX-License-Identifier: Apache-2.0

// The client stack of the terminal for the desktop E2E run and the performance level (brief
// M4-38): what the Go helper of services/server/test/terminal/clientstack wrote (the stack of
// M2-48 with a TLS front, a test CA and one host container), handed to the build as the define
// `TERMINAL_STACK` (base64 of the JSON, tool/e2e_suite.sh), because a desktop test app runs in its
// own sandbox and cannot read the helper's file. The control endpoint of the helper runs the
// pairing step in the host container and the canary scan of the stack (mode e2e).

import 'dart:convert';
import 'dart:io';

import 'e2e_world.dart';

/// The generator profiles of the host (clientstack/termgen, brief M4-38 Design note 2).
final class TerminalGenerator {
  TerminalGenerator._(Map<String, Object?> j)
    : rateProfile = j['rate_profile']! as String,
      rateLines = j['rate_lines']! as int,
      rateInterval = Duration(milliseconds: j['rate_interval_ms']! as int),
      burstProfile = j['burst_profile']! as String,
      burstChunkBytes = j['burst_chunk_bytes']! as int,
      burstChunks = j['burst_chunks']! as int;

  /// The profile that writes one numbered line per [rateInterval], [rateLines] lines.
  final String rateProfile;

  /// The number of lines of [rateProfile].
  final int rateLines;

  /// The time between two lines of [rateProfile].
  final Duration rateInterval;

  /// The profile that writes [burstChunks] chunks of [burstChunkBytes] bytes, one per line feed.
  final String burstProfile;

  /// The bytes of one chunk of [burstProfile].
  final int burstChunkBytes;

  /// The number of chunks of [burstProfile].
  final int burstChunks;
}

/// The answer of the pairing step: the fingerprint `bothy-host pair` printed and its exit code.
typedef PairOutcome = ({String fingerprint, int exitCode});

/// The connection data of the client stack.
final class TerminalStack implements E2eTarget {
  TerminalStack._(Map<String, Object?> j)
    : mode = j['mode']! as String,
      server = Uri.parse(j['origin']! as String),
      publishedPort = j['published_port']! as int,
      caPem = utf8.encode(j['ca_certificate_pem']! as String),
      instanceId = j['instance_id']! as String,
      hostId = j['host_id'] as String?,
      alexId = j['alex_id']! as String,
      alexCode = j['alex_enrollment_code']! as String,
      sessionUser = j['session_user']! as String,
      controlUrl = switch (j['control_url']) {
        final String u => Uri.parse(u),
        _ => null,
      },
      controlToken = j['control_token'] as String?,
      canary = j['canary'] as String?,
      generator = switch (j['generator']) {
        final Map<String, Object?> g => TerminalGenerator._(g),
        _ => null,
      };

  /// Parses the JSON of the helper.
  factory TerminalStack.parse(String json) =>
      TerminalStack._((jsonDecode(json) as Map).cast<String, Object?>());

  /// The stack of the define `TERMINAL_STACK`; throws a [StateError] naming the runner when it
  /// is missing (P-09: a run without a stack fails, it is never skipped).
  factory TerminalStack.fromEnvironment() {
    const encoded = String.fromEnvironment('TERMINAL_STACK');
    if (encoded.isEmpty) {
      throw StateError(
        'TERMINAL_STACK is not defined; run the terminal suite with make e2e-desktop or '
        'make perf-desktop (tools/ci/terminal_stack.py), never on its own',
      );
    }
    return TerminalStack.parse(utf8.decode(base64.decode(encoded)));
  }

  /// `e2e` or `perf`.
  final String mode;

  /// The instance origin the app types, `https://chat.example.com:8443`.
  @override
  final Uri server;

  /// The port of the TLS front on 127.0.0.1 that serves [server].
  final int publishedPort;

  /// The PEM of the test CA of the stack.
  final List<int> caPem;

  @override
  final String instanceId;

  /// The paired host (mode perf), or null (mode e2e: the test creates it).
  final String? hostId;

  /// Alex, an owner.
  final String alexId;

  /// The first one-time enrollment code of Alex (the M4-36 stack test); the suites ask for a
  /// fresh one with [enrollmentCode].
  final String alexCode;

  /// The session user of Alex on the host.
  final String sessionUser;

  /// The control endpoint on 127.0.0.1 (modes e2e and perf).
  final Uri? controlUrl;

  /// The bearer token of [controlUrl].
  final String? controlToken;

  /// The terminal content the E2E run types and the scans look for (mode e2e).
  final String? canary;

  /// The generator profiles.
  final TerminalGenerator? generator;

  /// Runs `bothy-host pair` with [code] in the host container of the stack and returns what it
  /// printed.
  Future<PairOutcome> pair(String code) async {
    final j = await _control('pair', {'code': code});
    return (
      fingerprint: j['fingerprint']! as String,
      exitCode: j['exit_code']! as int,
    );
  }

  /// A fresh enrollment code of Alex, issued by the case's Owner right now: only the newest code
  /// of a person is valid (08 3.2), so every run of a suite on the stack asks for its own.
  Future<String> enrollmentCode() async =>
      (await _control('enrollment-code', const {}))['code']! as String;

  /// The findings of the canary scan of the stack's logs, database dump, collector export and
  /// host journal; empty when the canary is nowhere.
  Future<List<String>> canaryFindings() async {
    final j = await _control('canary-check', const {});
    return [for (final f in j['findings']! as List) f as String];
  }

  Future<Map<String, Object?>> _control(
    String path,
    Map<String, Object?> body,
  ) => HttpOverrides.runWithHttpOverrides(() async {
    final control = controlUrl;
    if (control == null) throw StateError('the stack has no control endpoint');
    // An Android emulator reaches the loopback of its host at another address (brief M5-12).
    const hostLoopback = String.fromEnvironment('TERMINAL_STACK_HOST_LOOPBACK');
    final url = hostLoopback.isEmpty
        ? control
        : control.replace(host: hostLoopback);
    final client = HttpClient();
    try {
      final request = await client.postUrl(url.resolve('/$path'));
      request.headers
        ..set(HttpHeaders.authorizationHeader, 'Bearer $controlToken')
        ..contentType = ContentType.json;
      request.write(jsonEncode(body));
      final response = await request.close();
      final text = await utf8.decodeStream(response);
      if (response.statusCode != HttpStatus.ok) {
        throw StateError('control $path: ${response.statusCode} $text');
      }
      return (jsonDecode(text) as Map).cast<String, Object?>();
    } finally {
      client.close(force: true);
    }
  }, _Plain());
}

/// The default `HttpClient`: the control endpoint is plain HTTP on 127.0.0.1, outside the tunnel
/// and the test CA that the app's network uses.
final class _Plain extends HttpOverrides {}
