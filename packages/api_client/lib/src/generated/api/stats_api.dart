//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

import 'dart:async';

import 'package:built_value/json_object.dart';
import 'package:built_value/serializer.dart';
import 'package:dio/dio.dart';

import 'package:api_client/src/generated/api_util.dart';
import 'package:api_client/src/generated/model/client_metrics_batch.dart';
import 'package:api_client/src/generated/model/cost_breakdown.dart';
import 'package:api_client/src/generated/model/cost_estimate_page.dart';
import 'package:api_client/src/generated/model/cost_reconcile.dart';
import 'package:api_client/src/generated/model/cost_reconcile_result.dart';
import 'package:api_client/src/generated/model/cost_series.dart';
import 'package:api_client/src/generated/model/price.dart';
import 'package:api_client/src/generated/model/price_close.dart';
import 'package:api_client/src/generated/model/price_create.dart';
import 'package:api_client/src/generated/model/price_list.dart';
import 'package:api_client/src/generated/model/problem.dart';
import 'package:api_client/src/generated/model/stats_backend_report.dart';
import 'package:api_client/src/generated/model/stats_granularity.dart';
import 'package:api_client/src/generated/model/stats_live_frame.dart';
import 'package:api_client/src/generated/model/stats_metric.dart';
import 'package:api_client/src/generated/model/stats_series.dart';
import 'package:api_client/src/generated/model/stats_slo_report.dart';
import 'package:api_client/src/generated/model/stats_timeline.dart';
import 'package:built_collection/built_collection.dart';

class StatsApi {
  final Dio _dio;

  final Serializers _serializers;

  const StatsApi(this._dio, this._serializers);

