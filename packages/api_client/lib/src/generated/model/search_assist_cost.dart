//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'search_assist_cost.g.dart';

/// Estimate of the assistant's model call, for information only; not booked as a cost entry.
///
/// Properties:
/// * [amountMicros]
/// * [currency]
@BuiltValue()
abstract class SearchAssistCost
    implements Built<SearchAssistCost, SearchAssistCostBuilder> {
  @BuiltValueField(wireName: r'amount_micros')
  int get amountMicros;

  @BuiltValueField(wireName: r'currency')
  String get currency;

  SearchAssistCost._();

  factory SearchAssistCost([void updates(SearchAssistCostBuilder b)]) =
      _$SearchAssistCost;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SearchAssistCostBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<SearchAssistCost> get serializer =>
      _$SearchAssistCostSerializer();
}

class _$SearchAssistCostSerializer
    implements PrimitiveSerializer<SearchAssistCost> {
  @override
  final Iterable<Type> types = const [SearchAssistCost, _$SearchAssistCost];

  @override
  final String wireName = r'SearchAssistCost';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    SearchAssistCost object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'amount_micros';
    yield serializers.serialize(
      object.amountMicros,
      specifiedType: const FullType(int),
    );
    yield r'currency';
    yield serializers.serialize(
      object.currency,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    SearchAssistCost object, {
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
    required SearchAssistCostBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'amount_micros':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.amountMicros = valueDes;
          break;
        case r'currency':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.currency = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  SearchAssistCost deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SearchAssistCostBuilder();
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
