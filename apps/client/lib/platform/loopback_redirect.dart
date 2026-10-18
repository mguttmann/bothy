// SPDX-License-Identifier: Apache-2.0

/// The OIDC redirect of the desktop: a loopback listener on `127.0.0.1` with a free port
/// (docs/spec/15-client-architecture.md 8, RFC 8252 7.3).
library;

import 'dart:async';
import 'dart:io';

import 'package:auth/auth.dart';

/// [RedirectReceiver] on a short-lived HTTP listener bound to `127.0.0.1` port 0.
///
/// [redirectUri] starts listening and returns `http://127.0.0.1:<port>/callback`;
/// [waitForRedirect] answers the first request on that path with [doneText] (a plain page the
/// browser shows) and closes the listener. Any other path gets `404`. Only the loopback
/// interface is bound, so no other machine can reach it.
///
/// Example:
/// ```dart
/// final receiver = LoopbackRedirectReceiver(doneText: () => l.onboardingLoopbackDone);
/// final uri = await receiver.redirectUri();
/// final redirect = await receiver.waitForRedirect(uri, const Duration(minutes: 5));
/// ```
final class LoopbackRedirectReceiver implements RedirectReceiver {
  /// Creates the receiver; [doneText] is the text of the page after the redirect.
  LoopbackRedirectReceiver({required String Function() doneText})
    : _doneText = doneText; // ignore: prefer_initializing_formals

  final String Function() _doneText;
  HttpServer? _server;

  /// The path of the redirect.
  static const String callbackPath = '/callback';

  @override
  Future<Uri> redirectUri() async {
    await _server?.close(force: true);
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    _server = server;
    return Uri(
      scheme: 'http',
      host: InternetAddress.loopbackIPv4.address,
      port: server.port,
      path: callbackPath,
    );
  }

  @override
  Future<Uri> waitForRedirect(Uri expectedPrefix, Duration timeout) async {
    final server = _server;
    if (server == null) throw StateError('redirectUri was not called');
    final completer = Completer<Uri>();
    final subscription = server.listen((request) async {
      final matches =
          !completer.isCompleted && request.uri.path == expectedPrefix.path;
      request.response
        ..statusCode = matches ? HttpStatus.ok : HttpStatus.notFound
        ..headers.contentType = ContentType.text;
      if (matches) request.response.write(_doneText());
      await request.response.close();
      if (matches) {
        completer.complete(expectedPrefix.replace(query: request.uri.query));
      }
    });
    try {
      return await completer.future.timeout(timeout);
    } finally {
      await subscription.cancel();
      await server.close(force: true);
      if (identical(_server, server)) _server = null;
    }
  }
}
