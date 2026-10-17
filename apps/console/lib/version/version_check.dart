// SPDX-License-Identifier: Apache-2.0

/// The version check of the console (brief M6-02 Design note 5, docs/DECISIONS.md D-M6-02-6).
library;

import 'package:dio/dio.dart';

/// Whether the console build may talk to the server: the `spec_version` of its `version.json`
/// ([built]) equals `Discovery.spec_version` of the server ([server]). A missing or empty
/// version never matches, so a build without the version step blocks instead of guessing.
bool versionsMatch({required String? built, required String server}) =>
    built != null && built.isNotEmpty && built == server;

/// Reads `spec_version` of the `version.json` the server serves next to the console, at
/// `<origin>/console/version.json`; null when the file is missing or has no such member.
///
/// [adapter] replaces the browser's HTTP stack in tests. Network errors reach the caller.
Future<String?> readBuiltSpecVersion(
  Uri origin, {
  HttpClientAdapter? adapter,
}) async {
  final dio = Dio(BaseOptions(responseType: ResponseType.json));
  if (adapter != null) dio.httpClientAdapter = adapter;
  try {
    final response = await dio.getUri<Object?>(
      origin.replace(path: '/console/version.json'),
    );
    final data = response.data;
    final version = data is Map ? data['spec_version'] : null;
    return version is String ? version : null;
  } on DioException catch (e) {
    if (e.response != null) return null;
    rethrow;
  } finally {
    dio.close();
  }
}
