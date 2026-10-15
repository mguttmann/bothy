//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'stats_backend_denials.g.dart';

/// StatsBackendDenials
///
/// Properties:
/// * [i2Denied]
/// * [billingModeMismatch]
@BuiltValue()
abstract class StatsBackendDenials
    implements Built<StatsBackendDenials, StatsBackendDenialsBuilder> {
  @BuiltValueField(wireName: r'i2_denied')
  int get i2Denied;

  @BuiltValueField(wireName: r'billing_mode_mismatch')
  int get billingModeMismatch;

  StatsBackendDenials._();

  factory StatsBackendDenials([void updates(StatsBackendDenialsBuilder b)]) =
      _$StatsBackendDenials;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StatsBackendDenialsBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StatsBackendDenials> get serializer =>
      _$StatsBackendDenialsSerializer();
}

class _$StatsBackendDenialsSerializer
    implements PrimitiveSerializer<StatsBackendDenials> {
  @override
  final Iterable<Type> types = const [
    StatsBackendDenials,
    _$StatsBackendDenials,
  ];

  @override
  final String wireName = r'StatsBackendDenials';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StatsBackendDenials object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'i2_denied';
    yield serializers.serialize(
      object.i2Denied,
      specifiedType: const FullType(int),
    );
    yield r'billing_mode_mismatch';
    yield serializers.serialize(
      object.billingModeMismatch,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    StatsBackendDenials object, {
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
    required StatsBackendDenialsBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'i2_denied':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.i2Denied = valueDes;
          break;
        case r'billing_mode_mismatch':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.billingModeMismatch = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  StatsBackendDenials deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StatsBackendDenialsBuilder();
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
