// SPDX-License-Identifier: Apache-2.0

// The in-process host behind the fake server's stream endpoint (brief M4-36 Scope): every
// device stream that passed `identify` is relayed to `FakeHost` of
// `package:terminal_ui/testing.dart`, which checks the session token of Noise message 1 and
// answers the handshake; the fake server stays a relay that reads nothing.

import 'dart:async';
import 'dart:typed_data';

import 'package:sync_engine/testing.dart';
import 'package:terminal_ui/testing.dart' as ui;

/// Connects the stream endpoint of a fake server to an in-process [ui.FakeHost].
final class HostBridge implements FakeTerminalHostLink {
  /// Creates the bridge to [host].
  HostBridge(this.host) {
    // The fake server sends `terminal.hello` itself, with its DPoP nonce.
    host.sendHello = false;
  }

  /// The host the next stream goes to; a test swaps it for a host with another key.
  ui.FakeHost host;

  /// The host ends of the streams, in order.
  final List<FakeTerminalPipe> pipes = [];

  @override
  Future<void> connect(FakeTerminalPipe pipe) async {
    final target = host..sendHello = false;
    pipes.add(pipe);
    target.expectToken(pipe.sessionId, pipe.sessionToken);
    final channel = await ui.FakeTerminalTransport(target)
        .connect('/api/v1/terminal/sessions/${pipe.sessionId}/stream');
    channel.messages.listen(
      (m) {
        if (m is Uint8List) pipe.toDevice(m);
      },
      onDone: () {
        final code = channel.closeCode;
        if (code != null) pipe.close(code, channel.closeReason);
      },
    );
    pipe.fromDevice.listen((bytes) {
      try {
        channel.sendBinary(bytes);
      } on StateError {
        // The host closed this stream already; later device messages go nowhere.
      }
    });
    unawaited(pipe.done.then((_) => channel.close()));
  }
}
