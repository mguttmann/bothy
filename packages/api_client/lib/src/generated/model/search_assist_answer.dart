//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/cost_report.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/search_assist_pick.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'search_assist_answer.g.dart';

/// Answer of the assistant agent to `search.assist_requested` (action `search.assist.respond`).
///
/// Properties:
/// * [picks] - The sessions that answer the question, best first; at most `max_picks` of the request.
/// * [terms] - Further search terms (synonyms, the other language); at most `max_terms` of the request.
/// * [costs] - Usage of the model call, as in `run.finish`.
@BuiltValue()
abstract class SearchAssistAnswer
    implements Built<SearchAssistAnswer, SearchAssistAnswerBuilder> {
  /// The sessions that answer the question, best first; at most `max_picks` of the request.
  @BuiltValueField(wireName: r'picks')
  BuiltList<SearchAssistPick>? get picks;

  /// Further search terms (synonyms, the other language); at most `max_terms` of the request.
  @BuiltValueField(wireName: r'terms')
  BuiltList<String>? get terms;

  /// Usage of the model call, as in `run.finish`.
  @BuiltValueField(wireName: r'costs')
  BuiltList<CostReport>? get costs;

  SearchAssistAnswer._();

  factory SearchAssistAnswer([void updates(SearchAssistAnswerBuilder b)]) =
      _$SearchAssistAnswer;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SearchAssistAnswerBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<SearchAssistAnswer> get serializer =>
      _$SearchAssistAnswerSerializer();
}

class _$SearchAssistAnswerSerializer
    implements PrimitiveSerializer<SearchAssistAnswer> {
  @override
  final Iterable<Type> types = const [SearchAssistAnswer, _$SearchAssistAnswer];

  @override
  final String wireName = r'SearchAssistAnswer';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    SearchAssistAnswer object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.picks != null) {
      yield r'picks';
      yield serializers.serialize(
        object.picks,
        specifiedType: const FullType(BuiltList, [FullType(SearchAssistPick)]),
      );
    }
    if (object.terms != null) {
      yield r'terms';
      yield serializers.serialize(
        object.terms,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
    if (object.costs != null) {
      yield r'costs';
      yield serializers.serialize(
        object.costs,
        specifiedType: const FullType(BuiltList, [FullType(CostReport)]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    SearchAssistAnswer object, {
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
    required SearchAssistAnswerBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'picks':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(SearchAssistPick),
            ]),
          ) as BuiltList<SearchAssistPick>?;
          if (valueDes == null) continue;
          result.picks.replace(valueDes);
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
        case r'costs':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(CostReport),
            ]),
          ) as BuiltList<CostReport>?;
          if (valueDes == null) continue;
          result.costs.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  SearchAssistAnswer deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SearchAssistAnswerBuilder();
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
