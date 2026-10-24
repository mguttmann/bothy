// SPDX-License-Identifier: Apache-2.0

/// Fakes of the console's terminal tests: a WebSocket and an IndexedDB in memory behind the
/// seams of `lib/terminal/ports.dart`, and the answers of the host and terminal operations for
/// "Build host". No test opens a socket or a database.
library;

import 'dart:async';
import 'dart:typed_data';

import 'package:console/console.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../support/fakes.dart';

/// The host of the tests.
const buildHostId = '0192f4a0-7b1c-7a11-8c3e-4f5a6b7c8d9e';

/// The terminal session of the tests.
const terminalSessionId = '0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e0f';

/// The device of the console session, as the grant names it.
const consoleDeviceId = '0192f4a0-7b1c-7848-8501-1716705c7dbe';

/// The stream path of [terminalSessionId].
const streamPath = '/api/v1/terminal/sessions/$terminalSessionId/stream';

/// A static X25519 key in the form of `Host.noise_public_key`.
const hostNoiseKey = 'Zm9vYmFyZm9vYmFyZm9vYmFyZm9vYmFyZm9vYmFyMTI';

/// The fingerprint of [hostNoiseKey].
const hostFingerprint =
    '7K2Q-M9XD-0000-0000-0000-0000-0000-0000-0000-0000-0000-0000-0000';

/// A Host body of "Build host" with the profile `shell`.
Map<String, Object?> buildHost() => {
  'id': buildHostId,
  'name': 'Build host',
  'state': 'active',
  'llm_credentials': 'organization',
  'declared_server_host': false,
  'noise_public_key': hostNoiseKey,
  'key_fingerprint': hostFingerprint,
  'key_generation': 1,
  'profiles': [
    {'name': 'shell', 'display_name': 'Shell', 'kind': 'shell'},
  ],
  'doctor': <Object?>[],
  'connected': true,
  'created_at': '2026-10-10T09:30:00.000Z',
  'version': 1,
};

/// A TerminalSessionGrant body for [terminalSessionId] issued to [deviceId].
Map<String, Object?> grantBody({String deviceId = consoleDeviceId}) => {
  'session': {
    'id': terminalSessionId,
    'host_id': buildHostId,
    'user_id': '0192f4a0-7b1c-7f89-8d6b-6960453241bc',
    'device_id': deviceId,
    'profile': 'shell',
    'state': 'opening',
    'opened_at': '2026-10-10T09:31:00.000Z',
    'bytes_in': 0,
    'bytes_out': 0,
    'version': 1,
  },
  'session_token': 'token-1',
  'session_token_expires_at': '2026-10-10T09:31:30.000Z',
  'stream_path': streamPath,
  'noise_public_key': hostNoiseKey,
  'key_fingerprint': hostFingerprint,
  'key_generation': 1,
  'noise_protocol': 'Noise_NK_25519_ChaChaPoly_SHA256',
  'prologue_version': 'bothy-terminal/1',
};

/// Adds the host and terminal operations of "Build host" to [server].
void terminalRoutes(FakeServer server) {
  server.routes['GET /api/v1/hosts'] = (_) => Answer(200, {
    'items': [buildHost()],
  });
  server.routes['GET /api/v1/hosts/$buildHostId'] = (_) =>
      Answer(200, buildHost());
  server.routes['POST /api/v1/hosts/$buildHostId/terminal/sessions'] = (_) =>
      Answer(201, grantBody());
  server.routes['POST /api/v1/terminal/sessions/$terminalSessionId/attach'] = (
    _,
  ) => Answer(200, grantBody());
  server.routes['POST /api/v1/terminal/sessions/$terminalSessionId/close'] = (
    _,
  ) => Answer(200, grantBody()['session']);
}

/// One message the console put on a [FakeSocket].
sealed class SentMessage {
  const SentMessage();
}

/// A text message.
final class SentText extends SentMessage {
  const SentText(this.text);

  /// The text.
  final String text;
}

