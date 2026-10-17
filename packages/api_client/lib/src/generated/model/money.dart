//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'money.g.dart';

/// Amount in micro units of the currency. Never a float.
///
/// Properties:
/// * [amountMicros]
/// * [currency] - ISO 4217 code.
/// * [isEstimate] - True until the amount was reconciled with the provider invoice.
@BuiltValue()
abstract class Money implements Built<Money, MoneyBuilder> {
  @BuiltValueField(wireName: r'amount_micros')
  int get amountMicros;

  /// ISO 4217 code.
  @BuiltValueField(wireName: r'currency')
  String get currency;

  /// True until the amount was reconciled with the provider invoice.
  @BuiltValueField(wireName: r'is_estimate')
  bool get isEstimate;

  Money._();

  factory Money([void updates(MoneyBuilder b)]) = _$Money;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(MoneyBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<Money> get serializer => _$MoneySerializer();
}

class _$MoneySerializer implements PrimitiveSerializer<Money> {
  @override
  final Iterable<Type> types = const [Money, _$Money];

  @override
  final String wireName = r'Money';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    Money object, {
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
    yield r'is_estimate';
    yield serializers.serialize(
      object.isEstimate,
      specifiedType: const FullType(bool),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    Money object, {
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
    required MoneyBuilder result,
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
  Money deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = MoneyBuilder();
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
