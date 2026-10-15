//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/thread.dart';
import 'package:api_client/src/generated/model/message.dart';
import 'package:api_client/src/generated/model/search_hit_kind.dart';
import 'package:api_client/src/generated/model/file.dart';
import 'package:api_client/src/generated/model/search_hit_segments_inner.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'search_hit.g.dart';

/// One hit. `kind` names which of the optional objects is set.
///
/// Properties:
/// * [kind]
/// * [score] - Only with `sort = relevance`.
/// * [message]
/// * [thread]
/// * [file]
/// * [threadTitle]
/// * [channelName]
/// * [spaceName]
/// * [segments] - Excerpt split at the matches; no markup, HTML in the content stays text (14, 4.6).
@BuiltValue()
abstract class SearchHit implements Built<SearchHit, SearchHitBuilder> {
  @BuiltValueField(wireName: r'kind')
  SearchHitKind get kind;
  // enum kindEnum {  message,  thread,  file,  };

  /// Only with `sort = relevance`.
  @BuiltValueField(wireName: r'score')
  double? get score;

  @BuiltValueField(wireName: r'message')
  Message? get message;

  @BuiltValueField(wireName: r'thread')
  Thread? get thread;

  @BuiltValueField(wireName: r'file')
  File? get file;

  @BuiltValueField(wireName: r'thread_title')
  String? get threadTitle;

  @BuiltValueField(wireName: r'channel_name')
  String? get channelName;

  @BuiltValueField(wireName: r'space_name')
  String? get spaceName;

  /// Excerpt split at the matches; no markup, HTML in the content stays text (14, 4.6).
  @BuiltValueField(wireName: r'segments')
  BuiltList<SearchHitSegmentsInner> get segments;

  SearchHit._();

  factory SearchHit([void updates(SearchHitBuilder b)]) = _$SearchHit;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SearchHitBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<SearchHit> get serializer => _$SearchHitSerializer();
}

class _$SearchHitSerializer implements PrimitiveSerializer<SearchHit> {
  @override
  final Iterable<Type> types = const [SearchHit, _$SearchHit];

  @override
  final String wireName = r'SearchHit';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    SearchHit object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'kind';
    yield serializers.serialize(
      object.kind,
      specifiedType: const FullType(SearchHitKind),
    );
    if (object.score != null) {
      yield r'score';
      yield serializers.serialize(
        object.score,
        specifiedType: const FullType(double),
      );
    }
    if (object.message != null) {
      yield r'message';
      yield serializers.serialize(
        object.message,
        specifiedType: const FullType(Message),
      );
    }
    if (object.thread != null) {
      yield r'thread';
      yield serializers.serialize(
        object.thread,
        specifiedType: const FullType(Thread),
      );
    }
    if (object.file != null) {
      yield r'file';
      yield serializers.serialize(
        object.file,
        specifiedType: const FullType(File),
      );
    }
    if (object.threadTitle != null) {
      yield r'thread_title';
      yield serializers.serialize(
        object.threadTitle,
        specifiedType: const FullType(String),
      );
    }
    if (object.channelName != null) {
      yield r'channel_name';
      yield serializers.serialize(
        object.channelName,
        specifiedType: const FullType(String),
      );
    }
    if (object.spaceName != null) {
      yield r'space_name';
      yield serializers.serialize(
        object.spaceName,
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
    SearchHit object, {
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
    required SearchHitBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(SearchHitKind),
          ) as SearchHitKind;
          result.kind = valueDes;
          break;
        case r'score':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(double),
          ) as double?;
          if (valueDes == null) continue;
          result.score = valueDes;
          break;
        case r'message':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(Message),
          ) as Message?;
          if (valueDes == null) continue;
          result.message.replace(valueDes);
          break;
        case r'thread':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(Thread),
          ) as Thread?;
          if (valueDes == null) continue;
          result.thread.replace(valueDes);
          break;
        case r'file':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(File),
          ) as File?;
          if (valueDes == null) continue;
          result.file.replace(valueDes);
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
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
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
  SearchHit deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SearchHitBuilder();
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
