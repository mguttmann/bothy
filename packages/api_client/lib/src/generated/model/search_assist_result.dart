//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/search_assist_hit.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/search_assist_cost.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'search_assist_result.g.dart';

/// SearchAssistResult
///
/// Properties:
/// * [hits] - At most `search.assist.max_hits`; the assistant's picks first, then full-text hits.
/// * [mode] - `assisted` when the assistant answered, otherwise `fulltext`.
/// * [fallbackReason] - Only with `mode = fulltext`.
/// * [terms] - The search terms of this question (the person's and the assistant's); the app sends them back in `history`.
/// * [costEstimate]
@BuiltValue()
abstract class SearchAssistResult
    implements Built<SearchAssistResult, SearchAssistResultBuilder> {
  /// At most `search.assist.max_hits`; the assistant's picks first, then full-text hits.
  @BuiltValueField(wireName: r'hits')
  BuiltList<SearchAssistHit> get hits;

  /// `assisted` when the assistant answered, otherwise `fulltext`.
  @BuiltValueField(wireName: r'mode')
  SearchAssistResultModeEnum get mode;
  // enum modeEnum {  assisted,  fulltext,  };

  /// Only with `mode = fulltext`.
  @BuiltValueField(wireName: r'fallback_reason')
  SearchAssistResultFallbackReasonEnum? get fallbackReason;
  // enum fallbackReasonEnum {  assistant_not_configured,  assistant_unavailable,  assistant_timeout,  };

  /// The search terms of this question (the person's and the assistant's); the app sends them back in `history`.
  @BuiltValueField(wireName: r'terms')
  BuiltList<String> get terms;

  @BuiltValueField(wireName: r'cost_estimate')
  SearchAssistCost? get costEstimate;

  SearchAssistResult._();

  factory SearchAssistResult([void updates(SearchAssistResultBuilder b)]) =
      _$SearchAssistResult;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SearchAssistResultBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<SearchAssistResult> get serializer =>
      _$SearchAssistResultSerializer();
}

class _$SearchAssistResultSerializer
    implements PrimitiveSerializer<SearchAssistResult> {
  @override
  final Iterable<Type> types = const [SearchAssistResult, _$SearchAssistResult];

  @override
  final String wireName = r'SearchAssistResult';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    SearchAssistResult object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'hits';
    yield serializers.serialize(
      object.hits,
      specifiedType: const FullType(BuiltList, [FullType(SearchAssistHit)]),
    );
    yield r'mode';
    yield serializers.serialize(
      object.mode,
      specifiedType: const FullType(SearchAssistResultModeEnum),
    );
    if (object.fallbackReason != null) {
      yield r'fallback_reason';
      yield serializers.serialize(
        object.fallbackReason,
        specifiedType: const FullType(SearchAssistResultFallbackReasonEnum),
      );
    }
    yield r'terms';
    yield serializers.serialize(
      object.terms,
      specifiedType: const FullType(BuiltList, [FullType(String)]),
    );
    if (object.costEstimate != null) {
      yield r'cost_estimate';
      yield serializers.serialize(
        object.costEstimate,
        specifiedType: const FullType(SearchAssistCost),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    SearchAssistResult object, {
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
    required SearchAssistResultBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'hits':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(SearchAssistHit),
            ]),
          ) as BuiltList<SearchAssistHit>;
          result.hits.replace(valueDes);
          break;
        case r'mode':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(SearchAssistResultModeEnum),
          ) as SearchAssistResultModeEnum;
          result.mode = valueDes;
          break;
        case r'fallback_reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(
              SearchAssistResultFallbackReasonEnum,
            ),
          ) as SearchAssistResultFallbackReasonEnum?;
          if (valueDes == null) continue;
          result.fallbackReason = valueDes;
          break;
        case r'terms':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(String)]),
          ) as BuiltList<String>;
          result.terms.replace(valueDes);
          break;
        case r'cost_estimate':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(SearchAssistCost),
          ) as SearchAssistCost?;
          if (valueDes == null) continue;
          result.costEstimate.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  SearchAssistResult deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SearchAssistResultBuilder();
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

/// `assisted` when the assistant answered, otherwise `fulltext`.
class SearchAssistResultModeEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'assisted')
  static const SearchAssistResultModeEnum assisted =
      _$searchAssistResultModeEnum_assisted;
  @BuiltValueEnumConst(wireName: r'fulltext')
  static const SearchAssistResultModeEnum fulltext =
      _$searchAssistResultModeEnum_fulltext;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const SearchAssistResultModeEnum unknownDefaultOpenApi =
      _$searchAssistResultModeEnum_unknownDefaultOpenApi;

  static Serializer<SearchAssistResultModeEnum> get serializer =>
      _$searchAssistResultModeEnumSerializer;

  const SearchAssistResultModeEnum._(String name) : super(name);

  static BuiltSet<SearchAssistResultModeEnum> get values =>
      _$searchAssistResultModeEnumValues;
  static SearchAssistResultModeEnum valueOf(String name) =>
      _$searchAssistResultModeEnumValueOf(name);
}

/// Only with `mode = fulltext`.
class SearchAssistResultFallbackReasonEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'assistant_not_configured')
  static const SearchAssistResultFallbackReasonEnum assistantNotConfigured =
      _$searchAssistResultFallbackReasonEnum_assistantNotConfigured;
  @BuiltValueEnumConst(wireName: r'assistant_unavailable')
  static const SearchAssistResultFallbackReasonEnum assistantUnavailable =
      _$searchAssistResultFallbackReasonEnum_assistantUnavailable;
  @BuiltValueEnumConst(wireName: r'assistant_timeout')
  static const SearchAssistResultFallbackReasonEnum assistantTimeout =
      _$searchAssistResultFallbackReasonEnum_assistantTimeout;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const SearchAssistResultFallbackReasonEnum unknownDefaultOpenApi =
      _$searchAssistResultFallbackReasonEnum_unknownDefaultOpenApi;

  static Serializer<SearchAssistResultFallbackReasonEnum> get serializer =>
      _$searchAssistResultFallbackReasonEnumSerializer;

  const SearchAssistResultFallbackReasonEnum._(String name) : super(name);

  static BuiltSet<SearchAssistResultFallbackReasonEnum> get values =>
      _$searchAssistResultFallbackReasonEnumValues;
  static SearchAssistResultFallbackReasonEnum valueOf(String name) =>
      _$searchAssistResultFallbackReasonEnumValueOf(name);
}
