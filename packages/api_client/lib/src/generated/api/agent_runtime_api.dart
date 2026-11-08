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
import 'package:api_client/src/generated/model/agent_interview_builder_answer.dart';
import 'package:api_client/src/generated/model/agent_page.dart';
import 'package:api_client/src/generated/model/agent_session.dart';
import 'package:api_client/src/generated/model/agent_session_report.dart';
import 'package:api_client/src/generated/model/agent_triggerers.dart';
import 'package:api_client/src/generated/model/api_token_created.dart';
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
import 'package:api_client/src/generated/model/run_wake.dart';
import 'package:api_client/src/generated/model/run_wake_check_result.dart';
import 'package:api_client/src/generated/model/run_wake_create.dart';
import 'package:api_client/src/generated/model/run_wake_fire.dart';
import 'package:api_client/src/generated/model/run_wake_list.dart';
import 'package:api_client/src/generated/model/run_wake_recover.dart';
import 'package:api_client/src/generated/model/search_assist_answer.dart';
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

  /// Answer an interview request
  /// The builder agent answers an open &#x60;agent.interview_requested&#x60; once (26 6.1 item 2, 6.2): &#x60;kind &#x3D; questions&#x60; with one or two questions of at most &#x60;agents.interview.answer_max_chars&#x60; characters, or &#x60;kind &#x3D; draft&#x60; with a draft of the shape of 26 4.2 plus &#x60;name&#x60;, &#x60;handle&#x60;, &#x60;description&#x60; and an optional &#x60;channel_suggestion&#x60;; and the usage in &#x60;costs&#x60;. &#x60;404&#x60; for an unknown or forgotten request and for every principal but the builder agent the request was sent to, &#x60;410 gone&#x60; after &#x60;expires_at&#x60;, &#x60;409 conflict&#x60; with &#x60;reason: already_answered&#x60; for a second answer. The server decodes the answer strictly and never trusts it: a member outside the schema, a wrong type or a count outside the schema (an empty list, more than two questions) is refused by the request validation (&#x60;400&#x60;, the request stays open until &#x60;expires_at&#x60;); a &#x60;kind&#x60; without its member or with the member of the other kind, a draft that reaches a surface of 26 5.5 (a frontmatter, an inline shell form or an &#x60;@&#x60; import outside the agent folder) or a text that is no display text ends the request with &#x60;builder_invalid&#x60; and answers &#x60;400 validation_failed&#x60;. Any other finding of 26 4.3 or of the catalogue check (&#x60;mcp_not_in_catalogue&#x60;) goes back to the person as a field error of the draft; nothing is stored.
  ///
  /// Parameters:
  /// * [requestId]
  /// * [agentInterviewBuilderAnswer]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> answerAgentInterview({
    required String requestId,
    required AgentInterviewBuilderAnswer agentInterviewBuilderAnswer,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/agent-interview-requests/{request_id}/answer'.replaceAll(
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
      const _type = FullType(AgentInterviewBuilderAnswer);
      _bodyData = _serializers.serialize(
        agentInterviewBuilderAnswer,
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

  /// Answer a search assistant request
  /// The assistant agent answers an open &#x60;search.assist_requested&#x60; (14, 4.10; 09, 10.6); the first valid answer goes back to the person. &#x60;404&#x60; for an unknown or forgotten request and for every principal but the addressed agent, &#x60;410 gone&#x60; after the wait ended, &#x60;204&#x60; without effect for an answer after the first one. &#x60;400 validation_failed&#x60; for a reference that was not sent or is named twice (&#x60;unknown_ref&#x60;, &#x60;duplicate&#x60;), more picks than &#x60;max_picks&#x60; or more terms than &#x60;search.assist.max_terms&#x60; (&#x60;too_many_items&#x60;), an empty reason or term (&#x60;too_short&#x60;), a reason longer than &#x60;search.assist.reason_max_chars&#x60; or a term longer than 64 characters (&#x60;too_long&#x60;) or a text that is no display text (&#x60;invalid_utf8&#x60;, &#x60;control_character&#x60;, &#x60;bidi_control&#x60;); the request stays open.
  ///
  /// Parameters:
  /// * [requestId]
  /// * [searchAssistAnswer]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> answerSearchAssist({
    required String requestId,
    required SearchAssistAnswer searchAssistAnswer,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/search-assist-requests/{request_id}/answer'.replaceAll(
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
      const _type = FullType(SearchAssistAnswer);
      _bodyData = _serializers.serialize(
        searchAssistAnswer,
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

  /// Check a launch by a runner that did not accept the run
  /// Before a runner launches any process of a run it did not accept itself (a repeated try, the next attempt of an adopted run, the retry of a compaction, a start over), it asks the server (09 18.19.6, R-AG-48): the run is still &#x60;running&#x60;, its person still has &#x60;trigger&#x60; on the agent in the channel and the agent is still allowed there, the billing rules of 10 hold, and for tool class &#x60;host&#x60; the agent&#39;s &#x60;tool_class&#x60; is still &#x60;host&#x60; and SR2 holds for that person. On allow the answer carries the configured tool class and billing mode, which the runner compares with its own (&#x60;compare_actual&#x60;). A refusal is &#x60;409 conflict&#x60; with the reason of the failed check (&#x60;run_not_running&#x60; or the reason of a refused &#x60;agent.invoke&#x60;), audited as a denial; the server changes nothing, the runner finishes the run.
  ///
  /// Parameters:
  /// * [runId]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [RunWakeCheckResult] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<RunWakeCheckResult>> checkRunLaunch({
    required String runId,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/runs/{run_id}/launch-check'.replaceAll(
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
      validateStatus: validateStatus,
    );

    final _response = await _dio.request<Object>(
      _path,
      options: _options,
      cancelToken: cancelToken,
      onSendProgress: onSendProgress,
      onReceiveProgress: onReceiveProgress,
    );

    RunWakeCheckResult? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(RunWakeCheckResult),
            ) as RunWakeCheckResult;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<RunWakeCheckResult>(
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

  /// Check the next execution of a pending check
  /// Before every execution of a pending &#x60;command&#x60; and every &#x60;gh&#x60; read of a pending &#x60;pr_checks&#x60; the runner asks the server (09 5.8.3, R-AG-50): the wake-up is still &#x60;pending&#x60; and its &#x60;not_before&#x60; has passed; its person still has &#x60;trigger&#x60; on the agent in the channel and the agent is still allowed there; the channel is neither archived nor read-only; for &#x60;command&#x60; the agent&#39;s &#x60;tool_class&#x60; is still &#x60;host&#x60; and SR2 holds for that person; for &#x60;pr_checks&#x60; the person is still an owner. On allow the answer carries the configured tool class and billing mode (&#x60;compare_actual&#x60;). A failed check ends the wake-up &#x60;refused&#x60; with the reason of the check (for example &#x60;agent_not_reachable&#x60;, &#x60;channel_read_only&#x60;, &#x60;tool_class_owner_only&#x60;, &#x60;tool_class_mismatch&#x60;, &#x60;pr_checks_owner_only&#x60;), an audit entry, the note in the thread and the notification &#x60;wake_ended&#x60;, and answers &#x60;409 conflict&#x60; with that reason. A wake-up that already ended is &#x60;409 conflict&#x60; with &#x60;reason&#x60; &#x60;wake_not_pending&#x60;, one before its &#x60;not_before&#x60; &#x60;409 conflict&#x60; with &#x60;reason&#x60; &#x60;wake_not_due&#x60; (the wake-up stays pending).
  ///
  /// Parameters:
  /// * [wakeId]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [RunWakeCheckResult] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<RunWakeCheckResult>> checkRunWake({
    required String wakeId,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/wakes/{wake_id}/check'.replaceAll(
      '{'
      r'wake_id'
      '}',
      encodeQueryParameter(
        _serializers,
        wakeId,
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
      validateStatus: validateStatus,
    );

    final _response = await _dio.request<Object>(
      _path,
      options: _options,
      cancelToken: cancelToken,
      onSendProgress: onSendProgress,
      onReceiveProgress: onReceiveProgress,
    );

    RunWakeCheckResult? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(RunWakeCheckResult),
            ) as RunWakeCheckResult;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<RunWakeCheckResult>(
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

  /// Create a wake-up for an active run
  /// Creates a wake-up of the run&#39;s thread for the active run of the calling agent (09 5.8.2, stage AR): kind &#x60;at&#x60;, &#x60;pr_checks&#x60; or &#x60;command&#x60;, reason &#x60;agent&#x60; (from &#x60;bothy-wake&#x60;) or &#x60;usage_limit&#x60;. The wake-up acts for the run&#39;s &#x60;triggered_by&#x60;; it fires no earlier than &#x60;run.wake_min_delay&#x60; after its creation and no later than its &#x60;deadline&#x60; (at most &#x60;run.wake_max_wait&#x60;, for &#x60;usage_limit&#x60; counted from &#x60;not_before&#x60;). A &#x60;usage_limit&#x60; request carries the run&#39;s &#x60;outage&#x60;; a second request with the outage of a pending wake-up moves its &#x60;not_before&#x60; and answers it with &#x60;200&#x60;, a new outage gets its own wake-up, at most &#x60;run.usage_wake_max&#x60; per run. A refusal is &#x60;409 conflict&#x60; with the reason: &#x60;run_not_active&#x60;, &#x60;autonomous_not_allowed&#x60; (the person is no owner and the channel has no &#x60;allow_autonomous_runs&#x60;), &#x60;pr_checks_owner_only&#x60;, &#x60;tool_class_mismatch&#x60; (a &#x60;command&#x60; of an agent whose tool class is not &#x60;host&#x60;), &#x60;wake_max_pending&#x60;, &#x60;usage_wake_max&#x60;, or the reason of a refused &#x60;agent.invoke&#x60; of the person (for example &#x60;tool_class_owner_only&#x60;, &#x60;agent_not_reachable&#x60;, &#x60;channel_read_only&#x60;). A refused request stores nothing, except one with &#x60;deferred: true&#x60; and a refusal with &#x60;usage_wake_max&#x60;, which are stored in state &#x60;refused&#x60; with the note in the thread and the notification &#x60;wake_ended&#x60; (09 5.8.4). A wrong field is &#x60;400 validation_failed&#x60;. Audited.
  ///
  /// Parameters:
  /// * [runId]
  /// * [runWakeCreate]
  /// * [idempotencyKey] - UUIDv7 chosen by the client per logical operation. A repeat with the same request returns the stored response; a different request gives 422 `idempotency_key_reuse`; a repeat while the first is running gives 409 `request_in_progress`. Kept for `sync.idempotency_key_ttl`.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [RunWake] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<RunWake>> createRunWake({
    required String runId,
    required RunWakeCreate runWakeCreate,
    String? idempotencyKey,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/runs/{run_id}/wakes'.replaceAll(
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
      const _type = FullType(RunWakeCreate);
      _bodyData = _serializers.serialize(runWakeCreate, specifiedType: _type);
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

    RunWake? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(RunWake),
            ) as RunWake;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<RunWake>(
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

  /// Fire a checked wake-up, end it failed or return a skipped wake input
  /// The runner reports the outcome of a &#x60;pr_checks&#x60; or &#x60;command&#x60; wake-up (09 5.8.2): &#x60;fired&#x60; with the result text fires it with the checks of a new run, as a wake run when the thread has no active run, as a wake input &#x60;agent.wake_input&#x60; into the running run of the same person, and keeps it &#x60;pending&#x60; while another person&#39;s run is active; a refused fire ends it &#x60;refused&#x60; with note and notification. &#x60;failed&#x60; ends it &#x60;failed&#x60; with the reason after &#x60;runner.wake_check_failures&#x60; failures in a row. &#x60;input_skipped&#x60; reports a delivered wake input the runner did not write (&#x60;wake_input_check_failed&#x60;): the wake-up returns to &#x60;pending&#x60; and fires after that run as a wake run. A wake-up that ended is &#x60;409 conflict&#x60; with &#x60;reason&#x60; &#x60;wake_not_pending&#x60;; a fire before &#x60;not_before&#x60; is &#x60;409 conflict&#x60; with &#x60;reason&#x60; &#x60;wake_not_due&#x60;; an &#x60;at&#x60; wake-up is fired by the server only (&#x60;409 conflict&#x60;, &#x60;reason&#x60; &#x60;wake_fired_by_server&#x60;). Audited.
  ///
  /// Parameters:
  /// * [wakeId]
  /// * [runWakeFire]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [RunWake] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<RunWake>> fireRunWake({
    required String wakeId,
    required RunWakeFire runWakeFire,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/wakes/{wake_id}/fire'.replaceAll(
      '{'
      r'wake_id'
      '}',
      encodeQueryParameter(
        _serializers,
        wakeId,
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
      const _type = FullType(RunWakeFire);
      _bodyData = _serializers.serialize(runWakeFire, specifiedType: _type);
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

    RunWake? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(RunWake),
            ) as RunWake;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<RunWake>(
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

  /// Issue a token for a hosted agent
  /// Issues an API token for the built-in agent &#x60;agent_id&#x60; to its hosting runner (26 3.3 item 1). Only the runner named in the agent&#39;s &#x60;hosted_by&#x60; may call it, with its own DPoP-bound agent token, while both are active and the runner&#39;s last stored heartbeat reports &#x60;hosts_agents&#x60;; every other caller and every other case get &#x60;404&#x60; like an unknown agent, audited as a denial. The token has kind &#x60;agt&#x60;, is bound to the &#x60;dpop_jkt&#x60; of the calling token (a calling token without one is &#x60;409 conflict&#x60; with &#x60;reason&#x60; &#x60;dpop_required&#x60;) and expires after &#x60;agents.hosted_token_ttl&#x60;; every older active token of the agent from the same runner is revoked in the same transaction. One audit entry names the agent, the runner and the token prefix, never the value. The secret is shown once; a replay with the same &#x60;Idempotency-Key&#x60; answers &#x60;409 conflict&#x60; with &#x60;reason&#x60; &#x60;one_time_secret&#x60; (06, 7). A person never receives such a token: &#x60;createToken&#x60; for a built-in agent is &#x60;409 conflict&#x60; with &#x60;reason&#x60; &#x60;hosted_agent_token&#x60;.
  ///
  /// Parameters:
  /// * [agentId]
  /// * [idempotencyKey] - UUIDv7 chosen by the client per logical operation. A repeat with the same request returns the stored response; a different request gives 422 `idempotency_key_reuse`; a repeat while the first is running gives 409 `request_in_progress`. Kept for `sync.idempotency_key_ttl`.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [ApiTokenCreated] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<ApiTokenCreated>> issueHostedAgentToken({
    required String agentId,
    String? idempotencyKey,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/agents/me/hosted/{agent_id}/token'.replaceAll(
      '{'
      r'agent_id'
      '}',
      encodeQueryParameter(
        _serializers,
        agentId,
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
      validateStatus: validateStatus,
    );

    final _response = await _dio.request<Object>(
      _path,
      options: _options,
      cancelToken: cancelToken,
      onSendProgress: onSendProgress,
      onReceiveProgress: onReceiveProgress,
    );

    ApiTokenCreated? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(ApiTokenCreated),
            ) as ApiTokenCreated;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<ApiTokenCreated>(
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

  /// List the built-in agents this runner hosts
  /// The active agents whose &#x60;hosted_by&#x60; is the calling agent (26 3.2), ascending by id. A Claude Code runner with &#x60;runner.hosted_agents&#x60; reads it at start, on &#x60;agent.hosting_changed&#x60; and every &#x60;runner.channels_resync_interval&#x60;; for any other agent the list is empty. Only the agent itself may call it (agent-self).
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
  /// Returns a [Future] containing a [Response] with a [AgentPage] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<AgentPage>> listHostedAgents({
    String? cursor,
    int? limit,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/agents/me/hosted';
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
    };

    final _response = await _dio.request<Object>(
      _path,
      options: _options,
      queryParameters: _queryParameters,
      cancelToken: cancelToken,
      onSendProgress: onSendProgress,
      onReceiveProgress: onReceiveProgress,
    );

    AgentPage? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(AgentPage),
            ) as AgentPage;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<AgentPage>(
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

  /// List the pending wake-ups of this agent
  /// Every pending wake-up of the calling agent, ascending by id (09 5.8.5): a runner calls it at start and after every reconnect and learns changes in between from &#x60;run.wake_changed&#x60;. Only the agent itself may call it (agent-self).
  ///
  /// Parameters:
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [RunWakeList] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<RunWakeList>> listPendingRunWakes({
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/agents/me/wakes';
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

    RunWakeList? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(RunWakeList),
            ) as RunWakeList;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<RunWakeList>(
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
  /// Same as gateway op &#x60;run.update&#x60; (one use case). Only the run&#39;s own agent patches, only while the run is &#x60;queued&#x60;, &#x60;running&#x60; or &#x60;waiting_input&#x60;. A path or op outside the allowed set of 09 section 5.3 (for example &#x60;/state&#x60;) gives 400 &#x60;validation_failed&#x60; with &#x60;errors[].code&#x60; &#x60;patch_path_forbidden&#x60; and the &#x60;pointer&#x60; of the op. An ended run gives 409 &#x60;conflict&#x60; with &#x60;reason&#x60; &#x60;run_not_active&#x60;, checked before the version. A wrong &#x60;base_version&#x60; gives 409 &#x60;base_version_conflict&#x60; with the current document in &#x60;current&#x60;, exactly as &#x60;getRun&#x60; answers it. Set semantics: &#x60;add&#x60; and &#x60;replace&#x60; set a member also when it is absent, &#x60;remove&#x60; of an absent member is a no-op, and one stored patch raises the version by one. A follow-up state moves forward only (&#x60;queued&#x60; to &#x60;delivered&#x60; or &#x60;absorbed&#x60;, &#x60;delivered&#x60; to &#x60;absorbed&#x60;); a backward change gives 409 &#x60;invalid_state_transition&#x60;.
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

  /// Create the recovery or usage-limit wake-up of an ended run
  /// Creates a wake-up of kind &#x60;at&#x60; with reason &#x60;recovery&#x60; (lost background work, 09 18.19.9) or &#x60;usage_limit&#x60; (the usage window ended the run, 18.19.10) for an ended run of the calling agent in state &#x60;failed&#x60; (which &#x60;interrupted&#x60; and &#x60;stale&#x60; runs are), at most one per run and reason: a repetition answers the first one with &#x60;200&#x60;; a &#x60;usage_limit&#x60; request while a &#x60;usage_limit&#x60; wake-up of an outage of the run is pending answers that wake-up with &#x60;200&#x60; and changes nothing. A run that has not ended is &#x60;409 conflict&#x60; with &#x60;reason&#x60; &#x60;run_not_ended&#x60; and stores nothing; a run that ended &#x60;cancelled&#x60; or &#x60;completed&#x60; stores the wake-up in state &#x60;refused&#x60; with the note in the thread and the notification &#x60;wake_ended&#x60; and answers &#x60;409 conflict&#x60; with &#x60;reason&#x60; &#x60;run_not_failed&#x60;. A refusal of the checks of 09 5.8.6 (&#x60;autonomous_not_allowed&#x60;, &#x60;wake_max_pending&#x60;, &#x60;usage_wake_max&#x60;, a refused &#x60;agent.invoke&#x60;) stores the wake-up in state &#x60;refused&#x60; with the note in the thread and the notification &#x60;wake_ended&#x60;, and answers &#x60;409 conflict&#x60; with the reason. Audited.
  ///
  /// Parameters:
  /// * [runId]
  /// * [runWakeRecover]
  /// * [idempotencyKey] - UUIDv7 chosen by the client per logical operation. A repeat with the same request returns the stored response; a different request gives 422 `idempotency_key_reuse`; a repeat while the first is running gives 409 `request_in_progress`. Kept for `sync.idempotency_key_ttl`.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [RunWake] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<RunWake>> recoverRunWake({
    required String runId,
    required RunWakeRecover runWakeRecover,
    String? idempotencyKey,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/runs/{run_id}/wakes/recover'.replaceAll(
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
      const _type = FullType(RunWakeRecover);
      _bodyData = _serializers.serialize(runWakeRecover, specifiedType: _type);
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

    RunWake? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(RunWake),
            ) as RunWake;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<RunWake>(
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
  /// The adapter reports the session of its own active run in the thread (09, 9.1 and 9.5). &#x60;If-Match&#x60; is the compare-and-swap over &#x60;agent_sessions.version&#x60;: absent means the run has no session row yet, and the report creates it; a row that exists needs its version, otherwise 412 &#x60;version_conflict&#x60; with &#x60;current&#x60;. 409 &#x60;conflict&#x60; with &#x60;reason&#x60; &#x60;session_bound_elsewhere&#x60; (the reference is current in another thread), &#x60;session_changed&#x60; (the thread got another current session) or &#x60;session_not_bound&#x60; (&#x60;bind_failed&#x60; without a started session row).
  ///
  /// Parameters:
  /// * [threadId]
  /// * [agentSessionReport]
  /// * [idempotencyKey] - UUIDv7 chosen by the client per logical operation. A repeat with the same request returns the stored response; a different request gives 422 `idempotency_key_reuse`; a repeat while the first is running gives 409 `request_in_progress`. Kept for `sync.idempotency_key_ttl`.
  /// * [ifMatch] - Optional ETag; when given, a mismatch gives 412 `version_conflict`.
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
    String? ifMatch,
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
        if (ifMatch != null) r'If-Match': ifMatch,
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
  /// The adapter reports the whole state of its runtime (&#x60;agent_heartbeat.v1&#x60;, 09 section 4.8); the member &#x60;agent&#x60; of the gateway op &#x60;heartbeat&#x60; has the same effect. Only an agent may call it (&#x60;agent-self&#x60;); the body carries no agent id. The last heartbeat wins: an absent optional member means \&quot;not reported\&quot;, except &#x60;project_readiness&#x60;, which is applied only when present and changed since the last list this server process applied (&#x60;ApplyProjectReadiness&#x60; of M2-03, in its own transaction).  The server validates every member and the closed keys of &#x60;details&#x60; itself, also the patterns of names, versions and codes; a violation answers &#x60;400 validation_failed&#x60; with every field error; &#x60;rate_limit&#x60; carries only &#x60;state&#x60; and an optional &#x60;resets_at&#x60; (M6-14). A change of &#x60;reported_billing_mode&#x60; or &#x60;reported_tool_class&#x60;, also from or to \&quot;not reported\&quot;, is stored at once and audited with action &#x60;agent.heartbeat&#x60;; the other fields are stored at most once per &#x60;agent.heartbeat_persist_interval&#x60;. The presence becomes &#x60;offline&#x60; once &#x60;agent.presence_timeout&#x60; passes without a heartbeat. The server compares the reported with the configured mode at each heartbeat and returns the warnings of &#x60;AgentRuntimeStatus.warnings&#x60;; the reported mode is evidence for display, warning, audit and alarm and never a permission (BR-05). &#x60;interval_ms&#x60; is &#x60;sync.heartbeat_interval&#x60;.
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
  /// Mode &#x60;agent&#x60; of &#x60;voice.transcription_mode&#x60; only (12, 8.2). Only the operator agent of the thread whose message the voice file is attached to, with read access, may store the transcript; a file the caller cannot read, an unattached file of another uploader and an unknown id answer the same 404, and a visible caller who is not that operator gets 403 &#x60;forbidden&#x60;. Another mode, or a file that is not a voice format, gives 409 &#x60;conflict&#x60; with &#x60;reason&#x60; &#x60;transcription_unavailable&#x60;; a file that is not ready gives 409 &#x60;file_not_ready&#x60; or &#x60;file_blocked&#x60;. &#x60;If-Match&#x60; carries the transcript &#x60;ETag&#x60; of &#x60;getFile&#x60; or &#x60;getTranscript&#x60; (derived from status, source, language and text, 03 R-DM-48); without it or with &#x60;*&#x60; 428 &#x60;precondition_required&#x60;, with another value 412 &#x60;version_conflict&#x60; with the current &#x60;Transcript&#x60; in &#x60;current&#x60; and its &#x60;ETag&#x60;. &#x60;text&#x60; may be empty, must not contain U+0000 (&#x60;invalid&#x60;) and is at most &#x60;message.max_content_bytes&#x60; UTF-8 bytes (&#x60;too_long&#x60;). The answer has &#x60;status&#x60; &#x60;done&#x60; and &#x60;source&#x60; &#x60;agent&#x60; and the new &#x60;ETag&#x60;; a later submit with that value replaces the transcript. No audit entry.
  ///
  /// Parameters:
  /// * [fileId]
  /// * [ifMatch] - ETag of the version the change is based on, e.g. `\"7\"`. Mismatch gives 412 `version_conflict`.
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
    required String ifMatch,
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
      headers: <String, dynamic>{r'If-Match': ifMatch, ...?headers},
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
