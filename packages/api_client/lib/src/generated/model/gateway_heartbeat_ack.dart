//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'gateway_heartbeat_ack.g.dart';

/// GatewayHeartbeatAck
///
/// Properties:
/// * [serverTime] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class GatewayHeartbeatAck
    implements Built<GatewayHeartbeatAck, GatewayHeartbeatAckBuilder> {
  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'server_time')
  DateTime get serverTime;

  GatewayHeartbeatAck._();

  factory GatewayHeartbeatAck([void updates(GatewayHeartbeatAckBuilder b)]) =
      _$GatewayHeartbeatAck;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GatewayHeartbeatAckBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<GatewayHeartbeatAck> get serializer =>
      _$GatewayHeartbeatAckSerializer();
}

class _$GatewayHeartbeatAckSerializer
    implements PrimitiveSerializer<GatewayHeartbeatAck> {
  @override
  final Iterable<Type> types = const [
    GatewayHeartbeatAck,
    _$GatewayHeartbeatAck,
  ];

  @override
  final String wireName = r'GatewayHeartbeatAck';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    GatewayHeartbeatAck object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'server_time';
    yield serializers.serialize(
      object.serverTime,
      specifiedType: const FullType(DateTime),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    GatewayHeartbeatAck object, {
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
    required GatewayHeartbeatAckBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'server_time':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.serverTime = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  GatewayHeartbeatAck deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GatewayHeartbeatAckBuilder();
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
