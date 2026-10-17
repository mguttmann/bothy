// SPDX-License-Identifier: Apache-2.0

/// The ports through which the console reaches the browser, and the environment that bundles
/// them. The web build uses `browser_web.dart` (package:web); every other platform, the test VM
/// included, gets `browser_stub.dart`, which never touches the network.
library;

import 'package:dio/dio.dart';

import 'browser_stub.dart'
    if (dart.library.js_interop) 'browser_web.dart'
    as impl;

/// The tab's `sessionStorage`: the only place the console keeps the `client_verifier` of an
/// OIDC flow across the redirect (M6-01 Design note 1). Never `localStorage`, a cookie or the URL.
abstract interface class FlowStore {
  /// The value of [key], or null.
  String? read(String key);

  /// Stores [value] under [key].
  void write(String key, String value);

  /// Removes [key]; a missing key is no error.
  void delete(String key);
}

/// Navigation of the tab outside the app.
abstract interface class BrowserNavigation {
  /// Sends the tab to [url] (the authorization URL of the login service); the app unloads.
  void assign(Uri url);

  /// Reloads the page (after a version mismatch).
  void reload();

  /// Sets the `lang` attribute of the HTML document to [tag], so the semantics tree is read in
  /// the app language (brief M6-02 Design note 3).
  void setDocumentLanguage(String tag);
}

/// The browser's WebAuthn API (`navigator.credentials.get`).
abstract interface class PasskeyPort {
  /// Asks the browser for an assertion over [options], the `PublicKeyCredentialRequestOptions`
  /// in JSON form of the server (binary members as base64url), always with
  /// `userVerification: required`. Returns the `PublicKeyCredential` in JSON form.
  ///
  /// Throws [PasskeyCancelled] when the person cancels or no passkey answers.
  Future<Map<String, Object?>> get(Map<String, Object?> options);
}

/// The person cancelled the passkey prompt, or the browser offered no passkey.
class PasskeyCancelled implements Exception {
  /// Creates the error.
  const PasskeyCancelled();

  @override
  String toString() => 'PasskeyCancelled';
}

/// Everything the console needs from its host: the origin it is served from and the ports.
class ConsoleEnvironment {
  /// Creates the environment from its parts; [adapter] replaces the browser's HTTP stack in
  /// tests.
  const ConsoleEnvironment({
    required this.origin,
    required this.flowStore,
    required this.navigation,
    required this.passkeys,
    this.adapter,
  });

  /// The environment of the page the console runs in (the web build), or an inert one on any
  /// other platform, whose requests all fail without network.
  factory ConsoleEnvironment.browser() => impl.browserEnvironment();

  /// Origin of the page, which is also the origin of the API (same origin, 08 7.5).
  final Uri origin;

  /// The tab's sessionStorage.
  final FlowStore flowStore;

  /// Navigation of the tab.
  final BrowserNavigation navigation;

  /// The WebAuthn API.
  final PasskeyPort passkeys;

  /// The HTTP adapter; null means the browser's own (cookies of the same origin go along).
  final HttpClientAdapter? adapter;
}
