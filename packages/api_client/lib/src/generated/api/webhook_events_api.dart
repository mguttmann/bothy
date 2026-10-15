//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

import 'dart:async';

import 'package:built_value/json_object.dart';
import 'package:built_value/serializer.dart';
import 'package:dio/dio.dart';

import 'package:api_client/src/generated/model/access_changed_event.dart';
import 'package:api_client/src/generated/model/agent_config_changed_event.dart';
import 'package:api_client/src/generated/model/agent_followup_event.dart';
import 'package:api_client/src/generated/model/agent_invocation_event.dart';
import 'package:api_client/src/generated/model/agent_session_changed_event.dart';
import 'package:api_client/src/generated/model/agent_updated_event.dart';
import 'package:api_client/src/generated/model/channel_archived_event.dart';
import 'package:api_client/src/generated/model/channel_created_event.dart';
import 'package:api_client/src/generated/model/channel_settings_changed_event.dart';
import 'package:api_client/src/generated/model/channel_updated_event.dart';
import 'package:api_client/src/generated/model/command_changed_event.dart';
import 'package:api_client/src/generated/model/command_invoked_event.dart';
import 'package:api_client/src/generated/model/file_updated_event.dart';
import 'package:api_client/src/generated/model/instance_updated_event.dart';
import 'package:api_client/src/generated/model/interaction_created_event.dart';
import 'package:api_client/src/generated/model/message_created_event.dart';
import 'package:api_client/src/generated/model/message_deleted_event.dart';
import 'package:api_client/src/generated/model/message_updated_event.dart';
import 'package:api_client/src/generated/model/pin_added_event.dart';
import 'package:api_client/src/generated/model/pin_removed_event.dart';
import 'package:api_client/src/generated/model/reaction_added_event.dart';
import 'package:api_client/src/generated/model/reaction_removed_event.dart';
import 'package:api_client/src/generated/model/run_cancel_requested_event.dart';
import 'package:api_client/src/generated/model/run_state_changed_event.dart';
import 'package:api_client/src/generated/model/space_archived_event.dart';
import 'package:api_client/src/generated/model/space_created_event.dart';
import 'package:api_client/src/generated/model/space_updated_event.dart';
import 'package:api_client/src/generated/model/status_updated_event.dart';
import 'package:api_client/src/generated/model/thread_closed_event.dart';
import 'package:api_client/src/generated/model/thread_created_event.dart';
import 'package:api_client/src/generated/model/thread_moved_event.dart';
import 'package:api_client/src/generated/model/thread_moved_out_event.dart';
import 'package:api_client/src/generated/model/thread_reopened_event.dart';
import 'package:api_client/src/generated/model/thread_updated_event.dart';
import 'package:api_client/src/generated/model/user_updated_event.dart';

class WebhookEventsApi {
  final Dio _dio;

  final Serializers _serializers;

  const WebhookEventsApi(this._dio, this._serializers);

