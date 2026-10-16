// SPDX-License-Identifier: Apache-2.0

/// The ports of the gateway connection (15 7, brief M4-13 Scope): the WebSocket transport,
/// network status, app lifecycle and randomness. Time comes through the `Clock` port of
/// `auth` (M4-09).
///
/// The apps wire the production implementations (M4-18): `IoGatewayTransport` for the
/// transport, the platform's connectivity and lifecycle for [NetworkStatus] and
/// [AppLifecycle]; tests use fakes, so every flow runs without I/O (15 1 principle 5).
library;

import 'dart:math';

/// Opens WebSocket connections to the gateway.
abstract interface class GatewayTransport {
  /// Opens a connection to [url] (`wss://<host>/api/v1/gateway`, 04 6.1).
  ///
  /// Completes when the upgrade succeeded; throws on any failure (DNS, TCP, TLS, HTTP status),
  /// which the connection treats like a drop and answers with backoff (04 6.5).
  Future<GatewayChannel> connect(Uri url);
}

/// One open WebSocket of the gateway: text frames in both directions (04 2.3).
abstract interface class GatewayChannel {
  /// The text frames of the server, in order; the stream is done when the socket closed,
  /// from either side.
  Stream<String> get frames;

  /// Sends one text frame; the client sends only small frames, the server enforces
  /// `sync.ws_max_frame_bytes` with `4002` (04 6.4).
  void send(String text);

  /// Closes the socket with [code] and an optional [reason].
  Future<void> close(int code, [String? reason]);

  /// The close code once [frames] is done, or `null` when the socket dropped without one.
  int? get closeCode;

  /// The close reason once [frames] is done, or `null`.
  String? get closeReason;
}

/// Whether the device has a network, and every change (also a switch between networks, which
/// reports the same value again).
abstract interface class NetworkStatus {
  /// One value per change; `true` while a network is available.
  Stream<bool> get changes;

  /// Whether a network is available now.
  bool get online;
}

/// Whether the app is in the foreground.
enum AppActivity {
  /// Visible and active.
  foreground,

  /// In the background (mobile: no WebSocket, 15 7.2 `Suspended`).
  background,
}

/// The lifecycle of the app.
abstract interface class AppLifecycle {
  /// One value per change.
  Stream<AppActivity> get changes;

  /// The activity now.
  AppActivity get current;
}

/// Uniform random numbers for jitter (04 6.3 item 5, 04 6.5).
abstract interface class RandomSource {
  /// A value in `[0, 1)`.
  double nextDouble();
}

/// The [RandomSource] of the running app, on `Random.secure()`, so devices that lost the
/// server together do not draw the same delays.
final class SystemRandomSource implements RandomSource {
  /// Creates the source.
  SystemRandomSource() : _random = Random.secure();

  final Random _random;

  @override
  double nextDouble() => _random.nextDouble();
}