  /// End the validity of a price
  /// Sets &#x60;valid_to&#x60; of an open row, the only change a row ever gets (R-DM-23). The same end again answers 200 without change. 409 &#x60;conflict&#x60; with &#x60;reason&#x60; &#x60;price_closed&#x60; when the row is already closed with another end (a closed row is never reopened or moved; a gap is filled by a new row), &#x60;price_in_use&#x60; when a &#x60;computed&#x60; cost entry at or after the new end used the row.
  ///
  /// Parameters:
  /// * [priceId]
  /// * [priceClose]
  /// * [idempotencyKey] - UUIDv7 chosen by the client per logical operation. A repeat with the same request returns the stored response; a different request gives 422 `idempotency_key_reuse`; a repeat while the first is running gives 409 `request_in_progress`. Kept for `sync.idempotency_key_ttl`.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [Price] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<Price>> closePrice({
    required String priceId,
    required PriceClose priceClose,
    String? idempotencyKey,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/prices/{price_id}/close'.replaceAll(
      '{'
      r'price_id'
      '}',
      encodeQueryParameter(
        _serializers,
        priceId,
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
          {
            'type': 'apiKey',
            'name': 'consoleSession',
            'keyName': '__Host-console_session',
            'where': '',
          },
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
      const _type = FullType(PriceClose);
      _bodyData = _serializers.serialize(priceClose, specifiedType: _type);
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

    Price? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(Price),
            ) as Price;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<Price>(
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

  /// Add a price with source
  /// Owner with step-up, audited with row, source and retrieval time (10, 14). &#x60;source_url&#x60; is an absolute https URL without user info and fragment, &#x60;retrieved_at&#x60; not in the future, &#x60;currency&#x60; an ISO 4217 code. 409 &#x60;conflict&#x60; with &#x60;reason&#x60; &#x60;price_overlap&#x60; when the validity overlaps a row of the same provider, model and unit, &#x60;price_currency_mismatch&#x60; when an overlapping row of the same provider and model has another currency (one currency per price set).
  ///
  /// Parameters:
  /// * [priceCreate]
  /// * [idempotencyKey] - UUIDv7 chosen by the client per logical operation. A repeat with the same request returns the stored response; a different request gives 422 `idempotency_key_reuse`; a repeat while the first is running gives 409 `request_in_progress`. Kept for `sync.idempotency_key_ttl`.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [Price] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<Price>> createPrice({
    required PriceCreate priceCreate,
    String? idempotencyKey,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/prices';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        if (idempotencyKey != null) r'Idempotency-Key': idempotencyKey,
        ...?headers,
      },
      extra: <String, dynamic>{
        'secure': <Map<String, String>>[
          {
            'type': 'apiKey',
            'name': 'consoleSession',
            'keyName': '__Host-console_session',
            'where': '',
          },
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
      const _type = FullType(PriceCreate);
      _bodyData = _serializers.serialize(priceCreate, specifiedType: _type);
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

    Price? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(Price),
            ) as Price;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<Price>(
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

  /// Costs per space, agent or instance
  /// Valid cost sums (entries without a successor) per bucket and &#x60;billing_mode&#x60;, grouped by &#x60;group_by&#x60; (&#x60;space&#x60;, &#x60;agent&#x60;, &#x60;none&#x60;; default &#x60;none&#x60;). Authorized with &#x60;cost.read.aggregate&#x60; on the instance (owner) or, with &#x60;space_id&#x60;, on that space (&#x60;manage&#x60;); without &#x60;space_id&#x60; and without owner status &#x60;403&#x60;, an unreadable space &#x60;404&#x60;; only channels the caller may read count. &#x60;personal&#x60; and &#x60;organization&#x60; are never summed together; amounts of mode &#x60;personal&#x60; are information only and appear only when &#x60;billing.show_personal_estimates&#x60; is on. &#x60;is_estimate&#x60; is true when any summed entry is an estimate. One currency per response: &#x60;currency&#x60; selects it; without it a range whose entries have one currency answers in that currency (&#x60;USD&#x60; when there is none), and a range with several currencies is &#x60;400 validation_failed&#x60; with &#x60;errors[].code&#x60; &#x60;currency_required&#x60; and the currencies found in &#x60;detail&#x60;. Ranges, buckets, time zone and cache as for &#x60;getUsageStats&#x60;.
  ///
  /// Parameters:
  /// * [from] - Start of the range (inclusive).
  /// * [to] - End of the range (exclusive).
  /// * [granularity] - Bucket size; default `day`.
  /// * [groupBy] - Grouping of the buckets; default `none`.
  /// * [spaceId] - Restrict to a space.
  /// * [agentId] - Restrict to an agent.
  /// * [currency] - ISO 4217 code of the costs to sum; required when the range has costs in several currencies.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [CostSeries] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<CostSeries>> getCostStats({
    required DateTime from,
    required DateTime to,
    StatsGranularity? granularity,
    String? groupBy,
    String? spaceId,
    String? agentId,
    String? currency,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/stats/costs';
    final _options = Options(
      method: r'GET',
      headers: <String, dynamic>{...?headers},
      extra: <String, dynamic>{
        'secure': <Map<String, String>>[
          {
            'type': 'apiKey',
            'name': 'consoleSession',
            'keyName': '__Host-console_session',
            'where': '',
          },
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
      if (granularity != null)
        r'granularity': encodeQueryParameter(
          _serializers,
          granularity,
          const FullType(StatsGranularity),
        ),
      if (groupBy != null)
        r'group_by': encodeQueryParameter(
          _serializers,
          groupBy,
          const FullType(String),
        ),
      if (spaceId != null)
        r'space_id': encodeQueryParameter(
          _serializers,
          spaceId,
          const FullType(String),
        ),
      if (agentId != null)
        r'agent_id': encodeQueryParameter(
          _serializers,
          agentId,
          const FullType(String),
        ),
      if (currency != null)
        r'currency': encodeQueryParameter(
          _serializers,
          currency,
          const FullType(String),
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

    CostSeries? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(CostSeries),
            ) as CostSeries;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<CostSeries>(
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

  /// Error rates of runs and deliveries
  /// Metrics per bucket: &#x60;run_failure_rate&#x60; (failed per ended runs), &#x60;invocation_denials&#x60; (messages refused with &#x60;billing_mode_personal_not_holder&#x60; plus runs failed with &#x60;billing_mode_mismatch&#x60;); only for the instance (owner, without &#x60;space_id&#x60;): &#x60;webhook_failure_rate&#x60; (&#x60;failed&#x60; and &#x60;dead&#x60; per finished deliveries still within their retention), &#x60;push_failure_rate_&lt;provider&gt;&#x60; (registrations disabled for a provider reason per registrations active in the bucket), &#x60;scan_errors&#x60; and &#x60;scan_infected&#x60;. A rate without a denominator is left out. Without &#x60;space_id&#x60; there is no &#x60;group_key&#x60;; with &#x60;space_id&#x60; it is the space ID. HTTP error rates come from telemetry, not from this API. Authorized with &#x60;stats.read&#x60; on the instance (owner) or, with &#x60;space_id&#x60;, on that space (&#x60;manage&#x60;); without &#x60;space_id&#x60; and without owner status &#x60;403&#x60;, an unreadable space &#x60;404&#x60;. Every number counts only the channels the caller may read (&#x60;message.read&#x60;), intersected with the space; channels without a space (DMs) are not counted. No metric is grouped by person and no person is named. &#x60;from&#x60; and &#x60;to&#x60; are aligned down and up to the granularity (default &#x60;day&#x60;): &#x60;hour&#x60; in UTC, &#x60;day&#x60;, &#x60;week&#x60; (ISO, Monday) and &#x60;month&#x60; from local midnight of &#x60;stats.time_zone&#x60;, returned as UTC instants; a range longer than &#x60;stats.max_range&#x60;, with more than &#x60;api.stats_max_buckets&#x60; buckets or with &#x60;from &gt;&#x3D; to&#x60; is &#x60;400 validation_failed&#x60;. Buckets without a value are left out. The result is cached per caller scope for &#x60;stats.cache_ttl&#x60;.
  ///
  /// Parameters:
  /// * [from] - Start of the range (inclusive).
  /// * [to] - End of the range (exclusive).
  /// * [granularity] - Bucket size; default `day`.
  /// * [spaceId] - Restrict to a space.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [StatsSeries] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<StatsSeries>> getErrorStats({
    required DateTime from,
    required DateTime to,
    StatsGranularity? granularity,
    String? spaceId,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/stats/errors';
    final _options = Options(
      method: r'GET',
      headers: <String, dynamic>{...?headers},
      extra: <String, dynamic>{
        'secure': <Map<String, String>>[
          {
            'type': 'apiKey',
            'name': 'consoleSession',
            'keyName': '__Host-console_session',
            'where': '',
          },
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
      if (granularity != null)
        r'granularity': encodeQueryParameter(
          _serializers,
          granularity,
          const FullType(StatsGranularity),
        ),
      if (spaceId != null)
        r'space_id': encodeQueryParameter(
          _serializers,
          spaceId,
          const FullType(String),
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

    StatsSeries? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(StatsSeries),
            ) as StatsSeries;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<StatsSeries>(
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

  /// Runs per agent
  /// Metrics per agent and bucket of the end of a run: &#x60;runs&#x60;, &#x60;completed&#x60;, &#x60;failed&#x60;, &#x60;cancelled&#x60;, &#x60;duration_ms_p50&#x60;, &#x60;duration_ms_p95&#x60; (from start to end of the runs that started), and &#x60;failed_reason_&lt;code&gt;&#x60; for every &#x60;state_reason&#x60; code of failed runs that occurs. Grouped by agent (&#x60;group_key&#x60; is the agent ID). Authorized with &#x60;stats.read&#x60; on the instance (owner) or, with &#x60;space_id&#x60;, on that space (&#x60;manage&#x60;); without &#x60;space_id&#x60; and without owner status &#x60;403&#x60;, an unreadable space &#x60;404&#x60;. Every number counts only the channels the caller may read (&#x60;message.read&#x60;), intersected with the space; channels without a space (DMs) are not counted. No metric is grouped by person and no person is named. &#x60;from&#x60; and &#x60;to&#x60; are aligned down and up to the granularity (default &#x60;day&#x60;): &#x60;hour&#x60; in UTC, &#x60;day&#x60;, &#x60;week&#x60; (ISO, Monday) and &#x60;month&#x60; from local midnight of &#x60;stats.time_zone&#x60;, returned as UTC instants; a range longer than &#x60;stats.max_range&#x60;, with more than &#x60;api.stats_max_buckets&#x60; buckets or with &#x60;from &gt;&#x3D; to&#x60; is &#x60;400 validation_failed&#x60;. Buckets without a value are left out. The result is cached per caller scope for &#x60;stats.cache_ttl&#x60;.
  ///
  /// Parameters:
  /// * [from] - Start of the range (inclusive).
  /// * [to] - End of the range (exclusive).
  /// * [granularity] - Bucket size; default `day`.
  /// * [spaceId] - Restrict to a space.
  /// * [agentId] - Restrict to an agent.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [StatsSeries] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<StatsSeries>> getRunStats({
    required DateTime from,
    required DateTime to,
    StatsGranularity? granularity,
    String? spaceId,
    String? agentId,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/stats/runs';
    final _options = Options(
      method: r'GET',
      headers: <String, dynamic>{...?headers},
      extra: <String, dynamic>{
        'secure': <Map<String, String>>[
          {
            'type': 'apiKey',
            'name': 'consoleSession',
            'keyName': '__Host-console_session',
            'where': '',
          },
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
      if (granularity != null)
        r'granularity': encodeQueryParameter(
          _serializers,
          granularity,
          const FullType(StatsGranularity),
        ),
      if (spaceId != null)
        r'space_id': encodeQueryParameter(
          _serializers,
          spaceId,
          const FullType(String),
        ),
      if (agentId != null)
        r'agent_id': encodeQueryParameter(
          _serializers,
          agentId,
          const FullType(String),
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

    StatsSeries? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(StatsSeries),
            ) as StatsSeries;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<StatsSeries>(
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

  /// Backend policy view per agent
  /// Per agent: configured and reported billing mode, supported modes, switches, denied invocations (I2, I3), cost per mode and the rate-limit state reported by the adapter (docs/spec/25-visualization.md, section 6.2). Agents of channels visible to the caller; cost members only with &#x60;cost.read.aggregate&#x60;. Never credentials, subscriptions or runtime names of an instance.
  ///
  /// Parameters:
  /// * [from] - Start of the range (inclusive).
  /// * [to] - End of the range (exclusive).
  /// * [spaceId] - Restrict to a space; required without owner role.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [StatsBackendReport] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<StatsBackendReport>> getStatsBackends({
    required DateTime from,
    required DateTime to,
    String? spaceId,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/stats/backends';
    final _options = Options(
      method: r'GET',
      headers: <String, dynamic>{...?headers},
      extra: <String, dynamic>{
        'secure': <Map<String, String>>[
          {
            'type': 'apiKey',
            'name': 'consoleSession',
            'keyName': '__Host-console_session',
            'where': '',
          },
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
      if (spaceId != null)
        r'space_id': encodeQueryParameter(
          _serializers,
          spaceId,
          const FullType(String),
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

    StatsBackendReport? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(StatsBackendReport),
            ) as StatsBackendReport;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<StatsBackendReport>(
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

  /// Current live frame
  /// The latest live frame (&#x60;StatsLiveFrame&#x60;) of the instance or of one space, the same frame the gateway sends as &#x60;stats.live&#x60; (docs/spec/25-visualization.md, section 5). The server returns the base frame built for the current &#x60;stats.live_interval&#x60;; polling does not rebuild it. Filtered to the channels visible to the caller; cost members only with &#x60;cost.read.aggregate&#x60;. &#x60;403 feature_disabled&#x60; when &#x60;stats.live_enabled&#x60; is off.
  ///
  /// Parameters:
  /// * [spaceId] - Restrict to a space; required without owner role.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [StatsLiveFrame] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<StatsLiveFrame>> getStatsLive({
    String? spaceId,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/stats/live';
    final _options = Options(
      method: r'GET',
      headers: <String, dynamic>{...?headers},
      extra: <String, dynamic>{
        'secure': <Map<String, String>>[
          {
            'type': 'apiKey',
            'name': 'consoleSession',
            'keyName': '__Host-console_session',
            'where': '',
          },
          {'type': 'http', 'scheme': 'DPoP', 'name': 'dpopAuth'},
          {'type': 'http', 'scheme': 'bearer', 'name': 'bearerAuth'},
        ],
        ...?extra,
      },
      validateStatus: validateStatus,
    );

    final _queryParameters = <String, dynamic>{
      if (spaceId != null)
        r'space_id': encodeQueryParameter(
          _serializers,
          spaceId,
          const FullType(String),
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

    StatsLiveFrame? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(StatsLiveFrame),
            ) as StatsLiveFrame;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<StatsLiveFrame>(
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

  /// SLO traffic light from own measurement
  /// One entry per SLO of the Sloth files, computed from &#x60;stats_minutes&#x60; over the SLO period (docs/spec/25-visualization.md, section 6.4). Label \&quot;own measurement\&quot;: values may differ from Prometheus. Requires &#x60;stats.read&#x60; on the instance (owner); a space manager gets &#x60;403 forbidden&#x60;.
  ///
  /// Parameters:
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [StatsSloReport] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<StatsSloReport>> getStatsSlo({
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/stats/slo';
    final _options = Options(
      method: r'GET',
      headers: <String, dynamic>{...?headers},
      extra: <String, dynamic>{
        'secure': <Map<String, String>>[
          {
            'type': 'apiKey',
            'name': 'consoleSession',
            'keyName': '__Host-console_session',
            'where': '',
          },
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

    StatsSloReport? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(StatsSloReport),
            ) as StatsSloReport;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<StatsSloReport>(
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

  /// Minute buckets and runs for playback
  /// Minute aggregates from &#x60;stats_minutes&#x60; and run intervals for a range of at most &#x60;stats.timeline_max_range&#x60; inside &#x60;retention.stats_minutes&#x60; (docs/spec/25-visualization.md, section 6.5). Filtered to the channels visible to the caller. Rows without a channel (&#x60;sli.*&#x60;, gateway metrics) only with &#x60;stats.read&#x60; on the instance; &#x60;cost&#x60; only with &#x60;cost.read.aggregate&#x60;.
  ///
  /// Parameters:
  /// * [from] - Start of the range (inclusive), a full minute.
  /// * [to] - End of the range (exclusive), a full minute.
  /// * [spaceId] - Restrict to a space; required without owner role.
  /// * [metric] - Metrics to return; absent means all metrics the caller may see.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [StatsTimeline] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<StatsTimeline>> getStatsTimeline({
    required DateTime from,
    required DateTime to,
    String? spaceId,
    BuiltList<StatsMetric>? metric,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/stats/timeline';
    final _options = Options(
      method: r'GET',
      headers: <String, dynamic>{...?headers},
      extra: <String, dynamic>{
        'secure': <Map<String, String>>[
          {
            'type': 'apiKey',
            'name': 'consoleSession',
            'keyName': '__Host-console_session',
            'where': '',
          },
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
      if (spaceId != null)
        r'space_id': encodeQueryParameter(
          _serializers,
          spaceId,
          const FullType(String),
        ),
      if (metric != null)
        r'metric': encodeCollectionQueryParameter<StatsMetric>(
          _serializers,
          metric,
          const FullType(BuiltList, [FullType(StatsMetric)]),
          format: ListFormat.multi,
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

    StatsTimeline? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(StatsTimeline),
            ) as StatsTimeline;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<StatsTimeline>(
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

  /// Costs of a thread per run
  /// Valid cost entries of the thread (entries no correction supersedes), whatever channel they were booked in, per run (by &#x60;run_id&#x60; ascending, unpaginated) and summed per billing mode and currency without conversion. &#x60;tokens&#x60; are always present; amounts of runs in mode &#x60;personal&#x60; (their &#x60;costs&#x60; and their totals) appear only when the server key &#x60;billing.show_personal_estimates&#x60; is on (10, 12.2). Never a person: costs stand per run (20, 9). Without read on the thread the same 404 as for an unknown thread. A thread without entries answers empty lists.
  ///
  /// Parameters:
  /// * [threadId]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [CostBreakdown] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<CostBreakdown>> getThreadCosts({
    required String threadId,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/threads/{thread_id}/costs'.replaceAll(
      '{'
      r'thread_id'
      '}',
      encodeQueryParameter(
        _serializers,
        threadId,
        const FullType(String),
      ).toString(),
    );
    final _options = Options(
      method: r'GET',
      headers: <String, dynamic>{...?headers},
      extra: <String, dynamic>{
        'secure': <Map<String, String>>[
          {
            'type': 'apiKey',
            'name': 'consoleSession',
            'keyName': '__Host-console_session',
            'where': '',
          },
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

    CostBreakdown? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(CostBreakdown),
            ) as CostBreakdown;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<CostBreakdown>(
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

  /// Tool calls by tool name
  /// Metrics per tool name and bucket of the start of the call: &#x60;calls&#x60;, &#x60;failed&#x60;, &#x60;duration_ms_p50&#x60; (finished calls). Grouped by the tool name from &#x60;run_steps&#x60; (&#x60;group_key&#x60;); no step detail, no input and no output. Authorized with &#x60;stats.read&#x60; on the instance (owner) or, with &#x60;space_id&#x60;, on that space (&#x60;manage&#x60;); without &#x60;space_id&#x60; and without owner status &#x60;403&#x60;, an unreadable space &#x60;404&#x60;. Every number counts only the channels the caller may read (&#x60;message.read&#x60;), intersected with the space; channels without a space (DMs) are not counted. No metric is grouped by person and no person is named. &#x60;from&#x60; and &#x60;to&#x60; are aligned down and up to the granularity (default &#x60;day&#x60;): &#x60;hour&#x60; in UTC, &#x60;day&#x60;, &#x60;week&#x60; (ISO, Monday) and &#x60;month&#x60; from local midnight of &#x60;stats.time_zone&#x60;, returned as UTC instants; a range longer than &#x60;stats.max_range&#x60;, with more than &#x60;api.stats_max_buckets&#x60; buckets or with &#x60;from &gt;&#x3D; to&#x60; is &#x60;400 validation_failed&#x60;. Buckets without a value are left out. The result is cached per caller scope for &#x60;stats.cache_ttl&#x60;.
  ///
  /// Parameters:
  /// * [from] - Start of the range (inclusive).
  /// * [to] - End of the range (exclusive).
  /// * [granularity] - Bucket size; default `day`.
  /// * [spaceId] - Restrict to a space.
  /// * [agentId] - Restrict to an agent.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [StatsSeries] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<StatsSeries>> getToolCallStats({
    required DateTime from,
    required DateTime to,
    StatsGranularity? granularity,
    String? spaceId,
    String? agentId,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/stats/tool-calls';
    final _options = Options(
      method: r'GET',
      headers: <String, dynamic>{...?headers},
      extra: <String, dynamic>{
        'secure': <Map<String, String>>[
          {
            'type': 'apiKey',
            'name': 'consoleSession',
            'keyName': '__Host-console_session',
            'where': '',
          },
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
      if (granularity != null)
        r'granularity': encodeQueryParameter(
          _serializers,
          granularity,
          const FullType(StatsGranularity),
        ),
      if (spaceId != null)
        r'space_id': encodeQueryParameter(
          _serializers,
          spaceId,
          const FullType(String),
        ),
      if (agentId != null)
        r'agent_id': encodeQueryParameter(
          _serializers,
          agentId,
          const FullType(String),
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

    StatsSeries? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(StatsSeries),
            ) as StatsSeries;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<StatsSeries>(
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

  /// Usage per space
  /// Metrics per space and bucket: &#x60;messages&#x60; (messages not deleted) and by origin &#x60;messages_human&#x60;, &#x60;messages_agent&#x60;, &#x60;messages_app&#x60;, &#x60;messages_import&#x60;; &#x60;threads_created&#x60;, &#x60;threads_done&#x60; (threads closed in the bucket); &#x60;runs&#x60; (runs that ended in the bucket); &#x60;active_people&#x60; (distinct people with at least one message, only for granularity &#x60;week&#x60; or &#x60;month&#x60;; below &#x60;privacy.stats_min_group_size&#x60; the value is withheld and named in &#x60;below_min_group&#x60;); &#x60;storage_bytes&#x60; (current sum of the files of the space, in the last bucket only). Grouped by space (&#x60;group_key&#x60; is the space ID). Authorized with &#x60;stats.read&#x60; on the instance (owner) or, with &#x60;space_id&#x60;, on that space (&#x60;manage&#x60;); without &#x60;space_id&#x60; and without owner status &#x60;403&#x60;, an unreadable space &#x60;404&#x60;. Every number counts only the channels the caller may read (&#x60;message.read&#x60;), intersected with the space; channels without a space (DMs) are not counted. No metric is grouped by person and no person is named. &#x60;from&#x60; and &#x60;to&#x60; are aligned down and up to the granularity (default &#x60;day&#x60;): &#x60;hour&#x60; in UTC, &#x60;day&#x60;, &#x60;week&#x60; (ISO, Monday) and &#x60;month&#x60; from local midnight of &#x60;stats.time_zone&#x60;, returned as UTC instants; a range longer than &#x60;stats.max_range&#x60;, with more than &#x60;api.stats_max_buckets&#x60; buckets or with &#x60;from &gt;&#x3D; to&#x60; is &#x60;400 validation_failed&#x60;. Buckets without a value are left out. The result is cached per caller scope for &#x60;stats.cache_ttl&#x60;.
  ///
  /// Parameters:
  /// * [from] - Start of the range (inclusive).
  /// * [to] - End of the range (exclusive).
  /// * [granularity] - Bucket size; default `day`.
  /// * [spaceId] - Restrict to a space; required without owner role.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [StatsSeries] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<StatsSeries>> getUsageStats({
    required DateTime from,
    required DateTime to,
    StatsGranularity? granularity,
    String? spaceId,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/stats/usage';
    final _options = Options(
      method: r'GET',
      headers: <String, dynamic>{...?headers},
      extra: <String, dynamic>{
        'secure': <Map<String, String>>[
          {
            'type': 'apiKey',
            'name': 'consoleSession',
            'keyName': '__Host-console_session',
            'where': '',
          },
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
      if (granularity != null)
        r'granularity': encodeQueryParameter(
          _serializers,
          granularity,
          const FullType(StatsGranularity),
        ),
      if (spaceId != null)
        r'space_id': encodeQueryParameter(
          _serializers,
          spaceId,
          const FullType(String),
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

    StatsSeries? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(StatsSeries),
            ) as StatsSeries;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<StatsSeries>(
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

  /// Cost estimates per run for reconciliation
  /// For the reconciliation app with scope &#x60;costs:reconcile&#x60; and for owners in a session, without step-up (05, R-AZ-28). One item per run of the given agents with cost entries of mode &#x60;organization&#x60; whose &#x60;occurred_at&#x60; lies in the range, sorted by &#x60;run_id&#x60;; runs whose entries in the range mix currencies are left out. Entries of mode &#x60;personal&#x60; never appear (03 &#x60;cost_entries&#x60;). Items name no channel, thread or content; the list is evaluated on the instance and not filtered by visible channels (06, 4 point 2).
  ///
  /// Parameters:
  /// * [from] - Start of the range (inclusive).
  /// * [to] - End of the range (exclusive).
  /// * [agentId] - Agents whose runs are listed; repeat the parameter for several agents.
  /// * [cursor] - Cursor from `next_cursor` of the previous page.
  /// * [limit]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [CostEstimatePage] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<CostEstimatePage>> listCostEstimates({
    required DateTime from,
    required DateTime to,
    required BuiltList<String> agentId,
    String? cursor,
    int? limit,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/costs/estimates';
    final _options = Options(
      method: r'GET',
      headers: <String, dynamic>{...?headers},
      extra: <String, dynamic>{
        'secure': <Map<String, String>>[
          {
            'type': 'apiKey',
            'name': 'consoleSession',
            'keyName': '__Host-console_session',
            'where': '',
          },
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
      r'from': encodeQueryParameter(
        _serializers,
        from,
        const FullType(DateTime),
      ),
      r'to': encodeQueryParameter(_serializers, to, const FullType(DateTime)),
      r'agent_id': encodeCollectionQueryParameter<String>(
        _serializers,
        agentId,
        const FullType(BuiltList, [FullType(String)]),
        format: ListFormat.multi,
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

    CostEstimatePage? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(CostEstimatePage),
            ) as CostEstimatePage;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<CostEstimatePage>(
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

  /// List prices
  /// Every row of the price table, closed ones included, ordered by &#x60;provider&#x60;, &#x60;model&#x60;, &#x60;unit&#x60;, &#x60;valid_from&#x60; and &#x60;id&#x60;; &#x60;valid_to&#x60; is absent while a row is open. A small unpaginated list (06, 6 point 6). Owner only; an owner&#39;s personal token gets 403.
  ///
  /// Parameters:
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [PriceList] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<PriceList>> listPrices({
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/prices';
    final _options = Options(
      method: r'GET',
      headers: <String, dynamic>{...?headers},
      extra: <String, dynamic>{
        'secure': <Map<String, String>>[
          {
            'type': 'apiKey',
            'name': 'consoleSession',
            'keyName': '__Host-console_session',
            'where': '',
          },
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

    PriceList? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(PriceList),
            ) as PriceList;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<PriceList>(
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

  /// Reconcile estimates with invoices
  /// For an owner in a session with a fresh step-up; app, agent and personal tokens are refused (05, R-AZ-03). Stores new &#x60;reconciled&#x60; entries next to the estimates and never changes or deletes an entry (10, 12.4). An entry is matched when its run has at least one valid cost entry with &#x60;occurred_at&#x60; in the half-open period &#x60;[period_start, period_end)&#x60; and all of them have mode &#x60;organization&#x60;, the given &#x60;provider&#x60; and the entry&#39;s &#x60;currency&#x60;; otherwise its run ID is listed in &#x60;unmatched_run_ids&#x60;. The amount of a matched run is split over its valid entries in the period in proportion to their amounts, the remainder on the largest entry; a run whose valid entries in the period are already reconciled with the same sum is not written again. The job &#x60;billing.reconcile&#x60; then applies the entries to the run and agent session totals.
  ///
  /// Parameters:
  /// * [costReconcile]
  /// * [idempotencyKey] - UUIDv7 chosen by the client per logical operation. A repeat with the same request returns the stored response; a different request gives 422 `idempotency_key_reuse`; a repeat while the first is running gives 409 `request_in_progress`. Kept for `sync.idempotency_key_ttl`.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [CostReconcileResult] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<CostReconcileResult>> reconcileCosts({
    required CostReconcile costReconcile,
    String? idempotencyKey,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/costs/reconcile';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        if (idempotencyKey != null) r'Idempotency-Key': idempotencyKey,
        ...?headers,
      },
      extra: <String, dynamic>{
        'secure': <Map<String, String>>[
          {
            'type': 'apiKey',
            'name': 'consoleSession',
            'keyName': '__Host-console_session',
            'where': '',
          },
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
      const _type = FullType(CostReconcile);
      _bodyData = _serializers.serialize(costReconcile, specifiedType: _type);
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

    CostReconcileResult? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(CostReconcileResult),
            ) as CostReconcileResult;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<CostReconcileResult>(
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

  /// Submit anonymous client measurements
  /// Only when &#x60;privacy.client_metrics&#x60; is on; otherwise &#x60;403 feature_disabled&#x60;. At most once per &#x60;client.metrics.flush_interval&#x60; (15, section 17).
  ///
  /// Parameters:
  /// * [clientMetricsBatch]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> submitClientMetrics({
    required ClientMetricsBatch clientMetricsBatch,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/client-metrics';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{...?headers},
      extra: <String, dynamic>{
        'secure': <Map<String, String>>[
          {
            'type': 'apiKey',
            'name': 'consoleSession',
            'keyName': '__Host-console_session',
            'where': '',
          },
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
      const _type = FullType(ClientMetricsBatch);
      _bodyData = _serializers.serialize(
        clientMetricsBatch,
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

    return _response;
  }

  /// Submit reconciled cost entries (app)
  /// For the reconciliation app with scope &#x60;costs:reconcile&#x60;, without step-up. Only adds &#x60;reconciled&#x60; entries to existing runs; deletes nothing (10, 12.4). Matching, period, split and repeat rules as &#x60;reconcileCosts&#x60;: an entry is matched when its run has at least one valid cost entry with &#x60;occurred_at&#x60; in &#x60;[period_start, period_end)&#x60; and all of them have mode &#x60;organization&#x60;, the given &#x60;provider&#x60; and the entry&#39;s &#x60;currency&#x60;; unmatched run IDs are listed in &#x60;unmatched_run_ids&#x60;; a run already reconciled with the same sum is not written again. The job &#x60;billing.reconcile&#x60; then applies the entries to the run and agent session totals.
  ///
  /// Parameters:
  /// * [costReconcile]
  /// * [idempotencyKey] - UUIDv7 chosen by the client per logical operation. A repeat with the same request returns the stored response; a different request gives 422 `idempotency_key_reuse`; a repeat while the first is running gives 409 `request_in_progress`. Kept for `sync.idempotency_key_ttl`.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [CostReconcileResult] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<CostReconcileResult>> submitCostReconciliation({
    required CostReconcile costReconcile,
    String? idempotencyKey,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/costs/reconciliations';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        if (idempotencyKey != null) r'Idempotency-Key': idempotencyKey,
        ...?headers,
      },
      extra: <String, dynamic>{
        'secure': <Map<String, String>>[
          {
            'type': 'apiKey',
            'name': 'consoleSession',
            'keyName': '__Host-console_session',
            'where': '',
          },
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
      const _type = FullType(CostReconcile);
      _bodyData = _serializers.serialize(costReconcile, specifiedType: _type);
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

    CostReconcileResult? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(CostReconcileResult),
            ) as CostReconcileResult;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<CostReconcileResult>(
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
