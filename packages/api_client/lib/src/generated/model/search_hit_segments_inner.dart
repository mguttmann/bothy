//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'search_hit_segments_inner.g.dart';

/// SearchHitSegmentsInner
///
/// Properties:
/// * [text]
/// * [match]
@BuiltValue()
abstract class SearchHitSegmentsInner
    implements Built<SearchHitSegmentsInner, SearchHitSegmentsInnerBuilder> {
  @BuiltValueField(wireName: r'text')
  String get text;

  @BuiltValueField(wireName: r'match')
  bool get match;

  SearchHitSegmentsInner._();

  factory SearchHitSegmentsInner([
    void updates(SearchHitSegmentsInnerBuilder b),
  ]) = _$SearchHitSegmentsInner;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SearchHitSegmentsInnerBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<SearchHitSegmentsInner> get serializer =>
      _$SearchHitSegmentsInnerSerializer();
}

class _$SearchHitSegmentsInnerSerializer
    implements PrimitiveSerializer<SearchHitSegmentsInner> {
  @override
  final Iterable<Type> types = const [
    SearchHitSegmentsInner,
    _$SearchHitSegmentsInner,
  ];

  @override
  final String wireName = r'SearchHitSegmentsInner';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    SearchHitSegmentsInner object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'text';
    yield serializers.serialize(
      object.text,
      specifiedType: const FullType(String),
    );
    yield r'match';
    yield serializers.serialize(
      object.match,
      specifiedType: const FullType(bool),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    SearchHitSegmentsInner object, {
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
    required SearchHitSegmentsInnerBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'text':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.text = valueDes;
          break;
        case r'match':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.match = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  SearchHitSegmentsInner deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SearchHitSegmentsInnerBuilder();
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
