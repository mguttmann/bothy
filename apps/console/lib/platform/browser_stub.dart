// SPDX-License-Identifier: Apache-2.0

/// The environment outside a browser (the test VM): no storage across a reload, no navigation,
/// no passkeys and an HTTP adapter that fails every request without opening a socket.
library;

import 'dart:typed_data';

import 'package:dio/dio.dart';

import 'browser.dart';

/// The inert environment of a non-web platform.
ConsoleEnvironment browserEnvironment() => ConsoleEnvironment(
  origin: Uri.parse('http://localhost'),
  flowStore: _MemoryFlowStore(),
  navigation: _NoNavigation(),
  passkeys: _NoPasskeys(),
  adapter: _NoNetwork(),
);

class _MemoryFlowStore implements FlowStore {
  final Map<String, String> _entries = {};

  @override
  String? read(String key) => _entries[key];

  @override
  void write(String key, String value) => _entries[key] = value;

  @override
  void delete(String key) => _entries.remove(key);
}

class _NoNavigation implements BrowserNavigation {
  @override
  void assign(Uri url) {}

  @override
  void reload() {}

  @override
  void setDocumentLanguage(String tag) {}
}

class _NoPasskeys implements PasskeyPort {
  @override
  Future<Map<String, Object?>> get(Map<String, Object?> options) =>
      Future.error(const PasskeyCancelled());
}

class _NoNetwork implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) => Future.error(
    DioException.connectionError(
      requestOptions: options,
      reason: 'no network outside the browser',
    ),
  );

  @override
  void close({bool force = false}) {}
}
