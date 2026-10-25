// SPDX-License-Identifier: Apache-2.0

/// The ports through which the console reaches the browser, and the environment that bundles
/// them. The web build uses `browser_web.dart` (package:web); every other platform, the test VM
/// included, gets `browser_stub.dart`, which never touches the network.
library;

import 'dart:typed_data';

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

/// The browser's download of a file the console made itself (the audit export of A6, brief
/// M6-04). The bytes never reach browser storage: the file goes straight to the download.
abstract interface class FileDownloads {
  /// Offers [bytes] as a download named [name] with the media type [type].
  void save(String name, String type, Uint8List bytes);
}

/// Opening a link outside the console (brief M6-06 Design note 4): the telemetry link of the
/// statistics opens in a new tab without access to the console's window (`noopener`) and without
/// a referrer (`noreferrer`), never in an iframe (25 9.1).
abstract interface class ExternalLinks {
  /// Opens [url] in a new tab.
  void open(Uri url);
}

/// A file the person chose in the browser: its name and its bytes.
final class ChosenFile {
  /// Creates the file.
  const ChosenFile(this.name, this.bytes);

  /// The file name.
  final String name;

  /// The content.
  final Uint8List bytes;
}

/// The browser's file input (the manifest of an app, brief M6-08 Design note 1: a file, never a
/// URL). The bytes stay in memory; nothing reaches browser storage.
abstract interface class FileChooser {
  /// Lets the person choose one file of the types [accept] (the `accept` attribute, for example
  /// `.json,.yaml,.yml`) and returns it, or null when the person cancels. [tag] names the input
  /// in the page (`data-bothy-file-input`), so the browser tests of `make console-e2e` find it.
  Future<ChosenFile?> choose({required String accept, required String tag});
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
  /// Creates the environment from its parts; [downloads] saves files (null hides the audit
  /// export), [files] chooses files (null hides the install and update of apps), [links] opens
  /// a link in a new tab (null hides the telemetry link of the statistics), [adapter]
  /// replaces the browser's HTTP stack in tests.
  const ConsoleEnvironment({
    required this.origin,
    required this.flowStore,
    required this.navigation,
    required this.passkeys,
    this.downloads,
    this.files,
    this.links,
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

  /// The download of files, or null where the platform has none (the audit export is hidden).
  final FileDownloads? downloads;

  /// The file input, or null where the platform has none (no install or update of an app).
  final FileChooser? files;

  /// Opening links in a new tab, or null where the platform has none (no telemetry link).
  final ExternalLinks? links;

  /// The HTTP adapter; null means the browser's own (cookies of the same origin go along).
  final HttpClientAdapter? adapter;
}
