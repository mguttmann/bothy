//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/billing_mode.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'cost_total.g.dart';

/// CostTotal
///
/// Properties:
/// * [billingMode]
/// * [amountMicros]
/// * [currency] - ISO 4217 code.
/// * [isEstimate] - True while any summed entry is an estimate.
@BuiltValue()
abstract class CostTotal implements Built<CostTotal, CostTotalBuilder> {
  @BuiltValueField(wireName: r'billing_mode')
  BillingMode get billingMode;
  // enum billingModeEnum {  personal,  organization,  };

  @BuiltValueField(wireName: r'amount_micros')
  int get amountMicros;

  /// ISO 4217 code.
  @BuiltValueField(wireName: r'currency')
  String get currency;

  /// True while any summed entry is an estimate.
  @BuiltValueField(wireName: r'is_estimate')
  bool get isEstimate;

  CostTotal._();

  factory CostTotal([void updates(CostTotalBuilder b)]) = _$CostTotal;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(CostTotalBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<CostTotal> get serializer => _$CostTotalSerializer();
}

class _$CostTotalSerializer implements PrimitiveSerializer<CostTotal> {
  @override
  final Iterable<Type> types = const [CostTotal, _$CostTotal];

  @override
  final String wireName = r'CostTotal';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    CostTotal object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'billing_mode';
    yield serializers.serialize(
      object.billingMode,
      specifiedType: const FullType(BillingMode),
    );
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
    yield r'is_estimate';
    yield serializers.serialize(
      object.isEstimate,
      specifiedType: const FullType(bool),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    CostTotal object, {
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
    required CostTotalBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'billing_mode':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BillingMode),
          ) as BillingMode;
          result.billingMode = valueDes;
          break;
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
        case r'is_estimate':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.isEstimate = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  CostTotal deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = CostTotalBuilder();
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
