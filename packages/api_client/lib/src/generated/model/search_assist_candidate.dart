//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'search_assist_candidate.g.dart';

/// SearchAssistCandidate
///
/// Properties:
/// * [ref]
/// * [title] - Thread title; absent for the main thread of a text channel.
/// * [channelName]
/// * [spaceName]
/// * [at] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [excerpt] - Plain text of the best matching message, at most `search.assist.excerpt_max_chars`.
/// * [shownBefore] - The person saw this session in an earlier answer of the popup.
@BuiltValue()
abstract class SearchAssistCandidate
    implements Built<SearchAssistCandidate, SearchAssistCandidateBuilder> {
  @BuiltValueField(wireName: r'ref')
  String get ref;

  /// Thread title; absent for the main thread of a text channel.
  @BuiltValueField(wireName: r'title')
  String? get title;

  @BuiltValueField(wireName: r'channel_name')
  String get channelName;

  @BuiltValueField(wireName: r'space_name')
  String? get spaceName;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'at')
  DateTime get at;

  /// Plain text of the best matching message, at most `search.assist.excerpt_max_chars`.
  @BuiltValueField(wireName: r'excerpt')
  String get excerpt;

  /// The person saw this session in an earlier answer of the popup.
  @BuiltValueField(wireName: r'shown_before')
  bool get shownBefore;

  SearchAssistCandidate._();

  factory SearchAssistCandidate([
    void updates(SearchAssistCandidateBuilder b),
  ]) = _$SearchAssistCandidate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SearchAssistCandidateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<SearchAssistCandidate> get serializer =>
      _$SearchAssistCandidateSerializer();
}

class _$SearchAssistCandidateSerializer
    implements PrimitiveSerializer<SearchAssistCandidate> {
  @override
  final Iterable<Type> types = const [
    SearchAssistCandidate,
    _$SearchAssistCandidate,
  ];

  @override
  final String wireName = r'SearchAssistCandidate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    SearchAssistCandidate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'ref';
    yield serializers.serialize(
      object.ref,
      specifiedType: const FullType(String),
    );
    if (object.title != null) {
      yield r'title';
      yield serializers.serialize(
        object.title,
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
    yield r'excerpt';
    yield serializers.serialize(
      object.excerpt,
      specifiedType: const FullType(String),
    );
    yield r'shown_before';
    yield serializers.serialize(
      object.shownBefore,
      specifiedType: const FullType(bool),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    SearchAssistCandidate object, {
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
    required SearchAssistCandidateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'ref':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.ref = valueDes;
          break;
        case r'title':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.title = valueDes;
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
        case r'excerpt':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.excerpt = valueDes;
          break;
        case r'shown_before':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.shownBefore = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  SearchAssistCandidate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SearchAssistCandidateBuilder();
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
