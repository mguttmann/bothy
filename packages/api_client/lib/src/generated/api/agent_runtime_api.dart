//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

import 'dart:async';

import 'package:built_value/json_object.dart';
import 'package:built_value/serializer.dart';
import 'package:dio/dio.dart';

import 'package:api_client/src/generated/api_util.dart';
import 'package:api_client/src/generated/model/agent_channel_config.dart';
import 'package:api_client/src/generated/model/agent_channel_config_list.dart';
import 'package:api_client/src/generated/model/agent_session.dart';
import 'package:api_client/src/generated/model/agent_session_report.dart';
import 'package:api_client/src/generated/model/agent_triggerers.dart';
import 'package:api_client/src/generated/model/autocomplete_answer.dart';
import 'package:api_client/src/generated/model/consumer_ack.dart';
import 'package:api_client/src/generated/model/consumer_state.dart';
import 'package:api_client/src/generated/model/event_page.dart';
import 'package:api_client/src/generated/model/heartbeat.dart';
import 'package:api_client/src/generated/model/heartbeat_ack.dart';
import 'package:api_client/src/generated/model/interaction.dart';
import 'package:api_client/src/generated/model/interaction_respond.dart';
import 'package:api_client/src/generated/model/invocation_token_grant.dart';
import 'package:api_client/src/generated/model/invocation_token_renew.dart';
import 'package:api_client/src/generated/model/problem.dart';
import 'package:api_client/src/generated/model/run.dart';
import 'package:api_client/src/generated/model/run_await_input.dart';
import 'package:api_client/src/generated/model/run_finish.dart';
import 'package:api_client/src/generated/model/run_patch.dart';
import 'package:api_client/src/generated/model/run_patch_result.dart';
import 'package:api_client/src/generated/model/run_start.dart';
import 'package:api_client/src/generated/model/transcript.dart';
import 'package:api_client/src/generated/model/transcript_submit.dart';

class AgentRuntimeApi {
  final Dio _dio;

  final Serializers _serializers;

  const AgentRuntimeApi(this._dio, this._serializers);

