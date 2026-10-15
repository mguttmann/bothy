//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_heartbeat_ack.g.dart';

/// HostHeartbeatAck
///
/// Properties:
/// * [serverTime] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class HostHeartbeatAck
    implements Built<HostHeartbeatAck, HostHeartbeatAckBuilder> {
  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'server_time')
  DateTime? get serverTime;

  HostHeartbeatAck._();

  factory HostHeartbeatAck([void updates(HostHeartbeatAckBuilder b)]) =
      _$HostHeartbeatAck;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostHeartbeatAckBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostHeartbeatAck> get serializer =>
      _$HostHeartbeatAckSerializer();
}

class _$HostHeartbeatAckSerializer
    implements PrimitiveSerializer<HostHeartbeatAck> {
  @override
  final Iterable<Type> types = const [HostHeartbeatAck, _$HostHeartbeatAck];

  @override
  final String wireName = r'HostHeartbeatAck';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostHeartbeatAck object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.serverTime != null) {
      yield r'server_time';
      yield serializers.serialize(
        object.serverTime,
        specifiedType: const FullType(DateTime),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    HostHeartbeatAck object, {
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
    required HostHeartbeatAckBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'server_time':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
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
  HostHeartbeatAck deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostHeartbeatAckBuilder();
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
