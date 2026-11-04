//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/search_assist_turn.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'search_assist_query.g.dart';

/// One question to the search assistant (14, 4.10).
///
/// Properties:
/// * [q] - The question in the person's own words, at most `search.query_max_chars`.
/// * [history] - The earlier questions of the same popup, oldest first; at most `search.assist.max_turns`. Nothing of it is stored.
@BuiltValue()
abstract class SearchAssistQuery
    implements Built<SearchAssistQuery, SearchAssistQueryBuilder> {
  /// The question in the person's own words, at most `search.query_max_chars`.
  @BuiltValueField(wireName: r'q')
  String get q;

  /// The earlier questions of the same popup, oldest first; at most `search.assist.max_turns`. Nothing of it is stored.
  @BuiltValueField(wireName: r'history')
  BuiltList<SearchAssistTurn>? get history;

  SearchAssistQuery._();

  factory SearchAssistQuery([void updates(SearchAssistQueryBuilder b)]) =
      _$SearchAssistQuery;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SearchAssistQueryBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<SearchAssistQuery> get serializer =>
      _$SearchAssistQuerySerializer();
}

class _$SearchAssistQuerySerializer
    implements PrimitiveSerializer<SearchAssistQuery> {
  @override
  final Iterable<Type> types = const [SearchAssistQuery, _$SearchAssistQuery];

  @override
  final String wireName = r'SearchAssistQuery';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    SearchAssistQuery object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'q';
    yield serializers.serialize(
      object.q,
      specifiedType: const FullType(String),
    );
    if (object.history != null) {
      yield r'history';
      yield serializers.serialize(
        object.history,
        specifiedType: const FullType(BuiltList, [FullType(SearchAssistTurn)]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    SearchAssistQuery object, {
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
    required SearchAssistQueryBuilder result,
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
        case r'history':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(SearchAssistTurn),
            ]),
          ) as BuiltList<SearchAssistTurn>?;
          if (valueDes == null) continue;
          result.history.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  SearchAssistQuery deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SearchAssistQueryBuilder();
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