/// A binary message.
final class SentBinary extends SentMessage {
  const SentBinary(this.bytes);

  /// The bytes.
  final Uint8List bytes;
}

/// A WebSocket in memory: records what the console sends, delivers what the test pushes.
final class FakeSocket implements TerminalSocket {
  /// Creates the socket of [url].
  FakeSocket(this.url);

  /// The URL the console connected to.
  final Uri url;

  /// Every message the console sent, in order.
  final List<SentMessage> sent = [];

  final StreamController<Object> _incoming = StreamController<Object>();

  /// The code the console closed with, null while open.
  int? closedWith;

  @override
  int? closeCode;

  @override
  String? closeReason;

  @override
  Stream<Object> get messages => _incoming.stream;

  /// Delivers [message] (a String or a Uint8List) to the console.
  void push(Object message) => _incoming.add(message);

  @override
  void sendText(String text) => sent.add(SentText(text));

  @override
  void sendBinary(Uint8List bytes) => sent.add(SentBinary(bytes));

  @override
  Future<void> close([int code = 1000]) async {
    closedWith ??= code;
    if (!_incoming.isClosed) await _incoming.close();
  }
}

/// Connects every stream to a new [FakeSocket] and keeps them.
final class FakeSockets {
  /// The sockets in the order they were opened.
  final List<FakeSocket> opened = [];

  /// The connector for the transport.
  Future<TerminalSocket> connect(Uri url) async {
    final socket = FakeSocket(url);
    opened.add(socket);
    return socket;
  }
}

/// An IndexedDB of the origin in memory: one record per host, and whether the database was
/// deleted.
class FakePinDatabase implements PinDatabase {
  /// The stored records by host ID.
  final Map<String, Map<String, Object?>> records = {};

  /// How often the database was deleted.
  int deletions = 0;

  /// When set, [delete] waits for it, so a test sees the order of deletion and E7.
  Completer<void>? deleteGate;

  @override
  Future<Map<String, Object?>?> read(String hostId) async => records[hostId];

  @override
  Future<void> write(String hostId, Map<String, Object?> record) async =>
      records[hostId] = Map.of(record);

  @override
  Future<void> delete() async {
    await deleteGate?.future;
    records.clear();
    deletions++;
  }
}

/// Records how often the passkey setup notice was shown.
final class FakeSetupNotice {
  /// The number of notices.
  int shown = 0;

  /// The notice callback.
  Future<void> show() async => shown++;
}

/// A [ConsoleStepUpPrompt] that records the ways it was offered and answers [answer].
final class FakeStepUpPrompt implements ConsoleStepUpPrompt {
  /// Creates the prompt.
  FakeStepUpPrompt([this.answer = const StepUpWithPasskey()]);

  /// The answer of every question.
  ConsoleStepUpAnswer? answer;

  /// The purposes and ways of every question.
  final List<(String, ConsoleStepUpWays)> asked = [];

  @override
  Future<ConsoleStepUpAnswer?> ask(
    String purpose,
    ConsoleStepUpWays ways,
  ) async {
    asked.add((purpose, ways));
    return answer;
  }
}

/// An app frame with the theme of the design system, [locale] and the navigator [key], for the
/// dialogs of the terminal.
Widget dsApp(
  GlobalKey<NavigatorState> key, {
  Locale locale = const Locale('en'),
}) {
  const ds = DsContext(
    theme: DsThemeMode.light,
    density: DsDensityMode.comfortable,
    motion: DsMotionMode.reduced,
    accent: dsDefaultAccent,
  );
  return MaterialApp(
    navigatorKey: key,
    locale: locale,
    localizationsDelegates: AppLocalizations.localizationsDelegates,
    supportedLocales: AppLocalizations.supportedLocales,
    theme: buildDsTheme(ds),
    builder: (context, app) => DsThemeScope(dsContext: ds, child: app!),
    home: const Scaffold(body: SizedBox.shrink()),
  );
}
