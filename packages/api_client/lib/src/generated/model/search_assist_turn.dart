//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'search_assist_turn.g.dart';

/// SearchAssistTurn
///
/// Properties:
/// * [q] - An earlier question.
/// * [terms] - The `terms` of the answer to it.
/// * [threadIds] - The threads the answer to it showed; the assistant learns which candidates the person has seen already.
@BuiltValue()
abstract class SearchAssistTurn
    implements Built<SearchAssistTurn, SearchAssistTurnBuilder> {
  /// An earlier question.
  @BuiltValueField(wireName: r'q')
  String get q;

  /// The `terms` of the answer to it.
  @BuiltValueField(wireName: r'terms')
  BuiltList<String>? get terms;

  /// The threads the answer to it showed; the assistant learns which candidates the person has seen already.
  @BuiltValueField(wireName: r'thread_ids')
  BuiltList<String>? get threadIds;

  SearchAssistTurn._();

  factory SearchAssistTurn([void updates(SearchAssistTurnBuilder b)]) =
      _$SearchAssistTurn;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SearchAssistTurnBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<SearchAssistTurn> get serializer =>
      _$SearchAssistTurnSerializer();
}

class _$SearchAssistTurnSerializer
    implements PrimitiveSerializer<SearchAssistTurn> {
  @override
  final Iterable<Type> types = const [SearchAssistTurn, _$SearchAssistTurn];

  @override
  final String wireName = r'SearchAssistTurn';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    SearchAssistTurn object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'q';
    yield serializers.serialize(
      object.q,
      specifiedType: const FullType(String),
    );
    if (object.terms != null) {
      yield r'terms';
      yield serializers.serialize(
        object.terms,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
    if (object.threadIds != null) {
      yield r'thread_ids';
      yield serializers.serialize(
        object.threadIds,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    SearchAssistTurn object, {
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
    required SearchAssistTurnBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'q':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.q = valueDes;
          break;
        case r'terms':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.terms.replace(valueDes);
          break;
        case r'thread_ids':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.threadIds.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  SearchAssistTurn deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SearchAssistTurnBuilder();
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
