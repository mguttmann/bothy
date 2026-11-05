//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'search_assist_pick.g.dart';

/// SearchAssistPick
///
/// Properties:
/// * [ref] - The `ref` of a candidate of the request.
/// * [reason] - One sentence why, at most `search.assist.reason_max_chars`.
@BuiltValue()
abstract class SearchAssistPick
    implements Built<SearchAssistPick, SearchAssistPickBuilder> {
  /// The `ref` of a candidate of the request.
  @BuiltValueField(wireName: r'ref')
  String get ref;

  /// One sentence why, at most `search.assist.reason_max_chars`.
  @BuiltValueField(wireName: r'reason')
  String get reason;

  SearchAssistPick._();

  factory SearchAssistPick([void updates(SearchAssistPickBuilder b)]) =
      _$SearchAssistPick;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SearchAssistPickBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<SearchAssistPick> get serializer =>
      _$SearchAssistPickSerializer();
}

class _$SearchAssistPickSerializer
    implements PrimitiveSerializer<SearchAssistPick> {
  @override
  final Iterable<Type> types = const [SearchAssistPick, _$SearchAssistPick];

  @override
  final String wireName = r'SearchAssistPick';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    SearchAssistPick object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'ref';
    yield serializers.serialize(
      object.ref,
      specifiedType: const FullType(String),
    );
    yield r'reason';
    yield serializers.serialize(
      object.reason,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    SearchAssistPick object, {
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
    required SearchAssistPickBuilder result,
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
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.reason = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  SearchAssistPick deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SearchAssistPickBuilder();
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
