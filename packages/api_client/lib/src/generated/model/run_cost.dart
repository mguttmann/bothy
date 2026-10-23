//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/money.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/billing_mode.dart';
import 'package:api_client/src/generated/model/token_counts.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_cost.g.dart';

/// RunCost
///
/// Properties:
/// * [runId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [billingMode]
/// * [agentSessionId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [tokens]
/// * [costs] - One amount per currency; absent when amounts are hidden (mode `personal` without `billing.show_personal_estimates`).
/// * [unpriced] - A valid entry of the run has neither a price nor an amount of the adapter (\"Price missing\").
@BuiltValue()
abstract class RunCost implements Built<RunCost, RunCostBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'run_id')
  String get runId;

  @BuiltValueField(wireName: r'billing_mode')
  BillingMode get billingMode;
  // enum billingModeEnum {  personal,  organization,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'agent_session_id')
  String? get agentSessionId;

  @BuiltValueField(wireName: r'tokens')
  TokenCounts get tokens;

  /// One amount per currency; absent when amounts are hidden (mode `personal` without `billing.show_personal_estimates`).
  @BuiltValueField(wireName: r'costs')
  BuiltList<Money>? get costs;

  /// A valid entry of the run has neither a price nor an amount of the adapter (\"Price missing\").
  @BuiltValueField(wireName: r'unpriced')
  bool get unpriced;

  RunCost._();

  factory RunCost([void updates(RunCostBuilder b)]) = _$RunCost;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunCostBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunCost> get serializer => _$RunCostSerializer();
}

class _$RunCostSerializer implements PrimitiveSerializer<RunCost> {
  @override
  final Iterable<Type> types = const [RunCost, _$RunCost];

  @override
  final String wireName = r'RunCost';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunCost object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'run_id';
    yield serializers.serialize(
      object.runId,
      specifiedType: const FullType(String),
    );
    yield r'billing_mode';
    yield serializers.serialize(
      object.billingMode,
      specifiedType: const FullType(BillingMode),
    );
    if (object.agentSessionId != null) {
      yield r'agent_session_id';
      yield serializers.serialize(
        object.agentSessionId,
        specifiedType: const FullType(String),
      );
    }
    yield r'tokens';
    yield serializers.serialize(
      object.tokens,
      specifiedType: const FullType(TokenCounts),
    );
    if (object.costs != null) {
      yield r'costs';
      yield serializers.serialize(
        object.costs,
        specifiedType: const FullType(BuiltList, [FullType(Money)]),
      );
    }
    yield r'unpriced';
    yield serializers.serialize(
      object.unpriced,
      specifiedType: const FullType(bool),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    RunCost object, {
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
    required RunCostBuilder result,
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
        case r'billing_mode':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BillingMode),
          ) as BillingMode;
          result.billingMode = valueDes;
          break;
        case r'agent_session_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.agentSessionId = valueDes;
          break;
        case r'tokens':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(TokenCounts),
          ) as TokenCounts;
          result.tokens.replace(valueDes);
          break;
        case r'costs':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(Money),
            ]),
          ) as BuiltList<Money>?;
          if (valueDes == null) continue;
          result.costs.replace(valueDes);
          break;
        case r'unpriced':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.unpriced = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  RunCost deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunCostBuilder();
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
