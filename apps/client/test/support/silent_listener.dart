// SPDX-License-Identifier: Apache-2.0

// A network that drops the answers, for the headless flows against the fake server.

import 'dart:io';

import 'package:sync_engine/testing.dart';

/// A listener at the address of a stopped fake that accepts every connection and never
/// answers, as a network does that drops the answers (a VPN or tailnet that is off). The
/// caller closes it before it stops the fake instance.
final class SilentListener {
  SilentListener._(this._server);

  /// Stops [server] and listens at its address instead.
  static Future<SilentListener> replace(FakeServer server) async {
    final port = server.baseUri.port;
    await server.stop();
    final listener = SilentListener._(
      await ServerSocket.bind(InternetAddress.loopbackIPv4, port),
    );
    listener._server.listen(listener._held.add);
    return listener;
  }

  final ServerSocket _server;
  final List<Socket> _held = [];

  /// How many connections it accepted so far.
  int get connections => _held.length;

  /// Drops every connection and stops listening.
  Future<void> close() async {
    for (final socket in _held) {
      socket.destroy();
    }
    _held.clear();
    await _server.close();
  }
}
