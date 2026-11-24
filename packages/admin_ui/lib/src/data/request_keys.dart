// SPDX-License-Identifier: Apache-2.0

/// The `Idempotency-Key` of a creating request across repetitions by the person (04 5.2;
/// 06 7; 15 1 item 3).
library;

import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:dio/dio.dart';

/// The most requests without a final answer whose keys are kept; the oldest goes first.
const int maxOpenRequestKeys = 32;

/// The keys of creating requests that got no final answer.
///
/// The server stores the answer of a request under its key and answers a repetition with the
/// same key from that record (06 7). A key made per HTTP call would turn a repetition after a
/// lost answer into a second creation: a second grant, a `409` for a slug the first install
/// took. [run] therefore keeps the key of a [request] (its operation and content) while no
/// final answer arrived (no answer, `401`, `429`, `5xx`, `409 request_in_progress`; 06 7 item
/// 2) and sends the same request again with it; any final answer, success or problem, drops
/// it, so a repetition after a refusal is a new request. A changed request gets a new key.
///
/// Requests whose answer carries a one-time secret (tokens, codes, the address of an incoming
/// webhook) are not run here: their repetition answers `409 one_time_secret` (06 7).
///
/// Example:
/// ```dart
/// final keys = RequestKeys(ids.next);
/// await keys.run('createGroup Support', (key) => api.createGroup(idempotencyKey: key));
/// ```
final class RequestKeys {
  /// Creates the keys; [newKey] returns a fresh UUIDv7.
  RequestKeys(this._newKey);

  final String Function() _newKey;
  final Map<String, String> _open = {};

  /// Runs [call] with the key of [request]: the kept key of an earlier attempt of the same
  /// request without a final answer, else a new one.
  Future<T> run<T>(String request, Future<T> Function(String key) call) async {
    final key = _open.remove(request) ?? _newKey();
    try {
      return await call(key);
    } on DioException catch (e) {
      if (!_isFinal(e.response)) _keep(request, key);
      rethrow;
    }
  }

  void _keep(String request, String key) {
    _open[request] = key;
    while (_open.length > maxOpenRequestKeys) {
      _open.remove(_open.keys.first);
    }
  }

  /// Whether the server stored [response] under the key (06 7 item 2).
  static bool _isFinal(Response<Object?>? response) {
    final status = response?.statusCode;
    if (status == null || status == 401 || status == 429 || status >= 500) {
      return false;
    }
    if (status == 409) {
      return ApiProblem.fromResponse(response!).code != 'request_in_progress';
    }
    return true;
  }
}
