//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'cost_estimate.g.dart';

/// Estimate of one run in the requested range for the reconciliation (10, 12.4; 05, R-AZ-28). Names no channel, thread or content.
///
/// Properties:
/// * [runId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [agentId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [provider]
/// * [currency] - ISO 4217 code.
/// * [estimateMicros] - Sum in micro units of the run's estimate entries in the range whose successor, if any, is not an estimate; the estimate before any reconciliation.
/// * [reconciled] - True when every valid entry of the run in the range has `source = reconciled`.
@BuiltValue()
abstract class CostEstimate
    implements Built<CostEstimate, CostEstimateBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'run_id')
  String get runId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'agent_id')
  String get agentId;

  @BuiltValueField(wireName: r'provider')
  String get provider;

  /// ISO 4217 code.
  @BuiltValueField(wireName: r'currency')
  String get currency;

  /// Sum in micro units of the run's estimate entries in the range whose successor, if any, is not an estimate; the estimate before any reconciliation.
  @BuiltValueField(wireName: r'estimate_micros')
  int get estimateMicros;

  /// True when every valid entry of the run in the range has `source = reconciled`.
  @BuiltValueField(wireName: r'reconciled')
  bool get reconciled;

  CostEstimate._();

  factory CostEstimate([void updates(CostEstimateBuilder b)]) = _$CostEstimate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(CostEstimateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<CostEstimate> get serializer => _$CostEstimateSerializer();
}

class _$CostEstimateSerializer implements PrimitiveSerializer<CostEstimate> {
  @override
  final Iterable<Type> types = const [CostEstimate, _$CostEstimate];

  @override
  final String wireName = r'CostEstimate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    CostEstimate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'run_id';
    yield serializers.serialize(
      object.runId,
      specifiedType: const FullType(String),
    );
    yield r'agent_id';
    yield serializers.serialize(
      object.agentId,
      specifiedType: const FullType(String),
    );
    yield r'provider';
    yield serializers.serialize(
      object.provider,
      specifiedType: const FullType(String),
    );
    yield r'currency';
    yield serializers.serialize(
      object.currency,
      specifiedType: const FullType(String),
    );
    yield r'estimate_micros';
    yield serializers.serialize(
      object.estimateMicros,
      specifiedType: const FullType(int),
    );
    yield r'reconciled';
    yield serializers.serialize(
      object.reconciled,
      specifiedType: const FullType(bool),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    CostEstimate object, {
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
    required CostEstimateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'run_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.runId = valueDes;
          break;
        case r'agent_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.agentId = valueDes;
          break;
        case r'provider':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.provider = valueDes;
          break;
        case r'currency':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.currency = valueDes;
          break;
        case r'estimate_micros':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.estimateMicros = valueDes;
          break;
        case r'reconciled':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.reconciled = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  CostEstimate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = CostEstimateBuilder();
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
