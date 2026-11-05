//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/search_hit_segments_inner.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'search_assist_hit.g.dart';

/// One session the search found; open it at `/t/<thread_id>?m=<message_id>`.
///
/// Properties:
/// * [threadId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [messageId] - The best matching message of the thread; absent when only the title matched.
/// * [channelId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [threadTitle] - Absent for the main thread of a text channel.
/// * [channelName] - The channel, for a development session the project forum.
/// * [spaceName] - Absent for a direct message.
/// * [at] - Time of the matching message, or of the thread when only the title matched.
/// * [source_]
/// * [reason] - One sentence of the assistant why this is the session; only with `source = assistant`.
/// * [segments] - Excerpt split at the matches, as in `SearchHit`.
@BuiltValue()
abstract class SearchAssistHit
    implements Built<SearchAssistHit, SearchAssistHitBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'thread_id')
  String get threadId;

  /// The best matching message of the thread; absent when only the title matched.
  @BuiltValueField(wireName: r'message_id')
  String? get messageId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'channel_id')
  String get channelId;

  /// Absent for the main thread of a text channel.
  @BuiltValueField(wireName: r'thread_title')
  String? get threadTitle;

  /// The channel, for a development session the project forum.
  @BuiltValueField(wireName: r'channel_name')
  String get channelName;

  /// Absent for a direct message.
  @BuiltValueField(wireName: r'space_name')
  String? get spaceName;

  /// Time of the matching message, or of the thread when only the title matched.
  @BuiltValueField(wireName: r'at')
  DateTime get at;

  @BuiltValueField(wireName: r'source')
  SearchAssistHitSource_Enum get source_;
  // enum source_Enum {  assistant,  fulltext,  };

  /// One sentence of the assistant why this is the session; only with `source = assistant`.
  @BuiltValueField(wireName: r'reason')
  String? get reason;

  /// Excerpt split at the matches, as in `SearchHit`.
  @BuiltValueField(wireName: r'segments')
  BuiltList<SearchHitSegmentsInner> get segments;

  SearchAssistHit._();

  factory SearchAssistHit([void updates(SearchAssistHitBuilder b)]) =
      _$SearchAssistHit;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SearchAssistHitBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<SearchAssistHit> get serializer =>
      _$SearchAssistHitSerializer();
}

class _$SearchAssistHitSerializer
    implements PrimitiveSerializer<SearchAssistHit> {
  @override
  final Iterable<Type> types = const [SearchAssistHit, _$SearchAssistHit];

  @override
  final String wireName = r'SearchAssistHit';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    SearchAssistHit object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'thread_id';
    yield serializers.serialize(
      object.threadId,
      specifiedType: const FullType(String),
    );
    if (object.messageId != null) {
      yield r'message_id';
      yield serializers.serialize(
        object.messageId,
        specifiedType: const FullType(String),
      );
    }
    yield r'channel_id';
    yield serializers.serialize(
      object.channelId,
      specifiedType: const FullType(String),
    );
    if (object.threadTitle != null) {
      yield r'thread_title';
      yield serializers.serialize(
        object.threadTitle,
        specifiedType: const FullType(String),
      );
    }
    yield r'channel_name';
    yield serializers.serialize(
      object.channelName,
      specifiedType: const FullType(String),
    );
    if (object.spaceName != null) {
      yield r'space_name';
      yield serializers.serialize(
        object.spaceName,
        specifiedType: const FullType(String),
      );
    }
    yield r'at';
    yield serializers.serialize(
      object.at,
      specifiedType: const FullType(DateTime),
    );
    yield r'source';
    yield serializers.serialize(
      object.source_,
      specifiedType: const FullType(SearchAssistHitSource_Enum),
    );
    if (object.reason != null) {
      yield r'reason';
      yield serializers.serialize(
        object.reason,
        specifiedType: const FullType(String),
      );
    }
    yield r'segments';
    yield serializers.serialize(
      object.segments,
      specifiedType: const FullType(BuiltList, [
        FullType(SearchHitSegmentsInner),
      ]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    SearchAssistHit object, {
    FullType specifiedType = FullType.unspecified,
  }) {
    return _serializeProperties(
      serializers,
      object,
      specifiedType: specifiedType,
    ).toList();
  }

  void _deserializeProperties(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
    required List<Object?> serializedList,
    required SearchAssistHitBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'thread_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.threadId = valueDes;
          break;
        case r'message_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.messageId = valueDes;
          break;
        case r'channel_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.channelId = valueDes;
          break;
        case r'thread_title':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.threadTitle = valueDes;
          break;
        case r'channel_name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.channelName = valueDes;
          break;
        case r'space_name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.spaceName = valueDes;
          break;
        case r'at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.at = valueDes;
          break;
        case r'source':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(SearchAssistHitSource_Enum),
          ) as SearchAssistHitSource_Enum;
          result.source_ = valueDes;
          break;
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.reason = valueDes;
          break;
        case r'segments':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(SearchHitSegmentsInner),
            ]),
          ) as BuiltList<SearchHitSegmentsInner>;
          result.segments.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  SearchAssistHit deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SearchAssistHitBuilder();
    final serializedList = (serialized as Iterable<Object?>).toList();
    final unhandled = <Object?>[];
    _deserializeProperties(
      serializers,
      serialized,
      specifiedType: specifiedType,
      serializedList: serializedList,
      unhandled: unhandled,
      result: result,
    );
    return result.build();
  }
}

class SearchAssistHitSource_Enum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'assistant')
  static const SearchAssistHitSource_Enum assistant =
      _$searchAssistHitSourceEnum_assistant;
  @BuiltValueEnumConst(wireName: r'fulltext')
  static const SearchAssistHitSource_Enum fulltext =
      _$searchAssistHitSourceEnum_fulltext;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const SearchAssistHitSource_Enum unknownDefaultOpenApi =
      _$searchAssistHitSourceEnum_unknownDefaultOpenApi;

  static Serializer<SearchAssistHitSource_Enum> get serializer =>
      _$searchAssistHitSourceEnumSerializer;

  const SearchAssistHitSource_Enum._(String name) : super(name);

  static BuiltSet<SearchAssistHitSource_Enum> get values =>
      _$searchAssistHitSourceEnumValues;
  static SearchAssistHitSource_Enum valueOf(String name) =>
      _$searchAssistHitSourceEnumValueOf(name);
}
