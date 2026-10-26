// SPDX-License-Identifier: Apache-2.0

// The network of the desktop E2E app (brief M4-34 Design notes 1 and 2): every HTTP and WebSocket
// connection of the process trusts only the test CA of the run and goes through an HTTP CONNECT
// tunnel on 127.0.0.1 that the test can cut and open again. Cutting it is "the network of the
// app's host side is gone": open connections break, new ones are refused, and the server stays
// up. The address the app uses does not change, so the DPoP proofs keep naming
// server.public_url (08 7.3).

import 'dart:async';
import 'dart:convert';
import 'dart:io';

/// The tunnel and the TLS trust of the run; [install] makes them the default of `dart:io`.
final class E2eNetwork {
  E2eNetwork._(this._server, this._target, this._upstreamPort, this._context);

  /// Starts the tunnel for [target] (the host and port of the TLS front) on 127.0.0.1 port 0,
  /// with a TLS context that trusts only [caPem]. The tunnel connects to 127.0.0.1 at
  /// [upstreamPort], by default the port of [target]: the terminal stack of brief M4-38 serves
  /// the origin `https://chat.example.com:8443` from another published port, and the app keeps
  /// the origin, so its DPoP proofs and the TLS name stay those of server.public_url.
  static Future<E2eNetwork> start(
    Uri target,
    List<int> caPem, {
    int? upstreamPort,
  }) async {
    final server = await ServerSocket.bind(InternetAddress.loopbackIPv4, 0);
    final context = SecurityContext(withTrustedRoots: false)
      ..setTrustedCertificatesBytes(caPem);
    final network = E2eNetwork._(
      server,
      target,
      upstreamPort ?? target.port,
      context,
    );
    server.listen(
      (client) => unawaited(network._accept(client)),
      // A refused accept is no failure of a test: the next connection is accepted again.
      onError: (Object _) {},
    );
    return network;
  }

  final ServerSocket _server;
  final Uri _target;
  final int _upstreamPort;
  final SecurityContext _context;
  final Set<Socket> _open = {};
  bool _online = true;

  /// Whether the tunnel accepts connections.
  bool get online => _online;

  /// The number of tunnels refused while the network was cut.
  int refused = 0;

  /// Makes every `HttpClient` of the process (dio, the WebSocket of the gateway) trust only the
  /// test CA and connect through the tunnel. Call it once, before the app starts.
  void install() => HttpOverrides.global = _Overrides(this);

  /// Runs [body] with the `HttpClient`s it creates trusting the test CA but connecting directly,
  /// not through the tunnel: the actors of the suite stand for other devices, whose network a
  /// [cut] of the app's network does not touch.
  Future<T> direct<T>(Future<T> Function() body) =>
      HttpOverrides.runWithHttpOverrides(body, _Direct(this));

  /// Cuts the network: closes every open tunnel and refuses new ones until [restore].
  void cut() {
    _online = false;
    for (final s in _open.toList()) {
      s.destroy();
    }
    _open.clear();
  }

  /// Opens the network again.
  void restore() => _online = true;

  /// Closes the listener and every tunnel.
  Future<void> stop() async {
    cut();
    await _server.close();
  }

  Future<void> _accept(Socket client) async {
    // A peer that resets its end makes the pending writes of that socket fail; a tunnel ends
    // with its sockets, which is no failure of a test.
    unawaited(client.done.then<void>((_) {}, onError: (Object _) {}));
    // The head of one CONNECT request, at most a few hundred bytes.
    final head = <int>[];
    late final StreamSubscription<List<int>> sub;
    final ready = Completer<List<int>?>();
    sub = client.listen(
      (chunk) {
        if (ready.isCompleted) return;
        head.addAll(chunk);
        final end = _headEnd(head);
        if (end >= 0) {
          sub.pause();
          ready.complete(head.sublist(end));
        }
      },
      onError: (Object _) {
        if (!ready.isCompleted) ready.complete(null);
      },
      onDone: () {
        if (!ready.isCompleted) ready.complete(null);
      },
      cancelOnError: true,
    );
    final rest = await ready.future;
    if (rest == null) {
      // The client went before it sent a whole request.
      client.destroy();
      return;
    }
    final line = ascii.decode(
      head.sublist(0, head.indexOf(13)),
      allowInvalid: true,
    );
    final want = 'CONNECT ${_target.host}:${_target.port} ';
    if (!_online || !line.startsWith(want)) {
      if (!_online) refused++;
      try {
        client.add(
          ascii.encode('HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\n\r\n'),
        );
      } on Object {
        // The client is gone already.
      }
      client.destroy();
      await sub.cancel();
      return;
    }
    final Socket upstream;
    try {
      upstream = await Socket.connect(
        InternetAddress.loopbackIPv4,
        _upstreamPort,
      );
    } on SocketException {
      client.destroy();
      await sub.cancel();
      return;
    }
    unawaited(upstream.done.then<void>((_) {}, onError: (Object _) {}));
    if (!_online) {
      upstream.destroy();
      client.destroy();
      await sub.cancel();
      return;
    }
    _open
      ..add(client)
      ..add(upstream);
    void close() {
      _open
        ..remove(client)
        ..remove(upstream);
      client.destroy();
      upstream.destroy();
    }

    // A write to a socket the other side or a cut closed fails; the tunnel then ends.
    void forward(Socket to, List<int> data) {
      try {
        to.add(data);
      } on Object {
        close();
      }
    }

    forward(
      client,
      ascii.encode('HTTP/1.1 200 Connection established\r\n\r\n'),
    );
    if (rest.isNotEmpty) forward(upstream, rest);
    sub
      ..onData((data) => forward(upstream, data))
      ..onDone(close)
      ..onError((Object _) => close())
      ..resume();
    upstream.listen(
      (data) => forward(client, data),
      onDone: close,
      onError: (Object _) => close(),
      cancelOnError: true,
    );
  }

  /// The index after the blank line that ends an HTTP head, or -1.
  static int _headEnd(List<int> bytes) {
    for (var i = 3; i < bytes.length; i++) {
      if (bytes[i - 3] == 13 &&
          bytes[i - 2] == 10 &&
          bytes[i - 1] == 13 &&
          bytes[i] == 10) {
        return i + 1;
      }
    }
    return -1;
  }
}

final class _Overrides extends HttpOverrides {
  _Overrides(this._network);

  final E2eNetwork _network;

  @override
  HttpClient createHttpClient(SecurityContext? context) {
    // The context of the caller is ignored on purpose: the stack has only the test CA.
    final client = super.createHttpClient(_network._context);
    final proxy = 'PROXY 127.0.0.1:${_network._server.port}';
    client.findProxy = (_) => proxy;
    return client;
  }
}

final class _Direct extends HttpOverrides {
  _Direct(this._network);

  final E2eNetwork _network;

  @override
  HttpClient createHttpClient(SecurityContext? context) =>
      super.createHttpClient(_network._context);
}
