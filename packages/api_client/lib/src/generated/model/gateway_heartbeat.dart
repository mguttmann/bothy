//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/heartbeat.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'gateway_heartbeat.g.dart';

/// GatewayHeartbeat
///
/// Properties:
/// * [lastSeq]
/// * [agent] - Agents only; equivalent to `POST /agents/me/heartbeat`.
@BuiltValue()
abstract class GatewayHeartbeat
    implements Built<GatewayHeartbeat, GatewayHeartbeatBuilder> {
  @BuiltValueField(wireName: r'last_seq')
  int get lastSeq;

  /// Agents only; equivalent to `POST /agents/me/heartbeat`.
  @BuiltValueField(wireName: r'agent')
  Heartbeat? get agent;

  GatewayHeartbeat._();

  factory GatewayHeartbeat([void updates(GatewayHeartbeatBuilder b)]) =
      _$GatewayHeartbeat;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GatewayHeartbeatBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<GatewayHeartbeat> get serializer =>
      _$GatewayHeartbeatSerializer();
}

class _$GatewayHeartbeatSerializer
    implements PrimitiveSerializer<GatewayHeartbeat> {
  @override
  final Iterable<Type> types = const [GatewayHeartbeat, _$GatewayHeartbeat];

  @override
  final String wireName = r'GatewayHeartbeat';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    GatewayHeartbeat object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'last_seq';
    yield serializers.serialize(
      object.lastSeq,
      specifiedType: const FullType(int),
    );
    if (object.agent != null) {
      yield r'agent';
      yield serializers.serialize(
        object.agent,
        specifiedType: const FullType(Heartbeat),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    GatewayHeartbeat object, {
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
    required GatewayHeartbeatBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'last_seq':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.lastSeq = valueDes;
          break;
        case r'agent':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(Heartbeat),
          ) as Heartbeat?;
          if (valueDes == null) continue;
          result.agent.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  GatewayHeartbeat deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GatewayHeartbeatBuilder();
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
