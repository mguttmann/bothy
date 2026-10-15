//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'gateway_ack.g.dart';

/// GatewayAck
///
/// Properties:
/// * [seq]
@BuiltValue()
abstract class GatewayAck implements Built<GatewayAck, GatewayAckBuilder> {
  @BuiltValueField(wireName: r'seq')
  int get seq;

  GatewayAck._();

  factory GatewayAck([void updates(GatewayAckBuilder b)]) = _$GatewayAck;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GatewayAckBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<GatewayAck> get serializer => _$GatewayAckSerializer();
}

class _$GatewayAckSerializer implements PrimitiveSerializer<GatewayAck> {
  @override
  final Iterable<Type> types = const [GatewayAck, _$GatewayAck];

  @override
  final String wireName = r'GatewayAck';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    GatewayAck object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'seq';
    yield serializers.serialize(object.seq, specifiedType: const FullType(int));
  }

  @override
  Object serialize(
    Serializers serializers,
    GatewayAck object, {
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
    required GatewayAckBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'seq':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.seq = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  GatewayAck deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GatewayAckBuilder();
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