  /// Event &#x60;access.changed&#x60;
  /// Only for the subscribing app itself.
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [accessChangedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onAccessChangedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required AccessChangedEvent accessChangedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/access.changed';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(AccessChangedEvent);
      _bodyData = _serializers.serialize(
        accessChangedEvent,
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

  /// Event &#x60;agent.config_changed&#x60;
  /// Only for webhook subscriptions of the agent itself (webhook as agent transport, docs/spec/04-sync-protokoll.md section 15.6). Carries a freshly issued invocation token in the delivery fields.
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [agentConfigChangedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onAgentConfigChangedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required AgentConfigChangedEvent agentConfigChangedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/agent.config_changed';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(AgentConfigChangedEvent);
      _bodyData = _serializers.serialize(
        agentConfigChangedEvent,
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

  /// Event &#x60;agent.followup&#x60;
  /// Only for webhook subscriptions of the agent itself (webhook as agent transport, docs/spec/04-sync-protokoll.md section 15.6). Carries a freshly issued invocation token in the delivery fields.
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [agentFollowupEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onAgentFollowupEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required AgentFollowupEvent agentFollowupEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/agent.followup';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(AgentFollowupEvent);
      _bodyData = _serializers.serialize(
        agentFollowupEvent,
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

  /// Event &#x60;agent.invocation&#x60;
  /// Only for webhook subscriptions of the agent itself (webhook as agent transport, docs/spec/04-sync-protokoll.md section 15.6). Carries a freshly issued invocation token in the delivery fields.
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [agentInvocationEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onAgentInvocationEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required AgentInvocationEvent agentInvocationEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/agent.invocation';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(AgentInvocationEvent);
      _bodyData = _serializers.serialize(
        agentInvocationEvent,
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

  /// Event &#x60;agent_session.changed&#x60;
  ///
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [agentSessionChangedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onAgentSessionChangedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required AgentSessionChangedEvent agentSessionChangedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/agent_session.changed';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(AgentSessionChangedEvent);
      _bodyData = _serializers.serialize(
        agentSessionChangedEvent,
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

  /// Event &#x60;agent.updated&#x60;
  ///
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [agentUpdatedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onAgentUpdatedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required AgentUpdatedEvent agentUpdatedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/agent.updated';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(AgentUpdatedEvent);
      _bodyData = _serializers.serialize(
        agentUpdatedEvent,
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

  /// Event &#x60;channel.archived&#x60;
  ///
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [channelArchivedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onChannelArchivedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required ChannelArchivedEvent channelArchivedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/channel.archived';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(ChannelArchivedEvent);
      _bodyData = _serializers.serialize(
        channelArchivedEvent,
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

  /// Event &#x60;channel.created&#x60;
  ///
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [channelCreatedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onChannelCreatedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required ChannelCreatedEvent channelCreatedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/channel.created';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(ChannelCreatedEvent);
      _bodyData = _serializers.serialize(
        channelCreatedEvent,
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

  /// Event &#x60;channel.settings_changed&#x60;
  ///
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [channelSettingsChangedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onChannelSettingsChangedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required ChannelSettingsChangedEvent channelSettingsChangedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/channel.settings_changed';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(ChannelSettingsChangedEvent);
      _bodyData = _serializers.serialize(
        channelSettingsChangedEvent,
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

  /// Event &#x60;channel.updated&#x60;
  ///
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [channelUpdatedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onChannelUpdatedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required ChannelUpdatedEvent channelUpdatedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/channel.updated';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(ChannelUpdatedEvent);
      _bodyData = _serializers.serialize(
        channelUpdatedEvent,
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

  /// Event &#x60;command.changed&#x60;
  ///
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [commandChangedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onCommandChangedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required CommandChangedEvent commandChangedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/command.changed';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(CommandChangedEvent);
      _bodyData = _serializers.serialize(
        commandChangedEvent,
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

  /// Event &#x60;command.invoked&#x60;
  /// Only for the subscribing app itself.
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [commandInvokedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onCommandInvokedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required CommandInvokedEvent commandInvokedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/command.invoked';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(CommandInvokedEvent);
      _bodyData = _serializers.serialize(
        commandInvokedEvent,
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

  /// Event &#x60;file.updated&#x60;
  ///
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [fileUpdatedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onFileUpdatedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required FileUpdatedEvent fileUpdatedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/file.updated';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(FileUpdatedEvent);
      _bodyData = _serializers.serialize(
        fileUpdatedEvent,
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

  /// Event &#x60;instance.updated&#x60;
  ///
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [instanceUpdatedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onInstanceUpdatedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required InstanceUpdatedEvent instanceUpdatedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/instance.updated';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(InstanceUpdatedEvent);
      _bodyData = _serializers.serialize(
        instanceUpdatedEvent,
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

  /// Event &#x60;interaction.created&#x60;
  /// Only for the subscribing app itself.
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [interactionCreatedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onInteractionCreatedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required InteractionCreatedEvent interactionCreatedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/interaction.created';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(InteractionCreatedEvent);
      _bodyData = _serializers.serialize(
        interactionCreatedEvent,
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

  /// Event &#x60;message.created&#x60;
  ///
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [messageCreatedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onMessageCreatedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required MessageCreatedEvent messageCreatedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/message.created';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(MessageCreatedEvent);
      _bodyData = _serializers.serialize(
        messageCreatedEvent,
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

  /// Event &#x60;message.deleted&#x60;
  ///
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [messageDeletedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onMessageDeletedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required MessageDeletedEvent messageDeletedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/message.deleted';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(MessageDeletedEvent);
      _bodyData = _serializers.serialize(
        messageDeletedEvent,
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

  /// Event &#x60;message.updated&#x60;
  ///
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [messageUpdatedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onMessageUpdatedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required MessageUpdatedEvent messageUpdatedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/message.updated';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(MessageUpdatedEvent);
      _bodyData = _serializers.serialize(
        messageUpdatedEvent,
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

  /// Event &#x60;pin.added&#x60;
  ///
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [pinAddedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onPinAddedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required PinAddedEvent pinAddedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/pin.added';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(PinAddedEvent);
      _bodyData = _serializers.serialize(pinAddedEvent, specifiedType: _type);
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

  /// Event &#x60;pin.removed&#x60;
  ///
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [pinRemovedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onPinRemovedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required PinRemovedEvent pinRemovedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/pin.removed';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(PinRemovedEvent);
      _bodyData = _serializers.serialize(pinRemovedEvent, specifiedType: _type);
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

  /// Event &#x60;reaction.added&#x60;
  ///
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [reactionAddedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onReactionAddedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required ReactionAddedEvent reactionAddedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/reaction.added';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(ReactionAddedEvent);
      _bodyData = _serializers.serialize(
        reactionAddedEvent,
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

  /// Event &#x60;reaction.removed&#x60;
  ///
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [reactionRemovedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onReactionRemovedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required ReactionRemovedEvent reactionRemovedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/reaction.removed';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(ReactionRemovedEvent);
      _bodyData = _serializers.serialize(
        reactionRemovedEvent,
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

  /// Event &#x60;run.cancel_requested&#x60;
  /// Only for webhook subscriptions of the agent itself (webhook as agent transport, docs/spec/04-sync-protokoll.md section 15.6). Carries a freshly issued invocation token in the delivery fields.
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [runCancelRequestedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onRunCancelRequestedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required RunCancelRequestedEvent runCancelRequestedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/run.cancel_requested';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(RunCancelRequestedEvent);
      _bodyData = _serializers.serialize(
        runCancelRequestedEvent,
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

  /// Event &#x60;run.state_changed&#x60;
  ///
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [runStateChangedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onRunStateChangedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required RunStateChangedEvent runStateChangedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/run.state_changed';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(RunStateChangedEvent);
      _bodyData = _serializers.serialize(
        runStateChangedEvent,
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

  /// Event &#x60;space.archived&#x60;
  ///
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [spaceArchivedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onSpaceArchivedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required SpaceArchivedEvent spaceArchivedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/space.archived';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(SpaceArchivedEvent);
      _bodyData = _serializers.serialize(
        spaceArchivedEvent,
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

  /// Event &#x60;space.created&#x60;
  ///
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [spaceCreatedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onSpaceCreatedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required SpaceCreatedEvent spaceCreatedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/space.created';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(SpaceCreatedEvent);
      _bodyData = _serializers.serialize(
        spaceCreatedEvent,
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

  /// Event &#x60;space.updated&#x60;
  ///
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [spaceUpdatedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onSpaceUpdatedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required SpaceUpdatedEvent spaceUpdatedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/space.updated';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(SpaceUpdatedEvent);
      _bodyData = _serializers.serialize(
        spaceUpdatedEvent,
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

  /// Event &#x60;status.updated&#x60;
  ///
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [statusUpdatedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onStatusUpdatedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required StatusUpdatedEvent statusUpdatedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/status.updated';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(StatusUpdatedEvent);
      _bodyData = _serializers.serialize(
        statusUpdatedEvent,
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

  /// Event &#x60;thread.closed&#x60;
  ///
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [threadClosedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onThreadClosedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required ThreadClosedEvent threadClosedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/thread.closed';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(ThreadClosedEvent);
      _bodyData = _serializers.serialize(
        threadClosedEvent,
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

  /// Event &#x60;thread.created&#x60;
  ///
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [threadCreatedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onThreadCreatedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required ThreadCreatedEvent threadCreatedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/thread.created';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(ThreadCreatedEvent);
      _bodyData = _serializers.serialize(
        threadCreatedEvent,
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

  /// Event &#x60;thread.moved&#x60;
  ///
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [threadMovedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onThreadMovedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required ThreadMovedEvent threadMovedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/thread.moved';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(ThreadMovedEvent);
      _bodyData = _serializers.serialize(
        threadMovedEvent,
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

  /// Event &#x60;thread.moved_out&#x60;
  ///
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [threadMovedOutEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onThreadMovedOutEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required ThreadMovedOutEvent threadMovedOutEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/thread.moved_out';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(ThreadMovedOutEvent);
      _bodyData = _serializers.serialize(
        threadMovedOutEvent,
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

  /// Event &#x60;thread.reopened&#x60;
  ///
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [threadReopenedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onThreadReopenedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required ThreadReopenedEvent threadReopenedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/thread.reopened';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(ThreadReopenedEvent);
      _bodyData = _serializers.serialize(
        threadReopenedEvent,
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

  /// Event &#x60;thread.updated&#x60;
  ///
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [threadUpdatedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onThreadUpdatedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required ThreadUpdatedEvent threadUpdatedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/thread.updated';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(ThreadUpdatedEvent);
      _bodyData = _serializers.serialize(
        threadUpdatedEvent,
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

  /// Event &#x60;user.updated&#x60;
  ///
  ///
  /// Parameters:
  /// * [webhookId] - Event ID; stable across retries, use it to deduplicate.
  /// * [webhookTimestamp] - Unix seconds of this attempt. Reject values outside `webhook.receiver_tolerance`.
  /// * [webhookSignature] - Space-separated signatures `v1a,<base64>` (Ed25519) and, if enabled, `v1,<base64>` (HMAC-SHA256) over `{webhook-id}.{webhook-timestamp}.{body}`.
  /// * [userUpdatedEvent]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future]
  /// Throws [DioException] if API call or serialization fails
  Future<Response<void>> onUserUpdatedEvent({
    required String webhookId,
    required int webhookTimestamp,
    required String webhookSignature,
    required UserUpdatedEvent userUpdatedEvent,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/user.updated';
    final _options = Options(
      method: r'POST',
      headers: <String, dynamic>{
        r'webhook-id': webhookId,
        r'webhook-timestamp': webhookTimestamp,
        r'webhook-signature': webhookSignature,
        ...?headers,
      },
      extra: <String, dynamic>{'secure': <Map<String, String>>[], ...?extra},
      contentType: '',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      const _type = FullType(UserUpdatedEvent);
      _bodyData = _serializers.serialize(
        userUpdatedEvent,
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
}
