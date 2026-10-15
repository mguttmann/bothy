//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

import 'dart:async';

import 'package:built_value/json_object.dart';
import 'package:built_value/serializer.dart';
import 'package:dio/dio.dart';

import 'package:api_client/src/generated/api_util.dart';
import 'package:api_client/src/generated/model/host.dart';
import 'package:api_client/src/generated/model/host_confirm.dart';
import 'package:api_client/src/generated/model/host_create.dart';
import 'package:api_client/src/generated/model/host_llm_credentials_set.dart';
import 'package:api_client/src/generated/model/host_page.dart';
import 'package:api_client/src/generated/model/host_pair_request.dart';
import 'package:api_client/src/generated/model/host_pair_result.dart';
import 'package:api_client/src/generated/model/host_pairing_code.dart';
import 'package:api_client/src/generated/model/host_state_set.dart';
import 'package:api_client/src/generated/model/host_update.dart';
import 'package:api_client/src/generated/model/problem.dart';
import 'package:api_client/src/generated/model/reason_body.dart';

class HostsApi {
  final Dio _dio;

  final Serializers _serializers;

  const HostsApi(this._dio, this._serializers);

  /// Confirm the fingerprint and declare the LLM login
  /// The owner types the first &#x60;terminal.fingerprint_confirm_chars&#x60; characters shown by &#x60;bothy-host pair&#x60;, without separators, case-insensitive; a prefix of another length or with a character outside the Crockford alphabet (also &#x60;I&#x60;, &#x60;L&#x60;, &#x60;O&#x60;, &#x60;U&#x60;) gives &#x60;400 validation_failed&#x60; with &#x60;errors[].code&#x60; &#x60;invalid_format&#x60; and does not count. A mismatch gives &#x60;409 fingerprint_mismatch&#x60; with &#x60;attempts_remaining&#x60;; the failure counter is not part of &#x60;Host&#x60;, so &#x60;version&#x60; and the &#x60;If-Match&#x60; stay. After &#x60;terminal.fingerprint_max_attempts&#x60; failures the host falls back to &#x60;pairing&#x60; without keys (&#x60;attempts_remaining&#x60; &#x60;0&#x60;), and a new pairing code is needed. A failed attempt counts also under an &#x60;Idempotency-Key&#x60;; its replay returns the stored &#x60;409&#x60;. I6 is checked in the same transaction (&#x60;409 i6_violation&#x60;). &#x60;session_users&#x60; assigns each person their own session user on this host (ruling T12, 24 section 2.2); a name that belongs or belonged to another person on this host, or a second valid assignment of one person, gives &#x60;409 conflict&#x60; with &#x60;reason: session_user_taken&#x60;; a name the host did not report (&#x60;reported_session_users&#x60;) gives &#x60;400 validation_failed&#x60; with &#x60;errors[].code&#x60; &#x60;session_user_not_on_host&#x60;, and nothing is stored. Without an assignment a person gets no terminal there (SR12). Needs step-up.
  ///
  /// Parameters:
  /// * [hostId]
  /// * [ifMatch] - ETag of the version the change is based on, e.g. `\"7\"`. Mismatch gives 412 `version_conflict`.
  /// * [hostConfirm]
  /// * [idempotencyKey] - UUIDv7 chosen by the client per logical operation. A repeat with the same request returns the stored response; a different request gives 422 `idempotency_key_reuse`; a repeat while the first is running gives 409 `request_in_progress`. Kept for `sync.idempotency_key_ttl`.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [Host] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<Host>> confirmHost({
    required String hostId,
    required String ifMatch,
    required HostConfirm hostConfirm,
    String? idempotencyKey,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/hosts/{host_id}/confirm'.replaceAll(
      '{'
      r'host_id'
      '}',
      encodeQueryParameter(
        _serializers,
        hostId,
        const FullType(String),
      ).toString(),
    );
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        if (idempotencyKey != null) r'Idempotency-Key': idempotencyKey,
        r'If-Match': ifMatch,
        ...?headers,
      },
      extra: <String, dynamic>{
        'secure': <Map<String, String>>[
          {'type': 'http', 'scheme': 'DPoP', 'name': 'dpopAuth'},
          {'type': 'http', 'scheme': 'bearer', 'name': 'bearerAuth'},
        ],
        ...?extra,
      },
      contentType: 'application/json',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(HostConfirm);
      _bodyData = _serializers.serialize(hostConfirm, specifiedType: _type);
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _options.compose(_dio.options, _path),
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    final _response = await _dio.request<Object>(
      _path,
      data: _bodyData,
      options: _options,
      cancelToken: cancelToken,
      onSendProgress: onSendProgress,
      onReceiveProgress: onReceiveProgress,
    );

    Host? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(Host),
            ) as Host;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<Host>(
      data: _responseData,
      headers: _response.headers,
      isRedirect: _response.isRedirect,
      requestOptions: _response.requestOptions,
      redirects: _response.redirects,
      statusCode: _response.statusCode,
      statusMessage: _response.statusMessage,
      extra: _response.extra,
    );
  }

  /// Register a host and issue its pairing code
  /// Creates a host in state &#x60;pairing&#x60; and returns the pairing code &#x60;bothy_hpc_...&#x60; exactly once; it is valid for &#x60;terminal.pairing_code_ttl&#x60; (24, 3.2). Needs step-up. The code is a one-time secret: a replay with the same &#x60;Idempotency-Key&#x60; answers &#x60;409 conflict&#x60; with &#x60;reason&#x60; &#x60;one_time_secret&#x60; instead of repeating it (06, 7).
  ///
  /// Parameters:
  /// * [hostCreate]
  /// * [idempotencyKey] - UUIDv7 chosen by the client per logical operation. A repeat with the same request returns the stored response; a different request gives 422 `idempotency_key_reuse`; a repeat while the first is running gives 409 `request_in_progress`. Kept for `sync.idempotency_key_ttl`.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [HostPairingCode] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<HostPairingCode>> createHost({
    required HostCreate hostCreate,
    String? idempotencyKey,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/hosts';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        if (idempotencyKey != null) r'Idempotency-Key': idempotencyKey,
        ...?headers,
      },
      extra: <String, dynamic>{
        'secure': <Map<String, String>>[
          {'type': 'http', 'scheme': 'DPoP', 'name': 'dpopAuth'},
          {'type': 'http', 'scheme': 'bearer', 'name': 'bearerAuth'},
        ],
        ...?extra,
      },
      contentType: 'application/json',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(HostCreate);
      _bodyData = _serializers.serialize(hostCreate, specifiedType: _type);
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _options.compose(_dio.options, _path),
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    final _response = await _dio.request<Object>(
      _path,
      data: _bodyData,
      options: _options,
      cancelToken: cancelToken,
      onSendProgress: onSendProgress,
      onReceiveProgress: onReceiveProgress,
    );

    HostPairingCode? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(HostPairingCode),
            ) as HostPairingCode;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<HostPairingCode>(
      data: _responseData,
      headers: _response.headers,
      isRedirect: _response.isRedirect,
      requestOptions: _response.requestOptions,
      redirects: _response.redirects,
      statusCode: _response.statusCode,
      statusMessage: _response.statusMessage,
      extra: _response.extra,
    );
  }

  /// Read a host
  ///
  ///
  /// Parameters:
  /// * [hostId]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [Host] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<Host>> getHost({
    required String hostId,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/hosts/{host_id}'.replaceAll(
      '{'
      r'host_id'
      '}',
      encodeQueryParameter(
        _serializers,
        hostId,
        const FullType(String),
      ).toString(),
    );
    final _options = Options(
      method: r'GET',
      headers: <String, dynamic>{...?headers},
      extra: <String, dynamic>{
        'secure': <Map<String, String>>[
          {'type': 'http', 'scheme': 'DPoP', 'name': 'dpopAuth'},
          {'type': 'http', 'scheme': 'bearer', 'name': 'bearerAuth'},
        ],
        ...?extra,
      },
      validateStatus: validateStatus,
    );

    final _response = await _dio.request<Object>(
      _path,
      options: _options,
      cancelToken: cancelToken,
      onSendProgress: onSendProgress,
      onReceiveProgress: onReceiveProgress,
    );

    Host? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(Host),
            ) as Host;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<Host>(
      data: _responseData,
      headers: _response.headers,
      isRedirect: _response.isRedirect,
      requestOptions: _response.requestOptions,
      redirects: _response.redirects,
      statusCode: _response.statusCode,
      statusMessage: _response.statusMessage,
      extra: _response.extra,
    );
  }

  /// Issue a new pairing code (re-pairing)
  /// Sets the host to &#x60;pairing&#x60; from &#x60;pairing&#x60;, &#x60;confirming&#x60;, &#x60;active&#x60; or &#x60;disabled&#x60; (also &#x60;duplicate_connection&#x60;, whose reason is cleared); &#x60;retired&#x60; gives &#x60;409 invalid_state_transition&#x60;. From &#x60;confirming&#x60;, &#x60;active&#x60; and &#x60;disabled&#x60; it drops the stored keys at once; from &#x60;active&#x60; it ends the sessions (&#x60;host_rekeyed&#x60;) and closes the connections. Every owner gets &#x60;host.updated&#x60;, with &#x60;key_changed&#x60; exactly when stored keys were dropped (24, 3.3); &#x60;key_generation&#x60; rises at the next &#x60;pairHost&#x60;. A repeated call issues another code and invalidates the previous one. Needs step-up. The code is a one-time secret: a replay with the same &#x60;Idempotency-Key&#x60; answers &#x60;409 conflict&#x60; with &#x60;reason&#x60; &#x60;one_time_secret&#x60; instead of repeating it (06, 7).
  ///
  /// Parameters:
  /// * [hostId]
  /// * [reasonBody]
  /// * [idempotencyKey] - UUIDv7 chosen by the client per logical operation. A repeat with the same request returns the stored response; a different request gives 422 `idempotency_key_reuse`; a repeat while the first is running gives 409 `request_in_progress`. Kept for `sync.idempotency_key_ttl`.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [HostPairingCode] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<HostPairingCode>> issueHostPairingCode({
    required String hostId,
    required ReasonBody reasonBody,
    String? idempotencyKey,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/hosts/{host_id}/pairing-code'.replaceAll(
      '{'
      r'host_id'
      '}',
      encodeQueryParameter(
        _serializers,
        hostId,
        const FullType(String),
      ).toString(),
    );
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        if (idempotencyKey != null) r'Idempotency-Key': idempotencyKey,
        ...?headers,
      },
      extra: <String, dynamic>{
        'secure': <Map<String, String>>[
          {'type': 'http', 'scheme': 'DPoP', 'name': 'dpopAuth'},
          {'type': 'http', 'scheme': 'bearer', 'name': 'bearerAuth'},
        ],
        ...?extra,
      },
      contentType: 'application/json',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(ReasonBody);
      _bodyData = _serializers.serialize(reasonBody, specifiedType: _type);
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _options.compose(_dio.options, _path),
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    final _response = await _dio.request<Object>(
      _path,
      data: _bodyData,
      options: _options,
      cancelToken: cancelToken,
      onSendProgress: onSendProgress,
      onReceiveProgress: onReceiveProgress,
    );

    HostPairingCode? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(HostPairingCode),
            ) as HostPairingCode;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<HostPairingCode>(
      data: _responseData,
      headers: _response.headers,
      isRedirect: _response.isRedirect,
      requestOptions: _response.requestOptions,
      redirects: _response.redirects,
      statusCode: _response.statusCode,
      statusMessage: _response.statusMessage,
      extra: _response.extra,
    );
  }

  /// List hosts
  /// Owners see every host; other people see hosts they are assigned to or hold a &#x60;terminal&#x60; grant on (07, 2.12). Ordered by name.
  ///
  /// Parameters:
  /// * [cursor] - Cursor from `next_cursor` of the previous page.
  /// * [limit]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [HostPage] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<HostPage>> listHosts({
    String? cursor,
    int? limit,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/hosts';
    final _options = Options(
      method: r'GET',
      headers: <String, dynamic>{...?headers},
      extra: <String, dynamic>{
        'secure': <Map<String, String>>[
          {'type': 'http', 'scheme': 'DPoP', 'name': 'dpopAuth'},
          {'type': 'http', 'scheme': 'bearer', 'name': 'bearerAuth'},
        ],
        ...?extra,
      },
      validateStatus: validateStatus,
    );

    final _queryParameters = <String, dynamic>{
      if (cursor != null)
        r'cursor': encodeQueryParameter(
          _serializers,
          cursor,
          const FullType(String),
        ),
      if (limit != null)
        r'limit': encodeQueryParameter(
          _serializers,
          limit,
          const FullType(int),
        ),
    };

    final _response = await _dio.request<Object>(
      _path,
      options: _options,
      queryParameters: _queryParameters,
      cancelToken: cancelToken,
      onSendProgress: onSendProgress,
      onReceiveProgress: onReceiveProgress,
    );

    HostPage? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(HostPage),
            ) as HostPage;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<HostPage>(
      data: _responseData,
      headers: _response.headers,
      isRedirect: _response.isRedirect,
      requestOptions: _response.requestOptions,
      redirects: _response.redirects,
      statusCode: _response.statusCode,
      statusMessage: _response.statusMessage,
      extra: _response.extra,
    );
  }

  /// Redeem a pairing code from the host service
  /// Called by &#x60;bothy-host pair&#x60; without login. Registers the static X25519 key (Noise) and the Ed25519 key (login) of the host, its profiles and the names of its session users (&#x60;reported_session_users&#x60;, 24 section 2.2). The host becomes &#x60;confirming&#x60;, &#x60;key_generation&#x60; rises; an owner confirms the fingerprint (24, 3.2). Every code that does not work answers &#x60;401 unauthenticated&#x60; with the same body: a wrong form or checksum (checked without database access), an unknown, used, replaced or expired code; a retry after a lost answer gets &#x60;401&#x60; too, the operator asks for a new code. Key rules (&#x60;400 validation_failed&#x60;, the code stays valid): &#x60;noise_public_key&#x60; strict base64url of 32 bytes and no low-order point (&#x60;errors[].code&#x60; &#x60;invalid_key&#x60;); &#x60;auth_public_jwk&#x60; exactly &#x60;kty&#x60; &#x60;OKP&#x60;, &#x60;crv&#x60; &#x60;Ed25519&#x60; and &#x60;x&#x60; of 32 bytes (&#x60;invalid_key&#x60;), any other member such as the private &#x60;d&#x60; is &#x60;unknown_member&#x60;. Report rules: at least one session user (&#x60;empty&#x60;), names distinct without regard to case (&#x60;duplicate&#x60;) and at most &#x60;host.os_user_max_chars&#x60; characters (&#x60;too_long&#x60;), labels and profile names at most &#x60;terminal.report_label_max_chars&#x60; characters (&#x60;too_long&#x60;), each list at most &#x60;terminal.report_max_entries&#x60; entries (&#x60;limit_reached&#x60;), distinct profile names (&#x60;duplicate&#x60;), &#x60;host_version&#x60; SemVer without &#x60;v&#x60; (&#x60;invalid_format&#x60;). &#x60;409 conflict&#x60; with &#x60;reason&#x60; &#x60;host_key_in_use&#x60; when the login key belongs to another host, &#x60;host_key_reused&#x60; when the keys equal the keys a re-pairing dropped; the code stays valid.
  ///
  /// Parameters:
  /// * [hostPairRequest]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [HostPairResult] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<HostPairResult>> pairHost({
    required HostPairRequest hostPairRequest,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/hosts/pair';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{...?headers},
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: 'application/json',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(HostPairRequest);
      _bodyData = _serializers.serialize(hostPairRequest, specifiedType: _type);
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _options.compose(_dio.options, _path),
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    final _response = await _dio.request<Object>(
      _path,
      data: _bodyData,
      options: _options,
      cancelToken: cancelToken,
      onSendProgress: onSendProgress,
      onReceiveProgress: onReceiveProgress,
    );

    HostPairResult? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(HostPairResult),
            ) as HostPairResult;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<HostPairResult>(
      data: _responseData,
      headers: _response.headers,
      isRedirect: _response.isRedirect,
      requestOptions: _response.requestOptions,
      redirects: _response.redirects,
      statusCode: _response.statusCode,
      statusMessage: _response.statusMessage,
      extra: _response.extra,
    );
  }

  /// Declare the LLM login of a host
  /// &#x60;personal&#x60; needs &#x60;credential_holder_id&#x60;, a person that is not deleted; &#x60;none&#x60; and &#x60;organization&#x60; take none. Fails with &#x60;409 i6_violation&#x60; (member &#x60;conflicts&#x60;, one entry per host) if any valid grant on the host names another subject than the holder or a group; switching to &#x60;personal&#x60; or changing the holder ends the sessions of other people in the same transaction (&#x60;close_reason&#x60; &#x60;i6_violation&#x60;, 24, 7.3). A &#x60;retired&#x60; host answers &#x60;409 conflict&#x60;. The declaration the host already has answers &#x60;200&#x60; with the current host, without a new version, an event or a new rights version (audited with &#x60;no_change&#x60;). Needs step-up and the mandatory &#x60;reason&#x60;.
  ///
  /// Parameters:
  /// * [hostId]
  /// * [ifMatch] - ETag of the version the change is based on, e.g. `\"7\"`. Mismatch gives 412 `version_conflict`.
  /// * [hostLlmCredentialsSet]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [Host] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<Host>> setHostLlmCredentials({
    required String hostId,
    required String ifMatch,
    required HostLlmCredentialsSet hostLlmCredentialsSet,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/hosts/{host_id}/llm-credentials'.replaceAll(
      '{'
      r'host_id'
      '}',
      encodeQueryParameter(
        _serializers,
        hostId,
        const FullType(String),
      ).toString(),
    );
    final _options = Options(
      method: r'PUT',
      headers: <String, dynamic>{r'If-Match': ifMatch, ...?headers},
      extra: <String, dynamic>{
        'secure': <Map<String, String>>[
          {'type': 'http', 'scheme': 'DPoP', 'name': 'dpopAuth'},
          {'type': 'http', 'scheme': 'bearer', 'name': 'bearerAuth'},
        ],
        ...?extra,
      },
      contentType: 'application/json',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(HostLlmCredentialsSet);
      _bodyData = _serializers.serialize(
        hostLlmCredentialsSet,
        specifiedType: _type,
      );
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _options.compose(_dio.options, _path),
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    final _response = await _dio.request<Object>(
      _path,
      data: _bodyData,
      options: _options,
      cancelToken: cancelToken,
      onSendProgress: onSendProgress,
      onReceiveProgress: onReceiveProgress,
    );

    Host? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(Host),
            ) as Host;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<Host>(
      data: _responseData,
      headers: _response.headers,
      isRedirect: _response.isRedirect,
      requestOptions: _response.requestOptions,
      redirects: _response.redirects,
      statusCode: _response.statusCode,
      statusMessage: _response.statusMessage,
      extra: _response.extra,
    );
  }

  /// Disable, enable or retire a host
  /// &#x60;disabled&#x60; and &#x60;retired&#x60; end all sessions (&#x60;host_disabled&#x60;, &#x60;host_retired&#x60;) and close the connections of the host service. &#x60;retired&#x60; is final. Needs step-up.
  ///
  /// Parameters:
  /// * [hostId]
  /// * [ifMatch] - ETag of the version the change is based on, e.g. `\"7\"`. Mismatch gives 412 `version_conflict`.
  /// * [hostStateSet]
  /// * [idempotencyKey] - UUIDv7 chosen by the client per logical operation. A repeat with the same request returns the stored response; a different request gives 422 `idempotency_key_reuse`; a repeat while the first is running gives 409 `request_in_progress`. Kept for `sync.idempotency_key_ttl`.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [Host] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<Host>> setHostState({
    required String hostId,
    required String ifMatch,
    required HostStateSet hostStateSet,
    String? idempotencyKey,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/hosts/{host_id}/state'.replaceAll(
      '{'
      r'host_id'
      '}',
      encodeQueryParameter(
        _serializers,
        hostId,
        const FullType(String),
      ).toString(),
    );
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        if (idempotencyKey != null) r'Idempotency-Key': idempotencyKey,
        r'If-Match': ifMatch,
        ...?headers,
      },
      extra: <String, dynamic>{
        'secure': <Map<String, String>>[
          {'type': 'http', 'scheme': 'DPoP', 'name': 'dpopAuth'},
          {'type': 'http', 'scheme': 'bearer', 'name': 'bearerAuth'},
        ],
        ...?extra,
      },
      contentType: 'application/json',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(HostStateSet);
      _bodyData = _serializers.serialize(hostStateSet, specifiedType: _type);
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _options.compose(_dio.options, _path),
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    final _response = await _dio.request<Object>(
      _path,
      data: _bodyData,
      options: _options,
      cancelToken: cancelToken,
      onSendProgress: onSendProgress,
      onReceiveProgress: onReceiveProgress,
    );

    Host? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(Host),
            ) as Host;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<Host>(
      data: _responseData,
      headers: _response.headers,
      isRedirect: _response.isRedirect,
      requestOptions: _response.requestOptions,
      redirects: _response.redirects,
      statusCode: _response.statusCode,
      statusMessage: _response.statusMessage,
      extra: _response.extra,
    );
  }

  /// Change name or assignee of a host
  /// A new &#x60;assigned_user_id&#x60; ends the sessions of people who no longer qualify under SR10 (&#x60;access_revoked&#x60;). &#x60;add_session_users&#x60; assigns own session users (ruling T12; a taken name gives &#x60;409 conflict&#x60; with &#x60;reason: session_user_taken&#x60;, a name the host does not report in &#x60;reported_session_users&#x60; gives &#x60;400 validation_failed&#x60; with &#x60;errors[].code&#x60; &#x60;session_user_not_on_host&#x60;); &#x60;revoke_session_users&#x60; ends the assignment of the listed people, ends their sessions on the host (&#x60;access_revoked&#x60;) and keeps the name taken (24 section 2.2). Needs step-up.
  ///
  /// Parameters:
  /// * [hostId]
  /// * [ifMatch] - ETag of the version the change is based on, e.g. `\"7\"`. Mismatch gives 412 `version_conflict`.
  /// * [hostUpdate]
  /// * [idempotencyKey] - UUIDv7 chosen by the client per logical operation. A repeat with the same request returns the stored response; a different request gives 422 `idempotency_key_reuse`; a repeat while the first is running gives 409 `request_in_progress`. Kept for `sync.idempotency_key_ttl`.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [Host] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<Host>> updateHost({
    required String hostId,
    required String ifMatch,
    required HostUpdate hostUpdate,
    String? idempotencyKey,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/hosts/{host_id}'.replaceAll(
      '{'
      r'host_id'
      '}',
      encodeQueryParameter(
        _serializers,
        hostId,
        const FullType(String),
      ).toString(),
    );
    final _options = Options(
      method: r'PATCH',
      headers: <String, dynamic>{
        if (idempotencyKey != null) r'Idempotency-Key': idempotencyKey,
        r'If-Match': ifMatch,
        ...?headers,
      },
      extra: <String, dynamic>{
        'secure': <Map<String, String>>[
          {'type': 'http', 'scheme': 'DPoP', 'name': 'dpopAuth'},
          {'type': 'http', 'scheme': 'bearer', 'name': 'bearerAuth'},
        ],
        ...?extra,
      },
      contentType: 'application/json',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(HostUpdate);
      _bodyData = _serializers.serialize(hostUpdate, specifiedType: _type);
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _options.compose(_dio.options, _path),
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    final _response = await _dio.request<Object>(
      _path,
      data: _bodyData,
      options: _options,
      cancelToken: cancelToken,
      onSendProgress: onSendProgress,
      onReceiveProgress: onReceiveProgress,
    );

    Host? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(Host),
            ) as Host;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<Host>(
      data: _responseData,
      headers: _response.headers,
      isRedirect: _response.isRedirect,
      requestOptions: _response.requestOptions,
      redirects: _response.redirects,
      statusCode: _response.statusCode,
      statusMessage: _response.statusMessage,
      extra: _response.extra,
    );
  }
}
