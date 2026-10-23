//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

import 'dart:async';

import 'package:built_value/json_object.dart';
import 'package:built_value/serializer.dart';
import 'package:dio/dio.dart';

import 'package:api_client/src/generated/api_util.dart';
import 'package:api_client/src/generated/model/problem.dart';
import 'package:api_client/src/generated/model/search_hit_page.dart';
import 'package:api_client/src/generated/model/thread_state.dart';
import 'package:api_client/src/generated/model/thread_title_hit_list.dart';
import 'package:built_collection/built_collection.dart';

class SearchApi {
  final Dio _dio;

  final Serializers _serializers;

  const SearchApi(this._dio, this._serializers);

  /// Full-text search
  /// Results are filtered by &#x60;message.read&#x60;, &#x60;thread.read&#x60; and &#x60;file.read&#x60; inside the query; counts are not returned. Page size &#x60;search.page_size_default&#x60;, at most &#x60;search.page_size_max&#x60; (14, 4.7).
  ///
  /// Parameters:
  /// * [q] - Query in web search syntax: words, \"phrases\", `or`, `-word`.
  /// * [cursor] - Cursor from `next_cursor` of the previous page.
  /// * [limit]
  /// * [type] - Result type; default `messages` (14, 11).
  /// * [sort] - Order of the hits (14, 4.5).
  /// * [spaceId] - Only channels of these spaces, intersected with the visible ones.
  /// * [channelId] - Only these channels, intersected with the visible ones.
  /// * [threadId] - Restrict to a thread.
  /// * [authorId] - Author of the message or uploader of the file.
  /// * [agentId] - Threads operated by this agent.
  /// * [after] - Created at or after.
  /// * [before] - Created before.
  /// * [hasFile] - Only messages with at least one attachment.
  /// * [threadState] - Only open or closed threads.
  /// * [tagId] - Forum tags of the thread.
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [SearchHitPage] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<SearchHitPage>> search({
    required String q,
    String? cursor,
    int? limit,
    String? type = 'messages',
    String? sort = 'relevance',
    BuiltList<String>? spaceId,
    BuiltList<String>? channelId,
    String? threadId,
    String? authorId,
    String? agentId,
    DateTime? after,
    DateTime? before,
    bool? hasFile,
    ThreadState? threadState,
    BuiltList<String>? tagId,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/search';
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
      r'q': encodeQueryParameter(_serializers, q, const FullType(String)),
      if (type != null)
        r'type': encodeQueryParameter(
          _serializers,
          type,
          const FullType(String),
        ),
      if (sort != null)
        r'sort': encodeQueryParameter(
          _serializers,
          sort,
          const FullType(String),
        ),
      if (spaceId != null)
        r'space_id': encodeCollectionQueryParameter<String>(
          _serializers,
          spaceId,
          const FullType(BuiltList, [FullType(String)]),
          format: ListFormat.multi,
        ),
      if (channelId != null)
        r'channel_id': encodeCollectionQueryParameter<String>(
          _serializers,
          channelId,
          const FullType(BuiltList, [FullType(String)]),
          format: ListFormat.multi,
        ),
      if (threadId != null)
        r'thread_id': encodeQueryParameter(
          _serializers,
          threadId,
          const FullType(String),
        ),
      if (authorId != null)
        r'author_id': encodeQueryParameter(
          _serializers,
          authorId,
          const FullType(String),
        ),
      if (agentId != null)
        r'agent_id': encodeQueryParameter(
          _serializers,
          agentId,
          const FullType(String),
        ),
      if (after != null)
        r'after': encodeQueryParameter(
          _serializers,
          after,
          const FullType(DateTime),
        ),
      if (before != null)
        r'before': encodeQueryParameter(
          _serializers,
          before,
          const FullType(DateTime),
        ),
      if (hasFile != null)
        r'has_file': encodeQueryParameter(
          _serializers,
          hasFile,
          const FullType(bool),
        ),
      if (threadState != null)
        r'thread_state': encodeQueryParameter(
          _serializers,
          threadState,
          const FullType(ThreadState),
        ),
      if (tagId != null)
        r'tag_id': encodeCollectionQueryParameter<String>(
          _serializers,
          tagId,
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

    SearchHitPage? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(SearchHitPage),
            ) as SearchHitPage;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<SearchHitPage>(
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

  /// Prefix search over thread titles
  /// For the quick switcher (14, 4.9): words shorter than &#x60;search.prefix_min_chars&#x60; are dropped; same access filter as &#x60;/search&#x60;, newest activity first. Page size &#x60;search.page_size_default&#x60;, at most &#x60;search.page_size_max&#x60;.
  ///
  /// Parameters:
  /// * [q] - Typed prefix text.
  /// * [limit]
  /// * [cancelToken] - A [CancelToken] that can be used to cancel the operation
  /// * [headers] - Can be used to add additional headers to the request
  /// * [extras] - Can be used to add flags to the request
  /// * [validateStatus] - A [ValidateStatus] callback that can be used to determine request success based on the HTTP status of the response
  /// * [onSendProgress] - A [ProgressCallback] that can be used to get the send progress
  /// * [onReceiveProgress] - A [ProgressCallback] that can be used to get the receive progress
  ///
  /// Returns a [Future] containing a [Response] with a [ThreadTitleHitList] as data
  /// Throws [DioException] if API call or serialization fails
  Future<Response<ThreadTitleHitList>> searchThreadTitles({
    required String q,
    int? limit,
    CancelToken? cancelToken,
    Map<String, dynamic>? headers,
    Map<String, dynamic>? extra,
    ValidateStatus? validateStatus,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    final _path = r'/search/thread-titles';
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
      r'q': encodeQueryParameter(_serializers, q, const FullType(String)),
    };

    final _response = await _dio.request<Object>(
      _path,
      options: _options,
      queryParameters: _queryParameters,
      cancelToken: cancelToken,
      onSendProgress: onSendProgress,
      onReceiveProgress: onReceiveProgress,
    );

    ThreadTitleHitList? _responseData;

    try {
      final rawResponse = _response.data;
      _responseData = rawResponse == null
          ? null
          : _serializers.deserialize(
              rawResponse,
              specifiedType: const FullType(ThreadTitleHitList),
            ) as ThreadTitleHitList;
    } catch (error, stackTrace) {
      throw DioException(
        requestOptions: _response.requestOptions,
        response: _response,
        type: DioExceptionType.unknown,
        error: error,
        stackTrace: stackTrace,
      );
    }

    return Response<ThreadTitleHitList>(
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
