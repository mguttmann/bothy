//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

import 'dart:async';

import 'package:built_value/json_object.dart';
import 'package:built_value/serializer.dart';
import 'package:dio/dio.dart';

import 'package:api_client/src/generated/api_util.dart';
import 'package:api_client/src/generated/model/audit_decision.dart';
import 'package:api_client/src/generated/model/audit_event.dart';
import 'package:api_client/src/generated/model/audit_event_page.dart';
import 'package:api_client/src/generated/model/audit_verify_request.dart';
import 'package:api_client/src/generated/model/audit_verify_result.dart';
import 'package:api_client/src/generated/model/problem.dart';

class AuditApi {
  final Dio _dio;

  final Serializers _serializers;

  const AuditApi(this._dio, this._serializers);

  /// Export the audit log
  /// Streams one &#x60;AuditEvent&#x60; per line in ascending &#x60;seq&#x60;, with hashes for offline verification. The stream is the contiguous &#x60;seq&#x60; span from the smallest to the largest &#x60;seq&#x60; whose &#x60;occurred_at&#x60; lies in [&#x60;from&#x60;, &#x60;to&#x60;), so every line links the line before by &#x60;prev_hash&#x60;; a row inside the span whose &#x60;occurred_at&#x60; lies outside the range (an aggregated denial keeps the time of its first denial) is part of it. A gap inside the stream breaks the hash links (&#x60;prev_hash&#x60;); an error after the first line aborts the transfer instead of ending it, so a client sees a broken response, not a shorter complete one.
  ///
  /// Parameters:
  /// * [from] - Start of the range (inclusive).
  /// * [to] - End of the range (exclusive).
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [AuditEvent] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<AuditEvent>> exportAuditEvents({
    required DateTime from,
    required DateTime to,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/audit/export';
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
      r'from': encodeQueryParameter(
        _serializers,
        from,
        const FullType(DateTime),
      ),
      r'to': encodeQueryParameter(_serializers, to, const FullType(DateTime)),
    };

    final _response = await _dio.request<Object>(
      _path,
      options: _options,
      queryParameters: _queryParameters,
      cancelToken: cancelToken,
      onSendProgress: onSendProgress,
      onReceiveProgress: onReceiveProgress,
    );

    AuditEvent? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(AuditEvent),
            ) as AuditEvent;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<AuditEvent>(
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

  /// Query the audit log
  ///
  ///
  /// Parameters:
  /// * [cursor] - Cursor from `next_cursor` of the previous page.
  /// * [limit]
  /// * [actorId] - Actor principal.
  /// * [action] - Action ID.
  /// * [decision] - Decision.
  /// * [resourceKind] - Resource kind.
  /// * [resourceId] - Resource ID.
  /// * [from] - Occurred at or after.
  /// * [to] - Occurred before.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [AuditEventPage] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<AuditEventPage>> listAuditEvents({
    String? cursor,
    int? limit,
    String? actorId,
    String? action,
    AuditDecision? decision,
    String? resourceKind,
    String? resourceId,
    DateTime? from,
    DateTime? to,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/audit/events';
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
      if (actorId != null)
        r'actor_id': encodeQueryParameter(
          _serializers,
          actorId,
          const FullType(String),
        ),
      if (action != null)
        r'action': encodeQueryParameter(
          _serializers,
          action,
          const FullType(String),
        ),
      if (decision != null)
        r'decision': encodeQueryParameter(
          _serializers,
          decision,
          const FullType(AuditDecision),
        ),
      if (resourceKind != null)
        r'resource_kind': encodeQueryParameter(
          _serializers,
          resourceKind,
          const FullType(String),
        ),
      if (resourceId != null)
        r'resource_id': encodeQueryParameter(
          _serializers,
          resourceId,
          const FullType(String),
        ),
      if (from != null)
        r'from': encodeQueryParameter(
          _serializers,
          from,
          const FullType(DateTime),
        ),
      if (to != null)
        r'to': encodeQueryParameter(_serializers, to, const FullType(DateTime)),
    };

    final _response = await _dio.request<Object>(
      _path,
      options: _options,
      queryParameters: _queryParameters,
      cancelToken: cancelToken,
      onSendProgress: onSendProgress,
      onReceiveProgress: onReceiveProgress,
    );

    AuditEventPage? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(AuditEventPage),
            ) as AuditEventPage;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<AuditEventPage>(
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

  /// Verify the hash chain
  ///
  ///
  /// Parameters:
  /// * [auditVerifyRequest]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [AuditVerifyResult] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<AuditVerifyResult>> verifyAuditChain({
    required AuditVerifyRequest auditVerifyRequest,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/audit/verify';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{...?headers},
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
      const _type = FullType(AuditVerifyRequest);
      _bodyData = _serializers.serialize(
        auditVerifyRequest,
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

    AuditVerifyResult? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(AuditVerifyResult),
            ) as AuditVerifyResult;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<AuditVerifyResult>(
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
