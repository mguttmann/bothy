//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

import 'dart:async';

import 'package:built_value/json_object.dart';
import 'package:built_value/serializer.dart';
import 'package:dio/dio.dart';

import 'package:api_client/src/generated/api_util.dart';
import 'package:api_client/src/generated/model/import_batch.dart';
import 'package:api_client/src/generated/model/import_batch_result.dart';
import 'package:api_client/src/generated/model/import_cursor_set.dart';
import 'package:api_client/src/generated/model/import_file_create.dart';
import 'package:api_client/src/generated/model/import_file_slot.dart';
import 'package:api_client/src/generated/model/import_item_page.dart';
import 'package:api_client/src/generated/model/import_item_state.dart';
import 'package:api_client/src/generated/model/import_lead_set.dart';
import 'package:api_client/src/generated/model/import_source.dart';
import 'package:api_client/src/generated/model/import_source_create.dart';
import 'package:api_client/src/generated/model/import_source_page.dart';
import 'package:api_client/src/generated/model/problem.dart';
import 'package:api_client/src/generated/model/upload_complete.dart';
import 'package:api_client/src/generated/model/upload_part_result.dart';

class ImportApi {
  final Dio _dio;

  final Serializers _serializers;

  const ImportApi(this._dio, this._serializers);

  /// Write a batch of imported objects
  /// One transaction, at most &#x60;import.batch_max_items&#x60; objects. Every target must lie in a channel of this running source. Existing &#x60;posted&#x60; keys return their target without a duplicate or event (22, 7.2). Writes &#x60;thread.updated&#x60; and &#x60;import.progress&#x60; instead of one event per message. A source that is not &#x60;running&#x60; is &#x60;409 conflict&#x60; with &#x60;reason&#x60; &#x60;import_not_running&#x60;; a target outside the channels of the source is &#x60;409 conflict&#x60; with &#x60;reason&#x60; &#x60;target_outside_source&#x60;; a new channel whose name equals a live channel of its space is &#x60;409 conflict&#x60; with &#x60;reason&#x60; &#x60;name_collision&#x60;. The &#x60;data&#x60; of a structure item follows &#x60;ImportSpaceData&#x60;, &#x60;ImportChannelData&#x60;, &#x60;ImportForumTagData&#x60; or &#x60;ImportUserData&#x60;. A batch with &#x60;thread_origin_key&#x60; concerns exactly one thread (&#x60;thread:&lt;id&gt;&#x60;, or &#x60;channel:&lt;id&gt;&#x60; for the main thread of a text channel; a forum has none, &#x60;400 validation_failed&#x60;). Its optional first item is the &#x60;thread&#x60; item of that key (&#x60;ImportThreadData&#x60;), every other item a &#x60;message&#x60; (&#x60;ImportMessageData&#x60;) or a &#x60;pin&#x60; (&#x60;ImportPinData&#x60;); a structure item there, or a content item in a batch without &#x60;thread_origin_key&#x60;, is &#x60;400 validation_failed&#x60; at its &#x60;item_kind&#x60;. A thread that is neither posted nor created by the batch is &#x60;409 conflict&#x60; with &#x60;reason&#x60; &#x60;thread_not_imported&#x60;; a message whose author item is not posted &#x60;409 conflict&#x60; with &#x60;reason&#x60; &#x60;author_not_imported&#x60;. Imported messages have &#x60;origin &#x3D; import&#x60;, are silent, write no mentions and never trigger an agent; a posted thread only follows the mirror rules of 22 12.2 step 3. At its end a batch that changed something sets &#x60;threads.unread_floor_seq&#x60; to the end of the thread and appends one &#x60;thread.updated&#x60; and one &#x60;import.progress&#x60;, never &#x60;message.created&#x60; (04 15.8).
  ///
  /// Parameters:
  /// * [importId]
  /// * [importBatch]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [ImportBatchResult] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<ImportBatchResult>> applyImportBatch({
    required String importId,
    required ImportBatch importBatch,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/imports/{import_id}/batches'.replaceAll(
      '{'
      r'import_id'
      '}',
      encodeQueryParameter(
        _serializers,
        importId,
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
      const _type = FullType(ImportBatch);
      _bodyData = _serializers.serialize(importBatch, specifiedType: _type);
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

    ImportBatchResult? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(ImportBatchResult),
            ) as ImportBatchResult;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<ImportBatchResult>(
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

  /// Cancel an import
  /// Ends a &#x60;draft&#x60;, &#x60;running&#x60; or &#x60;paused&#x60; source as &#x60;failed&#x60; with the counter &#x60;server.cancelled&#x60; set to &#x60;true&#x60; in &#x60;stats&#x60; and sets &#x60;finished_at&#x60;. Any other state is &#x60;409 conflict&#x60; with &#x60;reason&#x60; &#x60;import_state&#x60;.
  ///
  /// Parameters:
  /// * [importId]
  /// * [idempotencyKey] - UUIDv7 chosen by the client per logical operation. A repeat with the same request returns the stored response; a different request gives 422 `idempotency_key_reuse`; a repeat while the first is running gives 409 `request_in_progress`. Kept for `sync.idempotency_key_ttl`.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [ImportSource] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<ImportSource>> cancelImport({
    required String importId,
    String? idempotencyKey,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/imports/{import_id}/cancel'.replaceAll(
      '{'
      r'import_id'
      '}',
      encodeQueryParameter(
        _serializers,
        importId,
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

    ImportSource? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(ImportSource),
            ) as ImportSource;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<ImportSource>(
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

  /// Complete an import
  /// When every channel is led by this server: stop the mirror, source &#x60;completed&#x60; (22, 13.5). Only from &#x60;running&#x60; or &#x60;paused&#x60;, else &#x60;409 conflict&#x60; with &#x60;reason&#x60; &#x60;import_state&#x60;. Every channel of the mapping whose target is not &#x60;skip&#x60; and whose channel item is &#x60;posted&#x60; must have &#x60;lead &#x3D; bothy&#x60;, else &#x60;409 conflict&#x60; with &#x60;reason&#x60; &#x60;lead_not_bothy&#x60;; mapped channels that were never imported do not block it.
  ///
  /// Parameters:
  /// * [importId]
  /// * [idempotencyKey] - UUIDv7 chosen by the client per logical operation. A repeat with the same request returns the stored response; a different request gives 422 `idempotency_key_reuse`; a repeat while the first is running gives 409 `request_in_progress`. Kept for `sync.idempotency_key_ttl`.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [ImportSource] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<ImportSource>> completeImport({
    required String importId,
    String? idempotencyKey,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/imports/{import_id}/complete'.replaceAll(
      '{'
      r'import_id'
      '}',
      encodeQueryParameter(
        _serializers,
        importId,
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

    ImportSource? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(ImportSource),
            ) as ImportSource;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<ImportSource>(
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

  /// Finish the upload of an imported file
  /// Completes the upload of the file of a file item of this source with the rules of &#x60;completeUpload&#x60; (12, 3.4): &#x60;parts&#x60; is checked against the confirmed parts, missing parts are &#x60;409 conflict&#x60; with &#x60;reason&#x60; &#x60;upload_incomplete&#x60;, another &#x60;etag&#x60; &#x60;409 conflict&#x60; with &#x60;reason&#x60; &#x60;part_conflict&#x60;, an expired session &#x60;410 gone&#x60;. The job &#x60;files.scan&#x60; checks size and SHA-256 against the declaration of &#x60;createImportFile&#x60;, detects the type and scans; it runs with the lowest job priority, so live uploads are scanned first (12, 11). The item becomes &#x60;posted&#x60; in the same transaction; a repeated call answers the posted item. It authorizes &#x60;import.apply&#x60; on the running source instead of &#x60;file.upload&#x60; (05 rule 1.8, 4.12). A file that is not the target of a file item of this source is &#x60;404&#x60;.
  ///
  /// Parameters:
  /// * [importId]
  /// * [fileId]
  /// * [uploadComplete]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [ImportFileSlot] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<ImportFileSlot>> completeImportFile({
    required String importId,
    required String fileId,
    required UploadComplete uploadComplete,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/imports/{import_id}/files/{file_id}/complete'
        .replaceAll(
          '{'
          r'import_id'
          '}',
          encodeQueryParameter(
            _serializers,
            importId,
            const FullType(String),
          ).toString(),
        )
        .replaceAll(
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
      const _type = FullType(UploadComplete);
      _bodyData = _serializers.serialize(uploadComplete, specifiedType: _type);
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

    ImportFileSlot? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(ImportFileSlot),
            ) as ImportFileSlot;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<ImportFileSlot>(
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

  /// Add an import source
  ///
  ///
  /// Parameters:
  /// * [importSourceCreate]
  /// * [idempotencyKey] - UUIDv7 chosen by the client per logical operation. A repeat with the same request returns the stored response; a different request gives 422 `idempotency_key_reuse`; a repeat while the first is running gives 409 `request_in_progress`. Kept for `sync.idempotency_key_ttl`.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [ImportSource] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<ImportSource>> createImport({
    required ImportSourceCreate importSourceCreate,
    String? idempotencyKey,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/imports';
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
      const _type = FullType(ImportSourceCreate);
      _bodyData = _serializers.serialize(
        importSourceCreate,
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

    ImportSource? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(ImportSource),
            ) as ImportSource;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<ImportSource>(
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

  /// Reserve an imported file
  /// Writes the &#x60;pending&#x60; item and reserves the file ID with an upload session of mode &#x60;direct&#x60;, without the quota check of &#x60;createUpload&#x60; (12, 11); the parts and the completion follow through &#x60;uploadImportFilePart&#x60; and &#x60;completeImportFile&#x60;, which a read-only mirror channel accepts (05 rule 1.8). A repeated call with the same key returns the same ID: with the open session, with a new session after an aborted or expired one, and without &#x60;upload&#x60; once the item is &#x60;posted&#x60; (22, 7.2). &#x60;channel_id&#x60; must be the target of a posted channel item of the source (&#x60;409 conflict&#x60;, &#x60;reason&#x60; &#x60;target_outside_source&#x60;), &#x60;author_id&#x60; the target of a posted user item (&#x60;409 conflict&#x60;, &#x60;reason&#x60; &#x60;author_not_imported&#x60;); the scan of an import file runs with the lowest job priority.
  ///
  /// Parameters:
  /// * [importId]
  /// * [importFileCreate]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [ImportFileSlot] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<ImportFileSlot>> createImportFile({
    required String importId,
    required ImportFileCreate importFileCreate,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/imports/{import_id}/files'.replaceAll(
      '{'
      r'import_id'
      '}',
      encodeQueryParameter(
        _serializers,
        importId,
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
      const _type = FullType(ImportFileCreate);
      _bodyData = _serializers.serialize(
        importFileCreate,
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

    ImportFileSlot? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(ImportFileSlot),
            ) as ImportFileSlot;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<ImportFileSlot>(
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

  /// Read progress of an import
  ///
  ///
  /// Parameters:
  /// * [importId]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [ImportSource] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<ImportSource>> getImport({
    required String importId,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/imports/{import_id}'.replaceAll(
      '{'
      r'import_id'
      '}',
      encodeQueryParameter(
        _serializers,
        importId,
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

    ImportSource? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(ImportSource),
            ) as ImportSource;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<ImportSource>(
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

  /// List imported items
  ///
  ///
  /// Parameters:
  /// * [importId]
  /// * [cursor] - Cursor from `next_cursor` of the previous page.
  /// * [limit]
  /// * [state] - Filter by state.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [ImportItemPage] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<ImportItemPage>> listImportItems({
    required String importId,
    String? cursor,
    int? limit,
    ImportItemState? state,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/imports/{import_id}/items'.replaceAll(
      '{'
      r'import_id'
      '}',
      encodeQueryParameter(
        _serializers,
        importId,
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
      if (state != null)
        r'state': encodeQueryParameter(
          _serializers,
          state,
          const FullType(ImportItemState),
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

    ImportItemPage? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(ImportItemPage),
            ) as ImportItemPage;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<ImportItemPage>(
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

  /// List import sources
  ///
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
  /// Returns a [Future] containing a [Response] with a [ImportSourcePage] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<ImportSourcePage>> listImports({
    String? cursor,
    int? limit,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/imports';
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

    ImportSourcePage? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(ImportSourcePage),
            ) as ImportSourcePage;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<ImportSourcePage>(
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

  /// Pause an import
  /// Moves a &#x60;running&#x60; source to &#x60;paused&#x60;; batches and cursor writes then answer &#x60;409 conflict&#x60; with &#x60;reason&#x60; &#x60;import_not_running&#x60;. Pausing a &#x60;paused&#x60; source answers it unchanged. Any other state is &#x60;409 conflict&#x60; with &#x60;reason&#x60; &#x60;import_state&#x60;.
  ///
  /// Parameters:
  /// * [importId]
  /// * [idempotencyKey] - UUIDv7 chosen by the client per logical operation. A repeat with the same request returns the stored response; a different request gives 422 `idempotency_key_reuse`; a repeat while the first is running gives 409 `request_in_progress`. Kept for `sync.idempotency_key_ttl`.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [ImportSource] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<ImportSource>> pauseImport({
    required String importId,
    String? idempotencyKey,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/imports/{import_id}/pause'.replaceAll(
      '{'
      r'import_id'
      '}',
      encodeQueryParameter(
        _serializers,
        importId,
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

    ImportSource? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(ImportSource),
            ) as ImportSource;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<ImportSource>(
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

  /// Advance the import cursor
  /// Only after a successful batch (22, 7.3). Replaces &#x60;cursor&#x60; with the given object and merges the top-level members of &#x60;stats&#x60; into the member &#x60;tool&#x60; of &#x60;ImportSource.stats&#x60; (the server&#39;s own counters stay under &#x60;server&#x60;). Only for a &#x60;running&#x60; source, else &#x60;409 conflict&#x60; with &#x60;reason&#x60; &#x60;import_not_running&#x60;.
  ///
  /// Parameters:
  /// * [importId]
  /// * [importCursorSet]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [ImportSource] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<ImportSource>> setImportCursor({
    required String importId,
    required ImportCursorSet importCursorSet,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/imports/{import_id}/cursor'.replaceAll(
      '{'
      r'import_id'
      '}',
      encodeQueryParameter(
        _serializers,
        importId,
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
      const _type = FullType(ImportCursorSet);
      _bodyData = _serializers.serialize(importCursorSet, specifiedType: _type);
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

    ImportSource? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(ImportSource),
            ) as ImportSource;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<ImportSource>(
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

  /// Set the leading side of a channel
  /// Switches &#x60;lead&#x60; of one mapped channel and the read-only state of its target channel in one transaction (22, 12.1 and 13). The channel must be in the mapping with a target other than &#x60;skip&#x60; (else &#x60;404&#x60;) and its channel item must be &#x60;posted&#x60; (else &#x60;409 conflict&#x60; with &#x60;reason&#x60; &#x60;channel_not_imported&#x60;). &#x60;lead &#x3D; discord&#x60; sets the target channel read-only and cancels its active runs; &#x60;lead &#x3D; bothy&#x60; lifts read-only. A &#x60;completed&#x60; or &#x60;failed&#x60; source is &#x60;409 conflict&#x60; with &#x60;reason&#x60; &#x60;import_state&#x60;. A lead equal to the current one changes nothing.
  ///
  /// Parameters:
  /// * [importId]
  /// * [importLeadSet]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [ImportSource] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<ImportSource>> setImportLead({
    required String importId,
    required ImportLeadSet importLeadSet,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/imports/{import_id}/leads'.replaceAll(
      '{'
      r'import_id'
      '}',
      encodeQueryParameter(
        _serializers,
        importId,
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
      const _type = FullType(ImportLeadSet);
      _bodyData = _serializers.serialize(importLeadSet, specifiedType: _type);
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

    ImportSource? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(ImportSource),
            ) as ImportSource;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<ImportSource>(
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

  /// Start or continue an import
  /// Moves a &#x60;draft&#x60; or &#x60;paused&#x60; source to &#x60;running&#x60; (22, 7.4); &#x60;started_at&#x60; is set on the first start. Starting a &#x60;running&#x60; source answers it unchanged. Any other state is &#x60;409 conflict&#x60; with &#x60;reason&#x60; &#x60;import_state&#x60;.
  ///
  /// Parameters:
  /// * [importId]
  /// * [idempotencyKey] - UUIDv7 chosen by the client per logical operation. A repeat with the same request returns the stored response; a different request gives 422 `idempotency_key_reuse`; a repeat while the first is running gives 409 `request_in_progress`. Kept for `sync.idempotency_key_ttl`.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [ImportSource] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<ImportSource>> startImport({
    required String importId,
    String? idempotencyKey,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/imports/{import_id}/start'.replaceAll(
      '{'
      r'import_id'
      '}',
      encodeQueryParameter(
        _serializers,
        importId,
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

    ImportSource? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(ImportSource),
            ) as ImportSource;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<ImportSource>(
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

  /// Upload one part of an imported file
  /// One part of the file of a file item of this source in mode &#x60;direct&#x60;, with the rules of &#x60;uploadPart&#x60; (12, 3.2): the same part number with the same &#x60;Content-Digest&#x60; answers 200 without new work, with another digest &#x60;409 conflict&#x60; with &#x60;reason&#x60; &#x60;part_conflict&#x60;; a wrong length or digest is &#x60;400 validation_failed&#x60; with &#x60;errors[].code&#x60; &#x60;part_digest_mismatch&#x60; and stores nothing; an expired session answers &#x60;410 gone&#x60; (a repeated &#x60;createImportFile&#x60; opens a new one). It authorizes &#x60;import.apply&#x60; on the running source instead of &#x60;file.upload&#x60;, so a read-only mirror channel accepts it (05 rule 1.8, 4.12). A file that is not the target of a file item of this source is &#x60;404&#x60;.
  ///
  /// Parameters:
  /// * [importId]
  /// * [fileId]
  /// * [partNumber]
  /// * [contentDigest] - RFC 9530 digest of the part with the member `sha-256`, e.g. `sha-256=:<base64>:`; other members are ignored.
  /// * [body]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [UploadPartResult] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<UploadPartResult>> uploadImportFilePart({
    required String importId,
    required String fileId,
    required int partNumber,
    required String contentDigest,
    required MultipartFile body,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/imports/{import_id}/files/{file_id}/parts/{part_number}'
        .replaceAll(
          '{'
          r'import_id'
          '}',
          encodeQueryParameter(
            _serializers,
            importId,
            const FullType(String),
          ).toString(),
        )
        .replaceAll(
          '{'
          r'file_id'
          '}',
          encodeQueryParameter(
            _serializers,
            fileId,
            const FullType(String),
          ).toString(),
        )
        .replaceAll(
          '{'
          r'part_number'
          '}',
          encodeQueryParameter(
            _serializers,
            partNumber,
            const FullType(int),
          ).toString(),
        );
    final _options = Options(
      method: r'PUT',
      headers: <String, dynamic>{r'Content-Digest': contentDigest, ...?headers},
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
      contentType: 'application/octet-stream',
      validateStatus: validateStatus,
    );

    dynamic _bodyData;

    try {
      _bodyData = body.finalize();
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

    UploadPartResult? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(UploadPartResult),
            ) as UploadPartResult;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<UploadPartResult>(
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
