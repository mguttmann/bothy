//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_interview_cost.g.dart';

/// Estimate of the builder's model call, for information only; not booked as a cost entry.
///
/// Properties:
/// * [amountMicros]
/// * [currency]
@BuiltValue()
abstract class AgentInterviewCost
    implements Built<AgentInterviewCost, AgentInterviewCostBuilder> {
  @BuiltValueField(wireName: r'amount_micros')
  int get amountMicros;

  @BuiltValueField(wireName: r'currency')
  String get currency;

  AgentInterviewCost._();

  factory AgentInterviewCost([void updates(AgentInterviewCostBuilder b)]) =
      _$AgentInterviewCost;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentInterviewCostBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentInterviewCost> get serializer =>
      _$AgentInterviewCostSerializer();
}

class _$AgentInterviewCostSerializer
    implements PrimitiveSerializer<AgentInterviewCost> {
  @override
  final Iterable<Type> types = const [AgentInterviewCost, _$AgentInterviewCost];

  @override
  final String wireName = r'AgentInterviewCost';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentInterviewCost object, {
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
    AgentInterviewCost object, {
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
    required AgentInterviewCostBuilder result,
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
  AgentInterviewCost deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentInterviewCostBuilder();
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