  /// Acknowledge events up to a sequence
  ///
  ///
  /// Parameters:
  /// * [consumer]
  /// * [consumerAck]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [ConsumerState] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<ConsumerState>> ackAgentEvents({
    required String consumer,
    required ConsumerAck consumerAck,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/agents/me/consumers/{consumer}/ack'.replaceAll(
      '{'
      r'consumer'
      '}',
      encodeQueryParameter(
        _serializers,
        consumer,
        const FullType(String),
      ).toString(),
    );
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
      const _type = FullType(ConsumerAck);
      _bodyData = _serializers.serialize(consumerAck, specifiedType: _type);
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

    ConsumerState? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(ConsumerState),
            ) as ConsumerState;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<ConsumerState>(
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

  /// Answer an autocomplete request
  /// The owner of the command answers an open request of &#x60;command.autocomplete_requested&#x60;; the first valid answer goes back to the person. &#x60;404&#x60; for an unknown or forgotten request, &#x60;410 gone&#x60; after the wait ended, &#x60;204&#x60; without effect for an answer after the first one. &#x60;400 validation_failed&#x60; for more than &#x60;command.autocomplete_max_choices&#x60; choices (&#x60;too_many_items&#x60;), an empty name or value (&#x60;too_short&#x60;) or a name or value that is no display text (&#x60;invalid_utf8&#x60;, &#x60;control_character&#x60;, &#x60;bidi_control&#x60;); the request stays open.
  ///
  /// Parameters:
  /// * [requestId]
  /// * [autocompleteAnswer]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> answerAutocomplete({
    required String requestId,
    required AutocompleteAnswer autocompleteAnswer,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/command-autocompletions/{request_id}/answer'.replaceAll(
      '{'
      r'request_id'
      '}',
      encodeQueryParameter(
        _serializers,
        requestId,
        const FullType(String),
      ).toString(),
    );
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
      const _type = FullType(AutocompleteAnswer);
      _bodyData = _serializers.serialize(
        autocompleteAnswer,
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

  /// Wait for input from a person
  ///
  ///
  /// Parameters:
  /// * [runId]
  /// * [runAwaitInput]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [Run] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<Run>> awaitRunInput({
    required String runId,
    required RunAwaitInput runAwaitInput,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/runs/{run_id}/await-input'.replaceAll(
      '{'
      r'run_id'
      '}',
      encodeQueryParameter(
        _serializers,
        runId,
        const FullType(String),
      ).toString(),
    );
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
      const _type = FullType(RunAwaitInput);
      _bodyData = _serializers.serialize(runAwaitInput, specifiedType: _type);
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

    Run? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(Run),
            ) as Run;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<Run>(
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

  /// End a run
  /// Repeating the same final state is idempotent and returns 200.
  ///
  /// Parameters:
  /// * [runId]
  /// * [runFinish]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [Run] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<Run>> finishRun({
    required String runId,
    required RunFinish runFinish,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/runs/{run_id}/finish'.replaceAll(
      '{'
      r'run_id'
      '}',
      encodeQueryParameter(
        _serializers,
        runId,
        const FullType(String),
      ).toString(),
    );
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
      const _type = FullType(RunFinish);
      _bodyData = _serializers.serialize(runFinish, specifiedType: _type);
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

    Run? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(Run),
            ) as Run;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<Run>(
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

  /// List who may trigger this agent
  /// Principals with effective &#x60;trigger&#x60; on a channel the agent operates, with &#x60;permission_version&#x60;. The adapter keeps it as allow list and reloads on &#x60;access.changed&#x60; (09, 15).
  ///
  /// Parameters:
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [AgentTriggerers] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<AgentTriggerers>> getAgentTriggerers({
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/agents/me/triggerers';
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

    AgentTriggerers? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(AgentTriggerers),
            ) as AgentTriggerers;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<AgentTriggerers>(
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

  /// Read the cursor of a consumer
  ///
  ///
  /// Parameters:
  /// * [consumer]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [ConsumerState] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<ConsumerState>> getConsumerState({
    required String consumer,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/agents/me/consumers/{consumer}'.replaceAll(
      '{'
      r'consumer'
      '}',
      encodeQueryParameter(
        _serializers,
        consumer,
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

    ConsumerState? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(ConsumerState),
            ) as ConsumerState;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<ConsumerState>(
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

  /// Read prompt, skills and project binding of a served channel
  ///
  ///
  /// Parameters:
  /// * [channelId]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [AgentChannelConfig] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<AgentChannelConfig>> getServedChannelConfig({
    required String channelId,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/agents/me/channels/{channel_id}/config'.replaceAll(
      '{'
      r'channel_id'
      '}',
      encodeQueryParameter(
        _serializers,
        channelId,
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

    AgentChannelConfig? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(AgentChannelConfig),
            ) as AgentChannelConfig;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<AgentChannelConfig>(
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

  /// List channels the agent serves
  ///
  ///
  /// Parameters:
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [AgentChannelConfigList] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<AgentChannelConfigList>> listServedChannels({
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/agents/me/channels';
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

    AgentChannelConfigList? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(AgentChannelConfigList),
            ) as AgentChannelConfigList;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<AgentChannelConfigList>(
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

  /// Apply a live-box patch
  /// Same as gateway op &#x60;run.update&#x60;. Paths outside the allowed set and &#x60;state&#x60; are rejected. A wrong &#x60;base_version&#x60; gives 409 &#x60;base_version_conflict&#x60; with the current document in &#x60;current&#x60;.
  ///
  /// Parameters:
  /// * [runId]
  /// * [runPatch]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [RunPatchResult] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<RunPatchResult>> patchRun({
    required String runId,
    required RunPatch runPatch,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/runs/{run_id}'.replaceAll(
      '{'
      r'run_id'
      '}',
      encodeQueryParameter(
        _serializers,
        runId,
        const FullType(String),
      ).toString(),
    );
    final _options = Options(
      method: r'PATCH',
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
      const _type = FullType(RunPatch);
      _bodyData = _serializers.serialize(runPatch, specifiedType: _type);
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

    RunPatchResult? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(RunPatchResult),
            ) as RunPatchResult;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<RunPatchResult>(
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

  /// Fetch events for a consumer without WebSocket
  /// Delivers from &#x60;acked_seq + 1&#x60;. Invocation events carry a fresh &#x60;invocation_token&#x60;. Unacknowledged events are delivered again after &#x60;agent.ack_timeout&#x60;.
  ///
  /// Parameters:
  /// * [consumer]
  /// * [limit] - At most `agent.max_inflight_events`.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [EventPage] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<EventPage>> pollAgentEvents({
    required String consumer,
    int? limit,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/agents/me/consumers/{consumer}/events'.replaceAll(
      '{'
      r'consumer'
      '}',
      encodeQueryParameter(
        _serializers,
        consumer,
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

    final _queryParameters = <String, dynamic>{
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

    EventPage? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(EventPage),
            ) as EventPage;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<EventPage>(
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

  /// Get a fresh invocation token for an own waiting run
  /// For a run in &#x60;queued&#x60; or &#x60;waiting_input&#x60; whose invocation token expired before the start (09 section 4.5.1). The body carries the current token; its expiry is not checked, but its signature, audience, run and &#x60;jti&#x60; are: the &#x60;jti&#x60; equals the current one (&#x60;runs.invocation_jti&#x60;) or the one the last renewal replaced (&#x60;runs.renewed_from_jti&#x60;, retry after a lost answer), and it is not consumed by a start (&#x60;runs.started_jti&#x60; is NULL or differs, SQL &#x60;started_jti IS DISTINCT FROM jti&#x60;). A retry with the replaced token issues a fresh token again; the run still starts at most once. The server repeats every check it runs before a delivery (&#x60;agent.invoke&#x60; for the triggering person with current permissions, I2, SR2, SR3); on failure the run becomes &#x60;failed&#x60; and the answer is 403 &#x60;agent_invocation_denied&#x60;. A replaced or consumed token gets 403 &#x60;invocation_token_invalid&#x60; with &#x60;reason: superseded&#x60;; a run that already started gets 409 &#x60;already_started&#x60;.
  ///
  /// Parameters:
  /// * [runId]
  /// * [invocationTokenRenew]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [InvocationTokenGrant] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<InvocationTokenGrant>> renewInvocationToken({
    required String runId,
    required InvocationTokenRenew invocationTokenRenew,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/runs/{run_id}/invocation-token'.replaceAll(
      '{'
      r'run_id'
      '}',
      encodeQueryParameter(
        _serializers,
        runId,
        const FullType(String),
      ).toString(),
    );
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
      const _type = FullType(InvocationTokenRenew);
      _bodyData = _serializers.serialize(
        invocationTokenRenew,
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

    InvocationTokenGrant? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(InvocationTokenGrant),
            ) as InvocationTokenGrant;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<InvocationTokenGrant>(
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

  /// Report the external session reference and model
  ///
  ///
  /// Parameters:
  /// * [threadId]
  /// * [agentSessionReport]
  /// * [idempotencyKey] - UUIDv7 chosen by the client per logical operation. A repeat with the same request returns the stored response; a different request gives 422 `idempotency_key_reuse`; a repeat while the first is running gives 409 `request_in_progress`. Kept for `sync.idempotency_key_ttl`.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [AgentSession] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<AgentSession>> reportAgentSession({
    required String threadId,
    required AgentSessionReport agentSessionReport,
    String? idempotencyKey,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/threads/{thread_id}/agent-sessions/report'.replaceAll(
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
      const _type = FullType(AgentSessionReport);
      _bodyData = _serializers.serialize(
        agentSessionReport,
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

    AgentSession? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(AgentSession),
            ) as AgentSession;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<AgentSession>(
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

  /// Acknowledge or answer an interaction
  ///
  ///
  /// Parameters:
  /// * [interactionId]
  /// * [interactionRespond]
  /// * [idempotencyKey] - UUIDv7 chosen by the client per logical operation. A repeat with the same request returns the stored response; a different request gives 422 `idempotency_key_reuse`; a repeat while the first is running gives 409 `request_in_progress`. Kept for `sync.idempotency_key_ttl`.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [Interaction] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<Interaction>> respondToInteraction({
    required String interactionId,
    required InteractionRespond interactionRespond,
    String? idempotencyKey,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/interactions/{interaction_id}/respond'.replaceAll(
      '{'
      r'interaction_id'
      '}',
      encodeQueryParameter(
        _serializers,
        interactionId,
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
      const _type = FullType(InteractionRespond);
      _bodyData = _serializers.serialize(
        interactionRespond,
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

    Interaction? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(Interaction),
            ) as Interaction;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<Interaction>(
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

  /// Resume a run that waited for input
  /// &#x60;waiting_input -&gt; running&#x60; with the token of the interaction or follow-up delivery. The consumed start token (its &#x60;jti&#x60; equals &#x60;runs.started_jti&#x60;) gets 403 &#x60;invocation_token_invalid&#x60; with &#x60;reason: superseded&#x60; (09 section 4.5).
  ///
  /// Parameters:
  /// * [runId]
  /// * [runStart]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [Run] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<Run>> resumeRun({
    required String runId,
    required RunStart runStart,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/runs/{run_id}/resume'.replaceAll(
      '{'
      r'run_id'
      '}',
      encodeQueryParameter(
        _serializers,
        runId,
        const FullType(String),
      ).toString(),
    );
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
      const _type = FullType(RunStart);
      _bodyData = _serializers.serialize(runStart, specifiedType: _type);
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

    Run? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(Run),
            ) as Run;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<Run>(
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

  /// Report state and actual billing mode
  /// The adapter reports the whole state of its runtime (&#x60;agent_heartbeat.v1&#x60;, 09 section 4.8); the member &#x60;agent&#x60; of the gateway op &#x60;heartbeat&#x60; has the same effect. Only an agent may call it (&#x60;agent-self&#x60;); the body carries no agent id. The last heartbeat wins: an absent optional member means \&quot;not reported\&quot;, except &#x60;project_readiness&#x60;, which is applied only when present and changed since the last list this server process applied (&#x60;ApplyProjectReadiness&#x60; of M2-03, in its own transaction).  The server validates every member and the closed keys of &#x60;details&#x60; itself, also the patterns of names, versions and codes; a violation answers &#x60;400 validation_failed&#x60; with every field error, &#x60;rate_limit&#x60; answers &#x60;errors[].code&#x60; &#x60;not_enabled&#x60; until it is built. A change of &#x60;reported_billing_mode&#x60; or &#x60;reported_tool_class&#x60;, also from or to \&quot;not reported\&quot;, is stored at once and audited with action &#x60;agent.heartbeat&#x60;; the other fields are stored at most once per &#x60;agent.heartbeat_persist_interval&#x60;. The presence becomes &#x60;offline&#x60; once &#x60;agent.presence_timeout&#x60; passes without a heartbeat. The server compares the reported with the configured mode at each heartbeat and returns the warnings of &#x60;AgentRuntimeStatus.warnings&#x60;; the reported mode is evidence for display, warning, audit and alarm and never a permission (BR-05). &#x60;interval_ms&#x60; is &#x60;sync.heartbeat_interval&#x60;.
  ///
  /// Parameters:
  /// * [heartbeat]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [HeartbeatAck] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<HeartbeatAck>> sendHeartbeat({
    required Heartbeat heartbeat,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/agents/me/heartbeat';
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
      const _type = FullType(Heartbeat);
      _bodyData = _serializers.serialize(heartbeat, specifiedType: _type);
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

    HeartbeatAck? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(HeartbeatAck),
            ) as HeartbeatAck;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<HeartbeatAck>(
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

  /// Start a queued run
  /// Compare-and-swap &#x60;queued -&gt; running&#x60; after checking the invocation token. Repeating the call with the same token after success returns 200 with &#x60;Idempotent-Replay: true&#x60; while the run is &#x60;running&#x60;; any other token gets 409 &#x60;already_started&#x60;. Once the run waits for input, the start token is consumed and gets 403 &#x60;invocation_token_invalid&#x60; with &#x60;reason: superseded&#x60;. A different &#x60;billing_mode_used&#x60; fails the run with &#x60;billing_mode_mismatch&#x60;; a &#x60;tool_class_used&#x60; above the &#x60;tool_class&#x60; of the token fails it with 409 &#x60;tool_class_mismatch&#x60;. A token error gives 403 &#x60;invocation_token_invalid&#x60; with &#x60;reason&#x60; &#x60;expired&#x60; (renew with &#x60;renewInvocationToken&#x60;, then start again), &#x60;superseded&#x60; (a newer delivery replaced it) or &#x60;invalid&#x60; (09 section 4.5).
  ///
  /// Parameters:
  /// * [runId]
  /// * [runStart]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [Run] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<Run>> startRun({
    required String runId,
    required RunStart runStart,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/runs/{run_id}/start'.replaceAll(
      '{'
      r'run_id'
      '}',
      encodeQueryParameter(
        _serializers,
        runId,
        const FullType(String),
      ).toString(),
    );
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
      const _type = FullType(RunStart);
      _bodyData = _serializers.serialize(runStart, specifiedType: _type);
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

    Run? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(Run),
            ) as Run;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<Run>(
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

  /// Store a transcript created by the agent
  ///
  ///
  /// Parameters:
  /// * [fileId]
  /// * [transcriptSubmit]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [Transcript] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<Transcript>> submitTranscript({
    required String fileId,
    required TranscriptSubmit transcriptSubmit,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/files/{file_id}/transcript'.replaceAll(
      '{'
      r'file_id'
      '}',
      encodeQueryParameter(
        _serializers,
        fileId,
        const FullType(String),
      ).toString(),
    );
    final _options = Options(
      method: r'PUT',
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
      const _type = FullType(TranscriptSubmit);
      _bodyData = _serializers.serialize(
        transcriptSubmit,
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

    Transcript? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(Transcript),
            ) as Transcript;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<Transcript>(
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
