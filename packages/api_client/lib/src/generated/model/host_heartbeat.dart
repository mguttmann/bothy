//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_heartbeat.g.dart';

/// HostHeartbeat
///
/// Properties:
/// * [activeSessions]
@BuiltValue()
abstract class HostHeartbeat
    implements Built<HostHeartbeat, HostHeartbeatBuilder> {
  @BuiltValueField(wireName: r'active_sessions')
  int get activeSessions;

  HostHeartbeat._();

  factory HostHeartbeat([void updates(HostHeartbeatBuilder b)]) =
      _$HostHeartbeat;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostHeartbeatBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostHeartbeat> get serializer =>
      _$HostHeartbeatSerializer();
}

class _$HostHeartbeatSerializer implements PrimitiveSerializer<HostHeartbeat> {
  @override
  final Iterable<Type> types = const [HostHeartbeat, _$HostHeartbeat];

  @override
  final String wireName = r'HostHeartbeat';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostHeartbeat object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'active_sessions';
    yield serializers.serialize(
      object.activeSessions,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    HostHeartbeat object, {
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
    required HostHeartbeatBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'active_sessions':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.activeSessions = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  HostHeartbeat deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostHeartbeatBuilder();
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
